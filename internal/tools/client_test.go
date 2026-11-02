// Package tools_test pins the curated client group. Each test drives a
// real mcpkit server over the SDK's in-memory transport pair against a
// real fake controller (no mocks of the client or the transport),
// mirroring the site group's pattern exemplar.
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

// allClientTools returns the five curated client tools from b.
func allClientTools(b *tools.Base) []mcpkit.Tool {
	return []mcpkit.Tool{b.ClientList(), b.ClientGet(), b.ClientBlock(), b.ClientUnblock(), b.ClientReconnect()}
}

// TestClientListRoundTrip pins omada_client_list: the required site_id is
// used in the request path, the query body carries the default paging, and
// the decoded rows come back in the JSONResult content.
func TestClientListRoundTrip(t *testing.T) {
	b, fake := newBase(t)
	mux := fake.Handler().(*http.ServeMux)
	mux.Handle("POST /openapi/v2/{omadacId}/sites/{siteId}/clients", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fakectrl.EnvelopeJSON(w, 0, map[string]any{
			"data": []map[string]any{
				{"mac": "aa:bb:cc:dd:ee:01", "name": "Laptop One"},
				{"mac": "aa:bb:cc:dd:ee:02", "name": "Phone Two"},
			},
		})
	}))

	cs := newSession(t, mcpkit.Options{ReadOnly: false}, allClientTools(b)...)
	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "omada_client_list",
		Arguments: map[string]any{
			"omadac_id": "omac-456",
			"site_id":   "site-1",
			"body":      map[string]any{"searchKey": "laptop"},
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	text, isError := toolText(result)
	if isError {
		t.Fatalf("omada_client_list errored: %s", text)
	}
	var got struct {
		Data []struct {
			Mac  string `json:"mac"`
			Name string `json:"name"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(text), &got); err != nil {
		t.Fatalf("content %q is not JSON: %v", text, err)
	}
	if len(got.Data) != 2 || got.Data[0].Name != "Laptop One" || got.Data[1].Name != "Phone Two" {
		t.Errorf("decoded rows = %+v, want two rows named Laptop One/Phone Two", got.Data)
	}

	req := entryFor(t, fake.Requests(), "/openapi/v2/omac-456/sites/site-1/clients")
	if req.Method != http.MethodPost {
		t.Errorf("method = %q, want POST", req.Method)
	}
	var sent struct {
		Page      int    `json:"page"`
		PageSize  int    `json:"pageSize"`
		SearchKey string `json:"searchKey"`
	}
	if err := json.Unmarshal(req.Body, &sent); err != nil {
		t.Fatalf("request body %q is not JSON: %v", req.Body, err)
	}
	if sent.Page != 1 || sent.PageSize != 20 || sent.SearchKey != "laptop" {
		t.Errorf("sent body = %+v, want default paging plus the searchKey", sent)
	}
}

// TestClientGetRoundTrip pins omada_client_get: the required site_id and
// client_mac are used in the request path and the decoded client entity
// comes back in the content.
func TestClientGetRoundTrip(t *testing.T) {
	b, fake := newBase(t)
	mux := fake.Handler().(*http.ServeMux)
	mux.Handle("GET /openapi/v1/{omadacId}/sites/{siteId}/clients/{clientMac}", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fakectrl.EnvelopeJSON(w, 0, map[string]any{
			"mac":  r.PathValue("clientMac"),
			"name": "Laptop One",
			"ip":   "192.168.1.50",
		})
	}))

	cs := newSession(t, mcpkit.Options{ReadOnly: false}, allClientTools(b)...)
	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "omada_client_get",
		Arguments: map[string]any{
			"site_id":    "site-1",
			"client_mac": "aa:bb:cc:dd:ee:01",
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	text, isError := toolText(result)
	if isError {
		t.Fatalf("omada_client_get errored: %s", text)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(text), &got); err != nil {
		t.Fatalf("content %q is not JSON: %v", text, err)
	}
	if got["mac"] != "aa:bb:cc:dd:ee:01" || got["name"] != "Laptop One" {
		t.Errorf("decoded entity = %v, want mac aa:bb:cc:dd:ee:01 named Laptop One", got)
	}

	entryFor(t, fake.Requests(), "/openapi/v1/"+omadacConfigured+"/sites/site-1/clients/aa:bb:cc:dd:ee:01")
}

// TestClientBlockRoundTrip pins omada_client_block: the POST hits
// /clients/{clientMac}/block and a null result payload comes back as an
// empty object.
func TestClientBlockRoundTrip(t *testing.T) {
	b, fake := newBase(t)
	mux := fake.Handler().(*http.ServeMux)
	mux.Handle("POST /openapi/v1/{omadacId}/sites/{siteId}/clients/{clientMac}/block", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fakectrl.EnvelopeJSON(w, 0, nil)
	}))

	cs := newSession(t, mcpkit.Options{ReadOnly: false}, allClientTools(b)...)
	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "omada_client_block",
		Arguments: map[string]any{
			"site_id":    "site-1",
			"client_mac": "aa:bb:cc:dd:ee:01",
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	text, isError := toolText(result)
	if isError {
		t.Fatalf("omada_client_block errored: %s", text)
	}
	if text != "{}" {
		t.Errorf("result = %q, want {} for a null result payload", text)
	}

	req := entryFor(t, fake.Requests(), "/openapi/v1/"+omadacConfigured+"/sites/site-1/clients/aa:bb:cc:dd:ee:01/block")
	if req.Method != http.MethodPost {
		t.Errorf("method = %q, want POST", req.Method)
	}
}

// TestClientUnblockRoundTrip pins omada_client_unblock: the POST hits
// /clients/{clientMac}/unblock and a null result payload comes back as an
// empty object.
func TestClientUnblockRoundTrip(t *testing.T) {
	b, fake := newBase(t)
	mux := fake.Handler().(*http.ServeMux)
	mux.Handle("POST /openapi/v1/{omadacId}/sites/{siteId}/clients/{clientMac}/unblock", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fakectrl.EnvelopeJSON(w, 0, nil)
	}))

	cs := newSession(t, mcpkit.Options{ReadOnly: false}, allClientTools(b)...)
	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "omada_client_unblock",
		Arguments: map[string]any{
			"site_id":    "site-1",
			"client_mac": "aa:bb:cc:dd:ee:01",
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	text, isError := toolText(result)
	if isError {
		t.Fatalf("omada_client_unblock errored: %s", text)
	}
	if text != "{}" {
		t.Errorf("result = %q, want {} for a null result payload", text)
	}

	req := entryFor(t, fake.Requests(), "/openapi/v1/"+omadacConfigured+"/sites/site-1/clients/aa:bb:cc:dd:ee:01/unblock")
	if req.Method != http.MethodPost {
		t.Errorf("method = %q, want POST", req.Method)
	}
}

// TestClientReconnectRoundTrip pins omada_client_reconnect: the POST hits
// /clients/{clientMac}/reconnect and a null result payload comes back as an
// empty object.
func TestClientReconnectRoundTrip(t *testing.T) {
	b, fake := newBase(t)
	mux := fake.Handler().(*http.ServeMux)
	mux.Handle("POST /openapi/v1/{omadacId}/sites/{siteId}/clients/{clientMac}/reconnect", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fakectrl.EnvelopeJSON(w, 0, nil)
	}))

	cs := newSession(t, mcpkit.Options{ReadOnly: false}, allClientTools(b)...)
	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "omada_client_reconnect",
		Arguments: map[string]any{
			"site_id":    "site-1",
			"client_mac": "aa:bb:cc:dd:ee:01",
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	text, isError := toolText(result)
	if isError {
		t.Fatalf("omada_client_reconnect errored: %s", text)
	}
	if text != "{}" {
		t.Errorf("result = %q, want {} for a null result payload", text)
	}

	req := entryFor(t, fake.Requests(), "/openapi/v1/"+omadacConfigured+"/sites/site-1/clients/aa:bb:cc:dd:ee:01/reconnect")
	if req.Method != http.MethodPost {
		t.Errorf("method = %q, want POST", req.Method)
	}
}

// TestClientBusinessError pins the envelope business-error path: a
// non-zero errorCode surfaces as a tool error carrying the APIError
// message, for a read tool and a write tool.
func TestClientBusinessError(t *testing.T) {
	b, fake := newBase(t)
	mux := fake.Handler().(*http.ServeMux)
	mux.Handle("GET /openapi/v1/{omadacId}/sites/{siteId}/clients/{clientMac}", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fakectrl.EnvelopeJSON(w, -1171, nil)
	}))
	mux.Handle("POST /openapi/v1/{omadacId}/sites/{siteId}/clients/{clientMac}/block", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fakectrl.EnvelopeJSON(w, -1172, nil)
	}))

	cs := newSession(t, mcpkit.Options{ReadOnly: false}, allClientTools(b)...)

	cases := []struct {
		name   string
		args   map[string]any
		wantIn string
	}{
		{name: "omada_client_get", args: map[string]any{"site_id": "site-1", "client_mac": "aa:bb:cc:dd:ee:01"}, wantIn: "API error -1171"},
		{name: "omada_client_block", args: map[string]any{"site_id": "site-1", "client_mac": "aa:bb:cc:dd:ee:01"}, wantIn: "API error -1172"},
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

// TestClientReadOnlyGating pins registration gating: with ReadOnly=true the
// mutating block/unblock/reconnect tools are absent from ListTools while the
// read tools are present; with ReadOnly=false all five are listed.
func TestClientReadOnlyGating(t *testing.T) {
	b, _ := newBase(t)

	cs := newSession(t, mcpkit.Options{ReadOnly: true}, allClientTools(b)...)
	listed, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	seen := make(map[string]bool)
	for _, tl := range listed.Tools {
		seen[tl.Name] = true
	}
	if !seen["omada_client_list"] || !seen["omada_client_get"] {
		t.Errorf("read-only: read tools missing, listed = %v", seen)
	}
	if seen["omada_client_block"] || seen["omada_client_unblock"] || seen["omada_client_reconnect"] {
		t.Errorf("read-only: mutating tools exposed, listed = %v", seen)
	}

	cs2 := newSession(t, mcpkit.Options{ReadOnly: false}, allClientTools(b)...)
	listed2, err := cs2.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	seen2 := make(map[string]bool)
	for _, tl := range listed2.Tools {
		seen2[tl.Name] = true
	}
	for _, want := range []string{"omada_client_list", "omada_client_get", "omada_client_block", "omada_client_unblock", "omada_client_reconnect"} {
		if !seen2[want] {
			t.Errorf("read-write: %s not listed, listed = %v", want, seen2)
		}
	}
}

// TestClientMissingRequiredSiteID pins required-arg validation: a missing
// site_id on omada_client_get fails as a tool error BEFORE any HTTP call
// reaches the fake controller (no route request, no extra token request);
// a present site_id with a missing client_mac fails on client_mac.
func TestClientMissingRequiredSiteID(t *testing.T) {
	b, fake := newBase(t)
	mux := fake.Handler().(*http.ServeMux)
	mux.Handle("GET /openapi/v1/{omadacId}/sites/{siteId}/clients/{clientMac}", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("fake controller received a route request; required-arg validation must fail first")
		fakectrl.EnvelopeJSON(w, 0, nil)
	}))

	before := len(fake.Requests())
	cs := newSession(t, mcpkit.Options{ReadOnly: false}, allClientTools(b)...)

	// Missing site_id.
	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "omada_client_get"})
	if err == nil {
		text, _ := toolText(result)
		t.Fatalf("omada_client_get: missing site_id: not a tool error, content %q", text)
	}
	if !strings.Contains(err.Error(), "site_id is required") {
		t.Errorf("omada_client_get: tool error %q does not name the missing site_id", err)
	}

	// Present site_id, missing client_mac.
	result, err = cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "omada_client_get",
		Arguments: map[string]any{"site_id": "site-1"},
	})
	if err == nil {
		text, _ := toolText(result)
		t.Fatalf("omada_client_get: missing client_mac: not a tool error, content %q", text)
	}
	if !strings.Contains(err.Error(), "client_mac is required") {
		t.Errorf("omada_client_get: tool error %q does not name the missing client_mac", err)
	}

	if after := len(fake.Requests()); after != before {
		t.Errorf("captured requests after failed calls = %d, want %d (validation must fail before any HTTP)", after, before)
	}
}
