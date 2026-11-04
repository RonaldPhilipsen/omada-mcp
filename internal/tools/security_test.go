// Package tools_test pins the curated security group: ACL rules
// (gateway/AP/switch) and MAC filtering. Each test drives a real mcpkit
// server over the SDK's in-memory transport pair against a real fake
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

func allSecurityTools(b *tools.Base) []mcpkit.Tool {
	return []mcpkit.Tool{
		b.AclGatewayList(), b.AclGatewayCreate(), b.AclGatewayModify(),
		b.AclApList(), b.AclApCreate(), b.AclApModify(),
		b.AclSwitchList(), b.AclSwitchCreate(), b.AclSwitchModify(),
		b.AclDelete(),
		b.MacFilterAllowList(), b.MacFilterDenyList(), b.MacFilterAdd(), b.MacFilterRemove(),
		b.SsidMacFilterModify(),
	}
}

func aclRow() map[string]any {
	return map[string]any{
		"id":              "acl-1",
		"index":           1,
		"description":     "Block IoT VLAN",
		"policy":          0,
		"status":          true,
		"sourceType":      0,
		"destinationType": 0,
		"destinationIds":  []string{"net-10"},
		"sourceIds":       []string{"net-10"},
		"protocols":       []int{6},
		"direction":       map[string]any{"lanToWan": true},
		"stateMode":       1,
		"states":          map[string]any{"established": true},
		"syslog":          true,
		"timeRangeId":     "tr-1",
	}
}

// TestAclGatewayListRoundTrip pins omada_acl_gateway_list: the required
// site_id is used in the request path, the request carries the default
// paging query, and the decoded ACL rows come back in the content.
func TestAclGatewayListRoundTrip(t *testing.T) {
	b, fake := newBase(t)
	mux := fake.Handler().(*http.ServeMux)
	mux.Handle("GET /openapi/v1/{omadacId}/sites/{siteId}/acls/osg-acls", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fakectrl.EnvelopeJSON(w, 0, map[string]any{"data": []map[string]any{aclRow()}})
	}))

	cs := newSession(t, mcpkit.Options{ReadOnly: false}, allSecurityTools(b)...)
	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "omada_acl_gateway_list",
		Arguments: map[string]any{"omadac_id": "omac-456", "site_id": "site-1"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	text, isError := toolText(result)
	if isError {
		t.Fatalf("omada_acl_gateway_list errored: %s", text)
	}
	var got struct {
		Data []struct {
			ID     string `json:"id"`
			Policy int    `json:"policy"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(text), &got); err != nil {
		t.Fatalf("content %q is not JSON: %v", text, err)
	}
	if len(got.Data) != 1 || got.Data[0].ID != "acl-1" || got.Data[0].Policy != 0 {
		t.Errorf("decoded rows = %+v, want one ACL acl-1 with policy 0 (drop)", got.Data)
	}

	req := entryFor(t, fake.Requests(), "/openapi/v1/omac-456/sites/site-1/acls/osg-acls")
	if req.Method != http.MethodGet {
		t.Errorf("method = %q, want GET", req.Method)
	}
	if req.Query != "page=1&pageSize=20" {
		t.Errorf("query = %q, want page=1&pageSize=20", req.Query)
	}
}

// TestAclGatewayCreateRoundTrip pins omada_acl_gateway_create: the body is
// sent to the controller mirroring the Omada request type and the created
// ACL comes back in the content.
func TestAclGatewayCreateRoundTrip(t *testing.T) {
	b, fake := newBase(t)
	mux := fake.Handler().(*http.ServeMux)
	mux.Handle("POST /openapi/v1/{omadacId}/sites/{siteId}/acls/osg-acls", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fakectrl.EnvelopeJSON(w, 0, aclRow())
	}))

	cs := newSession(t, mcpkit.Options{ReadOnly: false}, allSecurityTools(b)...)
	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "omada_acl_gateway_create",
		Arguments: map[string]any{
			"site_id": "site-1",
			"body": map[string]any{
				"description":     "Block IoT VLAN",
				"policy":          0,
				"status":          true,
				"sourceType":      0,
				"destinationType": 0,
				"destinationIds":  []string{"net-10"},
				"sourceIds":       []string{"net-10"},
				"protocols":       []int{6},
				"direction":       map[string]any{"lanToWan": true},
				"stateMode":       1,
				"states":          map[string]any{"established": true},
			},
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	text, isError := toolText(result)
	if isError {
		t.Fatalf("omada_acl_gateway_create errored: %s", text)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(text), &got); err != nil {
		t.Fatalf("content %q is not JSON: %v", text, err)
	}
	if got["id"] != "acl-1" || got["policy"] != float64(0) {
		t.Errorf("created entity = %v, want acl-1 with policy 0 (drop)", got)
	}

	req := entryFor(t, fake.Requests(), "/openapi/v1/"+omadacConfigured+"/sites/site-1/acls/osg-acls")
	if req.Method != http.MethodPost {
		t.Errorf("method = %q, want POST", req.Method)
	}
	var sent struct {
		Description     string   `json:"description"`
		Policy          int      `json:"policy"`
		Status          bool     `json:"status"`
		DestinationType int      `json:"destinationType"`
		DestinationIds  []string `json:"destinationIds"`
	}
	if err := json.Unmarshal(req.Body, &sent); err != nil {
		t.Fatalf("request body %q is not JSON: %v", req.Body, err)
	}
	if sent.Description != "Block IoT VLAN" || sent.Policy != 0 || !sent.Status || sent.DestinationType != 0 || len(sent.DestinationIds) != 1 {
		t.Errorf("sent body = %+v, want the mirrored ACL config fields", sent)
	}
}

// TestAclGatewayModifyRoundTrip pins omada_acl_gateway_modify: the acl_id
// goes into the request path and the body is sent mirroring the Omada
// request type.
func TestAclGatewayModifyRoundTrip(t *testing.T) {
	b, fake := newBase(t)
	mux := fake.Handler().(*http.ServeMux)
	mux.Handle("PUT /openapi/v1/{omadacId}/sites/{siteId}/acls/osg-acls/{aclId}", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fakectrl.EnvelopeJSON(w, 0, nil)
	}))

	cs := newSession(t, mcpkit.Options{ReadOnly: false}, allSecurityTools(b)...)
	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "omada_acl_gateway_modify",
		Arguments: map[string]any{
			"site_id": "site-1",
			"acl_id":  "acl-1",
			"body":    map[string]any{"status": false, "description": "disabled"},
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	text, isError := toolText(result)
	if isError {
		t.Fatalf("omada_acl_gateway_modify errored: %s", text)
	}
	if text != "{}" {
		t.Errorf("result = %q, want {} for a null result payload", text)
	}

	req := entryFor(t, fake.Requests(), "/openapi/v1/"+omadacConfigured+"/sites/site-1/acls/osg-acls/acl-1")
	if req.Method != http.MethodPut {
		t.Errorf("method = %q, want PUT", req.Method)
	}
	var sent struct {
		Status      bool   `json:"status"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal(req.Body, &sent); err != nil {
		t.Fatalf("request body %q is not JSON: %v", req.Body, err)
	}
	if sent.Status || sent.Description != "disabled" {
		t.Errorf("sent body = %+v, want status false and description disabled", sent)
	}
}

// TestAclApListRoundTrip pins omada_acl_ap_list: the decoded AP ACL rows
// come back from /acls/eap-acls.
func TestAclApListRoundTrip(t *testing.T) {
	b, fake := newBase(t)
	mux := fake.Handler().(*http.ServeMux)
	mux.Handle("GET /openapi/v1/{omadacId}/sites/{siteId}/acls/eap-acls", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fakectrl.EnvelopeJSON(w, 0, map[string]any{"data": []map[string]any{{"id": "acl-ap-1", "policy": "permit", "status": true}}})
	}))

	cs := newSession(t, mcpkit.Options{ReadOnly: false}, allSecurityTools(b)...)
	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "omada_acl_ap_list",
		Arguments: map[string]any{"site_id": "site-1"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	text, isError := toolText(result)
	if isError {
		t.Fatalf("omada_acl_ap_list errored: %s", text)
	}
	var got struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(text), &got); err != nil {
		t.Fatalf("content %q is not JSON: %v", text, err)
	}
	if len(got.Data) != 1 || got.Data[0].ID != "acl-ap-1" {
		t.Errorf("decoded rows = %+v, want one AP ACL acl-ap-1", got.Data)
	}

	entryFor(t, fake.Requests(), "/openapi/v1/"+omadacConfigured+"/sites/site-1/acls/eap-acls")
}

// TestAclApCreateRoundTrip pins omada_acl_ap_create: the body is sent to
// /acls/eap-acls mirroring the Omada request type.
func TestAclApCreateRoundTrip(t *testing.T) {
	b, fake := newBase(t)
	mux := fake.Handler().(*http.ServeMux)
	mux.Handle("POST /openapi/v1/{omadacId}/sites/{siteId}/acls/eap-acls", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fakectrl.EnvelopeJSON(w, 0, map[string]any{"id": "acl-ap-9", "policy": "deny"})
	}))

	cs := newSession(t, mcpkit.Options{ReadOnly: false}, allSecurityTools(b)...)
	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "omada_acl_ap_create",
		Arguments: map[string]any{
			"site_id": "site-1",
			"body":    map[string]any{"description": "AP rule", "policy": 0, "sourceType": 0, "destinationType": 0, "status": true},
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	text, isError := toolText(result)
	if isError {
		t.Fatalf("omada_acl_ap_create errored: %s", text)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(text), &got); err != nil {
		t.Fatalf("content %q is not JSON: %v", text, err)
	}
	if got["id"] != "acl-ap-9" {
		t.Errorf("created entity = %v, want acl-ap-9", got)
	}

	req := entryFor(t, fake.Requests(), "/openapi/v1/"+omadacConfigured+"/sites/site-1/acls/eap-acls")
	if req.Method != http.MethodPost {
		t.Errorf("method = %q, want POST", req.Method)
	}
}

// TestAclApModifyRoundTrip pins omada_acl_ap_modify: the acl_id goes into
// the /acls/eap-acls/{aclId} path and the body is sent.
func TestAclApModifyRoundTrip(t *testing.T) {
	b, fake := newBase(t)
	mux := fake.Handler().(*http.ServeMux)
	mux.Handle("PUT /openapi/v1/{omadacId}/sites/{siteId}/acls/eap-acls/{aclId}", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fakectrl.EnvelopeJSON(w, 0, nil)
	}))

	cs := newSession(t, mcpkit.Options{ReadOnly: false}, allSecurityTools(b)...)
	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "omada_acl_ap_modify",
		Arguments: map[string]any{
			"site_id": "site-1",
			"acl_id":  "acl-ap-1",
			"body":    map[string]any{"status": false},
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	text, isError := toolText(result)
	if isError {
		t.Fatalf("omada_acl_ap_modify errored: %s", text)
	}
	if text != "{}" {
		t.Errorf("result = %q, want {} for a null result payload", text)
	}

	req := entryFor(t, fake.Requests(), "/openapi/v1/"+omadacConfigured+"/sites/site-1/acls/eap-acls/acl-ap-1")
	if req.Method != http.MethodPut {
		t.Errorf("method = %q, want PUT", req.Method)
	}
}

// TestAclSwitchListRoundTrip pins omada_acl_switch_list: the decoded
// switch ACL rows come back from /acls/osw-acls.
func TestAclSwitchListRoundTrip(t *testing.T) {
	b, fake := newBase(t)
	mux := fake.Handler().(*http.ServeMux)
	mux.Handle("GET /openapi/v1/{omadacId}/sites/{siteId}/acls/osw-acls", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fakectrl.EnvelopeJSON(w, 0, map[string]any{"data": []map[string]any{{"id": "acl-sw-1", "policy": "deny", "status": true}}})
	}))

	cs := newSession(t, mcpkit.Options{ReadOnly: false}, allSecurityTools(b)...)
	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "omada_acl_switch_list",
		Arguments: map[string]any{"site_id": "site-1"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	text, isError := toolText(result)
	if isError {
		t.Fatalf("omada_acl_switch_list errored: %s", text)
	}
	var got struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(text), &got); err != nil {
		t.Fatalf("content %q is not JSON: %v", text, err)
	}
	if len(got.Data) != 1 || got.Data[0].ID != "acl-sw-1" {
		t.Errorf("decoded rows = %+v, want one switch ACL acl-sw-1", got.Data)
	}

	entryFor(t, fake.Requests(), "/openapi/v1/"+omadacConfigured+"/sites/site-1/acls/osw-acls")
}

// TestAclSwitchCreateRoundTrip pins omada_acl_switch_create: the body is
// sent to /acls/osw-acls mirroring the Omada request type.
func TestAclSwitchCreateRoundTrip(t *testing.T) {
	b, fake := newBase(t)
	mux := fake.Handler().(*http.ServeMux)
	mux.Handle("POST /openapi/v1/{omadacId}/sites/{siteId}/acls/osw-acls", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fakectrl.EnvelopeJSON(w, 0, map[string]any{"id": "acl-sw-9", "policy": "deny"})
	}))

	cs := newSession(t, mcpkit.Options{ReadOnly: false}, allSecurityTools(b)...)
	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "omada_acl_switch_create",
		Arguments: map[string]any{
			"site_id": "site-1",
			"body":    map[string]any{"description": "Switch rule", "policy": 0, "sourceType": 0, "destinationType": 0, "status": true},
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	text, isError := toolText(result)
	if isError {
		t.Fatalf("omada_acl_switch_create errored: %s", text)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(text), &got); err != nil {
		t.Fatalf("content %q is not JSON: %v", text, err)
	}
	if got["id"] != "acl-sw-9" {
		t.Errorf("created entity = %v, want acl-sw-9", got)
	}

	req := entryFor(t, fake.Requests(), "/openapi/v1/"+omadacConfigured+"/sites/site-1/acls/osw-acls")
	if req.Method != http.MethodPost {
		t.Errorf("method = %q, want POST", req.Method)
	}
}

// TestAclSwitchModifyRoundTrip pins omada_acl_switch_modify: the acl_id
// goes into the /acls/osw-acls/{aclId} path and the body is sent.
func TestAclSwitchModifyRoundTrip(t *testing.T) {
	b, fake := newBase(t)
	mux := fake.Handler().(*http.ServeMux)
	mux.Handle("PUT /openapi/v1/{omadacId}/sites/{siteId}/acls/osw-acls/{aclId}", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fakectrl.EnvelopeJSON(w, 0, nil)
	}))

	cs := newSession(t, mcpkit.Options{ReadOnly: false}, allSecurityTools(b)...)
	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "omada_acl_switch_modify",
		Arguments: map[string]any{
			"site_id": "site-1",
			"acl_id":  "acl-sw-1",
			"body":    map[string]any{"status": false},
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	text, isError := toolText(result)
	if isError {
		t.Fatalf("omada_acl_switch_modify errored: %s", text)
	}
	if text != "{}" {
		t.Errorf("result = %q, want {} for a null result payload", text)
	}

	req := entryFor(t, fake.Requests(), "/openapi/v1/"+omadacConfigured+"/sites/site-1/acls/osw-acls/acl-sw-1")
	if req.Method != http.MethodPut {
		t.Errorf("method = %q, want PUT", req.Method)
	}
}

// TestAclDeleteRoundTrip pins omada_acl_delete: the shared ACL delete
// hits /acls/{aclId} and a null result payload comes back as an empty
// object.
func TestAclDeleteRoundTrip(t *testing.T) {
	b, fake := newBase(t)
	mux := fake.Handler().(*http.ServeMux)
	mux.Handle("DELETE /openapi/v1/{omadacId}/sites/{siteId}/acls/{aclId}", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fakectrl.EnvelopeJSON(w, 0, nil)
	}))

	cs := newSession(t, mcpkit.Options{ReadOnly: false}, allSecurityTools(b)...)
	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "omada_acl_delete",
		Arguments: map[string]any{
			"site_id": "site-1",
			"acl_id":  "acl-1",
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	text, isError := toolText(result)
	if isError {
		t.Fatalf("omada_acl_delete errored: %s", text)
	}
	if text != "{}" {
		t.Errorf("result = %q, want {} for a null result payload", text)
	}

	req := entryFor(t, fake.Requests(), "/openapi/v1/"+omadacConfigured+"/sites/site-1/acls/acl-1")
	if req.Method != http.MethodDelete {
		t.Errorf("method = %q, want DELETE", req.Method)
	}
}

// TestMacFilterAllowListRoundTrip pins omada_mac_filter_allow_list: the
// allow grid rows come back from /mac-filters/allow with the default
// paging query.
func TestMacFilterAllowListRoundTrip(t *testing.T) {
	b, fake := newBase(t)
	mux := fake.Handler().(*http.ServeMux)
	mux.Handle("GET /openapi/v1/{omadacId}/sites/{siteId}/mac-filters/allow", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fakectrl.EnvelopeJSON(w, 0, map[string]any{
			"data": []map[string]any{{"id": "mf-1", "name": "Allowed", "macAddresses": []string{"aa:bb:cc:dd:ee:01"}}},
		})
	}))

	cs := newSession(t, mcpkit.Options{ReadOnly: false}, allSecurityTools(b)...)
	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "omada_mac_filter_allow_list",
		Arguments: map[string]any{"site_id": "site-1"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	text, isError := toolText(result)
	if isError {
		t.Fatalf("omada_mac_filter_allow_list errored: %s", text)
	}
	var got struct {
		Data []struct {
			ID           string   `json:"id"`
			MacAddresses []string `json:"macAddresses"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(text), &got); err != nil {
		t.Fatalf("content %q is not JSON: %v", text, err)
	}
	if len(got.Data) != 1 || got.Data[0].ID != "mf-1" || got.Data[0].MacAddresses[0] != "aa:bb:cc:dd:ee:01" {
		t.Errorf("decoded rows = %+v, want one allow entry mf-1", got.Data)
	}

	req := entryFor(t, fake.Requests(), "/openapi/v1/"+omadacConfigured+"/sites/site-1/mac-filters/allow")
	if req.Query != "page=1&pageSize=20" {
		t.Errorf("query = %q, want page=1&pageSize=20", req.Query)
	}
}

// TestMacFilterDenyListRoundTrip pins omada_mac_filter_deny_list: the
// deny grid rows come back from /mac-filters/deny.
func TestMacFilterDenyListRoundTrip(t *testing.T) {
	b, fake := newBase(t)
	mux := fake.Handler().(*http.ServeMux)
	mux.Handle("GET /openapi/v1/{omadacId}/sites/{siteId}/mac-filters/deny", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fakectrl.EnvelopeJSON(w, 0, map[string]any{
			"data": []map[string]any{{"id": "mf-2", "name": "Denied", "macAddresses": []string{"aa:bb:cc:dd:ee:02"}}},
		})
	}))

	cs := newSession(t, mcpkit.Options{ReadOnly: false}, allSecurityTools(b)...)
	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "omada_mac_filter_deny_list",
		Arguments: map[string]any{"site_id": "site-1"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	text, isError := toolText(result)
	if isError {
		t.Fatalf("omada_mac_filter_deny_list errored: %s", text)
	}
	var got struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(text), &got); err != nil {
		t.Fatalf("content %q is not JSON: %v", text, err)
	}
	if len(got.Data) != 1 || got.Data[0].ID != "mf-2" {
		t.Errorf("decoded rows = %+v, want one deny entry mf-2", got.Data)
	}

	entryFor(t, fake.Requests(), "/openapi/v1/"+omadacConfigured+"/sites/site-1/mac-filters/deny")
}

// TestMacFilterAddRoundTrip pins omada_mac_filter_add: the body is sent to
// /mac-filters mirroring the Omada request type and the created entry comes
// back in the content.
func TestMacFilterAddRoundTrip(t *testing.T) {
	b, fake := newBase(t)
	mux := fake.Handler().(*http.ServeMux)
	mux.Handle("POST /openapi/v1/{omadacId}/sites/{siteId}/mac-filters", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fakectrl.EnvelopeJSON(w, 0, map[string]any{"id": "mf-9", "name": "New", "type": "allow"})
	}))

	cs := newSession(t, mcpkit.Options{ReadOnly: false}, allSecurityTools(b)...)
	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "omada_mac_filter_add",
		Arguments: map[string]any{
			"site_id": "site-1",
			"body":    map[string]any{"name": "New", "type": 0, "filterMode": 0, "macAddresses": []map[string]any{{"macAddress": "aa:bb:cc:dd:ee:09"}}},
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	text, isError := toolText(result)
	if isError {
		t.Fatalf("omada_mac_filter_add errored: %s", text)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(text), &got); err != nil {
		t.Fatalf("content %q is not JSON: %v", text, err)
	}
	if got["id"] != "mf-9" {
		t.Errorf("created entry = %v, want mf-9", got)
	}

	req := entryFor(t, fake.Requests(), "/openapi/v1/"+omadacConfigured+"/sites/site-1/mac-filters")
	if req.Method != http.MethodPost {
		t.Errorf("method = %q, want POST", req.Method)
	}
	var sent struct {
		Name         string `json:"name"`
		Type         int    `json:"type"`
		FilterMode   int    `json:"filterMode"`
		MacAddresses []struct {
			MacAddress string `json:"macAddress"`
		} `json:"macAddresses"`
	}
	if err := json.Unmarshal(req.Body, &sent); err != nil {
		t.Fatalf("request body %q is not JSON: %v", req.Body, err)
	}
	if sent.Name != "New" || sent.Type != 0 || sent.FilterMode != 0 || len(sent.MacAddresses) != 1 || sent.MacAddresses[0].MacAddress != "aa:bb:cc:dd:ee:09" {
		t.Errorf("sent body = %+v, want the mirrored MAC filter fields", sent)
	}
}

// TestMacFilterRemoveRoundTrip pins omada_mac_filter_remove: the DELETE
// hits /mac-filters/{filterId} and a null result payload comes back as an
// empty object.
func TestMacFilterRemoveRoundTrip(t *testing.T) {
	b, fake := newBase(t)
	mux := fake.Handler().(*http.ServeMux)
	mux.Handle("DELETE /openapi/v1/{omadacId}/sites/{siteId}/mac-filters/{filterId}", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fakectrl.EnvelopeJSON(w, 0, nil)
	}))

	cs := newSession(t, mcpkit.Options{ReadOnly: false}, allSecurityTools(b)...)
	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "omada_mac_filter_remove",
		Arguments: map[string]any{
			"site_id":   "site-1",
			"filter_id": "mf-1",
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	text, isError := toolText(result)
	if isError {
		t.Fatalf("omada_mac_filter_remove errored: %s", text)
	}
	if text != "{}" {
		t.Errorf("result = %q, want {} for a null result payload", text)
	}

	req := entryFor(t, fake.Requests(), "/openapi/v1/"+omadacConfigured+"/sites/site-1/mac-filters/mf-1")
	if req.Method != http.MethodDelete {
		t.Errorf("method = %q, want DELETE", req.Method)
	}
}

// TestSsidMacFilterModifyRoundTrip pins omada_ssid_mac_filter_modify: the
// ssid_id goes into the request path and the body is sent mirroring the
// Omada request type.
func TestSsidMacFilterModifyRoundTrip(t *testing.T) {
	b, fake := newBase(t)
	mux := fake.Handler().(*http.ServeMux)
	mux.Handle("PATCH /openapi/v1/{omadacId}/sites/{siteId}/wireless-network/ssids/{ssidId}/mac-filter", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fakectrl.EnvelopeJSON(w, 0, nil)
	}))

	cs := newSession(t, mcpkit.Options{ReadOnly: false}, allSecurityTools(b)...)
	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "omada_ssid_mac_filter_modify",
		Arguments: map[string]any{
			"site_id": "site-1",
			"ssid_id": "ssid-1",
			"body":    map[string]any{"macFilterEnable": true, "macFilterId": "mf-1"},
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	text, isError := toolText(result)
	if isError {
		t.Fatalf("omada_ssid_mac_filter_modify errored: %s", text)
	}
	if text != "{}" {
		t.Errorf("result = %q, want {} for a null result payload", text)
	}

	req := entryFor(t, fake.Requests(), "/openapi/v1/"+omadacConfigured+"/sites/site-1/wireless-network/ssids/ssid-1/mac-filter")
	if req.Method != http.MethodPatch {
		t.Errorf("method = %q, want PATCH", req.Method)
	}
	var sent struct {
		MacFilterEnable bool   `json:"macFilterEnable"`
		MacFilterId     string `json:"macFilterId"`
	}
	if err := json.Unmarshal(req.Body, &sent); err != nil {
		t.Fatalf("request body %q is not JSON: %v", req.Body, err)
	}
	if !sent.MacFilterEnable || sent.MacFilterId != "mf-1" {
		t.Errorf("sent body = %+v, want macFilterEnable true and macFilterId mf-1", sent)
	}
}

// TestSecurityBusinessError pins the envelope business-error path: a
// non-zero errorCode surfaces as a tool error carrying the APIError
// message, for a read tool and a write tool.
func TestSecurityBusinessError(t *testing.T) {
	b, fake := newBase(t)
	mux := fake.Handler().(*http.ServeMux)
	mux.Handle("GET /openapi/v1/{omadacId}/sites/{siteId}/acls/osg-acls", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fakectrl.EnvelopeJSON(w, -1171, nil)
	}))
	mux.Handle("POST /openapi/v1/{omadacId}/sites/{siteId}/acls/osg-acls", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fakectrl.EnvelopeJSON(w, -1172, nil)
	}))

	cs := newSession(t, mcpkit.Options{ReadOnly: false}, allSecurityTools(b)...)

	cases := []struct {
		name   string
		args   map[string]any
		wantIn string
	}{
		{name: "omada_acl_gateway_list", args: map[string]any{"site_id": "site-1"}, wantIn: "API error -1171"},
		{name: "omada_acl_gateway_create", args: map[string]any{"site_id": "site-1", "body": map[string]any{"policy": 0, "status": true}}, wantIn: "API error -1172"},
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

// TestSecurityReadOnlyGating pins registration gating: with ReadOnly=true
// the mutating tools are absent from ListTools while the read tools are
// present; with ReadOnly=false all fifteen are listed.
func TestSecurityReadOnlyGating(t *testing.T) {
	b, _ := newBase(t)

	cs := newSession(t, mcpkit.Options{ReadOnly: true}, allSecurityTools(b)...)
	listed, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	seen := make(map[string]bool)
	for _, tl := range listed.Tools {
		seen[tl.Name] = true
	}
	for _, want := range []string{"omada_acl_gateway_list", "omada_acl_ap_list", "omada_acl_switch_list", "omada_mac_filter_allow_list", "omada_mac_filter_deny_list"} {
		if !seen[want] {
			t.Errorf("read-only: read tool %s missing, listed = %v", want, seen)
		}
	}
	for _, hidden := range []string{"omada_acl_gateway_create", "omada_acl_gateway_modify", "omada_acl_ap_create", "omada_acl_ap_modify", "omada_acl_switch_create", "omada_acl_switch_modify", "omada_acl_delete", "omada_mac_filter_add", "omada_mac_filter_remove", "omada_ssid_mac_filter_modify"} {
		if seen[hidden] {
			t.Errorf("read-only: mutating tool %s exposed, listed = %v", hidden, seen)
		}
	}

	cs2 := newSession(t, mcpkit.Options{ReadOnly: false}, allSecurityTools(b)...)
	listed2, err := cs2.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	seen2 := make(map[string]bool)
	for _, tl := range listed2.Tools {
		seen2[tl.Name] = true
	}
	for _, want := range []string{
		"omada_acl_gateway_list", "omada_acl_gateway_create", "omada_acl_gateway_modify",
		"omada_acl_ap_list", "omada_acl_ap_create", "omada_acl_ap_modify",
		"omada_acl_switch_list", "omada_acl_switch_create", "omada_acl_switch_modify",
		"omada_acl_delete",
		"omada_mac_filter_allow_list", "omada_mac_filter_deny_list", "omada_mac_filter_add", "omada_mac_filter_remove",
		"omada_ssid_mac_filter_modify",
	} {
		if !seen2[want] {
			t.Errorf("read-write: %s not listed, listed = %v", want, seen2)
		}
	}
}

// TestSecurityMissingRequiredArgs pins required-arg validation: missing
// site_id, acl_id and filter_id each fail as a tool error BEFORE any HTTP
// call reaches the fake controller.
func TestSecurityMissingRequiredArgs(t *testing.T) {
	b, fake := newBase(t)
	mux := fake.Handler().(*http.ServeMux)
	mux.Handle("DELETE /openapi/v1/{omadacId}/sites/{siteId}/acls/{aclId}", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("fake controller received a route request; required-arg validation must fail first")
		fakectrl.EnvelopeJSON(w, 0, nil)
	}))
	mux.Handle("DELETE /openapi/v1/{omadacId}/sites/{siteId}/mac-filters/{filterId}", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("fake controller received a route request; required-arg validation must fail first")
		fakectrl.EnvelopeJSON(w, 0, nil)
	}))

	before := len(fake.Requests())
	cs := newSession(t, mcpkit.Options{ReadOnly: false}, allSecurityTools(b)...)

	cases := []struct {
		name   string
		args   map[string]any
		wantIn string
	}{
		{name: "omada_acl_delete", args: nil, wantIn: "site_id is required"},
		{name: "omada_acl_delete", args: map[string]any{"site_id": "site-1"}, wantIn: "acl_id is required"},
		{name: "omada_mac_filter_remove", args: nil, wantIn: "site_id is required"},
		{name: "omada_mac_filter_remove", args: map[string]any{"site_id": "site-1"}, wantIn: "filter_id is required"},
	}
	for _, tc := range cases {
		result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: tc.name, Arguments: tc.args})
		if err == nil {
			text, _ := toolText(result)
			t.Fatalf("%s %v: not a tool error, content %q", tc.name, tc.args, text)
		}
		if !strings.Contains(err.Error(), tc.wantIn) {
			t.Errorf("%s %v: tool error %q does not name %q", tc.name, tc.args, err, tc.wantIn)
		}
	}

	if after := len(fake.Requests()); after != before {
		t.Errorf("captured requests after failed calls = %d, want %d (validation must fail before any HTTP)", after, before)
	}
}
