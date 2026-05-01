package db

import (
	"context"
	"testing"
	"time"

	domain "subtitle-delivery/internal/domain"

	miniredis "github.com/alicebob/miniredis/v2"
)

func TestRedisStoreSaveAndLatest(t *testing.T) {
	t.Parallel()

	server, err := miniredis.Run()
	if err != nil {
		t.Fatalf("expected miniredis to start, got error: %v", err)
	}
	defer server.Close()

	store, err := NewRedisStore(RedisConfig{
		Addr:      server.Addr(),
		KeyPrefix: "subtitle-delivery-test",
		TTL:       time.Minute,
	})
	if err != nil {
		t.Fatalf("expected redis store to initialize, got error: %v", err)
	}

	record := domain.Subtitle{
		ID:        "subtitle-1",
		SourceURL: "https://example.com/subtitle.srt",
		AccessURL: "http://localhost:8080/subtitle.vtt",
		Content:   "WEBVTT\n\n00:00:00.000 --> 00:00:01.000\nhello\n",
		CreatedAt: time.Now().UTC(),
	}

	if err := store.Save(context.Background(), record); err != nil {
		t.Fatalf("expected save to succeed, got error: %v", err)
	}

	latest, err := store.Latest(context.Background())
	if err != nil {
		t.Fatalf("expected latest to succeed, got error: %v", err)
	}
	if latest.ID != record.ID {
		t.Fatalf("expected latest id %q, got %q", record.ID, latest.ID)
	}
	if latest.Content != record.Content {
		t.Fatalf("expected latest content %q, got %q", record.Content, latest.Content)
	}
}

func TestRedisStoreLatestReturnsErrorWhenEmpty(t *testing.T) {
	t.Parallel()

	server, err := miniredis.Run()
	if err != nil {
		t.Fatalf("expected miniredis to start, got error: %v", err)
	}
	defer server.Close()

	store, err := NewRedisStore(RedisConfig{
		Addr:      server.Addr(),
		KeyPrefix: "subtitle-delivery-test",
		TTL:       time.Minute,
	})
	if err != nil {
		t.Fatalf("expected redis store to initialize, got error: %v", err)
	}

	if _, err := store.Latest(context.Background()); err == nil {
		t.Fatal("expected latest to fail when redis store is empty")
	}
}

func TestRedisStoreLatestReturnsErrorAfterTTLExpiration(t *testing.T) {
	t.Parallel()

	server, err := miniredis.Run()
	if err != nil {
		t.Fatalf("expected miniredis to start, got error: %v", err)
	}
	defer server.Close()

	store, err := NewRedisStore(RedisConfig{
		Addr:      server.Addr(),
		KeyPrefix: "subtitle-delivery-test",
		TTL:       time.Minute,
	})
	if err != nil {
		t.Fatalf("expected redis store to initialize, got error: %v", err)
	}

	record := domain.Subtitle{
		ID:        "subtitle-ttl",
		SourceURL: "https://example.com/subtitle.srt",
		AccessURL: "http://localhost:8080/subtitle.vtt",
		Content:   "WEBVTT\n\n00:00:00.000 --> 00:00:01.000\nhello\n",
		CreatedAt: time.Now().UTC(),
	}

	if err := store.Save(context.Background(), record); err != nil {
		t.Fatalf("expected save to succeed, got error: %v", err)
	}

	server.FastForward(2 * time.Minute)

	if _, err := store.Latest(context.Background()); err == nil {
		t.Fatal("expected latest to fail after ttl expiration")
	}
}

func TestRedisStoreLatestReturnsErrorWhenBackendBecomesUnavailable(t *testing.T) {
	t.Parallel()

	server, err := miniredis.Run()
	if err != nil {
		t.Fatalf("expected miniredis to start, got error: %v", err)
	}

	store, err := NewRedisStore(RedisConfig{
		Addr:      server.Addr(),
		KeyPrefix: "subtitle-delivery-test",
		TTL:       time.Minute,
	})
	if err != nil {
		server.Close()
		t.Fatalf("expected redis store to initialize, got error: %v", err)
	}

	server.Close()

	if _, err := store.Latest(context.Background()); err == nil {
		t.Fatal("expected latest to fail when backend is unavailable")
	}
}
