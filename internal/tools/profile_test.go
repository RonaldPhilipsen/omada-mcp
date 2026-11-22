// Group profile tool tests: each test drives a real mcpkit server over the
// SDK's in-memory transport pair against a real fake controller.
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

func allProfileTools(b *tools.Base) []mcpkit.Tool {
	return []mcpkit.Tool{
		b.GroupProfileList(), b.GroupProfileCreate(), b.GroupProfileModify(), b.GroupProfileDelete(),
	}
}

func groupRow() map[string]any {
	return map[string]any{
		"groupId":  "grp-1",
		"name":     "IOT-DNS-NTP",
		"type":     1,
		"ipList":   []map[string]any{{"ip": "172.16.20.7", "mask": 32}},
		"portList": []string{"53", "123"},
		"portType": 0,
	}
}

// TestGroupProfileListRoundTrip pins omada_group_profile_list: without
// group_type it hits /profiles/groups, with group_type it hits
// /profiles/groups/{groupType}, and the decoded rows come back.
func TestGroupProfileListRoundTrip(t *testing.T) {
	b, fake := newBase(t)
	mux := fake.Handler().(*http.ServeMux)
	mux.Handle("GET /openapi/v1/{omadacId}/sites/{siteId}/profiles/groups", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fakectrl.EnvelopeJSON(w, 0, map[string]any{"data": []map[string]any{groupRow()}})
	}))
	mux.Handle("GET /openapi/v1/{omadacId}/sites/{siteId}/profiles/groups/{groupType}", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fakectrl.EnvelopeJSON(w, 0, map[string]any{"data": []map[string]any{groupRow()}})
	}))

	cs := newSession(t, mcpkit.Options{ReadOnly: true}, allProfileTools(b)...)

	cases := []struct {
		name string
		args map[string]any
		path string
	}{
		{name: "all types", args: map[string]any{"site_id": "site-1"}, path: "/openapi/v1/" + omadacConfigured + "/sites/site-1/profiles/groups"},
		{name: "one type", args: map[string]any{"site_id": "site-1", "group_type": "1"}, path: "/openapi/v1/" + omadacConfigured + "/sites/site-1/profiles/groups/1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "omada_group_profile_list", Arguments: tc.args})
			if err != nil {
				t.Fatalf("CallTool: %v", err)
			}
			text, isError := toolText(result)
			if isError {
				t.Fatalf("omada_group_profile_list errored: %s", text)
			}
			var got struct {
				Data []struct {
					GroupID string `json:"groupId"`
					Name    string `json:"name"`
				} `json:"data"`
			}
			if err := json.Unmarshal([]byte(text), &got); err != nil {
				t.Fatalf("content %q is not JSON: %v", text, err)
			}
			if len(got.Data) != 1 || got.Data[0].GroupID != "grp-1" || got.Data[0].Name != "IOT-DNS-NTP" {
				t.Errorf("rows = %+v, want grp-1 IOT-DNS-NTP", got.Data)
			}
			req := entryFor(t, fake.Requests(), tc.path)
			if req.Method != http.MethodGet {
				t.Errorf("method = %q, want GET", req.Method)
			}
		})
	}
}

// TestGroupProfileCreateRoundTrip pins omada_group_profile_create: the body
// is POSTed to /profiles/groups mirroring wireless.CreateGroupOpenApiVO.
func TestGroupProfileCreateRoundTrip(t *testing.T) {
	b, fake := newBase(t)
	mux := fake.Handler().(*http.ServeMux)
	mux.Handle("POST /openapi/v1/{omadacId}/sites/{siteId}/profiles/groups", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fakectrl.EnvelopeJSON(w, 0, map[string]any{"id": "grp-9"})
	}))

	cs := newSession(t, mcpkit.Options{ReadOnly: false}, allProfileTools(b)...)
	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "omada_group_profile_create",
		Arguments: map[string]any{
			"site_id": "site-1",
			"body": map[string]any{
				"name":     "IOT-DNS-NTP",
				"type":     1,
				"ipList":   []map[string]any{{"ip": "172.16.20.7", "mask": 32}, {"ip": "172.16.20.53", "mask": 32}},
				"portType": 0,
				"portList": []string{"53", "123"},
			},
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	text, isError := toolText(result)
	if isError {
		t.Fatalf("omada_group_profile_create errored: %s", text)
	}
	if !strings.Contains(text, "grp-9") {
		t.Errorf("result = %q, want the created id grp-9", text)
	}

	req := entryFor(t, fake.Requests(), "/openapi/v1/"+omadacConfigured+"/sites/site-1/profiles/groups")
	if req.Method != http.MethodPost {
		t.Errorf("method = %q, want POST", req.Method)
	}
	var sent struct {
		Name   string `json:"name"`
		Type   int    `json:"type"`
		IPList []struct {
			IP   string `json:"ip"`
			Mask int    `json:"mask"`
		} `json:"ipList"`
		PortType int      `json:"portType"`
		PortList []string `json:"portList"`
	}
	if err := json.Unmarshal(req.Body, &sent); err != nil {
		t.Fatalf("request body %q is not JSON: %v", req.Body, err)
	}
	if sent.Name != "IOT-DNS-NTP" || sent.Type != 1 || len(sent.IPList) != 2 || sent.IPList[1].IP != "172.16.20.53" || sent.IPList[1].Mask != 32 ||
		sent.PortType != 0 || strings.Join(sent.PortList, ",") != "53,123" {
		t.Errorf("sent body = %+v, want the mirrored group profile fields", sent)
	}
}

// TestGroupProfileModifyRoundTrip pins omada_group_profile_modify: the
// group_type and group_id go into the PATCH path and the body is sent.
func TestGroupProfileModifyRoundTrip(t *testing.T) {
	b, fake := newBase(t)
	mux := fake.Handler().(*http.ServeMux)
	mux.Handle("PATCH /openapi/v1/{omadacId}/sites/{siteId}/profiles/groups/{groupType}/{groupId}", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fakectrl.EnvelopeJSON(w, 0, nil)
	}))

	cs := newSession(t, mcpkit.Options{ReadOnly: false}, allProfileTools(b)...)
	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "omada_group_profile_modify",
		Arguments: map[string]any{
			"site_id":    "site-1",
			"group_type": "0",
			"group_id":   "grp-1",
			"body":       map[string]any{"name": "Marantz", "type": 0, "ipList": []map[string]any{{"ip": "172.16.10.55", "mask": 32}}},
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	text, isError := toolText(result)
	if isError {
		t.Fatalf("omada_group_profile_modify errored: %s", text)
	}
	if text != "{}" {
		t.Errorf("result = %q, want {} for a null result payload", text)
	}

	req := entryFor(t, fake.Requests(), "/openapi/v1/"+omadacConfigured+"/sites/site-1/profiles/groups/0/grp-1")
	if req.Method != http.MethodPatch {
		t.Errorf("method = %q, want PATCH", req.Method)
	}
	var sent struct {
		Name string `json:"name"`
		Type int    `json:"type"`
	}
	if err := json.Unmarshal(req.Body, &sent); err != nil {
		t.Fatalf("request body %q is not JSON: %v", req.Body, err)
	}
	if sent.Name != "Marantz" || sent.Type != 0 {
		t.Errorf("sent body = %+v, want name Marantz and type 0", sent)
	}
}

// TestGroupProfileDeleteRoundTrip pins omada_group_profile_delete: the
// DELETE hits /profiles/groups/{groupType}/{groupId}.
func TestGroupProfileDeleteRoundTrip(t *testing.T) {
	b, fake := newBase(t)
	mux := fake.Handler().(*http.ServeMux)
	mux.Handle("DELETE /openapi/v1/{omadacId}/sites/{siteId}/profiles/groups/{groupType}/{groupId}", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fakectrl.EnvelopeJSON(w, 0, nil)
	}))

	cs := newSession(t, mcpkit.Options{ReadOnly: false}, allProfileTools(b)...)
	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "omada_group_profile_delete",
		Arguments: map[string]any{"site_id": "site-1", "group_type": "1", "group_id": "grp-1"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	text, isError := toolText(result)
	if isError {
		t.Fatalf("omada_group_profile_delete errored: %s", text)
	}

	req := entryFor(t, fake.Requests(), "/openapi/v1/"+omadacConfigured+"/sites/site-1/profiles/groups/1/grp-1")
	if req.Method != http.MethodDelete {
		t.Errorf("method = %q, want DELETE", req.Method)
	}
}

// TestGroupProfileReadOnlyGating pins registration gating: read-only mode
// lists only omada_group_profile_list.
func TestGroupProfileReadOnlyGating(t *testing.T) {
	b, _ := newBase(t)

	cs := newSession(t, mcpkit.Options{ReadOnly: true}, allProfileTools(b)...)
	listed, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, tl := range listed.Tools {
		names = append(names, tl.Name)
	}
	if strings.Join(names, ",") != "omada_group_profile_list" {
		t.Errorf("read-only tools = %v, want only omada_group_profile_list", names)
	}
}

// TestGroupProfileInvalidArgs pins argument validation: missing IDs and an
// unknown group_type are tool errors and send no request.
func TestGroupProfileInvalidArgs(t *testing.T) {
	b, fake := newBase(t)
	cs := newSession(t, mcpkit.Options{ReadOnly: false}, allProfileTools(b)...)

	cases := []struct {
		name   string
		args   map[string]any
		wantIn string
	}{
		{name: "omada_group_profile_list", args: map[string]any{}, wantIn: "site_id is required"},
		{name: "omada_group_profile_list", args: map[string]any{"site_id": "site-1", "group_type": "6"}, wantIn: "group_type must be one of"},
		{name: "omada_group_profile_create", args: map[string]any{"body": map[string]any{"name": "x", "type": 0}}, wantIn: "site_id is required"},
		{name: "omada_group_profile_modify", args: map[string]any{"site_id": "site-1", "group_type": "ip", "group_id": "grp-1", "body": map[string]any{}}, wantIn: "group_type must be one of"},
		{name: "omada_group_profile_delete", args: map[string]any{"site_id": "site-1", "group_type": "1"}, wantIn: "group_id is required"},
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
