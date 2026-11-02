// Package tools_test pins the curated lan group. Each test drives a real
// mcpkit server over the SDK's in-memory transport pair against a real fake
// controller (no mocks of the client or the transport), mirroring the site
// group's pattern exemplar.
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

// allLanTools returns the five curated lan tools from b.
func allLanTools(b *tools.Base) []mcpkit.Tool {
	return []mcpkit.Tool{b.LanNetworksList(), b.LanNetworkGet(), b.LanNetworkCreate(), b.LanNetworkDelete(), b.DhcpList()}
}

// TestLanNetworksListRoundTrip pins omada_lan_networks_list: the required
// site_id is used in the request path and the decoded networks come back in
// the JSONResult content.
func TestLanNetworksListRoundTrip(t *testing.T) {
	b, fake := newBase(t)
	mux := fake.Handler().(*http.ServeMux)
	mux.Handle("GET /openapi/v1/{omadacId}/sites/{siteId}/lan-networks/all", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fakectrl.EnvelopeJSON(w, 0, map[string]any{
			"data": []map[string]any{
				{"id": "net-1", "name": "LAN One", "vlan": 10},
				{"id": "net-2", "name": "LAN Two", "vlan": 20},
			},
		})
	}))

	cs := newSession(t, mcpkit.Options{ReadOnly: false}, allLanTools(b)...)
	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "omada_lan_networks_list",
		Arguments: map[string]any{"omadac_id": "omac-456", "site_id": "site-1"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	text, isError := toolText(result)
	if isError {
		t.Fatalf("omada_lan_networks_list errored: %s", text)
	}
	var got struct {
		Data []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
			Vlan int    `json:"vlan"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(text), &got); err != nil {
		t.Fatalf("content %q is not JSON: %v", text, err)
	}
	if len(got.Data) != 2 || got.Data[0].Name != "LAN One" || got.Data[1].Name != "LAN Two" {
		t.Errorf("decoded rows = %+v, want two rows named LAN One/LAN Two", got.Data)
	}

	req := entryFor(t, fake.Requests(), "/openapi/v1/omac-456/sites/site-1/lan-networks/all")
	if req.Method != http.MethodGet {
		t.Errorf("method = %q, want GET", req.Method)
	}
}

// TestLanNetworkGetRoundTrip pins omada_lan_network_get: the required
// site_id and network_id are used in the request path and the decoded
// network entity comes back in the content.
func TestLanNetworkGetRoundTrip(t *testing.T) {
	b, fake := newBase(t)
	mux := fake.Handler().(*http.ServeMux)
	mux.Handle("GET /openapi/v1/{omadacId}/sites/{siteId}/lan-networks/{networkId}", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fakectrl.EnvelopeJSON(w, 0, map[string]any{
			"id":   r.PathValue("networkId"),
			"name": "LAN One",
			"vlan": 10,
		})
	}))

	cs := newSession(t, mcpkit.Options{ReadOnly: false}, allLanTools(b)...)
	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "omada_lan_network_get",
		Arguments: map[string]any{
			"site_id":    "site-1",
			"network_id": "net-1",
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	text, isError := toolText(result)
	if isError {
		t.Fatalf("omada_lan_network_get errored: %s", text)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(text), &got); err != nil {
		t.Fatalf("content %q is not JSON: %v", text, err)
	}
	if got["id"] != "net-1" || got["name"] != "LAN One" {
		t.Errorf("decoded entity = %v, want id net-1 named LAN One", got)
	}

	entryFor(t, fake.Requests(), "/openapi/v1/"+omadacConfigured+"/sites/site-1/lan-networks/net-1")
}

// TestLanNetworkCreateRoundTrip pins omada_lan_network_create: the body is
// sent to the controller mirroring the Omada request type, and the created
// network comes back in the content.
func TestLanNetworkCreateRoundTrip(t *testing.T) {
	b, fake := newBase(t)
	mux := fake.Handler().(*http.ServeMux)
	mux.Handle("POST /openapi/v1/{omadacId}/sites/{siteId}/lan-networks", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fakectrl.EnvelopeJSON(w, 0, map[string]any{"id": "net-9", "name": "LAN New", "vlan": 30})
	}))

	cs := newSession(t, mcpkit.Options{ReadOnly: false}, allLanTools(b)...)
	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "omada_lan_network_create",
		Arguments: map[string]any{
			"site_id": "site-1",
			"body":    map[string]any{"name": "LAN New", "purpose": 0, "vlan": 30},
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	text, isError := toolText(result)
	if isError {
		t.Fatalf("omada_lan_network_create errored: %s", text)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(text), &got); err != nil {
		t.Fatalf("content %q is not JSON: %v", text, err)
	}
	if got["id"] != "net-9" || got["name"] != "LAN New" {
		t.Errorf("created entity = %v, want id net-9 named LAN New", got)
	}

	req := entryFor(t, fake.Requests(), "/openapi/v1/"+omadacConfigured+"/sites/site-1/lan-networks")
	if req.Method != http.MethodPost {
		t.Errorf("method = %q, want POST", req.Method)
	}
	var sent struct {
		Name    string `json:"name"`
		Purpose int    `json:"purpose"`
		Vlan    int    `json:"vlan"`
	}
	if err := json.Unmarshal(req.Body, &sent); err != nil {
		t.Fatalf("request body %q is not JSON: %v", req.Body, err)
	}
	if sent.Name != "LAN New" || sent.Purpose != 0 || sent.Vlan != 30 {
		t.Errorf("sent body = %+v, want the mirrored request fields", sent)
	}
}

// TestLanNetworkDeleteRoundTrip pins omada_lan_network_delete: the DELETE
// hits /lan-networks/{networkId} and a null result payload comes back as an
// empty object.
func TestLanNetworkDeleteRoundTrip(t *testing.T) {
	b, fake := newBase(t)
	mux := fake.Handler().(*http.ServeMux)
	mux.Handle("DELETE /openapi/v1/{omadacId}/sites/{siteId}/lan-networks/{networkId}", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fakectrl.EnvelopeJSON(w, 0, nil)
	}))

	cs := newSession(t, mcpkit.Options{ReadOnly: false}, allLanTools(b)...)
	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "omada_lan_network_delete",
		Arguments: map[string]any{
			"site_id":    "site-1",
			"network_id": "net-1",
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	text, isError := toolText(result)
	if isError {
		t.Fatalf("omada_lan_network_delete errored: %s", text)
	}
	if text != "{}" {
		t.Errorf("result = %q, want {} for a null result payload", text)
	}

	req := entryFor(t, fake.Requests(), "/openapi/v1/"+omadacConfigured+"/sites/site-1/lan-networks/net-1")
	if req.Method != http.MethodDelete {
		t.Errorf("method = %q, want DELETE", req.Method)
	}
}

// TestDhcpListRoundTrip pins omada_dhcp_list: the required site_id is used
// in the request path, the request carries the default paging query, and
// the decoded reservations come back in the content.
func TestDhcpListRoundTrip(t *testing.T) {
	b, fake := newBase(t)
	mux := fake.Handler().(*http.ServeMux)
	mux.Handle("GET /openapi/v1/{omadacId}/sites/{siteId}/setting/service/dhcp", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fakectrl.EnvelopeJSON(w, 0, map[string]any{
			"data": []map[string]any{
				{"mac": "aa:bb:cc:dd:ee:01", "ip": "192.168.1.100", "description": "Printer"},
			},
		})
	}))

	cs := newSession(t, mcpkit.Options{ReadOnly: false}, allLanTools(b)...)
	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "omada_dhcp_list",
		Arguments: map[string]any{"site_id": "site-1"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	text, isError := toolText(result)
	if isError {
		t.Fatalf("omada_dhcp_list errored: %s", text)
	}
	var got struct {
		Data []struct {
			Mac string `json:"mac"`
			Ip  string `json:"ip"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(text), &got); err != nil {
		t.Fatalf("content %q is not JSON: %v", text, err)
	}
	if len(got.Data) != 1 || got.Data[0].Ip != "192.168.1.100" {
		t.Errorf("decoded rows = %+v, want one row with ip 192.168.1.100", got.Data)
	}

	req := entryFor(t, fake.Requests(), "/openapi/v1/"+omadacConfigured+"/sites/site-1/setting/service/dhcp")
	if req.Method != http.MethodGet {
		t.Errorf("method = %q, want GET", req.Method)
	}
	if req.Query != "page=1&pageSize=20" {
		t.Errorf("query = %q, want page=1&pageSize=20", req.Query)
	}
}

// TestLanBusinessError pins the envelope business-error path: a non-zero
// errorCode surfaces as a tool error carrying the APIError message, for a
// read tool and a write tool.
func TestLanBusinessError(t *testing.T) {
	b, fake := newBase(t)
	mux := fake.Handler().(*http.ServeMux)
	mux.Handle("GET /openapi/v1/{omadacId}/sites/{siteId}/lan-networks/{networkId}", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fakectrl.EnvelopeJSON(w, -1171, nil)
	}))
	mux.Handle("POST /openapi/v1/{omadacId}/sites/{siteId}/lan-networks", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fakectrl.EnvelopeJSON(w, -1172, nil)
	}))

	cs := newSession(t, mcpkit.Options{ReadOnly: false}, allLanTools(b)...)

	cases := []struct {
		name   string
		args   map[string]any
		wantIn string
	}{
		{name: "omada_lan_network_get", args: map[string]any{"site_id": "site-1", "network_id": "net-1"}, wantIn: "API error -1171"},
		{name: "omada_lan_network_create", args: map[string]any{"site_id": "site-1", "body": map[string]any{"name": "X", "purpose": 0}}, wantIn: "API error -1172"},
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

// TestLanReadOnlyGating pins registration gating: with ReadOnly=true the
// mutating create/delete tools are absent from ListTools while the read
// tools are present; with ReadOnly=false all five are listed.
func TestLanReadOnlyGating(t *testing.T) {
	b, _ := newBase(t)

	cs := newSession(t, mcpkit.Options{ReadOnly: true}, allLanTools(b)...)
	listed, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	seen := make(map[string]bool)
	for _, tl := range listed.Tools {
		seen[tl.Name] = true
	}
	if !seen["omada_lan_networks_list"] || !seen["omada_lan_network_get"] || !seen["omada_dhcp_list"] {
		t.Errorf("read-only: read tools missing, listed = %v", seen)
	}
	if seen["omada_lan_network_create"] || seen["omada_lan_network_delete"] {
		t.Errorf("read-only: mutating tools exposed, listed = %v", seen)
	}

	cs2 := newSession(t, mcpkit.Options{ReadOnly: false}, allLanTools(b)...)
	listed2, err := cs2.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	seen2 := make(map[string]bool)
	for _, tl := range listed2.Tools {
		seen2[tl.Name] = true
	}
	for _, want := range []string{"omada_lan_networks_list", "omada_lan_network_get", "omada_lan_network_create", "omada_lan_network_delete", "omada_dhcp_list"} {
		if !seen2[want] {
			t.Errorf("read-write: %s not listed, listed = %v", want, seen2)
		}
	}
}

// TestLanMissingRequiredSiteID pins required-arg validation: a missing
// site_id on omada_lan_network_get fails as a tool error BEFORE any HTTP
// call reaches the fake controller (no route request, no extra token
// request); a present site_id with a missing network_id fails on
// network_id.
func TestLanMissingRequiredSiteID(t *testing.T) {
	b, fake := newBase(t)
	mux := fake.Handler().(*http.ServeMux)
	mux.Handle("GET /openapi/v1/{omadacId}/sites/{siteId}/lan-networks/{networkId}", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("fake controller received a route request; required-arg validation must fail first")
		fakectrl.EnvelopeJSON(w, 0, nil)
	}))

	before := len(fake.Requests())
	cs := newSession(t, mcpkit.Options{ReadOnly: false}, allLanTools(b)...)

	// Missing site_id.
	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "omada_lan_network_get"})
	if err == nil {
		text, _ := toolText(result)
		t.Fatalf("omada_lan_network_get: missing site_id: not a tool error, content %q", text)
	}
	if !strings.Contains(err.Error(), "site_id is required") {
		t.Errorf("omada_lan_network_get: tool error %q does not name the missing site_id", err)
	}

	// Present site_id, missing network_id.
	result, err = cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "omada_lan_network_get",
		Arguments: map[string]any{"site_id": "site-1"},
	})
	if err == nil {
		text, _ := toolText(result)
		t.Fatalf("omada_lan_network_get: missing network_id: not a tool error, content %q", text)
	}
	if !strings.Contains(err.Error(), "network_id is required") {
		t.Errorf("omada_lan_network_get: tool error %q does not name the missing network_id", err)
	}

	if after := len(fake.Requests()); after != before {
		t.Errorf("captured requests after failed calls = %d, want %d (validation must fail before any HTTP)", after, before)
	}
}
