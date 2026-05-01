package main

import (
	"log"
	"net/http"
	"os"
	"time"

	app "subtitle-delivery/internal/app"
	httpapi "subtitle-delivery/internal/httpapi"
	config "subtitle-delivery/internal/infrastructure/config"
)

func main() {
	rootDir, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	appConfig, err := config.Load(rootDir)
	if err != nil {
		log.Fatal(err)
	}

	store, err := app.NewStorage(appConfig)
	if err != nil {
		log.Fatal(err)
	}

	// if closer, ok := store.(interface{ Close() error }); ok {
	//
	// Como a aplicação precisa lidar com múltiplas implementações
	// Ex.:
	// case "memory":
	// return infrastructure.NewMemoryStore(...)
	// case "redis":
	// 	return db.NewRedisStore(...)

	// Um MemoryStore provavelmente não precisa fechar nada
	// Um RedisStore provavelmente tem conexão, logo, precisa de Close() (fechar a conexão)

	// Mas não quero forçar isso na interface service.Store, porque:
	// nem todo storage precisa de Close()
	// isso poluiria o contrato

	// O que acontece dentro do bloco:
	// Se store tiver Close(), ele será chamado no final (via defer)
	if closer, ok := store.(interface{ Close() error }); ok {
		defer func() {
			if err := closer.Close(); err != nil {
				log.Printf("store shutdown failed: %v", err)
			}
		}()
	}

	server := app.NewServer(app.Config{
		BaseURL:                 "http://localhost:" + appConfig.Port,
		MaxFileSize:             appConfig.MaxSubtitleSizeBytes,
		DefaultTTL:              appConfig.CacheTTL,
		Store:                   store,
		Fetcher:                 app.NewHTTPFetcher(10 * time.Second),
		RateLimiter:             httpapi.NewRateLimiter(appConfig.RateLimitBurst, appConfig.RateLimitWindow),
		AllowedOrigins:          appConfig.AllowedOrigins,
		AllowedProbeIPs:         appConfig.AllowedProbeIPs,
		HealthProtectionEnabled: appConfig.HealthProtectionEnabled,
	})

	log.Fatal(http.ListenAndServe(":"+appConfig.Port, server.Routes()))
}
