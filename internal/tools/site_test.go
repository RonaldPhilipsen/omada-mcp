// Package tools_test pins the curated site group: the pattern exemplar
// every later curated-group task copies. Each test drives a real mcpkit
// server over the SDK's in-memory transport pair against a real fake
// controller (no mocks of the client or the transport).
package tools_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/RonaldPhilipsen/omada-mcp/internal/config"
	"github.com/RonaldPhilipsen/omada-mcp/internal/fakectrl"
	"github.com/RonaldPhilipsen/omada-mcp/internal/mcpkit"
	"github.com/RonaldPhilipsen/omada-mcp/internal/omadaclient"
	"github.com/RonaldPhilipsen/omada-mcp/internal/tools"
)

// omadacConfigured is the OMADA_OMADAC_ID the test Base is configured
// with; tools that omit omadac_id must fall back to it.
const omadacConfigured = "omac-123"

// newBase builds a tools.Base whose controller is a fake controller with
// the configured omadac ID omadacConfigured.
func newBase(t *testing.T) (*tools.Base, *fakectrl.Fake) {
	t.Helper()
	fake := fakectrl.New(t)
	cfg := config.Config{
		BaseURL:      fake.URL(),
		OmadacID:     omadacConfigured,
		ClientID:     "client-id-value",
		ClientSecret: "client-secret-value",
	}
	c, err := omadaclient.New(cfg)
	if err != nil {
		t.Fatalf("omadaclient.New: %v", err)
	}
	return &tools.Base{Omc: c, Cfg: cfg}, fake
}

// allSiteTools returns the four curated site tools from b.
func allSiteTools(b *tools.Base) []mcpkit.Tool {
	return []mcpkit.Tool{b.SiteList(), b.SiteGet(), b.SiteCreate(), b.SiteDelete()}
}

// newSession wires the real server under test to a real SDK client
// session through the SDK's in-memory transport pair (same pattern as
// the mcpkit tests).
func newSession(t *testing.T, opt mcpkit.Options, tool ...mcpkit.Tool) *mcp.ClientSession {
	t.Helper()

	s := mcpkit.NewServer("omada", opt)
	for _, tl := range tool {
		s.Register(tl)
	}

	ctx := context.Background()
	srvT, cliT := mcp.NewInMemoryTransports()
	srvSession, err := s.MCPServer().Connect(ctx, srvT, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = srvSession.Close() })
	client := mcp.NewClient(&mcp.Implementation{Name: "tools-test-client", Version: "0.0.0"}, nil)
	cs, err := client.Connect(ctx, cliT, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

// toolText concatenates the text content of a result and reports IsError.
func toolText(result *mcp.CallToolResult) (string, bool) {
	var text string
	for _, c := range result.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			text += tc.Text
		}
	}
	return text, result.IsError
}

// entryFor finds the captured request with the given path.
func entryFor(t *testing.T, reqs []fakectrl.Entry, path string) *fakectrl.Entry {
	t.Helper()
	for i := range reqs {
		if reqs[i].Path == path {
			return &reqs[i]
		}
	}
	t.Fatalf("no captured request for %s: %+v", path, reqs)
	return nil
}

// TestSiteListRoundTrip pins omada_site_list: an explicit omadac_id is
// used in the request path, the decoded rows come back in the JSONResult
// content, and the request carries the default paging query.
func TestSiteListRoundTrip(t *testing.T) {
	b, fake := newBase(t)
	mux := fake.Handler().(*http.ServeMux)
	mux.Handle("GET /openapi/v1/{omadacId}/sites", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fakectrl.EnvelopeJSON(w, 0, map[string]any{
			"data": []map[string]any{
				{"site": "site-1", "name": "Site One"},
				{"site": "site-2", "name": "Site Two"},
			},
		})
	}))

	cs := newSession(t, mcpkit.Options{ReadOnly: false}, allSiteTools(b)...)
	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "omada_site_list",
		Arguments: map[string]any{"omadac_id": "omac-456"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	text, isError := toolText(result)
	if isError {
		t.Fatalf("omada_site_list errored: %s", text)
	}
	var got struct {
		Data []struct {
			Site string `json:"site"`
			Name string `json:"name"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(text), &got); err != nil {
		t.Fatalf("content %q is not JSON: %v", text, err)
	}
	if len(got.Data) != 2 || got.Data[0].Name != "Site One" || got.Data[1].Name != "Site Two" {
		t.Errorf("decoded rows = %+v, want two rows named Site One/Site Two", got.Data)
	}

	req := entryFor(t, fake.Requests(), "/openapi/v1/omac-456/sites")
	if req.Method != http.MethodGet {
		t.Errorf("method = %q, want GET", req.Method)
	}
	if req.Query != "page=1&pageSize=20" {
		t.Errorf("query = %q, want page=1&pageSize=20", req.Query)
	}
}

// TestSiteListZeroArgsDefaults pins Review Focus 4: an empty-args call
// falls back to the configured omadac ID and ships the default paging
// query page=1&pageSize=20.
func TestSiteListZeroArgsDefaults(t *testing.T) {
	b, fake := newBase(t)
	mux := fake.Handler().(*http.ServeMux)
	mux.Handle("GET /openapi/v1/{omadacId}/sites", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fakectrl.EnvelopeJSON(w, 0, map[string]any{
			"data": []map[string]any{{"site": "site-1", "name": "Site One"}},
		})
	}))

	cs := newSession(t, mcpkit.Options{ReadOnly: false}, allSiteTools(b)...)
	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "omada_site_list"})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if text, isError := toolText(result); isError {
		t.Fatalf("omada_site_list errored: %s", text)
	}

	req := entryFor(t, fake.Requests(), "/openapi/v1/"+omadacConfigured+"/sites")
	if req.Query != "page=1&pageSize=20" {
		t.Errorf("query = %q, want defaults page=1&pageSize=20", req.Query)
	}
}

// TestSiteGetRoundTrip pins omada_site_get: the required site_id is used
// in the request path and the decoded entity comes back in the content.
func TestSiteGetRoundTrip(t *testing.T) {
	b, fake := newBase(t)
	mux := fake.Handler().(*http.ServeMux)
	mux.Handle("GET /openapi/v1/{omadacId}/sites/{siteId}", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fakectrl.EnvelopeJSON(w, 0, map[string]any{
			"site":   r.PathValue("siteId"),
			"name":   "Site One",
			"region": "United States",
		})
	}))

	cs := newSession(t, mcpkit.Options{ReadOnly: false}, allSiteTools(b)...)
	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "omada_site_get",
		Arguments: map[string]any{"site_id": "site-1"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	text, isError := toolText(result)
	if isError {
		t.Fatalf("omada_site_get errored: %s", text)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(text), &got); err != nil {
		t.Fatalf("content %q is not JSON: %v", text, err)
	}
	if got["site"] != "site-1" || got["name"] != "Site One" {
		t.Errorf("decoded entity = %v, want site site-1 named Site One", got)
	}

	entryFor(t, fake.Requests(), "/openapi/v1/"+omadacConfigured+"/sites/site-1")
}

// TestSiteCreateRoundTrip pins omada_site_create: the body is sent to
// the controller mirroring the Omada request type, and the created
// entity comes back in the content.
func TestSiteCreateRoundTrip(t *testing.T) {
	b, fake := newBase(t)
	mux := fake.Handler().(*http.ServeMux)
	mux.Handle("POST /openapi/v1/{omadacId}/sites", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fakectrl.EnvelopeJSON(w, 0, map[string]any{"site": "site-9", "name": "New Site"})
	}))

	cs := newSession(t, mcpkit.Options{ReadOnly: false}, allSiteTools(b)...)
	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "omada_site_create",
		Arguments: map[string]any{
			"body": map[string]any{
				"name":     "New Site",
				"region":   "United States",
				"scenario": "SME",
				"timeZone": "UTC",
			},
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	text, isError := toolText(result)
	if isError {
		t.Fatalf("omada_site_create errored: %s", text)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(text), &got); err != nil {
		t.Fatalf("content %q is not JSON: %v", text, err)
	}
	if got["site"] != "site-9" || got["name"] != "New Site" {
		t.Errorf("created entity = %v, want site site-9 named New Site", got)
	}

	req := entryFor(t, fake.Requests(), "/openapi/v1/"+omadacConfigured+"/sites")
	if req.Method != http.MethodPost {
		t.Errorf("method = %q, want POST", req.Method)
	}
	var sent struct {
		Name     string `json:"name"`
		Region   string `json:"region"`
		Scenario string `json:"scenario"`
		TimeZone string `json:"timeZone"`
	}
	if err := json.Unmarshal(req.Body, &sent); err != nil {
		t.Fatalf("request body %q is not JSON: %v", req.Body, err)
	}
	if sent.Name != "New Site" || sent.Region != "United States" || sent.Scenario != "SME" || sent.TimeZone != "UTC" {
		t.Errorf("sent body = %+v, want the mirrored request fields", sent)
	}
}

// TestSiteDeleteRoundTrip pins omada_site_delete: the DELETE hits
// /sites/{siteId} and a null result payload comes back as an empty
// object.
func TestSiteDeleteRoundTrip(t *testing.T) {
	b, fake := newBase(t)
	mux := fake.Handler().(*http.ServeMux)
	mux.Handle("DELETE /openapi/v1/{omadacId}/sites/{siteId}", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fakectrl.EnvelopeJSON(w, 0, nil)
	}))

	cs := newSession(t, mcpkit.Options{ReadOnly: false}, allSiteTools(b)...)
	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "omada_site_delete",
		Arguments: map[string]any{"site_id": "site-1"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	text, isError := toolText(result)
	if isError {
		t.Fatalf("omada_site_delete errored: %s", text)
	}
	if text != "{}" {
		t.Errorf("result = %q, want {} for a null result payload", text)
	}

	req := entryFor(t, fake.Requests(), "/openapi/v1/"+omadacConfigured+"/sites/site-1")
	if req.Method != http.MethodDelete {
		t.Errorf("method = %q, want DELETE", req.Method)
	}
}

// TestSiteBusinessError pins the envelope business-error path: a
// non-zero errorCode surfaces as a tool error carrying the APIError
// message, for a read tool and a write tool.
func TestSiteBusinessError(t *testing.T) {
	b, fake := newBase(t)
	mux := fake.Handler().(*http.ServeMux)
	mux.Handle("GET /openapi/v1/{omadacId}/sites", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fakectrl.EnvelopeJSON(w, -1171, nil)
	}))
	mux.Handle("POST /openapi/v1/{omadacId}/sites", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fakectrl.EnvelopeJSON(w, -1172, nil)
	}))

	cs := newSession(t, mcpkit.Options{ReadOnly: false}, allSiteTools(b)...)

	cases := []struct {
		name   string
		args   map[string]any
		wantIn string
	}{
		{name: "omada_site_list", args: map[string]any{}, wantIn: "API error -1171"},
		{name: "omada_site_create", args: map[string]any{"body": map[string]any{"name": "X"}}, wantIn: "API error -1172"},
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

// TestSiteReadOnlyGating pins registration gating: with ReadOnly=true the
// mutating create/delete tools are absent from ListTools while the read
// tools are present; with ReadOnly=false all four are listed.
func TestSiteReadOnlyGating(t *testing.T) {
	b, _ := newBase(t)

	cs := newSession(t, mcpkit.Options{ReadOnly: true}, allSiteTools(b)...)
	listed, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	seen := make(map[string]bool)
	for _, tl := range listed.Tools {
		seen[tl.Name] = true
	}
	if !seen["omada_site_list"] || !seen["omada_site_get"] {
		t.Errorf("read-only: read tools missing, listed = %v", seen)
	}
	if seen["omada_site_create"] || seen["omada_site_delete"] {
		t.Errorf("read-only: mutating tools exposed, listed = %v", seen)
	}

	cs2 := newSession(t, mcpkit.Options{ReadOnly: false}, allSiteTools(b)...)
	listed2, err := cs2.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	seen2 := make(map[string]bool)
	for _, tl := range listed2.Tools {
		seen2[tl.Name] = true
	}
	for _, want := range []string{"omada_site_list", "omada_site_get", "omada_site_create", "omada_site_delete"} {
		if !seen2[want] {
			t.Errorf("read-write: %s not listed, listed = %v", want, seen2)
		}
	}
}

// TestSiteMissingRequiredSiteID pins required-arg validation: a missing
// site_id on omada_site_get/omada_site_delete fails as a tool error
// BEFORE any HTTP call reaches the fake controller (no route request, no
// extra token request).
func TestSiteMissingRequiredSiteID(t *testing.T) {
	b, fake := newBase(t)
	mux := fake.Handler().(*http.ServeMux)
	for _, pattern := range []string{
		"GET /openapi/v1/{omadacId}/sites/{siteId}",
		"DELETE /openapi/v1/{omadacId}/sites/{siteId}",
	} {
		mux.Handle(pattern, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			t.Error("fake controller received a route request; required-arg validation must fail first")
			fakectrl.EnvelopeJSON(w, 0, nil)
		}))
	}

	before := len(fake.Requests())
	cs := newSession(t, mcpkit.Options{ReadOnly: false}, allSiteTools(b)...)
	for _, name := range []string{"omada_site_get", "omada_site_delete"} {
		result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name})
		if err == nil {
			text, _ := toolText(result)
			t.Fatalf("%s: missing site_id: not a tool error, content %q", name, text)
		}
		if !strings.Contains(err.Error(), "site_id is required") {
			t.Errorf("%s: tool error %q does not name the missing site_id", name, err)
		}
	}
	if after := len(fake.Requests()); after != before {
		t.Errorf("captured requests after failed calls = %d, want %d (validation must fail before any HTTP)", after, before)
	}
}
