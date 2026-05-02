package httpapi

import (
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

func WithCORS(allowedOriginsByRouteAndMethod map[string]map[string]map[string]struct{}, next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		origin := request.Header.Get("Origin")
		if origin != "" {
			method := request.Method
			if request.Method == http.MethodOptions {
				requested := strings.ToUpper(strings.TrimSpace(request.Header.Get("Access-Control-Request-Method")))
				if requested != "" {
					method = requested
				}
			}

			allowedOrigins := allowedOriginsByRouteAndMethod[request.URL.Path][strings.ToUpper(method)]
			if len(allowedOrigins) > 0 {
				if _, allowed := allowedOrigins[origin]; !allowed {
					writeJSONError(response, http.StatusForbidden, "origin not allowed")
					return
				}
			}
			response.Header().Set("Access-Control-Allow-Origin", origin)
			response.Header().Set("Vary", "Origin")
			response.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			response.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		}
		if request.Method == http.MethodOptions {
			response.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(response, request)
	})
}

func WithGzip(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if !strings.Contains(request.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(response, request)
			return
		}

		response.Header().Set("Content-Encoding", "gzip")
		response.Header().Add("Vary", "Accept-Encoding")

		gzipWriter := gzip.NewWriter(response)
		defer gzipWriter.Close()

		wrapped := &gzipResponseWriter{ResponseWriter: response, writer: gzipWriter}
		next.ServeHTTP(wrapped, request)
	})
}

type gzipResponseWriter struct {
	http.ResponseWriter
	writer io.Writer
}

func (response *gzipResponseWriter) Write(payload []byte) (int, error) {
	return response.writer.Write(payload)
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
