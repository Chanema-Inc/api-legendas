package httpapi

import (
	"encoding/json"
	"net"
	"net/http"
)

func WithProbeIPAllowlist(enabled bool, allowedProbeIPs map[string]struct{}, next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if !isProbePath(request.URL.Path) {
			next.ServeHTTP(response, request)
			return
		}
		if !enabled {
			next.ServeHTTP(response, request)
			return
		}
		if len(allowedProbeIPs) == 0 {
			next.ServeHTTP(response, request)
			return
		}

		ip := request.RemoteAddr
		if host, _, err := net.SplitHostPort(request.RemoteAddr); err == nil {
			ip = host
		}
		if _, allowed := allowedProbeIPs[ip]; !allowed {
			writeJSONError(response, http.StatusForbidden, "probe endpoint not allowed")
			return
		}

		next.ServeHTTP(response, request)
	})
}

func WithCORS(allowedOrigins map[string]struct{}, next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		origin := request.Header.Get("Origin")
		if origin != "" {
			if _, allowed := allowedOrigins[origin]; !allowed {
				writeJSONError(response, http.StatusForbidden, "origin not allowed")
				return
			}

			response.Header().Set("Access-Control-Allow-Origin", origin)
			response.Header().Set("Vary", "Origin")
			response.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS") //por que options é aceito? cors envia options?
			response.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		}

		if request.Method == http.MethodOptions {
			response.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(response, request)
	})
}

func WithRateLimit(limiter *RateLimiter, next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if isProbePath(request.URL.Path) {
			next.ServeHTTP(response, request)
			return
		}
		if limiter != nil && !limiter.Allow(request.RemoteAddr) {
			writeJSONError(response, http.StatusTooManyRequests, "rate limit exceeded")
			return
		}
		next.ServeHTTP(response, request)
	})
}

func isProbePath(path string) bool {
	return path == "/health"
}

func writeJSONError(response http.ResponseWriter, statusCode int, message string) {
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(statusCode)
	_ = json.NewEncoder(response).Encode(map[string]string{"error": message})
}
