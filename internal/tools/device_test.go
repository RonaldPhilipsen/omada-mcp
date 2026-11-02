// Package tools_test pins the curated device group (including the
// topology tool). Each test drives a real mcpkit server over the SDK's
// in-memory transport pair against a real fake controller (no mocks of the
// client or the transport), mirroring the site group's pattern exemplar.
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

// allDeviceTools returns the four curated device/topology tools from b.
func allDeviceTools(b *tools.Base) []mcpkit.Tool {
	return []mcpkit.Tool{b.DeviceList(), b.DevicesBySite(), b.DeviceReboot(), b.TopologyGet()}
}

// TestDeviceListRoundTrip pins omada_device_list: the required site_id is
// used in the request path, the request carries the default paging query,
// and the decoded rows come back in the JSONResult content.
func TestDeviceListRoundTrip(t *testing.T) {
	b, fake := newBase(t)
	mux := fake.Handler().(*http.ServeMux)
	mux.Handle("GET /openapi/v1/{omadacId}/sites/{siteId}/devices", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fakectrl.EnvelopeJSON(w, 0, map[string]any{
			"data": []map[string]any{
				{"mac": "aa:bb:cc:00:00:01", "name": "Gateway One", "model": "ER605"},
				{"mac": "aa:bb:cc:00:00:02", "name": "AP Two", "model": "EAP650"},
			},
		})
	}))

	cs := newSession(t, mcpkit.Options{ReadOnly: false}, allDeviceTools(b)...)
	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "omada_device_list",
		Arguments: map[string]any{"omadac_id": "omac-456", "site_id": "site-1"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	text, isError := toolText(result)
	if isError {
		t.Fatalf("omada_device_list errored: %s", text)
	}
	var got struct {
		Data []struct {
			Mac   string `json:"mac"`
			Name  string `json:"name"`
			Model string `json:"model"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(text), &got); err != nil {
		t.Fatalf("content %q is not JSON: %v", text, err)
	}
	if len(got.Data) != 2 || got.Data[0].Name != "Gateway One" || got.Data[1].Name != "AP Two" {
		t.Errorf("decoded rows = %+v, want two rows named Gateway One/AP Two", got.Data)
	}

	req := entryFor(t, fake.Requests(), "/openapi/v1/omac-456/sites/site-1/devices")
	if req.Method != http.MethodGet {
		t.Errorf("method = %q, want GET", req.Method)
	}
	if req.Query != "page=1&pageSize=20" {
		t.Errorf("query = %q, want page=1&pageSize=20", req.Query)
	}
}

// TestDevicesBySiteRoundTrip pins omada_devices_by_site: the GET hits
// /devices/all with no paging query and the decoded rows come back in the
// content.
func TestDevicesBySiteRoundTrip(t *testing.T) {
	b, fake := newBase(t)
	mux := fake.Handler().(*http.ServeMux)
	mux.Handle("GET /openapi/v1/{omadacId}/sites/{siteId}/devices/all", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fakectrl.EnvelopeJSON(w, 0, map[string]any{
			"data": []map[string]any{
				{"mac": "aa:bb:cc:00:00:01", "name": "Gateway One"},
				{"mac": "aa:bb:cc:00:00:02", "name": "AP Two"},
				{"mac": "aa:bb:cc:00:00:03", "name": "Switch Three"},
			},
		})
	}))

	cs := newSession(t, mcpkit.Options{ReadOnly: false}, allDeviceTools(b)...)
	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "omada_devices_by_site",
		Arguments: map[string]any{"site_id": "site-1"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	text, isError := toolText(result)
	if isError {
		t.Fatalf("omada_devices_by_site errored: %s", text)
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
	if len(got.Data) != 3 || got.Data[2].Name != "Switch Three" {
		t.Errorf("decoded rows = %+v, want three rows ending Switch Three", got.Data)
	}

	req := entryFor(t, fake.Requests(), "/openapi/v1/"+omadacConfigured+"/sites/site-1/devices/all")
	if req.Method != http.MethodGet {
		t.Errorf("method = %q, want GET", req.Method)
	}
	if req.Query != "" {
		t.Errorf("query = %q, want no query", req.Query)
	}
}

// TestDeviceRebootRoundTrip pins omada_device_reboot: the required
// site_id and device_mac are used in the request path and a null result
// payload comes back as an empty object.
func TestDeviceRebootRoundTrip(t *testing.T) {
	b, fake := newBase(t)
	mux := fake.Handler().(*http.ServeMux)
	mux.Handle("POST /openapi/v1/{omadacId}/sites/{siteId}/devices/{deviceMac}/reboot", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fakectrl.EnvelopeJSON(w, 0, nil)
	}))

	cs := newSession(t, mcpkit.Options{ReadOnly: false}, allDeviceTools(b)...)
	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "omada_device_reboot",
		Arguments: map[string]any{
			"site_id":    "site-1",
			"device_mac": "aa:bb:cc:00:00:01",
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	text, isError := toolText(result)
	if isError {
		t.Fatalf("omada_device_reboot errored: %s", text)
	}
	if text != "{}" {
		t.Errorf("result = %q, want {} for a null result payload", text)
	}

	req := entryFor(t, fake.Requests(), "/openapi/v1/"+omadacConfigured+"/sites/site-1/devices/aa:bb:cc:00:00:01/reboot")
	if req.Method != http.MethodPost {
		t.Errorf("method = %q, want POST", req.Method)
	}
}

// TestTopologyGetRoundTrip pins omada_topology_get: the GET hits the v2
// topology path and the decoded topology comes back in the content.
func TestTopologyGetRoundTrip(t *testing.T) {
	b, fake := newBase(t)
	mux := fake.Handler().(*http.ServeMux)
	mux.Handle("GET /openapi/v2/{omadacId}/sites/{siteId}/topology", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fakectrl.EnvelopeJSON(w, 0, map[string]any{
			"topology": []map[string]any{
				{"type": "gateway", "mac": "aa:bb:cc:00:00:01"},
				{"type": "ap", "mac": "aa:bb:cc:00:00:02"},
			},
		})
	}))

	cs := newSession(t, mcpkit.Options{ReadOnly: false}, allDeviceTools(b)...)
	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "omada_topology_get",
		Arguments: map[string]any{"site_id": "site-1"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	text, isError := toolText(result)
	if isError {
		t.Fatalf("omada_topology_get errored: %s", text)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(text), &got); err != nil {
		t.Fatalf("content %q is not JSON: %v", text, err)
	}
	topo, ok := got["topology"].([]any)
	if !ok || len(topo) != 2 {
		t.Errorf("decoded topology = %v, want two entries", got["topology"])
	}

	entryFor(t, fake.Requests(), "/openapi/v2/"+omadacConfigured+"/sites/site-1/topology")
}

// TestDeviceBusinessError pins the envelope business-error path: a
// non-zero errorCode surfaces as a tool error carrying the APIError
// message, for a read tool and a write tool.
func TestDeviceBusinessError(t *testing.T) {
	b, fake := newBase(t)
	mux := fake.Handler().(*http.ServeMux)
	mux.Handle("GET /openapi/v1/{omadacId}/sites/{siteId}/devices", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fakectrl.EnvelopeJSON(w, -1171, nil)
	}))
	mux.Handle("POST /openapi/v1/{omadacId}/sites/{siteId}/devices/{deviceMac}/reboot", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fakectrl.EnvelopeJSON(w, -1172, nil)
	}))

	cs := newSession(t, mcpkit.Options{ReadOnly: false}, allDeviceTools(b)...)

	cases := []struct {
		name   string
		args   map[string]any
		wantIn string
	}{
		{name: "omada_device_list", args: map[string]any{"site_id": "site-1"}, wantIn: "API error -1171"},
		{name: "omada_device_reboot", args: map[string]any{"site_id": "site-1", "device_mac": "aa:bb:cc:00:00:01"}, wantIn: "API error -1172"},
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

// TestDeviceReadOnlyGating pins registration gating: with ReadOnly=true the
// mutating reboot tool is absent from ListTools while the read tools are
// present; with ReadOnly=false all four are listed.
func TestDeviceReadOnlyGating(t *testing.T) {
	b, _ := newBase(t)

	cs := newSession(t, mcpkit.Options{ReadOnly: true}, allDeviceTools(b)...)
	listed, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	seen := make(map[string]bool)
	for _, tl := range listed.Tools {
		seen[tl.Name] = true
	}
	if !seen["omada_device_list"] || !seen["omada_devices_by_site"] || !seen["omada_topology_get"] {
		t.Errorf("read-only: read tools missing, listed = %v", seen)
	}
	if seen["omada_device_reboot"] {
		t.Errorf("read-only: mutating tools exposed, listed = %v", seen)
	}

	cs2 := newSession(t, mcpkit.Options{ReadOnly: false}, allDeviceTools(b)...)
	listed2, err := cs2.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	seen2 := make(map[string]bool)
	for _, tl := range listed2.Tools {
		seen2[tl.Name] = true
	}
	for _, want := range []string{"omada_device_list", "omada_devices_by_site", "omada_device_reboot", "omada_topology_get"} {
		if !seen2[want] {
			t.Errorf("read-write: %s not listed, listed = %v", want, seen2)
		}
	}
}

// TestDeviceMissingRequiredSiteID pins required-arg validation: a missing
// site_id on omada_device_list fails as a tool error BEFORE any HTTP call
// reaches the fake controller (no route request, no extra token request);
// a present site_id with a missing device_mac fails on device_mac.
func TestDeviceMissingRequiredSiteID(t *testing.T) {
	b, fake := newBase(t)
	mux := fake.Handler().(*http.ServeMux)
	for _, pattern := range []string{
		"GET /openapi/v1/{omadacId}/sites/{siteId}/devices",
		"POST /openapi/v1/{omadacId}/sites/{siteId}/devices/{deviceMac}/reboot",
	} {
		mux.Handle(pattern, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			t.Error("fake controller received a route request; required-arg validation must fail first")
			fakectrl.EnvelopeJSON(w, 0, nil)
		}))
	}

	before := len(fake.Requests())
	cs := newSession(t, mcpkit.Options{ReadOnly: false}, allDeviceTools(b)...)

	// Missing site_id.
	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "omada_device_list"})
	if err == nil {
		text, _ := toolText(result)
		t.Fatalf("omada_device_list: missing site_id: not a tool error, content %q", text)
	}
	if !strings.Contains(err.Error(), "site_id is required") {
		t.Errorf("omada_device_list: tool error %q does not name the missing site_id", err)
	}

	// Present site_id, missing device_mac.
	result, err = cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "omada_device_reboot",
		Arguments: map[string]any{"site_id": "site-1"},
	})
	if err == nil {
		text, _ := toolText(result)
		t.Fatalf("omada_device_reboot: missing device_mac: not a tool error, content %q", text)
	}
	if !strings.Contains(err.Error(), "device_mac is required") {
		t.Errorf("omada_device_reboot: tool error %q does not name the missing device_mac", err)
	}

	if after := len(fake.Requests()); after != before {
		t.Errorf("captured requests after failed calls = %d, want %d (validation must fail before any HTTP)", after, before)
	}
}
