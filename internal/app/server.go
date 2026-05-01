package app

import (
	"net/http"
	"time"

	controller "subtitle-delivery/internal/controller"
	httpapi "subtitle-delivery/internal/httpapi"
	infrastructure "subtitle-delivery/internal/infrastructure"
	service "subtitle-delivery/internal/service"
)

type Server struct {
	baseURL                 string
	maxFileSize             int64
	defaultTTL              time.Duration
	store                   service.Store
	fetcher                 service.Fetcher
	subtitleController      *controller.HTTPController
	subtitleService         *service.SubtitleService
	rateLimiter             *httpapi.RateLimiter
	allowedOrigins          map[string]struct{}
	allowedProbeIPs         map[string]struct{}
	healthProtectionEnabled bool
	handler                 http.Handler
}

type Config struct {
	BaseURL                 string
	MaxFileSize             int64
	DefaultTTL              time.Duration
	Store                   service.Store
	Fetcher                 service.Fetcher
	RateLimiter             *httpapi.RateLimiter
	AllowedOrigins          map[string]struct{}
	AllowedProbeIPs         map[string]struct{}
	HealthProtectionEnabled bool
}

func NewServer(config Config) *Server {
	server := &Server{
		baseURL:                 config.BaseURL,
		maxFileSize:             config.MaxFileSize,
		defaultTTL:              config.DefaultTTL,
		store:                   config.Store,
		fetcher:                 config.Fetcher,
		rateLimiter:             config.RateLimiter,
		allowedOrigins:          config.AllowedOrigins,
		allowedProbeIPs:         config.AllowedProbeIPs,
		healthProtectionEnabled: config.HealthProtectionEnabled,
	}
	server.ensureDefaults()
	return server
}

func (server *Server) Routes() http.Handler {
	if server.handler != nil {
		return server.handler
	}
	server.ensureDefaults()

	mux := http.NewServeMux()
	httpapi.RegisterRoutes(mux, server.subtitleController)
	server.handler = httpapi.WithCORS(server.allowedOrigins,
		httpapi.WithProbeIPAllowlist(server.healthProtectionEnabled, server.allowedProbeIPs,
			httpapi.WithRateLimit(server.rateLimiter, mux)))

	return server.handler
}

func (server *Server) ensureDefaults() {
	if server.baseURL == "" {
		server.baseURL = "http://localhost:8080"
	}
	if server.maxFileSize == 0 {
		server.maxFileSize = 300 * 1024
	}
	if server.defaultTTL == 0 {
		server.defaultTTL = 10 * time.Minute
	}
	if server.store == nil {
		server.store = infrastructure.NewMemoryStore(server.defaultTTL)
	}
	if server.fetcher == nil {
		server.fetcher = infrastructure.NewHTTPFetcher(10 * time.Second)
	}
	if server.allowedOrigins == nil {
		server.allowedOrigins = map[string]struct{}{}
	}
	if server.allowedProbeIPs == nil {
		server.allowedProbeIPs = map[string]struct{}{}
	}
	if server.rateLimiter == nil {
		server.rateLimiter = httpapi.NewRateLimiter(60, time.Minute)
	}
	if server.subtitleService == nil {
		server.subtitleService = service.NewSubtitleService(server.maxFileSize, server.defaultTTL, server.store, server.fetcher)
	}

	if server.subtitleController == nil {
		server.subtitleController = controller.NewHTTPController(server.subtitleService)
	}
}
