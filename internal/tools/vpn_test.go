// Package tools_test pins the curated vpn group. Each test drives a real
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

func allVpnTools(b *tools.Base) []mcpkit.Tool {
	return []mcpkit.Tool{b.VpnList(), b.VpnS2sList(), b.VpnC2sServersList(), b.VpnS2sModify()}
}

// TestVpnListRoundTrip pins omada_vpn_list: the required site_id is used in
// the request path and the decoded VPNs come back in the JSONResult
// content.
func TestVpnListRoundTrip(t *testing.T) {
	b, fake := newBase(t)
	mux := fake.Handler().(*http.ServeMux)
	mux.Handle("GET /openapi/v1/{omadacId}/sites/{siteId}/vpn", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fakectrl.EnvelopeJSON(w, 0, map[string]any{
			"data": []map[string]any{
				{"id": "vpn-1", "name": "VPN One", "status": true},
				{"id": "vpn-2", "name": "VPN Two", "status": false},
			},
		})
	}))

	cs := newSession(t, mcpkit.Options{ReadOnly: false}, allVpnTools(b)...)
	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "omada_vpn_list",
		Arguments: map[string]any{"omadac_id": "omac-456", "site_id": "site-1"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	text, isError := toolText(result)
	if isError {
		t.Fatalf("omada_vpn_list errored: %s", text)
	}
	var got struct {
		Data []struct {
			ID     string `json:"id"`
			Name   string `json:"name"`
			Status bool   `json:"status"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(text), &got); err != nil {
		t.Fatalf("content %q is not JSON: %v", text, err)
	}
	if len(got.Data) != 2 || got.Data[0].Name != "VPN One" || got.Data[1].Name != "VPN Two" {
		t.Errorf("decoded rows = %+v, want two rows named VPN One/VPN Two", got.Data)
	}

	req := entryFor(t, fake.Requests(), "/openapi/v1/omac-456/sites/site-1/vpn")
	if req.Method != http.MethodGet {
		t.Errorf("method = %q, want GET", req.Method)
	}
}

// TestVpnS2sListRoundTrip pins omada_vpn_s2s_list: the GET hits
// /vpn/site-to-site-vpns and the decoded VPNs come back in the content.
func TestVpnS2sListRoundTrip(t *testing.T) {
	b, fake := newBase(t)
	mux := fake.Handler().(*http.ServeMux)
	mux.Handle("GET /openapi/v1/{omadacId}/sites/{siteId}/vpn/site-to-site-vpns", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fakectrl.EnvelopeJSON(w, 0, map[string]any{
			"data": []map[string]any{
				{"id": "vpn-1", "name": "S2S One"},
			},
		})
	}))

	cs := newSession(t, mcpkit.Options{ReadOnly: false}, allVpnTools(b)...)
	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "omada_vpn_s2s_list",
		Arguments: map[string]any{"site_id": "site-1"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	text, isError := toolText(result)
	if isError {
		t.Fatalf("omada_vpn_s2s_list errored: %s", text)
	}
	var got struct {
		Data []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(text), &got); err != nil {
		t.Fatalf("content %q is not JSON: %v", text, err)
	}
	if len(got.Data) != 1 || got.Data[0].Name != "S2S One" {
		t.Errorf("decoded rows = %+v, want one row named S2S One", got.Data)
	}

	entryFor(t, fake.Requests(), "/openapi/v1/"+omadacConfigured+"/sites/site-1/vpn/site-to-site-vpns")
}

// TestVpnC2sServersListRoundTrip pins omada_vpn_c2s_servers_list: the GET
// hits /vpn/client-to-site-vpn-servers and the decoded servers come back in
// the content.
func TestVpnC2sServersListRoundTrip(t *testing.T) {
	b, fake := newBase(t)
	mux := fake.Handler().(*http.ServeMux)
	mux.Handle("GET /openapi/v1/{omadacId}/sites/{siteId}/vpn/client-to-site-vpn-servers", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fakectrl.EnvelopeJSON(w, 0, map[string]any{
			"data": []map[string]any{
				{"id": "c2s-1", "name": "C2S Server One"},
			},
		})
	}))

	cs := newSession(t, mcpkit.Options{ReadOnly: false}, allVpnTools(b)...)
	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "omada_vpn_c2s_servers_list",
		Arguments: map[string]any{"site_id": "site-1"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	text, isError := toolText(result)
	if isError {
		t.Fatalf("omada_vpn_c2s_servers_list errored: %s", text)
	}
	var got struct {
		Data []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(text), &got); err != nil {
		t.Fatalf("content %q is not JSON: %v", text, err)
	}
	if len(got.Data) != 1 || got.Data[0].Name != "C2S Server One" {
		t.Errorf("decoded rows = %+v, want one row named C2S Server One", got.Data)
	}

	entryFor(t, fake.Requests(), "/openapi/v1/"+omadacConfigured+"/sites/site-1/vpn/client-to-site-vpn-servers")
}

// TestVpnS2sModifyRoundTrip pins omada_vpn_s2s_modify: the required
// site_id and vpn_id are used in the request path, the body is sent
// mirroring the Omada request type, and the modified VPN comes back in the
// content.
func TestVpnS2sModifyRoundTrip(t *testing.T) {
	b, fake := newBase(t)
	mux := fake.Handler().(*http.ServeMux)
	mux.Handle("PATCH /openapi/v1/{omadacId}/sites/{siteId}/vpn/site-to-site-vpns/{vpnId}", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fakectrl.EnvelopeJSON(w, 0, map[string]any{
			"id":     r.PathValue("vpnId"),
			"name":   "VPN One",
			"status": true,
		})
	}))

	cs := newSession(t, mcpkit.Options{ReadOnly: false}, allVpnTools(b)...)
	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "omada_vpn_s2s_modify",
		Arguments: map[string]any{
			"site_id": "site-1",
			"vpn_id":  "vpn-1",
			"body":    map[string]any{"name": "VPN One", "siteVpnType": 0, "status": true},
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	text, isError := toolText(result)
	if isError {
		t.Fatalf("omada_vpn_s2s_modify errored: %s", text)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(text), &got); err != nil {
		t.Fatalf("content %q is not JSON: %v", text, err)
	}
	if got["id"] != "vpn-1" || got["name"] != "VPN One" {
		t.Errorf("modified entity = %v, want id vpn-1 named VPN One", got)
	}

	req := entryFor(t, fake.Requests(), "/openapi/v1/"+omadacConfigured+"/sites/site-1/vpn/site-to-site-vpns/vpn-1")
	if req.Method != http.MethodPatch {
		t.Errorf("method = %q, want PATCH", req.Method)
	}
	var sent struct {
		Name        string `json:"name"`
		SiteVpnType int    `json:"siteVpnType"`
		Status      bool   `json:"status"`
	}
	if err := json.Unmarshal(req.Body, &sent); err != nil {
		t.Fatalf("request body %q is not JSON: %v", req.Body, err)
	}
	if sent.Name != "VPN One" || sent.SiteVpnType != 0 || !sent.Status {
		t.Errorf("sent body = %+v, want the mirrored request fields", sent)
	}
}

// TestVpnBusinessError pins the envelope business-error path: a non-zero
// errorCode surfaces as a tool error carrying the APIError message, for a
// read tool and a write tool.
func TestVpnBusinessError(t *testing.T) {
	b, fake := newBase(t)
	mux := fake.Handler().(*http.ServeMux)
	mux.Handle("GET /openapi/v1/{omadacId}/sites/{siteId}/vpn", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fakectrl.EnvelopeJSON(w, -1171, nil)
	}))
	mux.Handle("PATCH /openapi/v1/{omadacId}/sites/{siteId}/vpn/site-to-site-vpns/{vpnId}", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fakectrl.EnvelopeJSON(w, -1172, nil)
	}))

	cs := newSession(t, mcpkit.Options{ReadOnly: false}, allVpnTools(b)...)

	cases := []struct {
		name   string
		args   map[string]any
		wantIn string
	}{
		{name: "omada_vpn_list", args: map[string]any{"site_id": "site-1"}, wantIn: "API error -1171"},
		{name: "omada_vpn_s2s_modify", args: map[string]any{"site_id": "site-1", "vpn_id": "vpn-1", "body": map[string]any{"name": "X", "siteVpnType": 0, "status": true}}, wantIn: "API error -1172"},
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

// TestVpnReadOnlyGating pins registration gating: with ReadOnly=true the
// mutating modify tool is absent from ListTools while the read tools are
// present; with ReadOnly=false all four are listed.
func TestVpnReadOnlyGating(t *testing.T) {
	b, _ := newBase(t)

	cs := newSession(t, mcpkit.Options{ReadOnly: true}, allVpnTools(b)...)
	listed, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	seen := make(map[string]bool)
	for _, tl := range listed.Tools {
		seen[tl.Name] = true
	}
	if !seen["omada_vpn_list"] || !seen["omada_vpn_s2s_list"] || !seen["omada_vpn_c2s_servers_list"] {
		t.Errorf("read-only: read tools missing, listed = %v", seen)
	}
	if seen["omada_vpn_s2s_modify"] {
		t.Errorf("read-only: mutating tools exposed, listed = %v", seen)
	}

	cs2 := newSession(t, mcpkit.Options{ReadOnly: false}, allVpnTools(b)...)
	listed2, err := cs2.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	seen2 := make(map[string]bool)
	for _, tl := range listed2.Tools {
		seen2[tl.Name] = true
	}
	for _, want := range []string{"omada_vpn_list", "omada_vpn_s2s_list", "omada_vpn_c2s_servers_list", "omada_vpn_s2s_modify"} {
		if !seen2[want] {
			t.Errorf("read-write: %s not listed, listed = %v", want, seen2)
		}
	}
}

// TestVpnMissingRequiredSiteID pins required-arg validation: a missing
// site_id on omada_vpn_s2s_modify fails as a tool error BEFORE any HTTP
// call reaches the fake controller (no route request, no extra token
// request); a present site_id with a missing vpn_id fails on vpn_id.
func TestVpnMissingRequiredSiteID(t *testing.T) {
	b, fake := newBase(t)
	mux := fake.Handler().(*http.ServeMux)
	mux.Handle("PATCH /openapi/v1/{omadacId}/sites/{siteId}/vpn/site-to-site-vpns/{vpnId}", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("fake controller received a route request; required-arg validation must fail first")
		fakectrl.EnvelopeJSON(w, 0, nil)
	}))

	before := len(fake.Requests())
	cs := newSession(t, mcpkit.Options{ReadOnly: false}, allVpnTools(b)...)

	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "omada_vpn_s2s_modify"})
	if err == nil {
		text, _ := toolText(result)
		t.Fatalf("omada_vpn_s2s_modify: missing site_id: not a tool error, content %q", text)
	}
	if !strings.Contains(err.Error(), "site_id is required") {
		t.Errorf("omada_vpn_s2s_modify: tool error %q does not name the missing site_id", err)
	}

	result, err = cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "omada_vpn_s2s_modify",
		Arguments: map[string]any{"site_id": "site-1"},
	})
	if err == nil {
		text, _ := toolText(result)
		t.Fatalf("omada_vpn_s2s_modify: missing vpn_id: not a tool error, content %q", text)
	}
	if !strings.Contains(err.Error(), "vpn_id is required") {
		t.Errorf("omada_vpn_s2s_modify: tool error %q does not name the missing vpn_id", err)
	}

	if after := len(fake.Requests()); after != before {
		t.Errorf("captured requests after failed calls = %d, want %d (validation must fail before any HTTP)", after, before)
	}
}
