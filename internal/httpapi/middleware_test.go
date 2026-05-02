package httpapi

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func makeJWTForTest(t *testing.T, secret string) string {
	t.Helper()

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": "middleware-test",
		"exp": time.Now().Add(time.Hour).Unix(),
	})

	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("expected token to be signed, got error: %v", err)
	}

	return signed
}

func TestWithCORSAllowsConfiguredOrigin(t *testing.T) {
	t.Parallel()

	handler := WithCORS(map[string]map[string]map[string]struct{}{
		"/subtitle": {
			http.MethodPost: {"http://client.local": {}},
		},
	}, http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusOK)
	}))

	request := httptest.NewRequest(http.MethodOptions, "/subtitle", nil)
	request.Header.Set("Origin", "http://client.local")
	request.Header.Set("Access-Control-Request-Method", http.MethodPost)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, recorder.Code)
	}
	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "http://client.local" {
		t.Fatalf("expected echoed origin header, got %q", got)
	}
}

func TestWithCORSBlocksUnknownOrigin(t *testing.T) {
	t.Parallel()

	handler := WithCORS(map[string]map[string]map[string]struct{}{
		"/subtitle": {
			http.MethodGet: {"http://client.local": {}},
		},
	}, http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusOK)
	}))

	request := httptest.NewRequest(http.MethodGet, "/subtitle", nil)
	request.Header.Set("Origin", "http://blocked.local")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, recorder.Code)
	}
}

func TestWithCORSBlocksOriginWhenMethodAllowlistDiffers(t *testing.T) {
	t.Parallel()

	handler := WithCORS(map[string]map[string]map[string]struct{}{
		"/subtitle": {
			http.MethodGet:  {"http://client.local": {}},
			http.MethodPost: {"http://writer.local": {}},
		},
	}, http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusOK)
	}))

	request := httptest.NewRequest(http.MethodPost, "/subtitle", nil)
	request.Header.Set("Origin", "http://client.local")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, recorder.Code)
	}
}

func TestWithCORSAllowsAnyOriginWhenRouteHasNoAllowlist(t *testing.T) {
	t.Parallel()

	handler := WithCORS(map[string]map[string]map[string]struct{}{
		"/legenda": {
			http.MethodPost: {"http://writer.local": {}},
		},
	}, http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusOK)
	}))

	request := httptest.NewRequest(http.MethodGet, "/legenda", nil)
	request.Header.Set("Origin", "http://anyone.local")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d for GET with no allowlist, got %d", http.StatusOK, recorder.Code)
	}
	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "http://anyone.local" {
		t.Fatalf("expected echoed origin, got %q", got)
	}
}

func TestWithCORSAllowsAnyOriginForRouteWithNoConfiguration(t *testing.T) {
	t.Parallel()

	handler := WithCORS(map[string]map[string]map[string]struct{}{
		"/legenda": {
			http.MethodGet: {"http://client.local": {}},
		},
	}, http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusOK)
	}))

	request := httptest.NewRequest(http.MethodGet, "/other", nil)
	request.Header.Set("Origin", "http://any.local")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d for route with no allowlist, got %d", http.StatusOK, recorder.Code)
	}
}

func TestWithGzipCompressesResponseWhenAccepted(t *testing.T) {
	t.Parallel()

	handler := WithGzip(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "text/vtt")
		response.WriteHeader(http.StatusOK)
		_, _ = response.Write([]byte("WEBVTT\n\n00:00:00.000 --> 00:00:01.000\nhello\n"))
	}))

	request := httptest.NewRequest(http.MethodGet, "/legenda", nil)
	request.Header.Set("Accept-Encoding", "gzip")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}
	if got := recorder.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("expected gzip content-encoding, got %q", got)
	}

	gzipReader, err := gzip.NewReader(recorder.Body)
	if err != nil {
		t.Fatalf("expected gzip payload, got error: %v", err)
	}
	defer gzipReader.Close()

	decoded, err := io.ReadAll(gzipReader)
	if err != nil {
		t.Fatalf("failed to decode gzip payload: %v", err)
	}
	if string(decoded) == "" {
		t.Fatal("expected decoded payload to be non-empty")
	}
}

func TestWithRateLimitBlocksWhenLimiterRejects(t *testing.T) {
	t.Parallel()

	limiter := NewRateLimiter(1, time.Minute)
	handler := WithRateLimit(limiter, http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusOK)
	}))

	firstRequest := httptest.NewRequest(http.MethodGet, "/subtitle", nil)
	firstRecorder := httptest.NewRecorder()
	handler.ServeHTTP(firstRecorder, firstRequest)

	request := httptest.NewRequest(http.MethodGet, "/subtitle", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("expected status %d, got %d", http.StatusTooManyRequests, recorder.Code)
	}
}

func TestWithRateLimitPassesWhenLimiterAllows(t *testing.T) {
	t.Parallel()

	handler := WithRateLimit(NewRateLimiter(2, time.Minute), http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusAccepted)
	}))

	request := httptest.NewRequest(http.MethodGet, "/subtitle", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("expected status %d, got %d", http.StatusAccepted, recorder.Code)
	}
}

func TestWithRateLimitBypassesProbeEndpoints(t *testing.T) {
	t.Parallel()

	limiter := NewRateLimiter(1, time.Minute)
	handler := WithRateLimit(limiter, http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusOK)
	}))

	first := httptest.NewRequest(http.MethodGet, "/health", nil)
	firstRecorder := httptest.NewRecorder()
	handler.ServeHTTP(firstRecorder, first)

	second := httptest.NewRequest(http.MethodGet, "/health", nil)
	secondRecorder := httptest.NewRecorder()
	handler.ServeHTTP(secondRecorder, second)

	if secondRecorder.Code != http.StatusOK {
		t.Fatalf("expected status %d for /health, got %d", http.StatusOK, secondRecorder.Code)
	}
}

func TestWithJWTAuthRejectsMissingAuthorizationForProtectedRoute(t *testing.T) {
	t.Parallel()

	handler := WithJWTAuth("secret", http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusOK)
	}))

	request := httptest.NewRequest(http.MethodPost, "/legenda", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, recorder.Code)
	}
}

func TestWithJWTAuthAllowsValidTokenForProtectedRoute(t *testing.T) {
	t.Parallel()

	secret := "secret"
	handler := WithJWTAuth(secret, http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusAccepted)
	}))

	request := httptest.NewRequest(http.MethodPost, "/legenda", nil)
	request.Header.Set("Authorization", "Bearer "+makeJWTForTest(t, secret))
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("expected status %d, got %d", http.StatusAccepted, recorder.Code)
	}
}

func TestWithJWTAuthSkipsAuthenticationForGetLegenda(t *testing.T) {
	t.Parallel()

	handler := WithJWTAuth("secret", http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusOK)
	}))

	request := httptest.NewRequest(http.MethodGet, "/legenda", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}
}
