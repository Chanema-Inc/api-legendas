package infrastructure

import (
	"context"
	"errors"
	"sync"
	"time"

	domain "subtitle-delivery/internal/domain"
)

type MemoryStore struct {
	mu      sync.RWMutex
	latest  *domain.Subtitle
	entries map[string]domain.Subtitle
	ttl     time.Duration
}

func NewMemoryStore(defaultTTL time.Duration) *MemoryStore {
	return &MemoryStore{
		entries: map[string]domain.Subtitle{},
		ttl:     defaultTTL,
	}
}

func (store *MemoryStore) Save(_ context.Context, record domain.Subtitle) error {
	store.mu.Lock()
	defer store.mu.Unlock()

	store.cleanupExpiredLocked(time.Now().UTC())
	record.ExpiresAt = record.CreatedAt.Add(store.ttl)
	store.entries[record.ID] = record
	store.latest = &record
	return nil
}

func (store *MemoryStore) Latest(_ context.Context) (domain.Subtitle, error) {
	store.mu.Lock()
	defer store.mu.Unlock()

	store.cleanupExpiredLocked(time.Now().UTC())
	if store.latest == nil {
		return domain.Subtitle{}, errors.New("subtitle not found")
	}
	if _, exists := store.entries[store.latest.ID]; !exists {
		store.latest = nil
		return domain.Subtitle{}, errors.New("subtitle not found")
	}
	return *store.latest, nil
}

func (store *MemoryStore) Invalidate(id string) {
	store.mu.Lock()
	defer store.mu.Unlock()

	delete(store.entries, id)
	if store.latest != nil && store.latest.ID == id {
		store.latest = nil
	}
}

func (store *MemoryStore) CleanupExpired() {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.cleanupExpiredLocked(time.Now().UTC())
}

func (store *MemoryStore) Size() int {
	store.mu.RLock()
	defer store.mu.RUnlock()
	return len(store.entries)
}

func (store *MemoryStore) cleanupExpiredLocked(now time.Time) {
	for id, record := range store.entries {
		if now.After(record.ExpiresAt) {
			delete(store.entries, id)
			if store.latest != nil && store.latest.ID == id {
				store.latest = nil
			}
		}
	}
}
