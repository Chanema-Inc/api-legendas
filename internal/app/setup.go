package app

import (
	"fmt"
	"time"

	infrastructure "subtitle-delivery/internal/infrastructure"
	config "subtitle-delivery/internal/infrastructure/config"
	db "subtitle-delivery/internal/infrastructure/db"
	service "subtitle-delivery/internal/service"
)

func NewStorage(appConfig config.RuntimeConfig) (service.Store, error) {

	switch appConfig.Storage {
	case "", "memory":
		return infrastructure.NewMemoryStore(appConfig.CacheTTL), nil
	case "redis":
		return db.NewRedisStore(db.RedisConfig{
			Addr:      appConfig.RedisAddr,
			Password:  appConfig.RedisPassword,
			DB:        appConfig.RedisDB,
			KeyPrefix: appConfig.RedisKeyPrefix,
			TTL:       appConfig.CacheTTL,
		})
	default:
		return nil, fmt.Errorf("unsupported storage backend: %s", appConfig.Storage)
	}
}

// Isso deveria estar aqui?
func NewHTTPFetcher(timeout time.Duration) service.Fetcher {
	return infrastructure.NewHTTPFetcher(timeout)
}
