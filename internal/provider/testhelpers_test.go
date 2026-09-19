package provider_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	// Import provider package to access exported client constructor.
	// The client is package-private, so we duplicate a minimal version here.
	"net/http"
	"time"
)

// testCtx returns a background context for use in acceptance test helpers.
func testCtx(t *testing.T) context.Context {
	t.Helper()
	return context.Background()
}

// testHTTPClient is a minimal REST client used inside acceptance test helpers
// to perform out-of-band operations (e.g. deleting a resource externally).
type testHTTPClient struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

func newTestClient(t *testing.T) *testHTTPClient {
	t.Helper()
	url := os.Getenv("NXS_ANOMALY_URL")
	if url == "" {
		url = "http://localhost:8080"
	}
	return &testHTTPClient{
		baseURL: url,
		apiKey:  os.Getenv("NXS_ANOMALY_API_KEY"),
		http:    &http.Client{Timeout: 10 * time.Second},
	}
}

// delete performs an out-of-band deletion by another Terraform client.
func (c *testHTTPClient) delete(ctx context.Context, path string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-Nxs-Anomaly-Provisioner", "terraform")
	if c.apiKey != "" {
		req.Header.Set("X-API-Key", c.apiKey)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("external deletion failed: HTTP %d", resp.StatusCode)
	}
	return nil
}
