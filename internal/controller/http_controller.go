package controller

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	domain "subtitle-delivery/internal/domain"
	service "subtitle-delivery/internal/service"
)

type HTTPController struct {
	subtitleService SubtitleService
}

type SubtitleService interface {
	CreateSubtitle(ctx context.Context, sourceURL string) (service.CreateResult, error)
	LatestSubtitle(ctx context.Context) (domain.Subtitle, error)
}

func NewHTTPController(subtitleService SubtitleService) *HTTPController {
	return &HTTPController{subtitleService: subtitleService}
}

func (controller *HTTPController) HandleHealth(response http.ResponseWriter, _ *http.Request) {
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(response).Encode(map[string]string{"status": "ok"})
}

func (controller *HTTPController) HandleCreateSubtitle(response http.ResponseWriter, request *http.Request) {
	ctx := request.Context()

	var payload struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
		writeJSONError(response, http.StatusBadRequest, "corpo da requisicao invalido")
		return
	}

	result, err := controller.subtitleService.CreateSubtitle(ctx, payload.URL)
	if err != nil {
		writeJSONError(response, mapErrorToStatus(err), mapErrorToMessage(err))
		return
	}

	response.Header().Set("Content-Type", "application/json")
	response.Header().Set("Location", result.URL)
	response.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(response).Encode(map[string]string{
		"id":  result.ID,
		"url": result.URL,
	})
}

func (controller *HTTPController) HandleGetLatestSubtitle(response http.ResponseWriter, request *http.Request) {
	ctx := request.Context()

	record, err := controller.subtitleService.LatestSubtitle(ctx)
	if err != nil {
		writeJSONError(response, http.StatusNotFound, "nenhuma legenda valida disponivel")
		return
	}

	response.Header().Set("Content-Type", "text/vtt")
	response.WriteHeader(http.StatusOK)
	_, _ = response.Write([]byte(record.Content))
}

func mapErrorToStatus(err error) int {
	switch {
	case errors.Is(err, domain.ErrInvalidURL), errors.Is(err, domain.ErrUnsupportedFormat), errors.Is(err, domain.ErrInvalidContentFormat), errors.Is(err, domain.ErrMaliciousContent), errors.Is(err, domain.ErrEmptyContent):
		return http.StatusBadRequest
	case errors.Is(err, service.ErrSubtitleTooLarge):
		return http.StatusRequestEntityTooLarge
	default:
		return http.StatusBadGateway
	}
}

func mapErrorToMessage(err error) string {
	switch {
	case errors.Is(err, domain.ErrInvalidURL), errors.Is(err, domain.ErrUnsupportedFormat), errors.Is(err, domain.ErrInvalidContentFormat), errors.Is(err, domain.ErrMaliciousContent), errors.Is(err, domain.ErrEmptyContent), errors.Is(err, service.ErrSubtitleTooLarge):
		return err.Error()
	default:
		return "falha ao buscar legenda"
	}
}

func writeJSONError(response http.ResponseWriter, statusCode int, message string) {
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(statusCode)
	_ = json.NewEncoder(response).Encode(map[string]string{"error": message})
}
