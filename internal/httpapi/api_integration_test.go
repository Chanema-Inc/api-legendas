package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	controller "subtitle-delivery/internal/controller"
	httpapi "subtitle-delivery/internal/httpapi"
	infrastructure "subtitle-delivery/internal/infrastructure"
	service "subtitle-delivery/internal/service"
)

type stubFetcher struct {
	body []byte
	err  error
}

func (fetcher stubFetcher) Fetch(_ context.Context, _ string, _ int64) ([]byte, error) {
	if fetcher.err != nil {
		return nil, fetcher.err
	}
	if fetcher.body == nil {
		return nil, errors.New("missing stub fetcher body")
	}
	return fetcher.body, nil
}

func newServer(maxFileSize int64, ttl time.Duration, fetcher service.Fetcher, limiter *httpapi.RateLimiter, allowedOrigins map[string]struct{}) http.Handler {
	store := infrastructure.NewMemoryStore(ttl)
	subtitleService := service.NewSubtitleService(maxFileSize, ttl, store, fetcher)
	httpController := controller.NewHTTPController(subtitleService)

	mux := http.NewServeMux()
	httpapi.RegisterRoutes(mux, httpController)
	return httpapi.WithCORS(allowedOrigins, httpapi.WithProbeIPAllowlist(true, map[string]struct{}{}, httpapi.WithRateLimit(limiter, mux)))
}

func newDefaultTestServer(t *testing.T) http.Handler {
	t.Helper()
	return newServer(
		300*1024,
		10*time.Minute,
		stubFetcher{body: []byte("WEBVTT\n\n00:00:00.000 --> 00:00:02.000\nHello world\n")},
		httpapi.NewRateLimiter(60, time.Minute),
		map[string]struct{}{"http://client.local": {}},
	)
}

func createSubtitle(t *testing.T, server http.Handler, subtitleURL string) {
	t.Helper()

	body := bytes.NewBufferString(`{"url":"` + subtitleURL + `"}`)
	request := httptest.NewRequest(http.MethodPost, "/subtitle", body)
	request.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("expected create subtitle status %d, got %d", http.StatusCreated, recorder.Code)
	}
}

func TestPostLegendaStoresSubtitleAndReturnsLocation(t *testing.T) {
	t.Parallel()
	server := newDefaultTestServer(t)

	body := bytes.NewBufferString(`{"url":"https://example.com/movie.vtt"}`)
	request := httptest.NewRequest(http.MethodPost, "/subtitle", body)
	request.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, recorder.Code)
	}

	var response struct {
		ID  string `json:"id"`
		URL string `json:"url"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected valid JSON response, got error: %v", err)
	}
	if response.ID == "" {
		t.Fatal("expected a generated subtitle id")
	}
	if response.URL != "https://example.com/movie.vtt" {
		t.Fatalf("expected original source url, got %q", response.URL)
	}
	if got := recorder.Header().Get("Location"); got == "" {
		t.Fatal("expected Location header to be set")
	}
}

func TestGetLegendaReturnsLatestStoredSubtitle(t *testing.T) {
	t.Parallel()
	server := newDefaultTestServer(t)
	createSubtitle(t, server, "https://example.com/movie.vtt")

	request := httptest.NewRequest(http.MethodGet, "/subtitle", nil)
	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}

	var response struct {
		ID  string `json:"id"`
		URL string `json:"url"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected valid JSON response, got error: %v", err)
	}
	if response.ID == "" || response.URL == "" {
		t.Fatal("expected subtitle id and url in response")
	}
	if response.URL != "https://example.com/movie.vtt" {
		t.Fatalf("expected original source url, got %q", response.URL)
	}
}

func TestPostLegendaRejectsUnsupportedSubtitleExtension(t *testing.T) {
	t.Parallel()
	server := newDefaultTestServer(t)

	body := bytes.NewBufferString(`{"url":"https://example.com/file.txt"}`)
	request := httptest.NewRequest(http.MethodPost, "/subtitle", body)
	request.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, recorder.Code)
	}
}

func TestPostLegendaRejectsMaliciousSubtitleContent(t *testing.T) {
	t.Parallel()
	server := newServer(300*1024, 10*time.Minute, stubFetcher{body: []byte("WEBVTT\n\n<script>alert('x')</script>\n")}, httpapi.NewRateLimiter(60, time.Minute), map[string]struct{}{"http://client.local": {}})

	body := bytes.NewBufferString(`{"url":"https://example.com/movie.vtt"}`)
	request := httptest.NewRequest(http.MethodPost, "/subtitle", body)
	request.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, recorder.Code)
	}
}

func TestPostLegendaAcceptsSRTAndKeepsSourceURL(t *testing.T) {
	t.Parallel()
	server := newServer(300*1024, 10*time.Minute, stubFetcher{body: []byte("1\n00:00:00,000 --> 00:00:01,500\nHello\n")}, httpapi.NewRateLimiter(60, time.Minute), map[string]struct{}{"http://client.local": {}})
	createSubtitle(t, server, "https://example.com/movie.srt")

	request := httptest.NewRequest(http.MethodGet, "/subtitle", nil)
	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, request)

	var response struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected valid JSON response, got error: %v", err)
	}
	if response.URL != "https://example.com/movie.srt" {
		t.Fatalf("expected original source url, got %q", response.URL)
	}
}

func TestPostLegendaRejectsEmptyFetchedContent(t *testing.T) {
	t.Parallel()
	server := newServer(300*1024, 10*time.Minute, stubFetcher{body: []byte("   ")}, httpapi.NewRateLimiter(60, time.Minute), map[string]struct{}{"http://client.local": {}})

	request := httptest.NewRequest(http.MethodPost, "/subtitle", bytes.NewBufferString(`{"url":"https://example.com/movie.srt"}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, recorder.Code)
	}
}

func TestGetLegendaReturnsNotFoundWhenCacheIsEmpty(t *testing.T) {
	t.Parallel()
	server := newDefaultTestServer(t)
	request := httptest.NewRequest(http.MethodGet, "/subtitle", nil)
	recorder := httptest.NewRecorder()

	server.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, recorder.Code)
	}
}

func TestGetLegendaReturnsNotFoundWhenLatestSubtitleExpired(t *testing.T) {
	t.Parallel()
	server := newServer(300*1024, time.Millisecond, stubFetcher{body: []byte("WEBVTT\n\n00:00:00.000 --> 00:00:02.000\nHello world\n")}, httpapi.NewRateLimiter(60, time.Minute), map[string]struct{}{"http://client.local": {}})
	createSubtitle(t, server, "https://example.com/movie.vtt")
	time.Sleep(5 * time.Millisecond)

	request := httptest.NewRequest(http.MethodGet, "/subtitle", nil)
	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, recorder.Code)
	}
}

func TestAllowedOriginReceivesCORSHeaders(t *testing.T) {
	t.Parallel()
	server := newDefaultTestServer(t)
	request := httptest.NewRequest(http.MethodOptions, "/subtitle", nil)
	request.Header.Set("Origin", "http://client.local")
	request.Header.Set("Access-Control-Request-Method", http.MethodPost)
	recorder := httptest.NewRecorder()

	server.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, recorder.Code)
	}
	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "http://client.local" {
		t.Fatalf("expected allowed origin header, got %q", got)
	}
}

func TestBlockedOriginDoesNotReceiveCORSHeaders(t *testing.T) {
	t.Parallel()
	server := newDefaultTestServer(t)
	request := httptest.NewRequest(http.MethodPost, "/subtitle", bytes.NewBufferString(`{"url":"https://example.com/movie.vtt"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", "http://blocked.local")
	recorder := httptest.NewRecorder()

	server.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, recorder.Code)
	}
}

func TestRateLimiterBlocksRequestsAfterBurst(t *testing.T) {
	t.Parallel()
	server := newServer(300*1024, 10*time.Minute, stubFetcher{body: []byte("WEBVTT\n\n00:00:00.000 --> 00:00:02.000\nHello world\n")}, httpapi.NewRateLimiter(2, time.Minute), map[string]struct{}{"http://client.local": {}})

	for attempt := 0; attempt < 2; attempt++ {
		request := httptest.NewRequest(http.MethodPost, "/subtitle", bytes.NewBufferString(`{"url":"https://example.com/movie.vtt"}`))
		request.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()
		server.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusCreated {
			t.Fatalf("expected pre-limit status %d, got %d", http.StatusCreated, recorder.Code)
		}
	}

	request := httptest.NewRequest(http.MethodPost, "/subtitle", bytes.NewBufferString(`{"url":"https://example.com/movie.vtt"}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("expected status %d, got %d", http.StatusTooManyRequests, recorder.Code)
	}
}

func TestHealthReturnsOK(t *testing.T) {
	t.Parallel()
	server := newDefaultTestServer(t)
	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	recorder := httptest.NewRecorder()

	server.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}

	var response struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected valid JSON response, got error: %v", err)
	}
	if response.Status != "ok" {
		t.Fatalf("expected status ok, got %q", response.Status)
	}
}

func TestPostLegendaFetchesSubtitleFromLocalHTTPSource(t *testing.T) {
	t.Parallel()

	upstream := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "text/plain")
		_, _ = response.Write([]byte("1\n00:00:00,000 --> 00:00:01,500\nHello from local source\n"))
	}))
	defer upstream.Close()

	server := newServer(300*1024, 10*time.Minute, infrastructure.HTTPFetcher{Client: &http.Client{Timeout: 10 * time.Second}}, httpapi.NewRateLimiter(60, time.Minute), map[string]struct{}{"http://client.local": {}})

	createSubtitle(t, server, upstream.URL+"/subtitle.srt")

	request := httptest.NewRequest(http.MethodGet, "/subtitle", nil)
	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}

	var response struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected valid JSON response, got error: %v", err)
	}
	if response.URL != upstream.URL+"/subtitle.srt" {
		t.Fatalf("expected original source URL, got %q", response.URL)
	}
}

func TestPostLegendaRejectsFilesLargerThanConfiguredLimit(t *testing.T) {
	t.Parallel()

	server := newServer(
		5,
		10*time.Minute,
		stubFetcher{body: bytes.Repeat([]byte("a"), 10)},
		httpapi.NewRateLimiter(60, time.Minute),
		map[string]struct{}{"http://client.local": {}},
	)

	request := httptest.NewRequest(http.MethodPost, "/subtitle", bytes.NewBufferString(`{"url":"https://example.com/movie.vtt"}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected status %d, got %d", http.StatusRequestEntityTooLarge, recorder.Code)
	}
}

func TestCreateWithFailingFetcherReturnsBadGateway(t *testing.T) {
	t.Parallel()

	server := newServer(
		300*1024,
		10*time.Minute,
		stubFetcher{err: errors.New("network failed")},
		httpapi.NewRateLimiter(60, time.Minute),
		map[string]struct{}{"http://client.local": {}},
	)

	request := httptest.NewRequest(http.MethodPost, "/subtitle", bytes.NewBufferString(`{"url":"https://example.com/movie.vtt"}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadGateway {
		t.Fatalf("expected status %d, got %d", http.StatusBadGateway, recorder.Code)
	}
}

func TestLatestWithMissingEntryReturnsNotFound(t *testing.T) {
	t.Parallel()

	server := newServer(
		300*1024,
		10*time.Minute,
		stubFetcher{body: []byte("WEBVTT\n\n00:00:00.000 --> 00:00:02.000\nHello\n")},
		httpapi.NewRateLimiter(60, time.Minute),
		map[string]struct{}{},
	)

	request := httptest.NewRequest(http.MethodGet, "/subtitle", nil)
	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, recorder.Code)
	}
}

func TestCreatesStableResponseShape(t *testing.T) {
	t.Parallel()

	server := newDefaultTestServer(t)
	request := httptest.NewRequest(http.MethodPost, "/subtitle", bytes.NewBufferString(`{"url":"https://example.com/movie.vtt"}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, recorder.Code)
	}

	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected valid JSON response, got error: %v", err)
	}
	if _, ok := response["id"]; !ok {
		t.Fatal("expected response to include id")
	}
	if _, ok := response["url"]; !ok {
		t.Fatal("expected response to include url")
	}
}
