package config

import (
	"bufio"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type RuntimeConfig struct {
	Environment                    string
	Port                           string
	JWTSecret                      string
	Storage                        string
	UpstashRedisURL                string
	RedisAddr                      string
	RedisPassword                  string
	RedisDB                        int
	RedisKeyPrefix                 string
	MaxSubtitleSizeBytes           int64
	AllowedOriginsByRouteAndMethod map[string]map[string]map[string]struct{}
	CacheTTL                       time.Duration
	RateLimitBurst                 int
	RateLimitWindow                time.Duration
}

func Load(rootDir string) (RuntimeConfig, error) {
	devValues, err := parseEnvFile(filepath.Join(rootDir, ".env.dev"))
	if err != nil {
		return RuntimeConfig{}, err
	}

	prodValues, err := parseEnvFile(filepath.Join(rootDir, ".env.prod"))
	if err != nil {
		return RuntimeConfig{}, err
	}

	//Primeiro busca pela variável no ambiente, caso não encontre
	//Busca a variável no arquivo .env.dev, caso não encontre
	//Assume que está no ambiente de desenvolvimento (development)
	environment := firstNonEmpty(os.Getenv("APP_ENV"), devValues["APP_ENV"], "development")
	selectedValues := devValues
	if environment == "production" {
		selectedValues = mergeMaps(devValues, prodValues)
	}

	port := resolveSetting("APP_PORT", selectedValues)
	if port == "" {
		port = "8080"
	}

	//300kb é o valor padrão
	maxSubtitleSizeBytes, err := parseInt64(resolveSetting("MAX_SUBTITLE_SIZE_BYTES", selectedValues), 300*1024)
	if err != nil {
		return RuntimeConfig{}, err
	}

	cacheTTL, err := parseDuration(resolveSetting("CACHE_TTL", selectedValues), 5*time.Hour)
	if err != nil {
		return RuntimeConfig{}, err
	}

	rateLimitBurst, err := parseInt(resolveSetting("RATE_LIMIT_BURST", selectedValues), 60)
	if err != nil {
		return RuntimeConfig{}, err
	}

	rateLimitWindow, err := parseDuration(resolveSetting("RATE_LIMIT_WINDOW", selectedValues), time.Minute)
	if err != nil {
		return RuntimeConfig{}, err
	}

	redisDB, err := parseInt(resolveSetting("REDIS_DB", selectedValues), 0)
	if err != nil {
		return RuntimeConfig{}, err
	}

	allowedOriginsByRouteAndMethod := parseOriginsByRouteAndMethod(selectedValues)

	return RuntimeConfig{
		Environment:                    environment,
		Port:                           port,
		JWTSecret:                      resolveSetting("JWT_SECRET", selectedValues),
		Storage:                        firstNonEmpty(resolveSetting("STORAGE_BACKEND", selectedValues), "memory_cache"),
		UpstashRedisURL:                resolveSetting("UPSTASH_REDIS_URL", selectedValues),
		RedisAddr:                      resolveSetting("REDIS_ADDR", selectedValues),
		RedisPassword:                  resolveSetting("REDIS_PASSWORD", selectedValues),
		RedisDB:                        redisDB,
		RedisKeyPrefix:                 firstNonEmpty(resolveSetting("REDIS_KEY_PREFIX", selectedValues), "subtitle-delivery"),
		MaxSubtitleSizeBytes:           maxSubtitleSizeBytes,
		AllowedOriginsByRouteAndMethod: allowedOriginsByRouteAndMethod,
		CacheTTL:                       cacheTTL,
		RateLimitBurst:                 rateLimitBurst,
		RateLimitWindow:                rateLimitWindow,
	}, nil
}

func resolveSetting(key string, fallback map[string]string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return strings.TrimSpace(fallback[key])
}

func mergeMaps(primary map[string]string, secondary map[string]string) map[string]string {
	merged := map[string]string{}
	for key, value := range primary {
		merged[key] = value
	}
	for key, value := range secondary {
		if strings.TrimSpace(value) != "" {
			merged[key] = value
		}
	}
	return merged
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func parseEnvFile(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return map[string]string{}, nil
		}
		return nil, err
	}
	defer file.Close()

	values := map[string]string{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		key := strings.TrimSpace(parts[0])
		value := ""
		if len(parts) == 2 {
			value = strings.TrimSpace(parts[1])
		}
		values[key] = value
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return values, nil
}

func parseOriginsByRouteAndMethod(selectedValues map[string]string) map[string]map[string]map[string]struct{} {
	legendaPOST := parseCSVSet(resolveSetting("ALLOWED_ORIGINS_LEGENDA_POST", selectedValues))
	healthGET := parseCSVSet(resolveSetting("ALLOWED_ORIGINS_HEALTH_GET", selectedValues))

	result := map[string]map[string]map[string]struct{}{
		"/legenda": {
			httpMethodPost:    legendaPOST,
			httpMethodOptions: cloneSet(legendaPOST),
		},
		"/health": {
			httpMethodGet:     healthGET,
			httpMethodOptions: cloneSet(healthGET),
		},
	}

	return result
}

const (
	httpMethodGet     = "GET"
	httpMethodPost    = "POST"
	httpMethodOptions = "OPTIONS"
)

func parseCSVSet(raw string) map[string]struct{} {
	origins := map[string]struct{}{}
	for _, origin := range strings.Split(raw, ",") {
		trimmed := strings.TrimSpace(origin)
		if trimmed != "" {
			origins[trimmed] = struct{}{}
		}
	}
	return origins
}

func cloneSet(source map[string]struct{}) map[string]struct{} {
	cloned := map[string]struct{}{}
	for value := range source {
		cloned[value] = struct{}{}
	}
	return cloned
}

func unionSets(sets ...map[string]struct{}) map[string]struct{} {
	result := map[string]struct{}{}
	for _, set := range sets {
		for value := range set {
			result[value] = struct{}{}
		}
	}
	return result
}

func parseDuration(raw string, fallback time.Duration) (time.Duration, error) {
	if strings.TrimSpace(raw) == "" {
		return fallback, nil
	}
	return time.ParseDuration(raw)
}

func parseInt(raw string, fallback int) (int, error) {
	if strings.TrimSpace(raw) == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, err
	}
	return value, nil
}

func parseInt64(raw string, fallback int64) (int64, error) {
	if strings.TrimSpace(raw) == "" {
		return fallback, nil
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, err
	}
	return value, nil
}
