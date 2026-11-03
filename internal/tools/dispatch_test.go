// Package tools_test pins the generic dispatch tools (ops_lookup and
// ops_call), the escape hatch over the full generated operation index.
// Each test drives a real mcpkit server over the SDK's in-memory transport
// pair against a real fake controller (no mocks of the client or the
// transport).
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

// newBaseWithReadOnly builds a tools.Base whose config carries the given
// ReadOnly flag. ops_call enforces read-only from config (call time), so
// the fixture must set it to exercise the per-op rejection.
func newBaseWithReadOnly(t *testing.T, readOnly bool) (*tools.Base, *fakectrl.Fake) {
	t.Helper()
	fake := fakectrl.New(t)
	cfg := config.Config{
		BaseURL:      fake.URL(),
		OmadacID:     omadacConfigured,
		ClientID:     "client-id-value",
		ClientSecret: "client-secret-value",
		ReadOnly:     readOnly,
	}
	c, err := omadaclient.New(cfg)
	if err != nil {
		t.Fatalf("omadaclient.New: %v", err)
	}
	return &tools.Base{Omc: c, Cfg: cfg}, fake
}

type opHit struct {
	ID       string `json:"id"`
	Method   string `json:"method"`
	Path     string `json:"path"`
	Tag      string `json:"tag"`
	Summary  string `json:"summary"`
	Mutating bool   `json:"mutating"`
}

func dispatchTools(b *tools.Base) []mcpkit.Tool {
	return []mcpkit.Tool{b.OpsLookup(), b.OpsCall()}
}

func nonTokenRequests(reqs []fakectrl.Entry) []fakectrl.Entry {
	var out []fakectrl.Entry
	for _, r := range reqs {
		if r.Path != "/openapi/authorize/token" {
			out = append(out, r)
		}
	}
	return out
}

func lookupHits(t *testing.T, cs *mcp.ClientSession, args map[string]any) []opHit {
	t.Helper()
	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "ops_lookup", Arguments: args})
	if err != nil {
		t.Fatalf("ops_lookup CallTool: %v", err)
	}
	text, isError := toolText(result)
	if isError {
		t.Fatalf("ops_lookup errored: %s", text)
	}
	var hits []opHit
	if err := json.Unmarshal([]byte(text), &hits); err != nil {
		t.Fatalf("ops_lookup content %q is not a JSON array: %v", text, err)
	}
	return hits
}

// TestOpsLookupKeywordAndTag pins ops_lookup: a keyword (case insensitive,
// across id/summary/tag) finds the site-list op, and the tag filter narrows
// exclusively to the exact tag.
func TestOpsLookupKeywordAndTag(t *testing.T) {
	b, _ := newBase(t)
	cs := newSession(t, mcpkit.Options{ReadOnly: true}, dispatchTools(b)...)

	hits := lookupHits(t, cs, map[string]any{"keyword": "site list"})
	if len(hits) == 0 {
		t.Fatal("keyword 'site list' found no ops")
	}
	foundSiteList := false
	for _, h := range hits {
		if h.ID == "getSiteList" {
			foundSiteList = true
			if h.Method != "GET" || h.Mutating {
				t.Errorf("getSiteList = %+v, want GET non-mutating", h)
			}
		}
	}
	if !foundSiteList {
		t.Errorf("keyword 'site list' did not find getSiteList; hits = %+v", hits)
	}

	tagHits := lookupHits(t, cs, map[string]any{"tag": "Site", "limit": 50})
	if len(tagHits) == 0 {
		t.Fatal("tag 'Site' found no ops")
	}
	for _, h := range tagHits {
		if !strings.EqualFold(h.Tag, "Site") {
			t.Errorf("tag filter returned a non-Site op: %+v", h)
		}
	}
}

// TestOpsLookupPaging pins ops_lookup paging: limit=1 returns exactly one
// hit and the default (no limit) caps at 10.
func TestOpsLookupPaging(t *testing.T) {
	b, _ := newBase(t)
	cs := newSession(t, mcpkit.Options{ReadOnly: true}, b.OpsLookup())

	one := lookupHits(t, cs, map[string]any{"keyword": "site", "limit": 1})
	if len(one) != 1 {
		t.Errorf("limit=1 returned %d hits, want exactly 1", len(one))
	}

	def := lookupHits(t, cs, map[string]any{"keyword": "site"})
	if len(def) == 0 {
		t.Error("default limit returned no hits")
	}
	if len(def) > 10 {
		t.Errorf("default limit returned %d hits, want at most 10", len(def))
	}
}

// TestOpsCallGetSuccess pins ops_call on a GET op: the request carries the
// on-prem auth header from the auth transport, the omadac path param is
// filled from the configured default, and the decoded envelope result comes
// back with the status code.
func TestOpsCallGetSuccess(t *testing.T) {
	b, fake := newBase(t)
	mux := fake.Handler().(*http.ServeMux)
	mux.Handle("GET /openapi/v1/{omadacId}/sites", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fakectrl.EnvelopeJSON(w, 0, map[string]any{
			"data": []map[string]any{{"site": "site-1", "name": "Site One"}},
		})
	}))

	cs := newSession(t, mcpkit.Options{ReadOnly: true}, dispatchTools(b)...)
	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "ops_call",
		Arguments: map[string]any{"operation_id": "getSiteList"},
	})
	if err != nil {
		t.Fatalf("ops_call CallTool: %v", err)
	}
	text, isError := toolText(result)
	if isError {
		t.Fatalf("ops_call errored: %s", text)
	}
	var got struct {
		Status int             `json:"status"`
		Result json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal([]byte(text), &got); err != nil {
		t.Fatalf("content %q is not JSON: %v", text, err)
	}
	if got.Status != 200 {
		t.Errorf("status = %d, want 200", got.Status)
	}
	if len(got.Result) == 0 || string(got.Result) == "null" {
		t.Errorf("result payload is empty, want the decoded envelope result")
	}

	req := entryFor(t, fake.Requests(), "/openapi/v1/"+omadacConfigured+"/sites")
	if req.Method != http.MethodGet {
		t.Errorf("method = %q, want GET", req.Method)
	}
	if req.AuthHeader != "AccessToken=test-token" {
		t.Errorf("Authorization = %q, want AccessToken=test-token", req.AuthHeader)
	}
}

// TestOpsCallReadOnlyRejectsMutating pins read-only rejection: with
// ReadOnly=true a mutating op is rejected as a tool error whose text
// contains both "read-only" and the operation id, BEFORE any HTTP request
// reaches the fake controller (zero non-token route hits).
func TestOpsCallReadOnlyRejectsMutating(t *testing.T) {
	b, fake := newBaseWithReadOnly(t, true)
	mux := fake.Handler().(*http.ServeMux)
	mux.Handle("POST /openapi/v1/{omadacId}/sites", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("fake controller received a mutating route request in read-only mode")
		fakectrl.EnvelopeJSON(w, 0, nil)
	}))

	cs := newSession(t, mcpkit.Options{ReadOnly: true}, dispatchTools(b)...)
	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "ops_call",
		Arguments: map[string]any{"operation_id": "createNewSite", "body": map[string]any{"name": "X"}},
	})
	if err == nil {
		text, _ := toolText(result)
		t.Fatalf("read-only mutating call: not a tool error, content %q", text)
	}
	if !strings.Contains(err.Error(), "read-only") {
		t.Errorf("tool error %q does not contain 'read-only'", err)
	}
	if !strings.Contains(err.Error(), "createNewSite") {
		t.Errorf("tool error %q does not name the operation id", err)
	}
	if n := len(nonTokenRequests(fake.Requests())); n != 0 {
		t.Errorf("non-token route hits = %d, want 0 (read-only must reject before any HTTP)", n)
	}
}

// TestOpsCallWriteEnabledMutating pins the write-enabled path: with
// ReadOnly=false a mutating op executes and its body is sent to the
// controller.
func TestOpsCallWriteEnabledMutating(t *testing.T) {
	b, fake := newBase(t)
	mux := fake.Handler().(*http.ServeMux)
	mux.Handle("POST /openapi/v1/{omadacId}/sites", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fakectrl.EnvelopeJSON(w, 0, map[string]any{"site": "site-9", "name": "New Site"})
	}))

	cs := newSession(t, mcpkit.Options{ReadOnly: false}, dispatchTools(b)...)
	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "ops_call",
		Arguments: map[string]any{
			"operation_id": "createNewSite",
			"body":         map[string]any{"name": "New Site"},
		},
	})
	if err != nil {
		t.Fatalf("ops_call CallTool: %v", err)
	}
	text, isError := toolText(result)
	if isError {
		t.Fatalf("ops_call errored: %s", text)
	}
	var got struct {
		Status int `json:"status"`
	}
	if err := json.Unmarshal([]byte(text), &got); err != nil {
		t.Fatalf("content %q is not JSON: %v", text, err)
	}
	if got.Status != 200 {
		t.Errorf("status = %d, want 200", got.Status)
	}

	req := entryFor(t, fake.Requests(), "/openapi/v1/"+omadacConfigured+"/sites")
	if req.Method != http.MethodPost {
		t.Errorf("method = %q, want POST", req.Method)
	}
	var sent struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(req.Body, &sent); err != nil {
		t.Fatalf("request body %q is not JSON: %v", req.Body, err)
	}
	if sent.Name != "New Site" {
		t.Errorf("sent body name = %q, want New Site", sent.Name)
	}
}

// TestOpsCallMissingPathParam pins path-param validation: a non-omadacId
// placeholder that is not supplied in path_params fails as a tool error
// that names the unfilled placeholder, BEFORE any HTTP request.
func TestOpsCallMissingPathParam(t *testing.T) {
	b, fake := newBase(t)
	mux := fake.Handler().(*http.ServeMux)
	mux.Handle("GET /openapi/v1/{omadacId}/sites/{siteId}", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("fake controller received a route request; missing path param must fail first")
		fakectrl.EnvelopeJSON(w, 0, nil)
	}))

	before := len(fake.Requests())
	cs := newSession(t, mcpkit.Options{ReadOnly: true}, dispatchTools(b)...)
	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "ops_call",
		Arguments: map[string]any{"operation_id": "getSiteEntity"},
	})
	if err == nil {
		text, _ := toolText(result)
		t.Fatalf("missing path param: not a tool error, content %q", text)
	}
	if !strings.Contains(err.Error(), "siteId") {
		t.Errorf("tool error %q does not name the unfilled placeholder siteId", err)
	}
	if after := len(fake.Requests()); after != before {
		t.Errorf("captured requests after failed call = %d, want %d (validation must fail before any HTTP)", after, before)
	}
}

// TestOpsCallBusinessError pins the envelope business-error path: a
// non-zero errorCode surfaces as a tool error carrying the APIError code
// and message.
func TestOpsCallBusinessError(t *testing.T) {
	b, fake := newBase(t)
	mux := fake.Handler().(*http.ServeMux)
	mux.Handle("GET /openapi/v1/{omadacId}/sites", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fakectrl.EnvelopeJSON(w, -1171, nil)
	}))

	cs := newSession(t, mcpkit.Options{ReadOnly: true}, dispatchTools(b)...)
	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "ops_call",
		Arguments: map[string]any{"operation_id": "getSiteList"},
	})
	if err == nil {
		text, _ := toolText(result)
		t.Fatalf("business error: not a tool error, content %q", text)
	}
	if !strings.Contains(err.Error(), "API error -1171") {
		t.Errorf("tool error %q does not carry the APIError code + message", err)
	}
}

// TestOpsCallUnknownOperation pins unknown-operation handling: an
// operation_id not in the index fails as a tool error that names it, with
// no HTTP request.
func TestOpsCallUnknownOperation(t *testing.T) {
	b, fake := newBase(t)
	before := len(fake.Requests())
	cs := newSession(t, mcpkit.Options{ReadOnly: true}, dispatchTools(b)...)
	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "ops_call",
		Arguments: map[string]any{"operation_id": "doesNotExist"},
	})
	if err == nil {
		text, _ := toolText(result)
		t.Fatalf("unknown operation: not a tool error, content %q", text)
	}
	if !strings.Contains(err.Error(), "doesNotExist") {
		t.Errorf("tool error %q does not name the unknown operation_id", err)
	}
	if after := len(fake.Requests()); after != before {
		t.Errorf("captured requests after failed call = %d, want %d (unknown op must fail before any HTTP)", after, before)
	}
}

// TestOpsCallNoSecretLeak pins the secret-leak guarantee: no tool error or
// result may contain the configured ClientID or ClientSecret values.
func TestOpsCallNoSecretLeak(t *testing.T) {
	b, _ := newBase(t)
	cs := newSession(t, mcpkit.Options{ReadOnly: true}, dispatchTools(b)...)

	cases := []map[string]any{
		{"operation_id": "doesNotExist"},
		{"operation_id": "createNewSite", "body": map[string]any{"name": "X"}},
		{"operation_id": "getSiteEntity"},
	}
	for i, args := range cases {
		result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "ops_call", Arguments: args})
		var text string
		if err != nil {
			text = err.Error()
		} else {
			text, _ = toolText(result)
		}
		if strings.Contains(text, "client-id-value") {
			t.Errorf("case %d: error/result %q leaks the client ID", i, text)
		}
		if strings.Contains(text, "client-secret-value") {
			t.Errorf("case %d: error/result %q leaks the client secret", i, text)
		}
	}
}
