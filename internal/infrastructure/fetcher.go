package infrastructure

import (
	"context"
	"io"
	"net/http"
	"time"

	service "subtitle-delivery/internal/service"
)

type HTTPFetcher struct {
	Client *http.Client
}

func NewHTTPFetcher(timeout time.Duration) HTTPFetcher {
	return HTTPFetcher{Client: &http.Client{Timeout: timeout}}
}

func (fetcher HTTPFetcher) Fetch(ctx context.Context, targetURL string, maxBytes int64) ([]byte, error) {
	client := fetcher.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, err
	}

	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	limitedBody := io.LimitReader(response.Body, maxBytes+1)
	body, err := io.ReadAll(limitedBody)
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > maxBytes {
		return nil, service.ErrSubtitleTooLarge
	}
	return body, nil
}
