// Package tools_test pins the curated insight group. Each test drives a
// real mcpkit server over the SDK's in-memory transport pair against a real
// fake controller (no mocks of the client or the transport), mirroring the
// site group's pattern exemplar.
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

// allInsightTools returns the three curated insight tools from b.
func allInsightTools(b *tools.Base) []mcpkit.Tool {
	return []mcpkit.Tool{b.InsightOverview(), b.InsightUnresolvedCount(), b.InsightIncidents()}
}

// TestInsightOverviewRoundTrip pins omada_insight_overview: the required
// site_id is used in the request path, the explicit time range is shipped as
// the query, and the decoded statistics come back in the JSONResult
// content.
func TestInsightOverviewRoundTrip(t *testing.T) {
	b, fake := newBase(t)
	mux := fake.Handler().(*http.ServeMux)
	mux.Handle("GET /openapi/v1/{omadacId}/sites/{siteId}/anomaly/overview/statistics", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fakectrl.EnvelopeJSON(w, 0, map[string]any{
			"totalIncidents": 42,
			"resolved":       30,
		})
	}))

	cs := newSession(t, mcpkit.Options{ReadOnly: false}, allInsightTools(b)...)
	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "omada_insight_overview",
		Arguments: map[string]any{
			"omadac_id":  "omac-456",
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
		t.Fatalf("omada_insight_overview errored: %s", text)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(text), &got); err != nil {
		t.Fatalf("content %q is not JSON: %v", text, err)
	}
	if got["totalIncidents"] != float64(42) {
		t.Errorf("decoded statistics = %v, want totalIncidents 42", got)
	}

	req := entryFor(t, fake.Requests(), "/openapi/v1/omac-456/sites/site-1/anomaly/overview/statistics")
	if req.Method != http.MethodGet {
		t.Errorf("method = %q, want GET", req.Method)
	}
	if req.Query != "filters.startTime=1700000000000&filters.endTime=1700003600000" {
		t.Errorf("query = %q, want the explicit time range", req.Query)
	}
}

// TestInsightUnresolvedCountRoundTrip pins omada_insight_unresolved_count:
// the required site_id is used in the request path and the decoded count
// comes back in the content.
func TestInsightUnresolvedCountRoundTrip(t *testing.T) {
	b, fake := newBase(t)
	mux := fake.Handler().(*http.ServeMux)
	mux.Handle("GET /openapi/v1/{omadacId}/sites/{siteId}/anomaly/unresolved-incidents", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fakectrl.EnvelopeJSON(w, 0, map[string]any{"count": 12})
	}))

	cs := newSession(t, mcpkit.Options{ReadOnly: false}, allInsightTools(b)...)
	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "omada_insight_unresolved_count",
		Arguments: map[string]any{"site_id": "site-1"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	text, isError := toolText(result)
	if isError {
		t.Fatalf("omada_insight_unresolved_count errored: %s", text)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(text), &got); err != nil {
		t.Fatalf("content %q is not JSON: %v", text, err)
	}
	if got["count"] != float64(12) {
		t.Errorf("decoded count = %v, want count 12", got)
	}

	req := entryFor(t, fake.Requests(), "/openapi/v1/"+omadacConfigured+"/sites/site-1/anomaly/unresolved-incidents")
	if req.Method != http.MethodGet {
		t.Errorf("method = %q, want GET", req.Method)
	}
}

// TestInsightIncidentsRoundTrip pins omada_insight_incidents: the required
// site_id is used in the request path, the paging and time range are
// shipped as the query, and the decoded incidents come back in the content.
func TestInsightIncidentsRoundTrip(t *testing.T) {
	b, fake := newBase(t)
	mux := fake.Handler().(*http.ServeMux)
	mux.Handle("GET /openapi/v1/{omadacId}/sites/{siteId}/anomaly/incidents", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fakectrl.EnvelopeJSON(w, 0, map[string]any{
			"data": []map[string]any{
				{"id": "inc-1", "anomalyCode": "ANOMALY_1"},
				{"id": "inc-2", "anomalyCode": "ANOMALY_2"},
			},
		})
	}))

	cs := newSession(t, mcpkit.Options{ReadOnly: false}, allInsightTools(b)...)
	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "omada_insight_incidents",
		Arguments: map[string]any{
			"site_id":    "site-1",
			"page":       2,
			"page_size":  5,
			"start_time": int64(1700000000000),
			"end_time":   int64(1700003600000),
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	text, isError := toolText(result)
	if isError {
		t.Fatalf("omada_insight_incidents errored: %s", text)
	}
	var got struct {
		Data []struct {
			ID          string `json:"id"`
			AnomalyCode string `json:"anomalyCode"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(text), &got); err != nil {
		t.Fatalf("content %q is not JSON: %v", text, err)
	}
	if len(got.Data) != 2 || got.Data[0].ID != "inc-1" || got.Data[1].ID != "inc-2" {
		t.Errorf("decoded rows = %+v, want two rows inc-1/inc-2", got.Data)
	}

	req := entryFor(t, fake.Requests(), "/openapi/v1/"+omadacConfigured+"/sites/site-1/anomaly/incidents")
	if req.Method != http.MethodGet {
		t.Errorf("method = %q, want GET", req.Method)
	}
	if req.Query != "page=2&pageSize=5&filters.startTime=1700000000000&filters.endTime=1700003600000" {
		t.Errorf("query = %q, want page=2&pageSize=5 plus the time range", req.Query)
	}
}

// TestInsightBusinessError pins the envelope business-error path: a
// non-zero errorCode surfaces as a tool error carrying the APIError
// message, for two read tools.
func TestInsightBusinessError(t *testing.T) {
	b, fake := newBase(t)
	mux := fake.Handler().(*http.ServeMux)
	mux.Handle("GET /openapi/v1/{omadacId}/sites/{siteId}/anomaly/overview/statistics", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fakectrl.EnvelopeJSON(w, -1171, nil)
	}))
	mux.Handle("GET /openapi/v1/{omadacId}/sites/{siteId}/anomaly/unresolved-incidents", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fakectrl.EnvelopeJSON(w, -1172, nil)
	}))

	cs := newSession(t, mcpkit.Options{ReadOnly: false}, allInsightTools(b)...)

	cases := []struct {
		name   string
		args   map[string]any
		wantIn string
	}{
		{name: "omada_insight_overview", args: map[string]any{"site_id": "site-1"}, wantIn: "API error -1171"},
		{name: "omada_insight_unresolved_count", args: map[string]any{"site_id": "site-1"}, wantIn: "API error -1172"},
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

// TestInsightReadOnlyGating pins registration gating: the insight tools are
// all read-only, so with ReadOnly=true all three are present (nothing is
// hidden) and with ReadOnly=false all three are listed as well.
func TestInsightReadOnlyGating(t *testing.T) {
	b, _ := newBase(t)

	for _, readOnly := range []bool{true, false} {
		cs := newSession(t, mcpkit.Options{ReadOnly: readOnly}, allInsightTools(b)...)
		listed, err := cs.ListTools(context.Background(), nil)
		if err != nil {
			t.Fatal(err)
		}
		seen := make(map[string]bool)
		for _, tl := range listed.Tools {
			seen[tl.Name] = true
		}
		for _, want := range []string{"omada_insight_overview", "omada_insight_unresolved_count", "omada_insight_incidents"} {
			if !seen[want] {
				t.Errorf("readonly=%v: %s not listed, listed = %v", readOnly, want, seen)
			}
		}
	}
}

// TestInsightMissingRequiredSiteID pins required-arg validation: a missing
// site_id on omada_insight_overview fails as a tool error BEFORE any HTTP
// call reaches the fake controller (no route request, no extra token
// request).
func TestInsightMissingRequiredSiteID(t *testing.T) {
	b, fake := newBase(t)
	mux := fake.Handler().(*http.ServeMux)
	mux.Handle("GET /openapi/v1/{omadacId}/sites/{siteId}/anomaly/overview/statistics", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("fake controller received a route request; required-arg validation must fail first")
		fakectrl.EnvelopeJSON(w, 0, nil)
	}))

	before := len(fake.Requests())
	cs := newSession(t, mcpkit.Options{ReadOnly: false}, allInsightTools(b)...)

	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "omada_insight_overview"})
	if err == nil {
		text, _ := toolText(result)
		t.Fatalf("omada_insight_overview: missing site_id: not a tool error, content %q", text)
	}
	if !strings.Contains(err.Error(), "site_id is required") {
		t.Errorf("omada_insight_overview: tool error %q does not name the missing site_id", err)
	}

	if after := len(fake.Requests()); after != before {
		t.Errorf("captured requests after failed calls = %d, want %d (validation must fail before any HTTP)", after, before)
	}
}
