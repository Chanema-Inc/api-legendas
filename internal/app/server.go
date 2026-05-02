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
	baseURL                        string
	jwtSecret                      string
	maxFileSize                    int64
	defaultTTL                     time.Duration
	store                          service.Store
	fetcher                        service.Fetcher
	subtitleController             *controller.HTTPController
	subtitleService                *service.SubtitleService
	rateLimiter                    *httpapi.RateLimiter
	allowedOriginsByRouteAndMethod map[string]map[string]map[string]struct{}
	handler                        http.Handler
}

type Config struct {
	BaseURL                        string
	JWTSecret                      string
	MaxFileSize                    int64
	DefaultTTL                     time.Duration
	Store                          service.Store
	Fetcher                        service.Fetcher
	RateLimiter                    *httpapi.RateLimiter
	AllowedOriginsByRouteAndMethod map[string]map[string]map[string]struct{}
}

func NewServer(config Config) *Server {
	server := &Server{
		baseURL:                        config.BaseURL,
		jwtSecret:                      config.JWTSecret,
		maxFileSize:                    config.MaxFileSize,
		defaultTTL:                     config.DefaultTTL,
		store:                          config.Store,
		fetcher:                        config.Fetcher,
		rateLimiter:                    config.RateLimiter,
		allowedOriginsByRouteAndMethod: config.AllowedOriginsByRouteAndMethod,
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
	server.handler = httpapi.WithCORS(server.allowedOriginsByRouteAndMethod,
		httpapi.WithGzip(httpapi.WithRateLimit(server.rateLimiter, httpapi.WithJWTAuth(server.jwtSecret, mux))))

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
		server.defaultTTL = 5 * time.Hour
	}
	if server.store == nil {
		server.store = infrastructure.NewMemoryStore(server.defaultTTL)
	}
	if server.fetcher == nil {
		server.fetcher = infrastructure.NewHTTPFetcher(10 * time.Second)
	}
	if server.allowedOriginsByRouteAndMethod == nil {
		server.allowedOriginsByRouteAndMethod = map[string]map[string]map[string]struct{}{}
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
