// Package tools_test pins the curated log group. Each test drives a real
// mcpkit server over the SDK's in-memory transport pair against a real fake
// controller (no mocks of the client or the transport).
package tools_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/RonaldPhilipsen/omada-mcp/internal/fakectrl"
	"github.com/RonaldPhilipsen/omada-mcp/internal/mcpkit"
	"github.com/RonaldPhilipsen/omada-mcp/internal/tools"
)

func allLogTools(b *tools.Base) []mcpkit.Tool {
	return []mcpkit.Tool{b.LogAlerts(), b.LogEvents()}
}

// TestLogAlertsRoundTrip pins omada_log_alerts: the required site_id is used
// in the request path, the paging and time range are shipped as the query,
// and the decoded alerts come back in the JSONResult content.
func TestLogAlertsRoundTrip(t *testing.T) {
	b, fake := newBase(t)
	mux := fake.Handler().(*http.ServeMux)
	mux.Handle("GET /openapi/v1/{omadacId}/sites/{siteId}/logs/alerts", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fakectrl.EnvelopeJSON(w, 0, map[string]any{
			"data": []map[string]any{
				{"id": "alert-1", "level": 1, "message": "Device offline"},
				{"id": "alert-2", "level": 2, "message": "High CPU"},
			},
		})
	}))

	cs := newSession(t, mcpkit.Options{ReadOnly: false}, allLogTools(b)...)
	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "omada_log_alerts",
		Arguments: map[string]any{
			"omadac_id":  "omac-456",
			"site_id":    "site-1",
			"page_size":  10,
			"start_time": int64(1700000000000),
			"end_time":   int64(1700003600000),
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	text, isError := toolText(result)
	if isError {
		t.Fatalf("omada_log_alerts errored: %s", text)
	}
	var got struct {
		Data []struct {
			ID      string `json:"id"`
			Level   int    `json:"level"`
			Message string `json:"message"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(text), &got); err != nil {
		t.Fatalf("content %q is not JSON: %v", text, err)
	}
	if len(got.Data) != 2 || got.Data[0].Message != "Device offline" || got.Data[1].Message != "High CPU" {
		t.Errorf("decoded rows = %+v, want two rows Device offline/High CPU", got.Data)
	}

	req := entryFor(t, fake.Requests(), "/openapi/v1/omac-456/sites/site-1/logs/alerts")
	if req.Method != http.MethodGet {
		t.Errorf("method = %q, want GET", req.Method)
	}
	if req.Query != "page=1&pageSize=10&filters.timeStart=1700000000000&filters.timeEnd=1700003600000" {
		t.Errorf("query = %q, want page=1&pageSize=10 plus the time range", req.Query)
	}
}

// TestLogEventsRoundTrip pins omada_log_events: the required site_id is used
// in the request path, the paging and time range are shipped as the query,
// and the decoded events come back in the content.
func TestLogEventsRoundTrip(t *testing.T) {
	b, fake := newBase(t)
	mux := fake.Handler().(*http.ServeMux)
	mux.Handle("GET /openapi/v1/{omadacId}/sites/{siteId}/logs/events", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fakectrl.EnvelopeJSON(w, 0, map[string]any{
			"data": []map[string]any{
				{"id": "event-1", "message": "User logged in"},
			},
		})
	}))

	cs := newSession(t, mcpkit.Options{ReadOnly: false}, allLogTools(b)...)
	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "omada_log_events",
		Arguments: map[string]any{
			"site_id":    "site-1",
			"start_time": int64(1700000000000),
			"end_time":   int64(1700003600000),
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	text, isError := toolText(result)
	if isError {
		t.Fatalf("omada_log_events errored: %s", text)
	}
	var got struct {
		Data []struct {
			ID      string `json:"id"`
			Message string `json:"message"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(text), &got); err != nil {
		t.Fatalf("content %q is not JSON: %v", text, err)
	}
	if len(got.Data) != 1 || got.Data[0].Message != "User logged in" {
		t.Errorf("decoded rows = %+v, want one row User logged in", got.Data)
	}

	req := entryFor(t, fake.Requests(), "/openapi/v1/"+omadacConfigured+"/sites/site-1/logs/events")
	if req.Method != http.MethodGet {
		t.Errorf("method = %q, want GET", req.Method)
	}
	if req.Query != "page=1&pageSize=20&filters.timeStart=1700000000000&filters.timeEnd=1700003600000" {
		t.Errorf("query = %q, want default page=1&pageSize=20 plus the time range", req.Query)
	}
}

// TestLogBusinessError pins the envelope business-error path: a non-zero
// errorCode surfaces as a tool error carrying the APIError message, for two
// read tools.
func TestLogBusinessError(t *testing.T) {
	b, fake := newBase(t)
	mux := fake.Handler().(*http.ServeMux)
	mux.Handle("GET /openapi/v1/{omadacId}/sites/{siteId}/logs/alerts", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fakectrl.EnvelopeJSON(w, -1171, nil)
	}))
	mux.Handle("GET /openapi/v1/{omadacId}/sites/{siteId}/logs/events", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fakectrl.EnvelopeJSON(w, -1172, nil)
	}))

	cs := newSession(t, mcpkit.Options{ReadOnly: false}, allLogTools(b)...)

	cases := []struct {
		name   string
		args   map[string]any
		wantIn string
	}{
		{name: "omada_log_alerts", args: map[string]any{"site_id": "site-1"}, wantIn: "API error -1171"},
		{name: "omada_log_events", args: map[string]any{"site_id": "site-1"}, wantIn: "API error -1172"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: tc.name, Arguments: tc.args})
			if err == nil {
				text, _ := toolText(result)
				t.Fatalf("not a tool error, content %q", text)
			}
			if !strings.Contains(err.Error(), tc.wantIn) {
				t.Errorf("tool error %q does not carry the APIError message %q", err, tc.wantIn)
			}
		})
	}
}

// TestLogReadOnlyGating pins registration gating: the log tools are all
// read-only, so with ReadOnly=true both are present (nothing is hidden) and
// with ReadOnly=false both are listed as well.
func TestLogReadOnlyGating(t *testing.T) {
	b, _ := newBase(t)

	for _, readOnly := range []bool{true, false} {
		cs := newSession(t, mcpkit.Options{ReadOnly: readOnly}, allLogTools(b)...)
		listed, err := cs.ListTools(context.Background(), nil)
		if err != nil {
			t.Fatal(err)
		}
		seen := make(map[string]bool)
		for _, tl := range listed.Tools {
			seen[tl.Name] = true
		}
		for _, want := range []string{"omada_log_alerts", "omada_log_events"} {
			if !seen[want] {
				t.Errorf("readonly=%v: %s not listed, listed = %v", readOnly, want, seen)
			}
		}
	}
}

// TestLogMissingRequiredSiteID pins required-arg validation: a missing
// site_id on omada_log_alerts fails as a tool error BEFORE any HTTP call
// reaches the fake controller (no route request, no extra token request).
func TestLogMissingRequiredSiteID(t *testing.T) {
	b, fake := newBase(t)
	mux := fake.Handler().(*http.ServeMux)
	mux.Handle("GET /openapi/v1/{omadacId}/sites/{siteId}/logs/alerts", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("fake controller received a route request; required-arg validation must fail first")
		fakectrl.EnvelopeJSON(w, 0, nil)
	}))

	before := len(fake.Requests())
	cs := newSession(t, mcpkit.Options{ReadOnly: false}, allLogTools(b)...)

	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "omada_log_alerts"})
	if err == nil {
		text, _ := toolText(result)
		t.Fatalf("omada_log_alerts: missing site_id: not a tool error, content %q", text)
	}
	if !strings.Contains(err.Error(), "site_id is required") {
		t.Errorf("omada_log_alerts: tool error %q does not name the missing site_id", err)
	}

	if after := len(fake.Requests()); after != before {
		t.Errorf("captured requests after failed calls = %d, want %d (validation must fail before any HTTP)", after, before)
	}
}
