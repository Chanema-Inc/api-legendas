package httpapi

import (
	"net/http"

	controller "subtitle-delivery/internal/controller"
)

func RegisterRoutes(mux *http.ServeMux, subtitleController *controller.HTTPController) {
	mux.HandleFunc("/health", func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			response.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		subtitleController.HandleHealth(response, request)
	})

	mux.HandleFunc("/legenda", func(response http.ResponseWriter, request *http.Request) {
		switch request.Method {
		case http.MethodPost:
			subtitleController.HandleCreateSubtitle(response, request)
		case http.MethodGet:
			subtitleController.HandleGetLatestSubtitle(response, request)
		default:
			response.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
}
