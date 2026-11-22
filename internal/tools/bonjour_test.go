// Bonjour service profile and Bonjour rule tool tests: each test drives a
// real mcpkit server over the SDK's in-memory transport pair against a
// real fake controller.
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

func allBonjourTools(b *tools.Base) []mcpkit.Tool {
	return []mcpkit.Tool{
		b.BonjourServiceList(), b.BonjourServiceCreate(), b.BonjourServiceModify(), b.BonjourServiceDelete(),
		b.BonjourRuleList(), b.BonjourRuleCreate(), b.BonjourRuleModify(), b.BonjourRuleDelete(),
	}
}

const (
	bonjourServicePath = "/openapi/v1/{omadacId}/sites/{siteId}/profiles/bonjour-service"
	bonjourRulePath    = "/openapi/v1/{omadacId}/sites/{siteId}/setting/service/mdns"
)

// TestBonjourServiceListRoundTrip pins omada_bonjour_service_list: the GET
// hits /profiles/bonjour-service and the profiles come back.
func TestBonjourServiceListRoundTrip(t *testing.T) {
	b, fake := newBase(t)
	mux := fake.Handler().(*http.ServeMux)
	mux.Handle("GET "+bonjourServicePath, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fakectrl.EnvelopeJSON(w, 0, []map[string]any{{"id": "svc-1", "name": "Victron GX", "serviceIds": []string{"_http._tcp.local"}}})
	}))

	cs := newSession(t, mcpkit.Options{ReadOnly: true}, allBonjourTools(b)...)
	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "omada_bonjour_service_list",
		Arguments: map[string]any{"site_id": "site-1"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	text, isError := toolText(result)
	if isError {
		t.Fatalf("omada_bonjour_service_list errored: %s", text)
	}
	if !strings.Contains(text, "svc-1") {
		t.Errorf("content %q does not carry the profile", text)
	}

	req := entryFor(t, fake.Requests(), "/openapi/v1/"+omadacConfigured+"/sites/site-1/profiles/bonjour-service")
	if req.Method != http.MethodGet {
		t.Errorf("method = %q, want GET", req.Method)
	}
}

// TestBonjourServiceWriteRoundTrips pins the service create (POST), modify
// (PUT with the ID in the path) and delete (DELETE) requests and bodies.
func TestBonjourServiceWriteRoundTrips(t *testing.T) {
	b, fake := newBase(t)
	mux := fake.Handler().(*http.ServeMux)
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { fakectrl.EnvelopeJSON(w, 0, nil) })
	mux.Handle("POST "+bonjourServicePath, ok)
	mux.Handle("PUT "+bonjourServicePath+"/{id}", ok)
	mux.Handle("DELETE "+bonjourServicePath+"/{id}", ok)

	cs := newSession(t, mcpkit.Options{ReadOnly: false}, allBonjourTools(b)...)
	body := map[string]any{"name": "Victron GX", "serviceIds": []string{"_http._tcp.local"}}
	calls := []struct {
		name, method, path string
		args               map[string]any
	}{
		{"omada_bonjour_service_create", http.MethodPost, "/sites/site-1/profiles/bonjour-service", map[string]any{"site_id": "site-1", "body": body}},
		{"omada_bonjour_service_modify", http.MethodPut, "/sites/site-1/profiles/bonjour-service/svc-1", map[string]any{"site_id": "site-1", "id": "svc-1", "body": body}},
		{"omada_bonjour_service_delete", http.MethodDelete, "/sites/site-1/profiles/bonjour-service/svc-1", map[string]any{"site_id": "site-1", "id": "svc-1"}},
	}
	for _, c := range calls {
		t.Run(c.name, func(t *testing.T) {
			result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: c.name, Arguments: c.args})
			if err != nil {
				t.Fatalf("CallTool: %v", err)
			}
			if text, isError := toolText(result); isError {
				t.Fatalf("%s errored: %s", c.name, text)
			}
			var req *fakectrl.Entry
			for _, r := range fake.Requests() {
				if r.Method == c.method && r.Path == "/openapi/v1/"+omadacConfigured+c.path {
					req = &r
				}
			}
			if req == nil {
				t.Fatalf("no %s request to %s", c.method, c.path)
			}
			if c.method == http.MethodDelete {
				return
			}
			var sent struct {
				Name       string   `json:"name"`
				ServiceIds []string `json:"serviceIds"`
			}
			if err := json.Unmarshal(req.Body, &sent); err != nil {
				t.Fatalf("request body %q is not JSON: %v", req.Body, err)
			}
			if sent.Name != "Victron GX" || len(sent.ServiceIds) != 1 || sent.ServiceIds[0] != "_http._tcp.local" {
				t.Errorf("sent body = %+v, want the mirrored service profile", sent)
			}
		})
	}
}

// TestBonjourRuleListRoundTrip pins omada_bonjour_rule_list: the GET hits
// /setting/service/mdns with the default paging.
func TestBonjourRuleListRoundTrip(t *testing.T) {
	b, fake := newBase(t)
	mux := fake.Handler().(*http.ServeMux)
	mux.Handle("GET "+bonjourRulePath, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fakectrl.EnvelopeJSON(w, 0, map[string]any{"data": []map[string]any{{"id": "rule-1", "name": "IOT to Default"}}})
	}))

	cs := newSession(t, mcpkit.Options{ReadOnly: true}, allBonjourTools(b)...)
	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "omada_bonjour_rule_list",
		Arguments: map[string]any{"site_id": "site-1"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	text, isError := toolText(result)
	if isError {
		t.Fatalf("omada_bonjour_rule_list errored: %s", text)
	}
	if !strings.Contains(text, "rule-1") {
		t.Errorf("content %q does not carry the rule", text)
	}

	req := entryFor(t, fake.Requests(), "/openapi/v1/"+omadacConfigured+"/sites/site-1/setting/service/mdns")
	if req.Query != "page=1&pageSize=20" {
		t.Errorf("query = %q, want page=1&pageSize=20", req.Query)
	}
}

// TestBonjourRuleWriteRoundTrips pins the rule create (POST), modify
// (PATCH with the ID in the path) and delete (DELETE) requests and bodies.
func TestBonjourRuleWriteRoundTrips(t *testing.T) {
	b, fake := newBase(t)
	mux := fake.Handler().(*http.ServeMux)
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { fakectrl.EnvelopeJSON(w, 0, nil) })
	mux.Handle("POST "+bonjourRulePath, ok)
	mux.Handle("PATCH "+bonjourRulePath+"/{mdnsId}", ok)
	mux.Handle("DELETE "+bonjourRulePath+"/{mdnsId}", ok)

	cs := newSession(t, mcpkit.Options{ReadOnly: false}, allBonjourTools(b)...)
	body := map[string]any{
		"name": "IOT to Default", "status": true, "type": 1, "profileIds": []string{"svc-1"},
		"osg": map[string]any{"serviceNetworks": []string{"net-iot"}, "clientNetworks": []string{"net-default"}},
	}
	calls := []struct {
		name, method, path string
		args               map[string]any
	}{
		{"omada_bonjour_rule_create", http.MethodPost, "/sites/site-1/setting/service/mdns", map[string]any{"site_id": "site-1", "body": body}},
		{"omada_bonjour_rule_modify", http.MethodPatch, "/sites/site-1/setting/service/mdns/rule-1", map[string]any{"site_id": "site-1", "id": "rule-1", "body": body}},
		{"omada_bonjour_rule_delete", http.MethodDelete, "/sites/site-1/setting/service/mdns/rule-1", map[string]any{"site_id": "site-1", "id": "rule-1"}},
	}
	for _, c := range calls {
		t.Run(c.name, func(t *testing.T) {
			result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: c.name, Arguments: c.args})
			if err != nil {
				t.Fatalf("CallTool: %v", err)
			}
			if text, isError := toolText(result); isError {
				t.Fatalf("%s errored: %s", c.name, text)
			}
			var req *fakectrl.Entry
			for _, r := range fake.Requests() {
				if r.Method == c.method && r.Path == "/openapi/v1/"+omadacConfigured+c.path {
					req = &r
				}
			}
			if req == nil {
				t.Fatalf("no %s request to %s", c.method, c.path)
			}
			if c.method == http.MethodDelete {
				return
			}
			var sent struct {
				Name       string   `json:"name"`
				Status     bool     `json:"status"`
				Type       int      `json:"type"`
				ProfileIds []string `json:"profileIds"`
				Osg        struct {
					ServiceNetworks []string `json:"serviceNetworks"`
					ClientNetworks  []string `json:"clientNetworks"`
				} `json:"osg"`
			}
			if err := json.Unmarshal(req.Body, &sent); err != nil {
				t.Fatalf("request body %q is not JSON: %v", req.Body, err)
			}
			if sent.Name != "IOT to Default" || !sent.Status || sent.Type != 1 || len(sent.ProfileIds) != 1 ||
				len(sent.Osg.ServiceNetworks) != 1 || sent.Osg.ServiceNetworks[0] != "net-iot" ||
				len(sent.Osg.ClientNetworks) != 1 || sent.Osg.ClientNetworks[0] != "net-default" {
				t.Errorf("sent body = %+v, want the mirrored rule", sent)
			}
		})
	}
}

// TestBonjourToolsReadOnlyGating pins registration gating: read-only mode
// lists only the two list tools.
func TestBonjourToolsReadOnlyGating(t *testing.T) {
	b, _ := newBase(t)

	cs := newSession(t, mcpkit.Options{ReadOnly: true}, allBonjourTools(b)...)
	listed, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, tl := range listed.Tools {
		seen[tl.Name] = true
	}
	if len(seen) != 2 || !seen["omada_bonjour_service_list"] || !seen["omada_bonjour_rule_list"] {
		t.Errorf("read-only tools = %v, want only the two list tools", seen)
	}
}

// TestBonjourToolsInvalidArgs pins argument validation: missing IDs and
// incomplete bodies are tool errors and send no request.
func TestBonjourToolsInvalidArgs(t *testing.T) {
	b, fake := newBase(t)
	cs := newSession(t, mcpkit.Options{ReadOnly: false}, allBonjourTools(b)...)

	svc := map[string]any{"name": "Victron GX", "serviceIds": []string{"_http._tcp.local"}}
	cases := []struct {
		name   string
		args   map[string]any
		wantIn string
	}{
		{name: "omada_bonjour_service_list", args: map[string]any{}, wantIn: "site_id is required"},
		{name: "omada_bonjour_service_create", args: map[string]any{"site_id": "site-1", "body": map[string]any{"serviceIds": []string{"_http._tcp.local"}}}, wantIn: "body.name is required"},
		{name: "omada_bonjour_service_create", args: map[string]any{"site_id": "site-1", "body": map[string]any{"name": "x"}}, wantIn: "at least one service ID"},
		{name: "omada_bonjour_service_create", args: map[string]any{"site_id": "site-1", "body": map[string]any{"name": "x", "serviceIds": []string{"_a._tcp.local", "_b._tcp.local", "_c._tcp.local", "_d._tcp.local"}}}, wantIn: "at most 3"},
		{name: "omada_bonjour_service_modify", args: map[string]any{"site_id": "site-1", "body": svc}, wantIn: "id is required"},
		{name: "omada_bonjour_service_delete", args: map[string]any{"site_id": "site-1"}, wantIn: "id is required"},
		{name: "omada_bonjour_rule_list", args: map[string]any{}, wantIn: "site_id is required"},
		{name: "omada_bonjour_rule_create", args: map[string]any{"site_id": "site-1", "body": map[string]any{"name": "r", "status": true, "type": 1}}, wantIn: "body.profileIds"},
		{name: "omada_bonjour_rule_create", args: map[string]any{"site_id": "site-1", "body": map[string]any{"name": "r", "status": true, "type": 1, "profileIds": []string{"svc-1"}}}, wantIn: "body.osg.serviceNetworks"},
		{name: "omada_bonjour_rule_create", args: map[string]any{"site_id": "site-1", "body": map[string]any{"name": "r", "status": true, "type": 0, "profileIds": []string{"svc-1"}}}, wantIn: "body.ap.serviceVlan"},
		{name: "omada_bonjour_rule_delete", args: map[string]any{"site_id": "site-1"}, wantIn: "id is required"},
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
