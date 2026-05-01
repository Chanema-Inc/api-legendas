package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadUsesEnvironmentVariablesBeforeDevDefaults(t *testing.T) {
	rootDir := t.TempDir()
	writeFile(t, filepath.Join(rootDir, ".env.dev"), "APP_PORT=8080\nMAX_SUBTITLE_SIZE_BYTES=307200\nALLOWED_ORIGINS=http://client.local\nPROBE_ALLOWED_IPS=127.0.0.1\nHEALTH_PROTECTION_ENABLED=true\nCACHE_TTL=10m\nRATE_LIMIT_BURST=20\nRATE_LIMIT_WINDOW=1m\n")
	writeFile(t, filepath.Join(rootDir, ".env.prod"), "APP_PORT=\nMAX_SUBTITLE_SIZE_BYTES=\nALLOWED_ORIGINS=\nPROBE_ALLOWED_IPS=\nHEALTH_PROTECTION_ENABLED=\nCACHE_TTL=\nRATE_LIMIT_BURST=\nRATE_LIMIT_WINDOW=\n")

	t.Setenv("APP_PORT", "9090")
	t.Setenv("MAX_SUBTITLE_SIZE_BYTES", "1024")
	t.Setenv("ALLOWED_ORIGINS", "http://override.local")
	t.Setenv("APP_ENV", "development")

	config, err := Load(rootDir)
	if err != nil {
		t.Fatalf("expected config to load successfully, got error: %v", err)
	}

	if config.Port != "9090" {
		t.Fatalf("expected env port override, got %q", config.Port)
	}

	if config.MaxSubtitleSizeBytes != 1024 {
		t.Fatalf("expected env size override, got %d", config.MaxSubtitleSizeBytes)
	}

	if _, ok := config.AllowedOrigins["http://override.local"]; !ok {
		t.Fatal("expected overridden allowed origin to be loaded")
	}
	if _, ok := config.AllowedProbeIPs["127.0.0.1"]; !ok {
		t.Fatal("expected probe allowed ip to be loaded")
	}
	if !config.HealthProtectionEnabled {
		t.Fatal("expected health protection to be enabled")
	}
}

func TestLoadFallsBackToDevValuesInProductionWhenEnvVarIsMissing(t *testing.T) {
	rootDir := t.TempDir()
	writeFile(t, filepath.Join(rootDir, ".env.dev"), "APP_PORT=8080\nMAX_SUBTITLE_SIZE_BYTES=307200\nALLOWED_ORIGINS=http://client.local,http://admin.local\nPROBE_ALLOWED_IPS=127.0.0.1,::1\nHEALTH_PROTECTION_ENABLED=true\nCACHE_TTL=15m\nRATE_LIMIT_BURST=30\nRATE_LIMIT_WINDOW=1m\n")
	writeFile(t, filepath.Join(rootDir, ".env.prod"), "APP_PORT=\nMAX_SUBTITLE_SIZE_BYTES=\nALLOWED_ORIGINS=\nPROBE_ALLOWED_IPS=\nHEALTH_PROTECTION_ENABLED=\nCACHE_TTL=\nRATE_LIMIT_BURST=\nRATE_LIMIT_WINDOW=\n")

	t.Setenv("APP_ENV", "production")

	config, err := Load(rootDir)
	if err != nil {
		t.Fatalf("expected config to load successfully, got error: %v", err)
	}

	if config.Port != "8080" {
		t.Fatalf("expected dev fallback port, got %q", config.Port)
	}

	if config.MaxSubtitleSizeBytes != 307200 {
		t.Fatalf("expected dev fallback size, got %d", config.MaxSubtitleSizeBytes)
	}

	if _, ok := config.AllowedOrigins["http://admin.local"]; !ok {
		t.Fatal("expected fallback allowed origins to be loaded")
	}
	if !config.HealthProtectionEnabled {
		t.Fatal("expected health protection fallback to be enabled")
	}

}

func TestLoadReadsHealthProtectionEnabledFromEnv(t *testing.T) {
	rootDir := t.TempDir()
	writeFile(t, filepath.Join(rootDir, ".env.dev"), "HEALTH_PROTECTION_ENABLED=true\n")
	writeFile(t, filepath.Join(rootDir, ".env.prod"), "HEALTH_PROTECTION_ENABLED=\n")

	t.Setenv("HEALTH_PROTECTION_ENABLED", "false")

	config, err := Load(rootDir)
	if err != nil {
		t.Fatalf("expected config to load successfully, got error: %v", err)
	}

	if config.HealthProtectionEnabled {
		t.Fatal("expected health protection to be disabled by env override")
	}
}

func TestLoadSupportsRedisBackendSettings(t *testing.T) {
	rootDir := t.TempDir()
	writeFile(t, filepath.Join(rootDir, ".env.dev"), "STORAGE_BACKEND=memory_cache\nREDIS_ADDR=localhost:6379\nREDIS_DB=0\nREDIS_KEY_PREFIX=subtitle-delivery\nCACHE_TTL=10m\n")
	writeFile(t, filepath.Join(rootDir, ".env.prod"), "STORAGE_BACKEND=redis\nREDIS_ADDR=redis.internal:6379\nREDIS_PASSWORD=secret\nREDIS_DB=4\nREDIS_KEY_PREFIX=subtitle-prod\nCACHE_TTL=30m\n")

	t.Setenv("APP_ENV", "production")

	config, err := Load(rootDir)
	if err != nil {
		t.Fatalf("expected config to load successfully, got error: %v", err)
	}

	if config.Storage != "redis" {
		t.Fatalf("expected redis storage backend, got %q", config.Storage)
	}

	if config.RedisAddr != "redis.internal:6379" {
		t.Fatalf("expected redis address to be loaded, got %q", config.RedisAddr)
	}

	if config.RedisPassword != "secret" {
		t.Fatalf("expected redis password to be loaded, got %q", config.RedisPassword)
	}

	if config.RedisDB != 4 {
		t.Fatalf("expected redis db to be loaded, got %d", config.RedisDB)
	}

	if config.RedisKeyPrefix != "subtitle-prod" {
		t.Fatalf("expected redis key prefix to be loaded, got %q", config.RedisKeyPrefix)
	}

	if config.CacheTTL.Minutes() != 30 {
		t.Fatalf("expected cache ttl to be loaded, got %s", config.CacheTTL)
	}
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write file %s: %v", path, err)
	}
}
