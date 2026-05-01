package service

import (
	"context"
	"errors"
	"testing"
	"time"

	domain "subtitle-delivery/internal/domain"
)

func TestCreateSubtitleStoresNormalizedSubtitle(t *testing.T) {
	t.Parallel()

	store := &stubStore{}
	fetcher := stubFetcher{body: []byte("1\n00:00:00,000 --> 00:00:01,000\nhello\n")}
	subtitleService := NewSubtitleService(300*1024, 10*time.Minute, store, fetcher)

	result, err := subtitleService.CreateSubtitle(context.Background(), "https://example.com/subtitle.srt")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if result.ID == "" {
		t.Fatal("expected generated id")
	}
	if result.URL != "https://example.com/subtitle.srt" {
		t.Fatalf("expected original source url, got %q", result.URL)
	}
	if store.saved.ID == "" {
		t.Fatal("expected store save to be called")
	}
	if store.saved.SourceURL != "https://example.com/subtitle.srt" {
		t.Fatalf("expected source URL to be stored, got %q", store.saved.SourceURL)
	}
	if store.saved.Content != "" {
		t.Fatalf("expected content not to be persisted in this flow, got %q", store.saved.Content)
	}
}

func TestCreateSubtitleReturnsDomainValidationError(t *testing.T) {
	t.Parallel()

	subtitleService := NewSubtitleService(300*1024, 10*time.Minute, &stubStore{}, stubFetcher{})
	_, err := subtitleService.CreateSubtitle(context.Background(), "https://example.com/subtitle.txt")
	if !errors.Is(err, domain.ErrUnsupportedFormat) {
		t.Fatalf("expected ErrUnsupportedFormat, got %v", err)
	}
}

func TestCreateSubtitleReturnsTooLargeError(t *testing.T) {
	t.Parallel()

	subtitleService := NewSubtitleService(
		5,
		10*time.Minute,
		&stubStore{},
		stubFetcher{body: []byte("123456")},
	)

	_, err := subtitleService.CreateSubtitle(context.Background(), "https://example.com/subtitle.vtt")
	if !errors.Is(err, ErrSubtitleTooLarge) {
		t.Fatalf("expected ErrSubtitleTooLarge, got %v", err)
	}
}

func TestLatestSubtitleDelegatesToStore(t *testing.T) {
	t.Parallel()

	store := &stubStore{latest: domain.Subtitle{ID: "subtitle-1"}}
	subtitleService := NewSubtitleService(300*1024, 10*time.Minute, store, stubFetcher{})

	record, err := subtitleService.LatestSubtitle(context.Background())
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if record.ID != "subtitle-1" {
		t.Fatalf("expected latest subtitle id, got %q", record.ID)
	}
}

type stubStore struct {
	saved     domain.Subtitle
	latest    domain.Subtitle
	saveError error
	getError  error
}

func (store *stubStore) Save(_ context.Context, subtitle domain.Subtitle) error {
	store.saved = subtitle
	return store.saveError
}

func (store *stubStore) Latest(_ context.Context) (domain.Subtitle, error) {
	if store.getError != nil {
		return domain.Subtitle{}, store.getError
	}
	return store.latest, nil
}

type stubFetcher struct {
	body []byte
	err  error
}

func (fetcher stubFetcher) Fetch(_ context.Context, _ string, _ int64) ([]byte, error) {
	if fetcher.err != nil {
		return nil, fetcher.err
	}
	return fetcher.body, nil
}
