package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleHealthReturnsOK(t *testing.T) {
	t.Parallel()

	controller := NewHTTPController(nil)
	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	recorder := httptest.NewRecorder()

	controller.HandleHealth(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}

	var body map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("expected valid JSON response, got %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("expected ok status, got %q", body["status"])
	}
}
