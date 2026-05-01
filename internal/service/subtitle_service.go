package service

import (
	"context"
	"errors"
	"time"

	domain "subtitle-delivery/internal/domain"
)

var ErrSubtitleTooLarge = errors.New("subtitle exceeds configured size limit")

type Store interface {
	Save(context.Context, domain.Subtitle) error
	Latest(context.Context) (domain.Subtitle, error)
}

type Fetcher interface {
	Fetch(context.Context, string, int64) ([]byte, error)
}

type SubtitleService struct {
	maxFileSize int64
	defaultTTL  time.Duration
	store       Store
	fetcher     Fetcher
}

type CreateResult struct {
	ID  string
	URL string
}

func NewSubtitleService(maxFileSize int64, defaultTTL time.Duration, store Store, fetcher Fetcher) *SubtitleService {
	return &SubtitleService{
		maxFileSize: maxFileSize,
		defaultTTL:  defaultTTL,
		store:       store,
		fetcher:     fetcher,
	}
}

func (service *SubtitleService) CreateSubtitle(ctx context.Context, sourceURL string) (CreateResult, error) {
	if err := domain.ValidateSubtitleURL(sourceURL); err != nil {
		return CreateResult{}, err
	}

	body, err := service.fetcher.Fetch(ctx, sourceURL, service.maxFileSize)
	if err != nil {
		return CreateResult{}, err
	}
	if int64(len(body)) > service.maxFileSize {
		return CreateResult{}, ErrSubtitleTooLarge
	}

	if err := domain.ValidateSubtitleContent(body); err != nil {
		return CreateResult{}, err
	}

	id, err := domain.GenerateIdentifier()
	if err != nil {
		return CreateResult{}, err
	}

	record := domain.Subtitle{
		ID:        id,
		SourceURL: sourceURL,
		AccessURL: sourceURL,
		Content:   "",
		CreatedAt: time.Now().UTC(),
		ExpiresAt: time.Now().UTC().Add(service.defaultTTL),
		Valid:     true,
	}

	if err := service.store.Save(ctx, record); err != nil {
		return CreateResult{}, err
	}

	return CreateResult{
		ID:  id,
		URL: sourceURL,
	}, nil
}

func (service *SubtitleService) LatestSubtitle(ctx context.Context) (domain.Subtitle, error) {
	return service.store.Latest(ctx)
}
