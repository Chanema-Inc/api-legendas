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

func TestMemoryStoreSaveReplacesExistingSubtitle(t *testing.T) {
	t.Parallel()

	store := NewMemoryStore(time.Minute)

	first := domain.Subtitle{
		ID:        "subtitle-1",
		Content:   "WEBVTT\n\nfirst\n",
		CreatedAt: time.Now().UTC(),
	}
	second := domain.Subtitle{
		ID:        "subtitle-2",
		Content:   "WEBVTT\n\nsecond\n",
		CreatedAt: time.Now().UTC(),
	}

	if err := store.Save(context.Background(), first); err != nil {
		t.Fatalf("expected first save to succeed, got error: %v", err)
	}
	if err := store.Save(context.Background(), second); err != nil {
		t.Fatalf("expected second save to succeed, got error: %v", err)
	}

	if store.Size() != 1 {
		t.Fatalf("expected only 1 entry after second save, got %d", store.Size())
	}

	latest, err := store.Latest(context.Background())
	if err != nil {
		t.Fatalf("expected latest to succeed, got error: %v", err)
	}
	if latest.ID != "subtitle-2" {
		t.Fatalf("expected latest to be subtitle-2, got %q", latest.ID)
	}
}
