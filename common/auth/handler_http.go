package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type httpHandler struct {
	url     string
	client  *http.Client
	headers map[string]string
}

// NewHTTPHandler creates a new HTTP handler with the specified URL and optional headers
func NewHTTPHandler(url string, headers map[string]string) HandleFunc {
	if headers == nil {
		headers = make(map[string]string)
	}

	return &httpHandler{
		url:     url,
		client:  &http.Client{Timeout: 30 * time.Second},
		headers: headers,
	}
}

func (h *httpHandler) Do(ctx context.Context, au AuthorizedHandler, send ...map[string]any) (map[string]any, error) {
	// Authenticate the payload
	authData, err := au.Make(ctx, send...)
	if err != nil {
		return nil, fmt.Errorf("authentication failed: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.url, bytes.NewReader(authData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/octet-stream")
	for k, v := range h.headers {
		req.Header.Set(k, v)
	}

	// Send the request
	resp, err := h.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Read and parse the response
	var respData []byte
	if resp.ContentLength > 0 {
		respData = make([]byte, resp.ContentLength)
		_, err = resp.Body.Read(respData)
	} else {
		respData, err = io.ReadAll(resp.Body)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	res := map[string]any{}
	err = json.Unmarshal(respData, &res)
	return res, err
}
