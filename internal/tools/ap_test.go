// Package tools_test pins the curated ap group. Each test drives a real
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

func allApTools(b *tools.Base) []mcpkit.Tool {
	return []mcpkit.Tool{b.ApRadioConfigGet(), b.ApRadioConfigModify(), b.ApRadiosGet(), b.ApAvailableChannelGet()}
}

// TestApRadioConfigGetRoundTrip pins omada_ap_radio_config_get: the required
// site_id and ap_mac are used in the request path and the decoded radio
// config comes back in the content.
func TestApRadioConfigGetRoundTrip(t *testing.T) {
	b, fake := newBase(t)
	mux := fake.Handler().(*http.ServeMux)
	mux.Handle("GET /openapi/v1/{omadacId}/sites/{siteId}/aps/{apMac}/radio-config", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fakectrl.EnvelopeJSON(w, 0, map[string]any{
			"radioSetting2g": map[string]any{"radioEnable": true, "channel": "6", "channelWidth": "2", "txPower": 14, "txPowerLevel": 1},
			"radioSetting5g": map[string]any{"radioEnable": true, "channel": "5", "channelWidth": "5", "txPower": 16, "txPowerLevel": 1},
		})
	}))

	cs := newSession(t, mcpkit.Options{ReadOnly: false}, allApTools(b)...)
	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "omada_ap_radio_config_get",
		Arguments: map[string]any{
			"omadac_id": "omac-456",
			"site_id":   "site-1",
			"ap_mac":    "aa-bb-cc-dd-ee-01",
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	text, isError := toolText(result)
	if isError {
		t.Fatalf("omada_ap_radio_config_get errored: %s", text)
	}
	var got struct {
		RadioSetting5g struct {
			Channel      string `json:"channel"`
			ChannelWidth string `json:"channelWidth"`
			TxPowerLevel int    `json:"txPowerLevel"`
		} `json:"radioSetting5g"`
	}
	if err := json.Unmarshal([]byte(text), &got); err != nil {
		t.Fatalf("content %q is not JSON: %v", text, err)
	}
	if got.RadioSetting5g.Channel != "5" || got.RadioSetting5g.ChannelWidth != "5" || got.RadioSetting5g.TxPowerLevel != 1 {
		t.Errorf("decoded radioSetting5g = %+v, want channel 5, width 5, txPowerLevel 1", got.RadioSetting5g)
	}

	req := entryFor(t, fake.Requests(), "/openapi/v1/omac-456/sites/site-1/aps/aa-bb-cc-dd-ee-01/radio-config")
	if req.Method != http.MethodGet {
		t.Errorf("method = %q, want GET", req.Method)
	}
}

// TestApRadioConfigModifyRoundTrip pins omada_ap_radio_config_modify: the
// body is sent to the controller mirroring the Omada request type and a
// null result payload comes back as an empty object.
func TestApRadioConfigModifyRoundTrip(t *testing.T) {
	b, fake := newBase(t)
	mux := fake.Handler().(*http.ServeMux)
	mux.Handle("PATCH /openapi/v1/{omadacId}/sites/{siteId}/aps/{apMac}/radio-config", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fakectrl.EnvelopeJSON(w, 0, nil)
	}))

	cs := newSession(t, mcpkit.Options{ReadOnly: false}, allApTools(b)...)
	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "omada_ap_radio_config_modify",
		Arguments: map[string]any{
			"site_id": "site-1",
			"ap_mac":  "aa-bb-cc-dd-ee-01",
			"body": map[string]any{
				"radioSetting5g": map[string]any{
					"radioEnable":       true,
					"channel":           "5",
					"channelWidth":      "5",
					"txPowerLevel":      1,
					"wirelessMode":      -2,
					"autoSwitchOffWifi": false,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	text, isError := toolText(result)
	if isError {
		t.Fatalf("omada_ap_radio_config_modify errored: %s", text)
	}
	if text != "{}" {
		t.Errorf("result = %q, want {} for a null result payload", text)
	}

	req := entryFor(t, fake.Requests(), "/openapi/v1/"+omadacConfigured+"/sites/site-1/aps/aa-bb-cc-dd-ee-01/radio-config")
	if req.Method != http.MethodPatch {
		t.Errorf("method = %q, want PATCH", req.Method)
	}
	var sent struct {
		RadioSetting5g struct {
			Channel      string `json:"channel"`
			ChannelWidth string `json:"channelWidth"`
			TxPowerLevel int    `json:"txPowerLevel"`
			WirelessMode int    `json:"wirelessMode"`
		} `json:"radioSetting5g"`
	}
	if err := json.Unmarshal(req.Body, &sent); err != nil {
		t.Fatalf("request body %q is not JSON: %v", req.Body, err)
	}
	if sent.RadioSetting5g.Channel != "5" || sent.RadioSetting5g.ChannelWidth != "5" || sent.RadioSetting5g.TxPowerLevel != 1 || sent.RadioSetting5g.WirelessMode != -2 {
		t.Errorf("sent body = %+v, want the mirrored radio setting fields", sent.RadioSetting5g)
	}
}

// TestApRadiosGetRoundTrip pins omada_ap_radios_get: the operational radio
// state (actual operating channel, width, tx power) comes back in the
// content.
func TestApRadiosGetRoundTrip(t *testing.T) {
	b, fake := newBase(t)
	mux := fake.Handler().(*http.ServeMux)
	mux.Handle("GET /openapi/v1/{omadacId}/sites/{siteId}/aps/{apMac}/radios", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fakectrl.EnvelopeJSON(w, 0, map[string]any{
			"wp2g": map[string]any{"actualChannel": "6   / 2437MHz", "bandWidth": "20MHz", "txPower": 14},
			"wp5g": map[string]any{"actualChannel": "56  / 5280MHz", "bandWidth": "80MHz", "txPower": 16},
		})
	}))

	cs := newSession(t, mcpkit.Options{ReadOnly: false}, allApTools(b)...)
	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "omada_ap_radios_get",
		Arguments: map[string]any{
			"site_id": "site-1",
			"ap_mac":  "aa-bb-cc-dd-ee-01",
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	text, isError := toolText(result)
	if isError {
		t.Fatalf("omada_ap_radios_get errored: %s", text)
	}
	var got struct {
		Wp5g struct {
			ActualChannel string `json:"actualChannel"`
			BandWidth     string `json:"bandWidth"`
		} `json:"wp5g"`
	}
	if err := json.Unmarshal([]byte(text), &got); err != nil {
		t.Fatalf("content %q is not JSON: %v", text, err)
	}
	if got.Wp5g.ActualChannel != "56  / 5280MHz" || got.Wp5g.BandWidth != "80MHz" {
		t.Errorf("decoded wp5g = %+v, want actual channel 56 at 80MHz", got.Wp5g)
	}

	entryFor(t, fake.Requests(), "/openapi/v1/"+omadacConfigured+"/sites/site-1/aps/aa-bb-cc-dd-ee-01/radios")
}

// TestApAvailableChannelGetRoundTrip pins omada_ap_available_channel_get:
// the per-radio channel index list (index, channel, freq, supported
// widths) comes back in the content.
func TestApAvailableChannelGetRoundTrip(t *testing.T) {
	b, fake := newBase(t)
	mux := fake.Handler().(*http.ServeMux)
	mux.Handle("GET /openapi/v1/{omadacId}/sites/{siteId}/aps/{apMac}/available-channel", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fakectrl.EnvelopeJSON(w, 0, []map[string]any{
			{
				"radioId": 1,
				"apChannelDetailList": []map[string]any{
					{"channel": 36, "freq": 5180, "index": 1, "availableChannelWidthList": []int{20, 40, 80, 160}},
					{"channel": 104, "freq": 5520, "index": 10, "availableChannelWidthList": []int{20, 40, 80, 160}},
				},
			},
		})
	}))

	cs := newSession(t, mcpkit.Options{ReadOnly: false}, allApTools(b)...)
	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "omada_ap_available_channel_get",
		Arguments: map[string]any{
			"site_id": "site-1",
			"ap_mac":  "aa-bb-cc-dd-ee-01",
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	text, isError := toolText(result)
	if isError {
		t.Fatalf("omada_ap_available_channel_get errored: %s", text)
	}
	var got []struct {
		RadioId             int `json:"radioId"`
		ApChannelDetailList []struct {
			Channel int `json:"channel"`
			Index   int `json:"index"`
		} `json:"apChannelDetailList"`
	}
	if err := json.Unmarshal([]byte(text), &got); err != nil {
		t.Fatalf("content %q is not JSON: %v", text, err)
	}
	if len(got) != 1 || len(got[0].ApChannelDetailList) != 2 {
		t.Fatalf("decoded rows = %+v, want one radio with two channels", got)
	}
	if got[0].ApChannelDetailList[1].Channel != 104 || got[0].ApChannelDetailList[1].Index != 10 {
		t.Errorf("decoded second channel = %+v, want channel 104 at index 10", got[0].ApChannelDetailList[1])
	}

	entryFor(t, fake.Requests(), "/openapi/v1/"+omadacConfigured+"/sites/site-1/aps/aa-bb-cc-dd-ee-01/available-channel")
}

// TestApBusinessError pins the envelope business-error path: a non-zero
// errorCode surfaces as a tool error carrying the APIError message, for a
// read tool and a write tool.
func TestApBusinessError(t *testing.T) {
	b, fake := newBase(t)
	mux := fake.Handler().(*http.ServeMux)
	mux.Handle("GET /openapi/v1/{omadacId}/sites/{siteId}/aps/{apMac}/radio-config", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fakectrl.EnvelopeJSON(w, -1171, nil)
	}))
	mux.Handle("PATCH /openapi/v1/{omadacId}/sites/{siteId}/aps/{apMac}/radio-config", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fakectrl.EnvelopeJSON(w, -1172, nil)
	}))

	cs := newSession(t, mcpkit.Options{ReadOnly: false}, allApTools(b)...)

	cases := []struct {
		name   string
		args   map[string]any
		wantIn string
	}{
		{name: "omada_ap_radio_config_get", args: map[string]any{"site_id": "site-1", "ap_mac": "aa-bb-cc-dd-ee-01"}, wantIn: "API error -1171"},
		{name: "omada_ap_radio_config_modify", args: map[string]any{"site_id": "site-1", "ap_mac": "aa-bb-cc-dd-ee-01", "body": map[string]any{}}, wantIn: "API error -1172"},
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

// TestApReadOnlyGating pins registration gating: with ReadOnly=true the
// mutating modify tool is absent from ListTools while the read tools are
// present; with ReadOnly=false all four are listed.
func TestApReadOnlyGating(t *testing.T) {
	b, _ := newBase(t)

	cs := newSession(t, mcpkit.Options{ReadOnly: true}, allApTools(b)...)
	listed, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	seen := make(map[string]bool)
	for _, tl := range listed.Tools {
		seen[tl.Name] = true
	}
	if !seen["omada_ap_radio_config_get"] || !seen["omada_ap_radios_get"] || !seen["omada_ap_available_channel_get"] {
		t.Errorf("read-only: read tools missing, listed = %v", seen)
	}
	if seen["omada_ap_radio_config_modify"] {
		t.Errorf("read-only: mutating tool exposed, listed = %v", seen)
	}

	cs2 := newSession(t, mcpkit.Options{ReadOnly: false}, allApTools(b)...)
	listed2, err := cs2.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	seen2 := make(map[string]bool)
	for _, tl := range listed2.Tools {
		seen2[tl.Name] = true
	}
	for _, want := range []string{"omada_ap_radio_config_get", "omada_ap_radio_config_modify", "omada_ap_radios_get", "omada_ap_available_channel_get"} {
		if !seen2[want] {
			t.Errorf("read-write: %s not listed, listed = %v", want, seen2)
		}
	}
}

// TestApMissingRequiredArgs pins required-arg validation: a missing site_id
// on omada_ap_radio_config_get fails as a tool error BEFORE any HTTP call
// reaches the fake controller; a present site_id with a missing ap_mac
// fails on ap_mac.
func TestApMissingRequiredArgs(t *testing.T) {
	b, fake := newBase(t)
	mux := fake.Handler().(*http.ServeMux)
	mux.Handle("GET /openapi/v1/{omadacId}/sites/{siteId}/aps/{apMac}/radio-config", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("fake controller received a route request; required-arg validation must fail first")
		fakectrl.EnvelopeJSON(w, 0, nil)
	}))

	before := len(fake.Requests())
	cs := newSession(t, mcpkit.Options{ReadOnly: false}, allApTools(b)...)

	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "omada_ap_radio_config_get"})
	if err == nil {
		text, _ := toolText(result)
		t.Fatalf("omada_ap_radio_config_get: missing site_id: not a tool error, content %q", text)
	}
	if !strings.Contains(err.Error(), "site_id is required") {
		t.Errorf("omada_ap_radio_config_get: tool error %q does not name the missing site_id", err)
	}

	result, err = cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "omada_ap_radio_config_get",
		Arguments: map[string]any{"site_id": "site-1"},
	})
	if err == nil {
		text, _ := toolText(result)
		t.Fatalf("omada_ap_radio_config_get: missing ap_mac: not a tool error, content %q", text)
	}
	if !strings.Contains(err.Error(), "ap_mac is required") {
		t.Errorf("omada_ap_radio_config_get: tool error %q does not name the missing ap_mac", err)
	}

	if after := len(fake.Requests()); after != before {
		t.Errorf("captured requests after failed calls = %d, want %d (validation must fail before any HTTP)", after, before)
	}
}
