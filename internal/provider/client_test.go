package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// ── strFromMap ────────────────────────────────────────────────────────────────

func TestStrFromMap(t *testing.T) {
	tests := []struct {
		name string
		m    map[string]any
		key  string
		want string
	}{
		{"string value", map[string]any{"k": "hello"}, "k", "hello"},
		{"int converted", map[string]any{"k": 42}, "k", "42"},
		{"nil value", map[string]any{"k": nil}, "k", ""},
		{"missing key", map[string]any{"other": "x"}, "k", ""},
		{"empty map", map[string]any{}, "k", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := strFromMap(tc.m, tc.key)
			if got != tc.want {
				t.Errorf("strFromMap(%q) = %q; want %q", tc.key, got, tc.want)
			}
		})
	}
}

// ── boolFromMap ───────────────────────────────────────────────────────────────

func TestBoolFromMap(t *testing.T) {
	tests := []struct {
		name string
		m    map[string]any
		key  string
		def  bool
		want bool
	}{
		{"true", map[string]any{"k": true}, "k", false, true},
		{"false", map[string]any{"k": false}, "k", true, false},
		{"non-bool uses default", map[string]any{"k": "true"}, "k", false, false},
		{"missing uses default true", map[string]any{}, "k", true, true},
		{"missing uses default false", map[string]any{}, "k", false, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := boolFromMap(tc.m, tc.key, tc.def)
			if got != tc.want {
				t.Errorf("boolFromMap(%q) = %v; want %v", tc.key, got, tc.want)
			}
		})
	}
}

// ── int64FromMap ──────────────────────────────────────────────────────────────

func TestInt64FromMap(t *testing.T) {
	tests := []struct {
		name string
		m    map[string]any
		key  string
		def  int64
		want int64
	}{
		{"float64", map[string]any{"k": float64(7)}, "k", 0, 7},
		{"int64", map[string]any{"k": int64(99)}, "k", 0, 99},
		{"int", map[string]any{"k": int(5)}, "k", 0, 5},
		{"string uses default", map[string]any{"k": "7"}, "k", 3, 3},
		{"missing uses default", map[string]any{}, "k", 42, 42},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := int64FromMap(tc.m, tc.key, tc.def)
			if got != tc.want {
				t.Errorf("int64FromMap(%q) = %d; want %d", tc.key, got, tc.want)
			}
		})
	}
}

// ── stringSliceFromMap ────────────────────────────────────────────────────────

func TestStringSliceFromMap(t *testing.T) {
	t.Run("normal slice", func(t *testing.T) {
		m := map[string]any{"ids": []any{"a", "b", "c"}}
		got := stringSliceFromMap(m, "ids")
		if len(got) != 3 || got[0] != "a" || got[2] != "c" {
			t.Errorf("unexpected result: %v", got)
		}
	})
	t.Run("nil value", func(t *testing.T) {
		m := map[string]any{"ids": nil}
		got := stringSliceFromMap(m, "ids")
		if got != nil {
			t.Errorf("expected nil, got %v", got)
		}
	})
	t.Run("missing key", func(t *testing.T) {
		got := stringSliceFromMap(map[string]any{}, "ids")
		if got != nil {
			t.Errorf("expected nil, got %v", got)
		}
	})
	t.Run("non-string items skipped", func(t *testing.T) {
		m := map[string]any{"ids": []any{"x", 123, "y"}}
		got := stringSliceFromMap(m, "ids")
		if len(got) != 2 || got[0] != "x" || got[1] != "y" {
			t.Errorf("unexpected result: %v", got)
		}
	})
	t.Run("empty slice", func(t *testing.T) {
		m := map[string]any{"ids": []any{}}
		got := stringSliceFromMap(m, "ids")
		if len(got) != 0 {
			t.Errorf("expected empty, got %v", got)
		}
	})
}

// ── mapSliceFromMap ───────────────────────────────────────────────────────────

func TestMapSliceFromMap(t *testing.T) {
	t.Run("normal", func(t *testing.T) {
		m := map[string]any{
			"items": []any{
				map[string]any{"a": "1"},
				map[string]any{"b": "2"},
			},
		}
		got := mapSliceFromMap(m, "items")
		if len(got) != 2 {
			t.Errorf("expected 2 items, got %d", len(got))
		}
		if got[0]["a"] != "1" {
			t.Errorf("unexpected first item: %v", got[0])
		}
	})
	t.Run("nil value", func(t *testing.T) {
		m := map[string]any{"items": nil}
		got := mapSliceFromMap(m, "items")
		if got != nil {
			t.Errorf("expected nil, got %v", got)
		}
	})
	t.Run("non-map items skipped", func(t *testing.T) {
		m := map[string]any{
			"items": []any{
				map[string]any{"ok": true},
				"not-a-map",
				42,
			},
		}
		got := mapSliceFromMap(m, "items")
		if len(got) != 1 {
			t.Errorf("expected 1 item, got %d", len(got))
		}
	})
}

// ── isNotFound ────────────────────────────────────────────────────────────────

func TestIsNotFound(t *testing.T) {
	t.Run("404 apiError", func(t *testing.T) {
		if !isNotFound(&apiError{Status: 404, Message: "not found"}) {
			t.Error("expected true for 404 apiError")
		}
	})
	t.Run("500 apiError", func(t *testing.T) {
		if isNotFound(&apiError{Status: 500, Message: "internal"}) {
			t.Error("expected false for 500 apiError")
		}
	})
	t.Run("generic error", func(t *testing.T) {
		if isNotFound(fmt.Errorf("something went wrong")) {
			t.Error("expected false for generic error")
		}
	})
}

// ── apiError ──────────────────────────────────────────────────────────────────

func TestAPIErrorMessage(t *testing.T) {
	err := &apiError{Status: 422, Message: "validation failed"}
	want := "API error 422: validation failed"
	if err.Error() != want {
		t.Errorf("got %q; want %q", err.Error(), want)
	}
}

// ── HTTP client (mock server) ─────────────────────────────────────────────────

func newMockServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *client) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv, newClient(srv.URL, "test-key")
}

func TestClientGet_Success(t *testing.T) {
	srv, c := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.Header.Get("X-API-Key") != "test-key" {
			t.Errorf("missing X-API-Key header")
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "usr-1", "name": "Alice"})
	})
	_ = srv

	result, err := c.get(context.Background(), "/api/v1/users/usr-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["id"] != "usr-1" {
		t.Errorf("id = %v; want usr-1", result["id"])
	}
}

func TestClientGet_NotFound(t *testing.T) {
	_, c := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": "user not found"})
	})

	_, err := c.get(context.Background(), "/api/v1/users/missing")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !isNotFound(err) {
		t.Errorf("expected 404 error, got: %v", err)
	}
}

func TestClientPost_Success(t *testing.T) {
	var gotBody map[string]any
	_, c := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "usr-new", "name": gotBody["name"]})
	})

	result, err := c.post(context.Background(), "/api/v1/users", map[string]any{"name": "Bob"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["id"] != "usr-new" {
		t.Errorf("id = %v; want usr-new", result["id"])
	}
	if gotBody["name"] != "Bob" {
		t.Errorf("request body name = %v; want Bob", gotBody["name"])
	}
}

func TestClientPut_Success(t *testing.T) {
	var gotBody map[string]any
	_, c := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("expected PUT, got %s", r.Method)
		}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "usr-1", "name": gotBody["name"]})
	})

	result, err := c.put(context.Background(), "/api/v1/users/usr-1", map[string]any{"name": "Alice Updated"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotBody["name"] != "Alice Updated" {
		t.Errorf("request body name = %v; want Alice Updated", gotBody["name"])
	}
	_ = result
}

func TestClientDelete_Success(t *testing.T) {
	called := false
	_, c := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		called = true
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"deleted": true})
	})

	err := c.delete(context.Background(), "/api/v1/users/usr-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("handler not called")
	}
}

func TestClientDelete_NotFound(t *testing.T) {
	_, c := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": "not found"})
	})

	err := c.delete(context.Background(), "/api/v1/users/gone")
	if err == nil {
		t.Fatal("expected 404 error, got nil")
	}
	if !isNotFound(err) {
		t.Errorf("expected isNotFound=true, got false; err=%v", err)
	}
}

func TestClientDo_ServerError(t *testing.T) {
	_, c := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": "internal server error"})
	})

	_, err := c.get(context.Background(), "/api/v1/users")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	ae, ok := err.(*apiError)
	if !ok {
		t.Fatalf("expected *apiError, got %T", err)
	}
	if ae.Status != 500 {
		t.Errorf("expected status 500, got %d", ae.Status)
	}
	if ae.Message != "internal server error" {
		t.Errorf("unexpected message: %q", ae.Message)
	}
}

func TestClientDo_EmptyResponse(t *testing.T) {
	_, c := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	result, err := c.get(context.Background(), "/path")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty map, got %v", result)
	}
}

func TestClientDo_InvalidJSON(t *testing.T) {
	_, c := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("not json at all"))
	})

	_, err := c.get(context.Background(), "/path")
	if err == nil {
		t.Fatal("expected unmarshal error, got nil")
	}
}

func TestClientAPIKeyInHeader(t *testing.T) {
	var gotKey string
	_, c := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("X-API-Key")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{})
	})

	c.apiKey = "super-secret"
	_, _ = c.get(context.Background(), "/path")
	if gotKey != "super-secret" {
		t.Errorf("X-API-Key = %q; want %q", gotKey, "super-secret")
	}
}

func TestClientNoAPIKey(t *testing.T) {
	var gotKey string
	_, c := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("X-API-Key")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{})
	})

	c.apiKey = ""
	_, _ = c.get(context.Background(), "/path")
	if gotKey != "" {
		t.Errorf("expected empty X-API-Key, got %q", gotKey)
	}
}

func TestClientDeclaresTerraformProvisioner(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-Nxs-Anomaly-Provisioner"); got != "terraform" {
			t.Errorf("provisioner = %q, want terraform", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items": [], "total": 0}`))
	}))
	defer srv.Close()
	if _, err := newClient(srv.URL, "test-key").get(context.Background(), "/api/v1/users"); err != nil {
		t.Fatal(err)
	}
}
