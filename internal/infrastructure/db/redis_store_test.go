package db

import (
	"context"
	"testing"
	"time"

	domain "subtitle-delivery/internal/domain"

	miniredis "github.com/alicebob/miniredis/v2"
)

func TestRedisOptionsFromConfigUsesTCPForPlainAddress(t *testing.T) {
	t.Parallel()

	options, err := redisOptionsFromConfig(RedisConfig{
		Addr:     "localhost:6379",
		Password: "secret",
		DB:       3,
	})
	if err != nil {
		t.Fatalf("expected plain redis config to succeed, got error: %v", err)
	}
	if options.Network != "tcp" {
		t.Fatalf("expected network tcp, got %q", options.Network)
	}
	if options.Addr != "localhost:6379" {
		t.Fatalf("expected addr localhost:6379, got %q", options.Addr)
	}
	if options.Password != "secret" {
		t.Fatalf("expected password secret, got %q", options.Password)
	}
	if options.DB != 3 {
		t.Fatalf("expected db 3, got %d", options.DB)
	}
}

func TestRedisOptionsFromConfigParsesUpstashURL(t *testing.T) {
	t.Parallel()

	options, err := redisOptionsFromConfig(RedisConfig{
		UpstashURL: "rediss://default:secret@global-hero-12345.upstash.io:6379",
	})
	if err != nil {
		t.Fatalf("expected upstash url parsing to succeed, got error: %v", err)
	}
	if options.Network != "tcp" {
		t.Fatalf("expected network tcp, got %q", options.Network)
	}
	if options.Addr != "global-hero-12345.upstash.io:6379" {
		t.Fatalf("expected upstash addr, got %q", options.Addr)
	}
	if options.Password != "secret" {
		t.Fatalf("expected parsed password, got %q", options.Password)
	}
	if options.TLSConfig == nil {
		t.Fatal("expected rediss upstash url to enable tls")
	}
}

func TestRedisOptionsFromConfigReturnsErrorForInvalidUpstashURL(t *testing.T) {
	t.Parallel()

	if _, err := redisOptionsFromConfig(RedisConfig{UpstashURL: "rediss://%zz"}); err == nil {
		t.Fatal("expected invalid upstash url to fail")
	}
}

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
