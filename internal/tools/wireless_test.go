// Package tools_test pins the curated wireless group. Each test drives a
// real mcpkit server over the SDK's in-memory transport pair against a real
// fake controller (no mocks of the client or the transport).
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

func allWirelessTools(b *tools.Base) []mcpkit.Tool {
	return []mcpkit.Tool{b.WirelessWlansList(), b.WirelessSsidsList(), b.WirelessSsidGet(), b.WirelessSsidBandSteerModify()}
}

// TestWirelessWlansListRoundTrip pins omada_wireless_wlans_list: the
// required site_id is used in the request path and the decoded WLANs come
// back in the content.
func TestWirelessWlansListRoundTrip(t *testing.T) {
	b, fake := newBase(t)
	mux := fake.Handler().(*http.ServeMux)
	mux.Handle("GET /openapi/v1/{omadacId}/sites/{siteId}/wireless-network/wlans", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fakectrl.EnvelopeJSON(w, 0, []map[string]any{
			{"wlanId": "wlan-1", "name": "Default", "primary": true},
		})
	}))

	cs := newSession(t, mcpkit.Options{ReadOnly: false}, allWirelessTools(b)...)
	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "omada_wireless_wlans_list",
		Arguments: map[string]any{"omadac_id": "omac-456", "site_id": "site-1"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	text, isError := toolText(result)
	if isError {
		t.Fatalf("omada_wireless_wlans_list errored: %s", text)
	}
	var got []struct {
		WlanId string `json:"wlanId"`
		Name   string `json:"name"`
	}
	if err := json.Unmarshal([]byte(text), &got); err != nil {
		t.Fatalf("content %q is not JSON: %v", text, err)
	}
	if len(got) != 1 || got[0].WlanId != "wlan-1" || got[0].Name != "Default" {
		t.Errorf("decoded rows = %+v, want one WLAN wlan-1 named Default", got)
	}

	req := entryFor(t, fake.Requests(), "/openapi/v1/omac-456/sites/site-1/wireless-network/wlans")
	if req.Method != http.MethodGet {
		t.Errorf("method = %q, want GET", req.Method)
	}
}

// TestWirelessSsidsListRoundTrip pins omada_wireless_ssids_list: the
// required site_id is used in the request path and the decoded SSID list
// (grouped per WLAN) comes back in the content.
func TestWirelessSsidsListRoundTrip(t *testing.T) {
	b, fake := newBase(t)
	mux := fake.Handler().(*http.ServeMux)
	mux.Handle("GET /openapi/v1/{omadacId}/sites/{siteId}/wireless-network/ssids", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fakectrl.EnvelopeJSON(w, 0, []map[string]any{
			{
				"wlanId":   "wlan-1",
				"wlanName": "Default",
				"ssidList": []map[string]any{
					{"ssidId": "ssid-1", "ssidName": "Lisro"},
					{"ssidId": "ssid-2", "ssidName": "Lisro_IOT"},
				},
			},
		})
	}))

	cs := newSession(t, mcpkit.Options{ReadOnly: false}, allWirelessTools(b)...)
	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "omada_wireless_ssids_list",
		Arguments: map[string]any{"site_id": "site-1"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	text, isError := toolText(result)
	if isError {
		t.Fatalf("omada_wireless_ssids_list errored: %s", text)
	}
	var got []struct {
		WlanId   string `json:"wlanId"`
		SsidList []struct {
			SsidId   string `json:"ssidId"`
			SsidName string `json:"ssidName"`
		} `json:"ssidList"`
	}
	if err := json.Unmarshal([]byte(text), &got); err != nil {
		t.Fatalf("content %q is not JSON: %v", text, err)
	}
	if len(got) != 1 || len(got[0].SsidList) != 2 || got[0].SsidList[0].SsidName != "Lisro" {
		t.Errorf("decoded rows = %+v, want one WLAN with two SSIDs, first named Lisro", got)
	}

	entryFor(t, fake.Requests(), "/openapi/v1/"+omadacConfigured+"/sites/site-1/wireless-network/ssids")
}

// TestWirelessSsidGetRoundTrip pins omada_wireless_ssid_get: the required
// site_id, wlan_id and ssid_id are used in the request path and the decoded
// SSID detail comes back in the content.
func TestWirelessSsidGetRoundTrip(t *testing.T) {
	b, fake := newBase(t)
	mux := fake.Handler().(*http.ServeMux)
	mux.Handle("GET /openapi/v1/{omadacId}/sites/{siteId}/wireless-network/wlans/{wlanId}/ssids/{ssidId}", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fakectrl.EnvelopeJSON(w, 0, map[string]any{
			"ssidId":    r.PathValue("ssidId"),
			"name":      "Lisro",
			"band":      3,
			"security":  3,
			"enable11r": true,
			"bandSteer": map[string]any{"mode": 1},
		})
	}))

	cs := newSession(t, mcpkit.Options{ReadOnly: false}, allWirelessTools(b)...)
	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "omada_wireless_ssid_get",
		Arguments: map[string]any{
			"site_id": "site-1",
			"wlan_id": "wlan-1",
			"ssid_id": "ssid-1",
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	text, isError := toolText(result)
	if isError {
		t.Fatalf("omada_wireless_ssid_get errored: %s", text)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(text), &got); err != nil {
		t.Fatalf("content %q is not JSON: %v", text, err)
	}
	if got["ssidId"] != "ssid-1" || got["name"] != "Lisro" {
		t.Errorf("decoded entity = %v, want ssid-1 named Lisro", got)
	}

	entryFor(t, fake.Requests(), "/openapi/v1/"+omadacConfigured+"/sites/site-1/wireless-network/wlans/wlan-1/ssids/ssid-1")
}

// TestWirelessSsidBandSteerModifyRoundTrip pins
// omada_wireless_ssid_band_steer_modify: the required site_id, ssid_id and
// mode are sent as a {"mode":N} body to the controller and a null result
// payload comes back as an empty object.
func TestWirelessSsidBandSteerModifyRoundTrip(t *testing.T) {
	b, fake := newBase(t)
	mux := fake.Handler().(*http.ServeMux)
	mux.Handle("PATCH /openapi/v1/{omadacId}/sites/{siteId}/wireless-network/ssids/{ssidId}/band-steer", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fakectrl.EnvelopeJSON(w, 0, nil)
	}))

	cs := newSession(t, mcpkit.Options{ReadOnly: false}, allWirelessTools(b)...)
	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "omada_wireless_ssid_band_steer_modify",
		Arguments: map[string]any{
			"site_id": "site-1",
			"ssid_id": "ssid-1",
			"mode":    1,
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	text, isError := toolText(result)
	if isError {
		t.Fatalf("omada_wireless_ssid_band_steer_modify errored: %s", text)
	}
	if text != "{}" {
		t.Errorf("result = %q, want {} for a null result payload", text)
	}

	req := entryFor(t, fake.Requests(), "/openapi/v1/"+omadacConfigured+"/sites/site-1/wireless-network/ssids/ssid-1/band-steer")
	if req.Method != http.MethodPatch {
		t.Errorf("method = %q, want PATCH", req.Method)
	}
	var sent struct {
		Mode int `json:"mode"`
	}
	if err := json.Unmarshal(req.Body, &sent); err != nil {
		t.Fatalf("request body %q is not JSON: %v", req.Body, err)
	}
	if sent.Mode != 1 {
		t.Errorf("sent body mode = %d, want 1", sent.Mode)
	}
}

// TestWirelessSsidBandSteerInvalidMode pins mode validation: an out-of-range
// mode fails as a tool error BEFORE any HTTP call reaches the fake
// controller.
func TestWirelessSsidBandSteerInvalidMode(t *testing.T) {
	b, fake := newBase(t)
	mux := fake.Handler().(*http.ServeMux)
	mux.Handle("PATCH /openapi/v1/{omadacId}/sites/{siteId}/wireless-network/ssids/{ssidId}/band-steer", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("fake controller received a route request; mode validation must fail first")
		fakectrl.EnvelopeJSON(w, 0, nil)
	}))

	before := len(fake.Requests())
	cs := newSession(t, mcpkit.Options{ReadOnly: false}, allWirelessTools(b)...)

	for _, mode := range []any{-1, 4} {
		result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
			Name: "omada_wireless_ssid_band_steer_modify",
			Arguments: map[string]any{
				"site_id": "site-1",
				"ssid_id": "ssid-1",
				"mode":    mode,
			},
		})
		if err == nil {
			text, _ := toolText(result)
			t.Fatalf("mode %v: not a tool error, content %q", mode, text)
		}
		if !strings.Contains(err.Error(), "mode must be") {
			t.Errorf("mode %v: tool error %q does not name the invalid mode", mode, err)
		}
	}

	if after := len(fake.Requests()); after != before {
		t.Errorf("captured requests after failed calls = %d, want %d (validation must fail before any HTTP)", after, before)
	}
}

// TestWirelessBusinessError pins the envelope business-error path: a
// non-zero errorCode surfaces as a tool error carrying the APIError message,
// for a read tool and a write tool.
func TestWirelessBusinessError(t *testing.T) {
	b, fake := newBase(t)
	mux := fake.Handler().(*http.ServeMux)
	mux.Handle("GET /openapi/v1/{omadacId}/sites/{siteId}/wireless-network/wlans/{wlanId}/ssids/{ssidId}", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fakectrl.EnvelopeJSON(w, -1171, nil)
	}))
	mux.Handle("PATCH /openapi/v1/{omadacId}/sites/{siteId}/wireless-network/ssids/{ssidId}/band-steer", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fakectrl.EnvelopeJSON(w, -1172, nil)
	}))

	cs := newSession(t, mcpkit.Options{ReadOnly: false}, allWirelessTools(b)...)

	cases := []struct {
		name   string
		args   map[string]any
		wantIn string
	}{
		{name: "omada_wireless_ssid_get", args: map[string]any{"site_id": "site-1", "wlan_id": "wlan-1", "ssid_id": "ssid-1"}, wantIn: "API error -1171"},
		{name: "omada_wireless_ssid_band_steer_modify", args: map[string]any{"site_id": "site-1", "ssid_id": "ssid-1", "mode": 1}, wantIn: "API error -1172"},
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

// TestWirelessReadOnlyGating pins registration gating: with ReadOnly=true
// the mutating band-steer tool is absent from ListTools while the read
// tools are present; with ReadOnly=false all four are listed.
func TestWirelessReadOnlyGating(t *testing.T) {
	b, _ := newBase(t)

	cs := newSession(t, mcpkit.Options{ReadOnly: true}, allWirelessTools(b)...)
	listed, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	seen := make(map[string]bool)
	for _, tl := range listed.Tools {
		seen[tl.Name] = true
	}
	if !seen["omada_wireless_wlans_list"] || !seen["omada_wireless_ssids_list"] || !seen["omada_wireless_ssid_get"] {
		t.Errorf("read-only: read tools missing, listed = %v", seen)
	}
	if seen["omada_wireless_ssid_band_steer_modify"] {
		t.Errorf("read-only: mutating tool exposed, listed = %v", seen)
	}

	cs2 := newSession(t, mcpkit.Options{ReadOnly: false}, allWirelessTools(b)...)
	listed2, err := cs2.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	seen2 := make(map[string]bool)
	for _, tl := range listed2.Tools {
		seen2[tl.Name] = true
	}
	for _, want := range []string{"omada_wireless_wlans_list", "omada_wireless_ssids_list", "omada_wireless_ssid_get", "omada_wireless_ssid_band_steer_modify"} {
		if !seen2[want] {
			t.Errorf("read-write: %s not listed, listed = %v", want, seen2)
		}
	}
}

// TestWirelessMissingRequiredArgs pins required-arg validation: a missing
// site_id on omada_wireless_ssid_get fails as a tool error BEFORE any HTTP
// call reaches the fake controller; a present site_id with a missing
// wlan_id fails on wlan_id; a missing ssid_id fails on ssid_id.
func TestWirelessMissingRequiredArgs(t *testing.T) {
	b, fake := newBase(t)
	mux := fake.Handler().(*http.ServeMux)
	mux.Handle("GET /openapi/v1/{omadacId}/sites/{siteId}/wireless-network/wlans/{wlanId}/ssids/{ssidId}", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("fake controller received a route request; required-arg validation must fail first")
		fakectrl.EnvelopeJSON(w, 0, nil)
	}))

	before := len(fake.Requests())
	cs := newSession(t, mcpkit.Options{ReadOnly: false}, allWirelessTools(b)...)

	cases := []struct {
		args   map[string]any
		wantIn string
	}{
		{args: nil, wantIn: "site_id is required"},
		{args: map[string]any{"site_id": "site-1"}, wantIn: "wlan_id is required"},
		{args: map[string]any{"site_id": "site-1", "wlan_id": "wlan-1"}, wantIn: "ssid_id is required"},
	}
	for _, tc := range cases {
		result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "omada_wireless_ssid_get", Arguments: tc.args})
		if err == nil {
			text, _ := toolText(result)
			t.Fatalf("omada_wireless_ssid_get %v: not a tool error, content %q", tc.args, text)
		}
		if !strings.Contains(err.Error(), tc.wantIn) {
			t.Errorf("omada_wireless_ssid_get %v: tool error %q does not name %q", tc.args, err, tc.wantIn)
		}
	}

	if after := len(fake.Requests()); after != before {
		t.Errorf("captured requests after failed calls = %d, want %d (validation must fail before any HTTP)", after, before)
	}
}
