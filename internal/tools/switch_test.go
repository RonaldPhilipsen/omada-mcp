// Switch port, LAN profile and client IP setting tool tests: each test
// drives a real mcpkit server over the SDK's in-memory transport pair
// against a real fake controller.
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

func allSwitchTools(b *tools.Base) []mcpkit.Tool {
	return []mcpkit.Tool{
		b.SwitchPortsOverview(), b.LanProfilesList(), b.SwitchPortModify(), b.SwitchPortEnable(), b.ClientIpSettingModify(),
	}
}

// TestSwitchPortsOverviewRoundTrip pins omada_switch_ports_overview: the
// request carries the default paging (page 1, pageSize 100) and the rows
// come back.
func TestSwitchPortsOverviewRoundTrip(t *testing.T) {
	b, fake := newBase(t)
	mux := fake.Handler().(*http.ServeMux)
	mux.Handle("GET /openapi/v1/{omadacId}/sites/{siteId}/switches/ports/overview", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fakectrl.EnvelopeJSON(w, 0, map[string]any{"data": []map[string]any{{"switchMac": "A8-29-48-EE-EB-1D", "port": 2, "profileId": "prof-1"}}})
	}))

	cs := newSession(t, mcpkit.Options{ReadOnly: true}, allSwitchTools(b)...)
	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "omada_switch_ports_overview",
		Arguments: map[string]any{"site_id": "site-1"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	text, isError := toolText(result)
	if isError {
		t.Fatalf("omada_switch_ports_overview errored: %s", text)
	}
	if !strings.Contains(text, "A8-29-48-EE-EB-1D") {
		t.Errorf("content %q does not carry the port row", text)
	}

	req := entryFor(t, fake.Requests(), "/openapi/v1/"+omadacConfigured+"/sites/site-1/switches/ports/overview")
	if req.Method != http.MethodGet {
		t.Errorf("method = %q, want GET", req.Method)
	}
	if req.Query != "page=1&pageSize=100" {
		t.Errorf("query = %q, want page=1&pageSize=100", req.Query)
	}
}

// TestLanProfilesListRoundTrip pins omada_lan_profiles_list: the GET hits
// /lan-profiles with paging and the rows come back.
func TestLanProfilesListRoundTrip(t *testing.T) {
	b, fake := newBase(t)
	mux := fake.Handler().(*http.ServeMux)
	mux.Handle("GET /openapi/v1/{omadacId}/sites/{siteId}/lan-profiles", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fakectrl.EnvelopeJSON(w, 0, map[string]any{"data": []map[string]any{{"id": "prof-1", "name": "All"}}})
	}))

	cs := newSession(t, mcpkit.Options{ReadOnly: true}, allSwitchTools(b)...)
	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "omada_lan_profiles_list",
		Arguments: map[string]any{"site_id": "site-1", "page_size": 50},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	text, isError := toolText(result)
	if isError {
		t.Fatalf("omada_lan_profiles_list errored: %s", text)
	}
	if !strings.Contains(text, "prof-1") {
		t.Errorf("content %q does not carry the profile row", text)
	}

	req := entryFor(t, fake.Requests(), "/openapi/v1/"+omadacConfigured+"/sites/site-1/lan-profiles")
	if req.Query != "page=1&pageSize=50" {
		t.Errorf("query = %q, want page=1&pageSize=50", req.Query)
	}
}

// TestSwitchPortModifyRoundTrip pins omada_switch_port_modify: switch_mac
// and port go into the PATCH path and the body mirrors OswPortSettingVO.
func TestSwitchPortModifyRoundTrip(t *testing.T) {
	b, fake := newBase(t)
	mux := fake.Handler().(*http.ServeMux)
	mux.Handle("PATCH /openapi/v1/{omadacId}/sites/{siteId}/switches/{switchMac}/ports/{port}", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fakectrl.EnvelopeJSON(w, 0, nil)
	}))

	cs := newSession(t, mcpkit.Options{ReadOnly: false}, allSwitchTools(b)...)
	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "omada_switch_port_modify",
		Arguments: map[string]any{
			"site_id":    "site-1",
			"switch_mac": "A8-29-48-EE-EB-1D",
			"port":       "2",
			"body":       map[string]any{"profileId": "prof-1", "profileVlanOverrideEnable": true, "nativeNetworkId": "net-iot"},
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	text, isError := toolText(result)
	if isError {
		t.Fatalf("omada_switch_port_modify errored: %s", text)
	}

	req := entryFor(t, fake.Requests(), "/openapi/v1/"+omadacConfigured+"/sites/site-1/switches/A8-29-48-EE-EB-1D/ports/2")
	if req.Method != http.MethodPatch {
		t.Errorf("method = %q, want PATCH", req.Method)
	}
	var sent struct {
		ProfileID                 string `json:"profileId"`
		ProfileVlanOverrideEnable bool   `json:"profileVlanOverrideEnable"`
		NativeNetworkID           string `json:"nativeNetworkId"`
	}
	if err := json.Unmarshal(req.Body, &sent); err != nil {
		t.Fatalf("request body %q is not JSON: %v", req.Body, err)
	}
	if sent.ProfileID != "prof-1" || !sent.ProfileVlanOverrideEnable || sent.NativeNetworkID != "net-iot" {
		t.Errorf("sent body = %+v, want the mirrored port fields", sent)
	}
}

// TestSwitchPortEnableRoundTrip pins omada_switch_port_enable: the PUT
// hits the dedicated /status endpoint, and enable false and true become
// status 0 and 1.
func TestSwitchPortEnableRoundTrip(t *testing.T) {
	b, fake := newBase(t)
	mux := fake.Handler().(*http.ServeMux)
	mux.Handle("PUT /openapi/v1/{omadacId}/sites/{siteId}/switches/{switchMac}/ports/{port}/status", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fakectrl.EnvelopeJSON(w, 0, nil)
	}))

	cs := newSession(t, mcpkit.Options{ReadOnly: false}, allSwitchTools(b)...)
	path := "/openapi/v1/" + omadacConfigured + "/sites/site-1/switches/A8-29-48-EE-EB-1D/ports/2/status"
	for _, tc := range []struct {
		enable bool
		want   string
	}{{false, `{"status":0}`}, {true, `{"status":1}`}} {
		before := len(fake.Requests())
		result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
			Name: "omada_switch_port_enable",
			Arguments: map[string]any{
				"site_id":    "site-1",
				"switch_mac": "A8-29-48-EE-EB-1D",
				"port":       "2",
				"enable":     tc.enable,
			},
		})
		if err != nil {
			t.Fatalf("CallTool(enable=%v): %v", tc.enable, err)
		}
		text, isError := toolText(result)
		if isError {
			t.Fatalf("omada_switch_port_enable(enable=%v) errored: %s", tc.enable, text)
		}
		req := entryFor(t, fake.Requests()[before:], path)
		if req.Method != http.MethodPut {
			t.Errorf("enable=%v: method = %q, want PUT", tc.enable, req.Method)
		}
		if strings.TrimSpace(string(req.Body)) != tc.want {
			t.Errorf("enable=%v: body = %s, want %s", tc.enable, req.Body, tc.want)
		}
	}
}

// TestClientIpSettingModifyRoundTrip pins omada_client_ip_setting_modify:
// the client MAC goes into the PATCH path under /network/sites and the
// body mirrors OpenApiClientIpSetting.
func TestClientIpSettingModifyRoundTrip(t *testing.T) {
	b, fake := newBase(t)
	mux := fake.Handler().(*http.ServeMux)
	mux.Handle("PATCH /openapi/v1/{omadacId}/network/sites/{siteId}/cmd/clients/{clientMac}/update-ipSetting", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fakectrl.EnvelopeJSON(w, 0, nil)
	}))

	cs := newSession(t, mcpkit.Options{ReadOnly: false}, allSwitchTools(b)...)
	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "omada_client_ip_setting_modify",
		Arguments: map[string]any{
			"site_id":    "site-1",
			"client_mac": "8C-83-E8-95-46-AF",
			"body":       map[string]any{"useFixedAddr": true, "netId": "net-iot", "ip": "172.16.10.100"},
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	text, isError := toolText(result)
	if isError {
		t.Fatalf("omada_client_ip_setting_modify errored: %s", text)
	}

	req := entryFor(t, fake.Requests(), "/openapi/v1/"+omadacConfigured+"/network/sites/site-1/cmd/clients/8C-83-E8-95-46-AF/update-ipSetting")
	if req.Method != http.MethodPatch {
		t.Errorf("method = %q, want PATCH", req.Method)
	}
	var sent struct {
		UseFixedAddr bool   `json:"useFixedAddr"`
		NetID        string `json:"netId"`
		IP           string `json:"ip"`
	}
	if err := json.Unmarshal(req.Body, &sent); err != nil {
		t.Fatalf("request body %q is not JSON: %v", req.Body, err)
	}
	if !sent.UseFixedAddr || sent.NetID != "net-iot" || sent.IP != "172.16.10.100" {
		t.Errorf("sent body = %+v, want the mirrored IP setting", sent)
	}
}

// TestSwitchToolsReadOnlyGating pins registration gating: read-only mode
// lists only the two read tools.
func TestSwitchToolsReadOnlyGating(t *testing.T) {
	b, _ := newBase(t)

	cs := newSession(t, mcpkit.Options{ReadOnly: true}, allSwitchTools(b)...)
	listed, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, tl := range listed.Tools {
		seen[tl.Name] = true
	}
	if len(seen) != 2 || !seen["omada_switch_ports_overview"] || !seen["omada_lan_profiles_list"] {
		t.Errorf("read-only tools = %v, want only the two list tools", seen)
	}
}

// TestSwitchToolsInvalidArgs pins argument validation: missing IDs and a
// fixed IP without ip or netId are tool errors and send no request.
func TestSwitchToolsInvalidArgs(t *testing.T) {
	b, fake := newBase(t)
	cs := newSession(t, mcpkit.Options{ReadOnly: false}, allSwitchTools(b)...)

	cases := []struct {
		name   string
		args   map[string]any
		wantIn string
	}{
		{name: "omada_switch_ports_overview", args: map[string]any{}, wantIn: "site_id is required"},
		{name: "omada_lan_profiles_list", args: map[string]any{}, wantIn: "site_id is required"},
		{name: "omada_switch_port_modify", args: map[string]any{"site_id": "site-1", "port": "2", "body": map[string]any{}}, wantIn: "switch_mac is required"},
		{name: "omada_switch_port_modify", args: map[string]any{"site_id": "site-1", "switch_mac": "AA-BB-CC-DD-EE-FF", "body": map[string]any{}}, wantIn: "port is required"},
		{name: "omada_switch_port_enable", args: map[string]any{"site_id": "site-1", "port": "2", "enable": false}, wantIn: "switch_mac is required"},
		{name: "omada_switch_port_enable", args: map[string]any{"site_id": "site-1", "switch_mac": "AA-BB-CC-DD-EE-FF", "port": "2"}, wantIn: "enable is required"},
		{name: "omada_client_ip_setting_modify", args: map[string]any{"site_id": "site-1", "body": map[string]any{"useFixedAddr": false}}, wantIn: "client_mac is required"},
		{name: "omada_client_ip_setting_modify", args: map[string]any{"site_id": "site-1", "client_mac": "AA-BB-CC-DD-EE-FF", "body": map[string]any{"useFixedAddr": true, "netId": "net-iot"}}, wantIn: "body.ip and body.netId are required"},
	}
	for _, tc := range cases {
		t.Run(tc.name+"/"+tc.wantIn, func(t *testing.T) {
			before := len(fake.Requests())
			result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: tc.name, Arguments: tc.args})
			if err == nil {
				text, _ := toolText(result)
				t.Fatalf("not a tool error, content %q", text)
			}
			if !strings.Contains(err.Error(), tc.wantIn) {
				t.Errorf("tool error %q does not contain %q", err, tc.wantIn)
			}
			if after := len(fake.Requests()); after != before {
				t.Errorf("invalid call sent %d request(s)", after-before)
			}
		})
	}
}
