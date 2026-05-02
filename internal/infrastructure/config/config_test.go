package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadUsesEnvironmentVariablesBeforeDevDefaults(t *testing.T) {
	rootDir := t.TempDir()
	writeFile(t, filepath.Join(rootDir, ".env.dev"), "APP_PORT=8080\nMAX_SUBTITLE_SIZE_BYTES=307200\nALLOWED_ORIGINS_LEGENDA_POST=https://subtitle-api.fly.dev\nALLOWED_ORIGINS_HEALTH_GET=https://service.onrender.com\nCACHE_TTL=5h\nRATE_LIMIT_BURST=20\nRATE_LIMIT_WINDOW=1m\n")
	writeFile(t, filepath.Join(rootDir, ".env.prod"), "")

	t.Setenv("APP_PORT", "9090")
	t.Setenv("MAX_SUBTITLE_SIZE_BYTES", "1024")
	t.Setenv("ALLOWED_ORIGINS_LEGENDA_POST", "https://override.fly.dev")
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

	if origins := config.AllowedOriginsByRouteAndMethod["/legenda"]["GET"]; len(origins) != 0 {
		t.Fatal("expected legenda GET to have no allowlist (open)")
	}
	if _, ok := config.AllowedOriginsByRouteAndMethod["/legenda"]["POST"]["https://override.fly.dev"]; !ok {
		t.Fatal("expected legenda POST origin override from environment")
	}
	if _, ok := config.AllowedOriginsByRouteAndMethod["/health"]["GET"]["https://service.onrender.com"]; !ok {
		t.Fatal("expected health GET origin from env file")
	}
}

func TestLoadFallsBackToDevValuesInProductionWhenEnvVarIsMissing(t *testing.T) {
	rootDir := t.TempDir()
	writeFile(t, filepath.Join(rootDir, ".env.dev"), "APP_PORT=8080\nMAX_SUBTITLE_SIZE_BYTES=307200\nALLOWED_ORIGINS_LEGENDA_POST=https://subtitle-api.fly.dev\nALLOWED_ORIGINS_HEALTH_GET=https://service.onrender.com\nCACHE_TTL=5h\nRATE_LIMIT_BURST=30\nRATE_LIMIT_WINDOW=1m\n")
	writeFile(t, filepath.Join(rootDir, ".env.prod"), "APP_PORT=\nMAX_SUBTITLE_SIZE_BYTES=\nALLOWED_ORIGINS_LEGENDA_POST=\nALLOWED_ORIGINS_HEALTH_GET=\nCACHE_TTL=\nRATE_LIMIT_BURST=\nRATE_LIMIT_WINDOW=\n")

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

	if origins := config.AllowedOriginsByRouteAndMethod["/legenda"]["GET"]; len(origins) != 0 {
		t.Fatal("expected legenda GET to have no allowlist (open)")
	}
	if _, ok := config.AllowedOriginsByRouteAndMethod["/legenda"]["POST"]["https://subtitle-api.fly.dev"]; !ok {
		t.Fatal("expected /legenda POST origin fallback to dev file")
	}
	if _, ok := config.AllowedOriginsByRouteAndMethod["/health"]["GET"]["https://service.onrender.com"]; !ok {
		t.Fatal("expected /health GET origin fallback to dev file")
	}
}

func TestLoadUsesFiveHoursAsDefaultCacheTTL(t *testing.T) {
	rootDir := t.TempDir()
	writeFile(t, filepath.Join(rootDir, ".env.dev"), "")
	writeFile(t, filepath.Join(rootDir, ".env.prod"), "")

	config, err := Load(rootDir)
	if err != nil {
		t.Fatalf("expected config to load successfully, got error: %v", err)
	}

	if config.CacheTTL.Hours() != 5 {
		t.Fatalf("expected default cache ttl to be 5h, got %s", config.CacheTTL)
	}
}

func TestLoadBuildsOptionsAllowlistFromRouteMethods(t *testing.T) {
	rootDir := t.TempDir()
	writeFile(t, filepath.Join(rootDir, ".env.dev"), "ALLOWED_ORIGINS_LEGENDA_POST=https://subtitle-api.fly.dev\n")
	writeFile(t, filepath.Join(rootDir, ".env.prod"), "")

	config, err := Load(rootDir)
	if err != nil {
		t.Fatalf("expected config to load successfully, got error: %v", err)
	}

	optionsOrigins := config.AllowedOriginsByRouteAndMethod["/legenda"]["OPTIONS"]
	if _, ok := optionsOrigins["https://subtitle-api.fly.dev"]; !ok {
		t.Fatal("expected /legenda OPTIONS to include POST origin")
	}
}

func TestLoadSupportsMethodSpecificAllowedOrigins(t *testing.T) {
	rootDir := t.TempDir()
	writeFile(t, filepath.Join(rootDir, ".env.dev"), "ALLOWED_ORIGINS=http://default.local\nALLOWED_ORIGINS_GET=https://watch.local\nALLOWED_ORIGINS_POST=https://bot.local\n")
	writeFile(t, filepath.Join(rootDir, ".env.prod"), "ALLOWED_ORIGINS=\nALLOWED_ORIGINS_GET=\nALLOWED_ORIGINS_POST=\n")

	config, err := Load(rootDir)
	if err != nil {
		t.Fatalf("expected config to load successfully, got error: %v", err)
	}

	if _, ok := config.AllowedOriginsByMethod["GET"]["https://watch.local"]; !ok {
		t.Fatal("expected GET origin to use method-specific allowlist")
	}

	if _, ok := config.AllowedOriginsByMethod["POST"]["https://bot.local"]; !ok {
		t.Fatal("expected POST origin to use method-specific allowlist")
	}

	if _, ok := config.AllowedOriginsByMethod["POST"]["http://default.local"]; ok {
		t.Fatal("expected POST origin list to not include default when method-specific list is provided")
	}

	if _, ok := config.AllowedOriginsByMethod["OPTIONS"]["http://default.local"]; !ok {
		t.Fatal("expected OPTIONS origin list to fallback to default allowlist")
	}
}

func TestLoadSupportsRouteAndMethodSpecificAllowedOrigins(t *testing.T) {
	rootDir := t.TempDir()
	writeFile(t, filepath.Join(rootDir, ".env.dev"), "ALLOWED_ORIGINS=http://default.local\nALLOWED_ORIGINS_LEGENDA_GET=https://cytube.local\nALLOWED_ORIGINS_LEGENDA_POST=https://bot.local\n")
	writeFile(t, filepath.Join(rootDir, ".env.prod"), "ALLOWED_ORIGINS=\nALLOWED_ORIGINS_LEGENDA_GET=\nALLOWED_ORIGINS_LEGENDA_POST=\n")

	config, err := Load(rootDir)
	if err != nil {
		t.Fatalf("expected config to load successfully, got error: %v", err)
	}

	if _, ok := config.AllowedOriginsByRouteAndMethod["/legenda"]["GET"]["https://cytube.local"]; !ok {
		t.Fatal("expected route-specific GET origin for /legenda")
	}

	if _, ok := config.AllowedOriginsByRouteAndMethod["/legenda"]["POST"]["https://bot.local"]; !ok {
		t.Fatal("expected route-specific POST origin for /legenda")
	}

	if _, ok := config.AllowedOriginsByRouteAndMethod["/legenda"]["POST"]["http://default.local"]; ok {
		t.Fatal("expected route-specific POST list to not include global default when overridden")
	}

	if _, ok := config.AllowedOriginsByRouteAndMethod["/legenda"]["OPTIONS"]["http://default.local"]; !ok {
		t.Fatal("expected OPTIONS for /legenda to fallback to global default")
	}

	if _, ok := config.AllowedOriginsByRouteAndMethod["/health"]["GET"]["http://default.local"]; !ok {
		t.Fatal("expected GET for /health to fallback to global default")
	}
}

func TestLoadSupportsRedisBackendSettings(t *testing.T) {
	rootDir := t.TempDir()
	writeFile(t, filepath.Join(rootDir, ".env.dev"), "STORAGE_BACKEND=memory_cache\nREDIS_ADDR=localhost:6379\nREDIS_DB=0\nREDIS_KEY_PREFIX=subtitle-delivery\nCACHE_TTL=5h\n")
	writeFile(t, filepath.Join(rootDir, ".env.prod"), "STORAGE_BACKEND=redis\nREDIS_ADDR=redis.internal:6379\nREDIS_PASSWORD=secret\nREDIS_DB=4\nREDIS_KEY_PREFIX=subtitle-prod\nCACHE_TTL=6h\n")

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

	if config.CacheTTL.Hours() != 6 {
		t.Fatalf("expected cache ttl to be loaded from production file, got %s", config.CacheTTL)
	}
}

func TestLoadSupportsUpstashRedisURL(t *testing.T) {
	rootDir := t.TempDir()
	writeFile(t, filepath.Join(rootDir, ".env.dev"), "STORAGE_BACKEND=redis\nUPSTASH_REDIS_URL=rediss://default:secret@global-hero-12345.upstash.io:6379\nCACHE_TTL=5h\n")
	writeFile(t, filepath.Join(rootDir, ".env.prod"), "STORAGE_BACKEND=\nUPSTASH_REDIS_URL=\nCACHE_TTL=\n")

	config, err := Load(rootDir)
	if err != nil {
		t.Fatalf("expected config to load successfully, got error: %v", err)
	}

	if config.Storage != "redis" {
		t.Fatalf("expected redis storage backend, got %q", config.Storage)
	}

	if config.UpstashRedisURL != "rediss://default:secret@global-hero-12345.upstash.io:6379" {
		t.Fatalf("expected upstash redis url to be loaded, got %q", config.UpstashRedisURL)
	}
	if config.RedisAddr != "" {
		t.Fatalf("expected redis addr to remain empty, got %q", config.RedisAddr)
	}
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write file %s: %v", path, err)
	}
}
