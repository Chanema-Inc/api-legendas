package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestWithCORSAllowsConfiguredOrigin(t *testing.T) {
	t.Parallel()

	handler := WithCORS(map[string]struct{}{"http://client.local": {}}, http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
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
		t.Fatalf("expected allowed origin header, got %q", got)
	}
}

func TestWithCORSBlocksUnknownOrigin(t *testing.T) {
	t.Parallel()

	handler := WithCORS(map[string]struct{}{"http://client.local": {}}, http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
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

func TestWithProbeIPAllowlistBlocksUnknownIP(t *testing.T) {
	t.Parallel()

	handler := WithProbeIPAllowlist(true, map[string]struct{}{"127.0.0.1": {}}, http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusOK)
	}))

	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	request.RemoteAddr = "10.0.0.10:45678"
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, recorder.Code)
	}
}

func TestWithProbeIPAllowlistAllowsConfiguredIP(t *testing.T) {
	t.Parallel()

	handler := WithProbeIPAllowlist(true, map[string]struct{}{"127.0.0.1": {}}, http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusOK)
	}))

	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	request.RemoteAddr = "127.0.0.1:45678"
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}
}

func TestWithProbeIPAllowlistDisabledBypassesProtection(t *testing.T) {
	t.Parallel()

	handler := WithProbeIPAllowlist(false, map[string]struct{}{"127.0.0.1": {}}, http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusOK)
	}))

	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	request.RemoteAddr = "10.0.0.10:45678"
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}
}
