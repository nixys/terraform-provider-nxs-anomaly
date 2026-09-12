package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// ── listAll ───────────────────────────────────────────────────────────────────

func TestListAll_SinglePage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		limit := r.URL.Query().Get("limit")
		offset := r.URL.Query().Get("offset")
		if limit != "100" || offset != "0" {
			t.Errorf("unexpected query: limit=%s offset=%s", limit, offset)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items": []any{
				map[string]any{"id": "usr-1", "name": "Alice"},
				map[string]any{"id": "usr-2", "name": "Bob"},
			},
			"total":  2,
			"limit":  100,
			"offset": 0,
		})
	}))
	defer srv.Close()

	c := newClient(srv.URL, "")
	items, err := c.listAll(context.Background(), "/api/v1/users")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0]["id"] != "usr-1" {
		t.Errorf("first item id = %v; want usr-1", items[0]["id"])
	}
}

func TestListAll_MultiPage(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		offset := r.URL.Query().Get("offset")
		var items []any
		switch offset {
		case "0":
			for i := 0; i < 100; i++ {
				items = append(items, map[string]any{"id": i})
			}
		case "100":
			for i := 100; i < 150; i++ {
				items = append(items, map[string]any{"id": i})
			}
		default:
			items = []any{}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items":  items,
			"total":  150,
			"limit":  100,
			"offset": offset,
		})
	}))
	defer srv.Close()

	c := newClient(srv.URL, "")
	items, err := c.listAll(context.Background(), "/api/v1/users")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 150 {
		t.Errorf("expected 150 items, got %d", len(items))
	}
	if calls != 2 {
		t.Errorf("expected 2 HTTP calls, got %d", calls)
	}
}

func TestListAll_Empty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items": []any{},
			"total": 0,
		})
	}))
	defer srv.Close()

	c := newClient(srv.URL, "")
	items, err := c.listAll(context.Background(), "/api/v1/users")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 items, got %d", len(items))
	}
}

func TestListAll_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": "internal error"})
	}))
	defer srv.Close()

	c := newClient(srv.URL, "")
	_, err := c.listAll(context.Background(), "/api/v1/users")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ── userItemFromAPIVal ────────────────────────────────────────────────────────

func TestUserItemFromAPIVal_Basic(t *testing.T) {
	m := map[string]any{
		"id":                   "usr-1",
		"name":                 "Alice",
		"username":             "alice",
		"email":                "alice@example.com",
		"phone":                "",
		"telegram_id":          "",
		"timezone":             "UTC",
		"on_duty":              false,
		"priority":             float64(0),
		"notification_targets": []any{},
		"created_at":           "2026-01-01T00:00:00Z",
		"updated_at":           "2026-01-01T00:00:00Z",
	}
	obj, diags := userItemFromAPIVal(m)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	attrs := obj.Attributes()
	if s, ok := attrs["id"].(types.String); !ok || s.ValueString() != "usr-1" {
		t.Errorf("id should be usr-1")
	}
	if s, ok := attrs["name"].(types.String); !ok || s.ValueString() != "Alice" {
		t.Errorf("name should be Alice")
	}
	targets, ok := attrs["notification_targets"].(types.List)
	if !ok {
		t.Fatal("notification_targets should be types.List")
	}
	if len(targets.Elements()) != 0 {
		t.Errorf("expected 0 notification_targets, got %d", len(targets.Elements()))
	}
}

func TestUserItemFromAPIVal_WithTargets(t *testing.T) {
	m := map[string]any{
		"id":   "usr-2",
		"name": "Bob",
		"notification_targets": []any{
			map[string]any{"channel": "telegram", "target": "99999"},
		},
	}
	obj, diags := userItemFromAPIVal(m)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	targets := obj.Attributes()["notification_targets"].(types.List)
	if len(targets.Elements()) != 1 {
		t.Fatalf("expected 1 target, got %d", len(targets.Elements()))
	}
	tAttrs := targets.Elements()[0].(types.Object).Attributes()
	if s, ok := tAttrs["channel"].(types.String); !ok || s.ValueString() != "telegram" {
		t.Errorf("channel should be telegram")
	}
}

// ── teamItemFromAPIVal ────────────────────────────────────────────────────────

func TestTeamItemFromAPIVal(t *testing.T) {
	m := map[string]any{
		"id":         "team-1",
		"name":       "Ops",
		"member_ids": []any{"usr-1", "usr-2"},
		"created_at": "2026-01-01T00:00:00Z",
		"updated_at": "2026-01-01T00:00:00Z",
	}
	obj, diags := teamItemFromAPIVal(m)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	attrs := obj.Attributes()
	if s, ok := attrs["id"].(types.String); !ok || s.ValueString() != "team-1" {
		t.Errorf("id should be team-1")
	}
	members := attrs["member_ids"].(types.List)
	if len(members.Elements()) != 2 {
		t.Errorf("expected 2 members, got %d", len(members.Elements()))
	}
}

func TestTeamItemFromAPIVal_NoMembers(t *testing.T) {
	m := map[string]any{"id": "team-2", "name": "Empty"}
	obj, diags := teamItemFromAPIVal(m)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	members := obj.Attributes()["member_ids"].(types.List)
	if len(members.Elements()) != 0 {
		t.Errorf("expected 0 members, got %d", len(members.Elements()))
	}
}

// ── integrationItemFromAPIVal ─────────────────────────────────────────────────

func TestIntegrationItemFromAPIVal_WithRoutes(t *testing.T) {
	m := map[string]any{
		"id":          "int-1",
		"name":        "Prometheus",
		"key":         "key-abc",
		"type":        "alertmanager",
		"source_type": "alertmanager",
		"group_by":    []any{"alertname"},
		"routes": []any{
			map[string]any{
				"id":                  "route-1",
				"name":                "labels-route",
				"match_type":          "labels",
				"is_default":          false,
				"labels":              map[string]any{"severity": "critical"},
				"escalation_chain_id": "esc-1",
			},
			map[string]any{
				"id":         "route-2",
				"name":       "default",
				"match_type": "all",
				"is_default": true,
			},
		},
		"created_at": "2026-01-01T00:00:00Z",
		"updated_at": "2026-01-01T00:00:00Z",
	}
	obj, diags := integrationItemFromAPIVal(m)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	attrs := obj.Attributes()
	if s, ok := attrs["key"].(types.String); !ok || s.ValueString() != "key-abc" {
		t.Errorf("key should be key-abc")
	}

	routes := attrs["routes"].(types.List)
	if len(routes.Elements()) != 2 {
		t.Fatalf("expected 2 routes, got %d", len(routes.Elements()))
	}
	route0Attrs := routes.Elements()[0].(types.Object).Attributes()
	if s, ok := route0Attrs["match_type"].(types.String); !ok || s.ValueString() != "labels" {
		t.Errorf("first route match_type should be labels")
	}
	labelsMap := route0Attrs["labels"].(types.Map)
	if v, ok := labelsMap.Elements()["severity"].(types.String); !ok || v.ValueString() != "critical" {
		t.Errorf("labels[severity] should be critical")
	}

	route1Attrs := routes.Elements()[1].(types.Object).Attributes()
	if s, ok := route1Attrs["match_type"].(types.String); !ok || s.ValueString() != "all" {
		t.Errorf("second route match_type should be all")
	}
	if v, ok := route1Attrs["is_default"].(types.Bool); !ok || !v.ValueBool() {
		t.Errorf("second route is_default should be true")
	}
}

func TestIntegrationItemFromAPIVal_Empty(t *testing.T) {
	m := map[string]any{"id": "int-2", "name": "Simple", "key": "k"}
	obj, diags := integrationItemFromAPIVal(m)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	attrs := obj.Attributes()
	routes := attrs["routes"].(types.List)
	if len(routes.Elements()) != 0 {
		t.Errorf("expected 0 routes, got %d", len(routes.Elements()))
	}
	groupBy := attrs["group_by"].(types.List)
	if len(groupBy.Elements()) != 0 {
		t.Errorf("expected 0 group_by, got %d", len(groupBy.Elements()))
	}
}

// ── type filter ───────────────────────────────────────────────────────────────

func TestIntegrationsDataSource_TypeFilter(t *testing.T) {
	// Simulate filtering: 3 items, only 2 match type "webhook".
	all := []map[string]any{
		{"id": "i1", "name": "A", "type": "webhook"},
		{"id": "i2", "name": "B", "type": "alertmanager"},
		{"id": "i3", "name": "C", "type": "webhook"},
	}
	filter := "webhook"
	var matched []map[string]any
	for _, m := range all {
		if strFromMap(m, "type") == filter {
			matched = append(matched, m)
		}
	}
	if len(matched) != 2 {
		t.Errorf("expected 2 webhook integrations, got %d", len(matched))
	}
	for _, m := range matched {
		if strFromMap(m, "type") != "webhook" {
			t.Errorf("non-webhook item in filtered result: %v", m)
		}
	}
}
