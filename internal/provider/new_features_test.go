package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// ── retry logic ───────────────────────────────────────────────────────────────

func TestClientRetry_RetriesOn503(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "service unavailable"})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "usr-1"})
	}))
	defer srv.Close()

	c := newClientWithOptions(srv.URL, "", srv.Client(), 2)
	result, err := c.get(context.Background(), "/api/v1/users/usr-1")
	if err != nil {
		t.Fatalf("expected success after retries, got: %v", err)
	}
	if result["id"] != "usr-1" {
		t.Errorf("unexpected result: %v", result)
	}
	if calls != 3 {
		t.Errorf("expected 3 calls (2 retries), got %d", calls)
	}
}

func TestClientRetry_NoRetryOn400(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": "bad request"})
	}))
	defer srv.Close()

	c := newClientWithOptions(srv.URL, "", srv.Client(), 3)
	_, err := c.get(context.Background(), "/path")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if calls != 1 {
		t.Errorf("expected no retries on 400, got %d calls", calls)
	}
}

func TestClientRetry_ExhaustsRetries(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusBadGateway)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": "bad gateway"})
	}))
	defer srv.Close()

	c := newClientWithOptions(srv.URL, "", srv.Client(), 2)
	_, err := c.get(context.Background(), "/path")
	if err == nil {
		t.Fatal("expected error after retries exhausted")
	}
	if calls != 3 {
		t.Errorf("expected 3 calls (initial + 2 retries), got %d", calls)
	}
}

// ── validator coverage ────────────────────────────────────────────────────────

func TestValidPriorities(t *testing.T) {
	for _, p := range validPriorities {
		if p == "" {
			t.Errorf("empty priority in list")
		}
	}
	if len(validPriorities) != 3 {
		t.Errorf("expected 3 priorities, got %d", len(validPriorities))
	}
}

func TestValidStepKinds_AllPresent(t *testing.T) {
	required := []string{"WAIT", "NOTIFY_USER", "NOTIFY_SCHEDULE", "NOTIFY_TEAM",
		"NOTIFY_EMERGENCY", "NOTIFY_DUTY_USERS", "TRIGGER_WEBHOOK", "CREATE_ISSUE", "RESOLVE", "REPEAT"}
	set := make(map[string]bool, len(validStepKinds))
	for _, k := range validStepKinds {
		set[k] = true
	}
	for _, r := range required {
		if !set[r] {
			t.Errorf("missing step kind: %s", r)
		}
	}
}

func TestValidIntegrationTypes_AllPresent(t *testing.T) {
	required := []string{"webhook", "alertmanager", "pagerduty", "victorops", "grafana-alerting"}
	set := make(map[string]bool, len(validIntegrationTypes))
	for _, k := range validIntegrationTypes {
		set[k] = true
	}
	for _, r := range required {
		if !set[r] {
			t.Errorf("missing integration type: %s", r)
		}
	}
}

// ── escalation chain: new step kinds ─────────────────────────────────────────

func TestEscalationChainModelFromAPI_AllStepKinds(t *testing.T) {
	m := map[string]any{
		"id":   "esc-all",
		"name": "All Kinds",
		"steps": []any{
			map[string]any{"id": "s1", "kind": "NOTIFY_EMERGENCY", "user_id": "usr-1"},
			map[string]any{"id": "s2", "kind": "NOTIFY_DUTY_USERS", "team_id": "team-1", "fallback_to_all": true},
			map[string]any{"id": "s3", "kind": "CREATE_ISSUE", "url": "https://redmine.example.com", "tracker_type": "redmine", "project": "ops"},
			map[string]any{"id": "s4", "kind": "RESOLVE"},
			map[string]any{"id": "s5", "kind": "REPEAT", "from_position": float64(0), "max_repeat_count": float64(3), "cooldown_minutes": float64(60)},
		},
	}
	model := escalationChainModelFromAPI(m)
	steps := model.Steps.Elements()
	if len(steps) != 5 {
		t.Fatalf("expected 5 steps, got %d", len(steps))
	}
	assertAttrStr(t, steps[0].(types.Object).Attributes(), "kind", "NOTIFY_EMERGENCY")
	assertAttrStr(t, steps[0].(types.Object).Attributes(), "user_id", "usr-1")

	assertAttrStr(t, steps[1].(types.Object).Attributes(), "kind", "NOTIFY_DUTY_USERS")

	assertAttrStr(t, steps[2].(types.Object).Attributes(), "kind", "CREATE_ISSUE")
	assertAttrStr(t, steps[2].(types.Object).Attributes(), "tracker_type", "redmine")

	assertAttrStr(t, steps[3].(types.Object).Attributes(), "kind", "RESOLVE")

	s5 := steps[4].(types.Object).Attributes()
	assertAttrStr(t, s5, "kind", "REPEAT")
	if v, ok := s5["max_repeat_count"].(types.Int64); !ok || v.ValueInt64() != 3 {
		t.Errorf("max_repeat_count should be 3")
	}
}

// ── integration: notification_policy and templates ────────────────────────────

func TestIntegrationModelFromAPI_WithPolicyAndTemplates(t *testing.T) {
	m := map[string]any{
		"id":   "int-pt",
		"name": "With Policy",
		"key":  "k-1",
		"notification_policy": map[string]any{
			"channels":               []any{"webhook", "telegram"},
			"batch_timeout_seconds":  float64(30),
			"batch_deadline_seconds": float64(60),
			"epic_threshold_count":   float64(10),
			"epic_threshold_seconds": float64(300),
			"emergency_user_id":      "usr-emergency",
			"epic_user_id":           "usr-epic",
		},
		"templates": map[string]any{
			"webhook": "{{ .Title }}",
			"email":   "Alert: {{ .Title }}",
		},
	}
	model := integrationModelFromAPI(m)

	if model.NotificationPolicy.IsNull() {
		t.Fatal("notification_policy should not be null")
	}
	pAttrs := model.NotificationPolicy.Attributes()
	if ch, ok := pAttrs["channels"].(types.List); !ok || len(ch.Elements()) != 2 {
		t.Errorf("channels should have 2 elements")
	}
	if v, ok := pAttrs["batch_timeout_seconds"].(types.Int64); !ok || v.ValueInt64() != 30 {
		t.Errorf("batch_timeout_seconds should be 30")
	}
	if s, ok := pAttrs["emergency_user_id"].(types.String); !ok || s.ValueString() != "usr-emergency" {
		t.Errorf("emergency_user_id should be usr-emergency")
	}

	if model.Templates.IsNull() {
		t.Fatal("templates should not be null")
	}
	tElems := model.Templates.Elements()
	if len(tElems) != 2 {
		t.Errorf("expected 2 templates, got %d", len(tElems))
	}
	if v, ok := tElems["webhook"].(types.String); !ok || v.ValueString() != "{{ .Title }}" {
		t.Errorf("webhook template mismatch")
	}
}

// ── provider config helpers ───────────────────────────────────────────────────

func TestEnvBool(t *testing.T) {
	t.Setenv("TEST_ENVBOOL", "true")
	if !envBool("TEST_ENVBOOL", false) {
		t.Error("expected true")
	}
	if !envBool("TEST_ENVBOOL_MISSING", true) {
		t.Error("expected default true for missing key")
	}
	t.Setenv("TEST_ENVBOOL_BAD", "notabool")
	if envBool("TEST_ENVBOOL_BAD", false) {
		t.Error("expected default false for bad value")
	}
}

func TestEnvInt64(t *testing.T) {
	t.Setenv("TEST_ENVINT64", "42")
	if envInt64("TEST_ENVINT64", 0) != 42 {
		t.Error("expected 42")
	}
	if envInt64("TEST_ENVINT64_MISSING", 99) != 99 {
		t.Error("expected default 99")
	}
}

// ── new data source helpers ───────────────────────────────────────────────────

func TestLookupByName_Found(t *testing.T) {
	items := []map[string]any{
		{"id": "1", "name": "Alpha"},
		{"id": "2", "name": "Beta"},
		{"id": "3", "name": "Gamma"},
	}
	result := lookupByName(items, "Beta")
	if result == nil {
		t.Fatal("expected to find Beta")
	}
	if result["id"] != "2" {
		t.Errorf("id = %v; want 2", result["id"])
	}
}

func TestLookupByName_NotFound(t *testing.T) {
	items := []map[string]any{{"id": "1", "name": "Alpha"}}
	result := lookupByName(items, "Missing")
	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

func TestLookupByName_EmptyList(t *testing.T) {
	result := lookupByName(nil, "anything")
	if result != nil {
		t.Error("expected nil for empty list")
	}
}

// ── scheduleItemFromAPIVal ────────────────────────────────────────────────────

func TestScheduleItemFromAPIVal(t *testing.T) {
	m := map[string]any{
		"id":       "sch-1",
		"name":     "Weekdays",
		"timezone": "UTC",
		"team_id":  "team-1",
		"shifts": []any{
			map[string]any{
				"id":         "shift-1",
				"user_id":    "usr-1",
				"start_at":   "2026-06-01T09:00:00Z",
				"end_at":     "2026-06-01T18:00:00Z",
				"recurrence": "weekly",
			},
		},
	}
	obj, diags := scheduleItemFromAPIVal(m)
	if diags.HasError() {
		t.Fatalf("unexpected diags: %v", diags)
	}
	attrs := obj.Attributes()
	if s, ok := attrs["name"].(types.String); !ok || s.ValueString() != "Weekdays" {
		t.Errorf("name should be Weekdays")
	}
	shifts := attrs["shifts"].(types.List)
	if len(shifts.Elements()) != 1 {
		t.Errorf("expected 1 shift, got %d", len(shifts.Elements()))
	}
}

func TestScheduleItemFromAPIVal_NoTeam(t *testing.T) {
	m := map[string]any{"id": "sch-2", "name": "No Team"}
	obj, diags := scheduleItemFromAPIVal(m)
	if diags.HasError() {
		t.Fatalf("unexpected diags: %v", diags)
	}
	teamID, ok := obj.Attributes()["team_id"].(types.String)
	if !ok {
		t.Fatal("team_id should be types.String")
	}
	if !teamID.IsNull() {
		t.Errorf("team_id should be null, got %q", teamID.ValueString())
	}
}

// ── escalationChainItemFromAPIVal ─────────────────────────────────────────────

func TestEscalationChainItemFromAPIVal(t *testing.T) {
	m := map[string]any{
		"id":   "esc-1",
		"name": "Critical",
		"steps": []any{
			map[string]any{"id": "s1", "kind": "WAIT", "delay_minutes": float64(5)},
		},
	}
	obj, diags := escalationChainItemFromAPIVal(m)
	if diags.HasError() {
		t.Fatalf("unexpected diags: %v", diags)
	}
	attrs := obj.Attributes()
	if s, ok := attrs["name"].(types.String); !ok || s.ValueString() != "Critical" {
		t.Errorf("name should be Critical")
	}
	steps := attrs["steps"].(types.List)
	if len(steps.Elements()) != 1 {
		t.Errorf("expected 1 step, got %d", len(steps.Elements()))
	}
}

// ── mock resource lifecycle ───────────────────────────────────────────────────

func TestUserResourceCreateSendsCorrectPayload(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":       "usr-new",
			"name":     gotBody["name"],
			"username": "auto",
			"priority": gotBody["priority"],
			"timezone": gotBody["timezone"],
		})
	}))
	defer srv.Close()

	c := newClient(srv.URL, "key")
	result, err := c.post(context.Background(), "/api/v1/users", map[string]any{
		"name":     "Alice",
		"priority": "high",
		"timezone": "UTC",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotBody["priority"] != "high" {
		t.Errorf("priority sent as %v; want high", gotBody["priority"])
	}
	if result["id"] != "usr-new" {
		t.Errorf("id = %v; want usr-new", result["id"])
	}
}

func TestIntegrationResourceCreate_RoutesPayload(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":     "int-new",
			"name":   gotBody["name"],
			"key":    "auto-key",
			"type":   gotBody["type"],
			"routes": gotBody["routes"],
		})
	}))
	defer srv.Close()

	c := newClient(srv.URL, "key")
	_, err := c.post(context.Background(), "/api/v1/integrations", map[string]any{
		"name": "Test",
		"type": "alertmanager",
		"routes": []any{
			map[string]any{
				"name":       "default",
				"match_type": "all",
				"is_default": true,
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	routes, ok := gotBody["routes"].([]any)
	if !ok || len(routes) != 1 {
		t.Fatalf("expected 1 route in payload, got %v", gotBody["routes"])
	}
	route := routes[0].(map[string]any)
	if route["match_type"] != "all" {
		t.Errorf("match_type = %v; want all", route["match_type"])
	}
	if route["is_default"] != true {
		t.Errorf("is_default = %v; want true", route["is_default"])
	}
}

// ── ImportState interface assertions ─────────────────────────────────────────

// Compile-time verification that all resources implement ResourceWithImportState.
// Adding these var _ lines causes a build error if any resource is missing the method.
var (
	_ resource.ResourceWithImportState = &UserResource{}
	_ resource.ResourceWithImportState = &TeamResource{}
	_ resource.ResourceWithImportState = &ScheduleResource{}
	_ resource.ResourceWithImportState = &EscalationChainResource{}
	_ resource.ResourceWithImportState = &IntegrationResource{}
	_ resource.ResourceWithImportState = &ChatopsChannelResource{}
)

// ── strDefault helper ─────────────────────────────────────────────────────────

func TestStrDefault(t *testing.T) {
	if strDefault("hello", "world") != "hello" {
		t.Error("expected non-empty string to be returned as-is")
	}
	if strDefault("", "world") != "world" {
		t.Error("expected default for empty string")
	}
	if strDefault("", "") != "" {
		t.Error("expected empty default")
	}
}

func TestClientWithCustomTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"ok":true}`)
	}))
	defer srv.Close()

	// Verify client can be created with custom options without panic.
	c := newClientWithOptions(srv.URL, "", srv.Client(), 0)
	if c == nil {
		t.Fatal("expected non-nil client")
	}
}
