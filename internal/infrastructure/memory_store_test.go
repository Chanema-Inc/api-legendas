package infrastructure

import (
	"context"
	"testing"
	"time"

	domain "subtitle-delivery/internal/domain"
)

func TestMemoryStoreCanInvalidateSpecificSubtitle(t *testing.T) {
	t.Parallel()

	store := NewMemoryStore(time.Minute)
	record := domain.Subtitle{
		ID:        "subtitle-1",
		AccessURL: "http://localhost:8080/subtitle.vtt",
		Content:   "WEBVTT\n\n00:00:00.000 --> 00:00:01.000\nhello\n",
		CreatedAt: time.Now().UTC(),
	}

	if err := store.Save(context.Background(), record); err != nil {
		t.Fatalf("expected save to succeed, got error: %v", err)
	}

	store.Invalidate("subtitle-1")

	if _, err := store.Latest(context.Background()); err == nil {
		t.Fatal("expected latest subtitle lookup to fail after invalidation")
	}
}

func TestMemoryStoreCleanupRemovesExpiredEntries(t *testing.T) {
	t.Parallel()

	store := NewMemoryStore(time.Millisecond)
	record := domain.Subtitle{
		ID:        "subtitle-1",
		AccessURL: "http://localhost:8080/subtitle.vtt",
		Content:   "WEBVTT\n\n00:00:00.000 --> 00:00:01.000\nhello\n",
		CreatedAt: time.Now().UTC(),
	}

	if err := store.Save(context.Background(), record); err != nil {
		t.Fatalf("expected save to succeed, got error: %v", err)
	}

	time.Sleep(5 * time.Millisecond)
	store.CleanupExpired()

	if store.Size() != 0 {
		t.Fatal("expected expired subtitle entry to be cleaned up")
	}
}
