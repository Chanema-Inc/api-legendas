package httpapi

import (
	"compress/gzip"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
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

func WithJWTAuth(secret string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if !requiresJWT(request) {
			next.ServeHTTP(response, request)
			return
		}

		token, err := extractBearerToken(request.Header.Get("Authorization"))
		if err != nil {
			writeJSONError(response, http.StatusUnauthorized, err.Error())
			return
		}

		if strings.TrimSpace(secret) == "" {
			writeJSONError(response, http.StatusInternalServerError, "jwt secret not configured")
			return
		}

		parsedToken, err := jwt.Parse(token, func(parsedToken *jwt.Token) (any, error) {
			if _, ok := parsedToken.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, errors.New("unexpected signing method")
			}
			return []byte(secret), nil
		}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg(), jwt.SigningMethodHS384.Alg(), jwt.SigningMethodHS512.Alg()}))
		if err != nil || !parsedToken.Valid {
			writeJSONError(response, http.StatusUnauthorized, "invalid authorization token")
			return
		}

		next.ServeHTTP(response, request)
	})
}

func requiresJWT(request *http.Request) bool {
	return request.URL.Path == "/legenda" && request.Method == http.MethodPost
}

func extractBearerToken(headerValue string) (string, error) {
	headerValue = strings.TrimSpace(headerValue)
	if headerValue == "" {
		return "", errors.New("missing authorization header")
	}

	parts := strings.SplitN(headerValue, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", errors.New("invalid authorization header format")
	}

	token := strings.TrimSpace(parts[1])
	if token == "" {
		return "", errors.New("invalid authorization header format")
	}

	return token, nil
}

func isProbePath(path string) bool {
	return path == "/health"
}

func writeJSONError(response http.ResponseWriter, statusCode int, message string) {
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(statusCode)
	_ = json.NewEncoder(response).Encode(map[string]string{"error": message})
}
