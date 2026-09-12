package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	maxRetries int
}

func newClient(baseURL, apiKey string) *client {
	return newClientWithOptions(baseURL, apiKey, &http.Client{Timeout: 30 * time.Second}, 0)
}

func newClientWithOptions(baseURL, apiKey string, httpClient *http.Client, maxRetries int) *client {
	return &client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		apiKey:     apiKey,
		httpClient: httpClient,
		maxRetries: maxRetries,
	}
}

type apiError struct {
	Status  int
	Message string
}

func (e *apiError) Error() string {
	return fmt.Sprintf("API error %d: %s", e.Status, e.Message)
}

func isNotFound(err error) bool {
	if e, ok := err.(*apiError); ok {
		return e.Status == http.StatusNotFound
	}
	return false
}

func (c *client) do(ctx context.Context, method, path string, body any) (map[string]any, error) {
	var bodyData []byte
	if body != nil {
		var err error
		bodyData, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
	}

	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		result, err := c.doOnce(ctx, method, path, bodyData)
		if err == nil {
			return result, nil
		}
		if ae, ok := err.(*apiError); ok {
			// Retry only on transient server errors.
			if ae.Status != 502 && ae.Status != 503 && ae.Status != 504 {
				return nil, err
			}
		}
		lastErr = err
	}
	return nil, lastErr
}

func (c *client) doOnce(ctx context.Context, method, path string, bodyData []byte) (map[string]any, error) {
	var reqBody io.Reader
	if len(bodyData) > 0 {
		reqBody = bytes.NewReader(bodyData)
	}

	url := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Nxs-Anomaly-Provisioner", "terraform")
	if c.apiKey != "" {
		req.Header.Set("X-API-Key", c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode >= 400 {
		var errResp map[string]any
		_ = json.Unmarshal(respData, &errResp)
		msg := ""
		if m, ok := errResp["error"].(string); ok {
			msg = m
		} else {
			msg = string(respData)
		}
		return nil, &apiError{Status: resp.StatusCode, Message: msg}
	}

	if len(respData) == 0 || resp.StatusCode == http.StatusNoContent {
		return map[string]any{}, nil
	}

	var result map[string]any
	if err := json.Unmarshal(respData, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	return result, nil
}

func (c *client) get(ctx context.Context, path string) (map[string]any, error) {
	return c.do(ctx, http.MethodGet, path, nil)
}

func (c *client) post(ctx context.Context, path string, body any) (map[string]any, error) {
	return c.do(ctx, http.MethodPost, path, body)
}

func (c *client) put(ctx context.Context, path string, body any) (map[string]any, error) {
	return c.do(ctx, http.MethodPut, path, body)
}

func (c *client) delete(ctx context.Context, path string) error {
	_, err := c.do(ctx, http.MethodDelete, path, nil)
	return err
}

func strFromMap(m map[string]any, key string) string {
	if v, ok := m[key]; ok && v != nil {
		return fmt.Sprintf("%v", v)
	}
	return ""
}

func boolFromMap(m map[string]any, key string, def bool) bool {
	if v, ok := m[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return def
}

func int64FromMap(m map[string]any, key string, def int64) int64 {
	if v, ok := m[key]; ok {
		switch t := v.(type) {
		case float64:
			return int64(t)
		case int64:
			return t
		case int:
			return int64(t)
		}
	}
	return def
}

func float64FromMap(m map[string]any, key string, def float64) float64 {
	if v, ok := m[key]; ok {
		if f, ok := v.(float64); ok {
			return f
		}
	}
	return def
}

func stringSliceFromMap(m map[string]any, key string) []string {
	v, ok := m[key]
	if !ok || v == nil {
		return nil
	}
	list, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(list))
	for _, item := range list {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func strDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

// listAll fetches every item from a paginated collection endpoint.
func (c *client) listAll(ctx context.Context, path string) ([]map[string]any, error) {
	const pageSize = 100
	var all []map[string]any
	for offset := 0; ; offset += pageSize {
		result, err := c.get(ctx, fmt.Sprintf("%s?limit=%d&offset=%d", path, pageSize, offset))
		if err != nil {
			return nil, err
		}
		items := mapSliceFromMap(result, "items")
		all = append(all, items...)
		total := int(int64FromMap(result, "total", 0))
		if len(all) >= total || len(items) == 0 {
			break
		}
	}
	return all, nil
}

func mapSliceFromMap(m map[string]any, key string) []map[string]any {
	v, ok := m[key]
	if !ok || v == nil {
		return nil
	}
	list, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]map[string]any, 0, len(list))
	for _, item := range list {
		if mp, ok := item.(map[string]any); ok {
			out = append(out, mp)
		}
	}
	return out
}
