// Package wire_test pins the config-driven tool assembly end to end:
// OMADA_* env fixture → config.FromEnv → omadaclient.New (a real preflight
// token against the fake controller) → wire.Build → a real mcpkit server →
// a real SDK MCP client session (in-memory transport pair, or streamable
// HTTP over a real local port) → fakectrl. No mocks of the project's own
// packages anywhere.
package wire_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/RonaldPhilipsen/omada-mcp/internal/config"
	"github.com/RonaldPhilipsen/omada-mcp/internal/fakectrl"
	"github.com/RonaldPhilipsen/omada-mcp/internal/mcpkit"
	"github.com/RonaldPhilipsen/omada-mcp/internal/omadaclient"
	"github.com/RonaldPhilipsen/omada-mcp/internal/wire"
)

// curatedTools is the full curated catalog, pinned name by name.
var curatedTools = []string{
	"omada_site_list", "omada_site_get", "omada_site_create", "omada_site_delete",
	"omada_client_list", "omada_client_get", "omada_client_block", "omada_client_unblock", "omada_client_reconnect",
	"omada_device_list", "omada_devices_by_site", "omada_device_reboot", "omada_topology_get",
	"omada_lan_networks_list", "omada_lan_network_get", "omada_lan_network_create", "omada_lan_network_delete", "omada_dhcp_list",
	"omada_vpn_list", "omada_vpn_s2s_list", "omada_vpn_c2s_servers_list", "omada_vpn_s2s_modify",
	"omada_insight_overview", "omada_insight_unresolved_count", "omada_insight_incidents",
	"omada_log_alerts", "omada_log_events",
}

// mutatingTools is the curated subset that read-only mode hides.
var mutatingTools = []string{
	"omada_site_create", "omada_site_delete",
	"omada_client_block", "omada_client_unblock", "omada_client_reconnect",
	"omada_device_reboot",
	"omada_lan_network_create", "omada_lan_network_delete",
	"omada_vpn_s2s_modify",
}

// dispatchTools is the escape-hatch pair registered only when
// OMADA_ENABLE_DISPATCH is true.
var dispatchTools = []string{"ops_lookup", "ops_call"}

// buildServer runs the full assembly from an env fixture: config.FromEnv →
// omadaclient.New (real preflight token against the fake) → wire.Build.
// env overrides the four required fixture vars; the rest stay at the
// config defaults (read-only, no dispatch, streamable-http, all groups).
func buildServer(t *testing.T, env map[string]string) (*mcpkit.Server, *fakectrl.Fake) {
	t.Helper()
	fake := fakectrl.New(t)

	fixture := map[string]string{
		config.EnvBaseURL:      fake.URL(),
		config.EnvOmadacID:     "omac-e2e",
		config.EnvClientID:     "client-id-value",
		config.EnvClientSecret: "client-secret-value",
	}
	for k, v := range env {
		fixture[k] = v
	}

	cfg, err := config.FromEnv(func(k string) string { return fixture[k] })
	if err != nil {
		t.Fatalf("config.FromEnv: %v", err)
	}
	omc, err := omadaclient.New(cfg)
	if err != nil {
		t.Fatalf("omadaclient.New: %v", err)
	}
	srv, err := wire.Build(cfg, omc)
	if err != nil {
		t.Fatalf("wire.Build: %v", err)
	}
	return srv, fake
}

// newSession connects a real SDK client session to the server over the
// SDK's in-memory transport pair.
func newSession(t *testing.T, srv *mcpkit.Server) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()
	srvT, cliT := mcp.NewInMemoryTransports()
	srvSession, err := srv.MCPServer().Connect(ctx, srvT, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = srvSession.Close() })
	client := mcp.NewClient(&mcp.Implementation{Name: "wire-e2e-client", Version: "0.0.0"}, nil)
	cs, err := client.Connect(ctx, cliT, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

func toolNames(t *testing.T, cs *mcp.ClientSession) []string {
	t.Helper()
	listed, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(listed.Tools))
	for _, tl := range listed.Tools {
		names = append(names, tl.Name)
	}
	sort.Strings(names)
	return names
}

func wantExactly(t *testing.T, names []string, wants ...[]string) {
	t.Helper()
	want := make([]string, 0, 32)
	for _, w := range wants {
		want = append(want, w...)
	}
	sort.Strings(want)
	if !sameSet(names, want) {
		t.Fatalf("listed tools (%d) = %v, want exactly %v", len(names), names, want)
	}
}

func toolText(result *mcp.CallToolResult) (string, bool) {
	var text string
	for _, c := range result.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			text += tc.Text
		}
	}
	return text, result.IsError
}

func subtract(a, b []string) []string {
	drop := make(map[string]bool, len(b))
	for _, x := range b {
		drop[x] = true
	}
	var out []string
	for _, x := range a {
		if !drop[x] {
			out = append(out, x)
		}
	}
	return out
}

func contains(list []string, x string) bool {
	for _, v := range list {
		if v == x {
			return true
		}
	}
	return false
}

func sameSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	seen := make(map[string]bool, len(a))
	for _, x := range a {
		seen[x] = true
	}
	for _, x := range b {
		if !seen[x] {
			return false
		}
	}
	return true
}

func addSiteListRoute(fake *fakectrl.Fake) {
	mux := fake.Handler().(*http.ServeMux)
	mux.Handle("GET /openapi/v1/{omadacId}/sites", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fakectrl.EnvelopeJSON(w, 0, map[string]any{
			"data": []map[string]any{
				{"site": "site-1", "name": "Site One"},
				{"site": "site-2", "name": "Site Two"},
			},
		})
	}))
}

func assertSiteRows(t *testing.T, text string) {
	t.Helper()
	var got struct {
		Data []struct {
			Site string `json:"site"`
			Name string `json:"name"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(text), &got); err != nil {
		t.Fatalf("content %q is not JSON: %v", text, err)
	}
	if len(got.Data) != 2 || got.Data[0].Name != "Site One" || got.Data[1].Name != "Site Two" {
		t.Fatalf("decoded rows = %+v, want the two fake rows Site One/Site Two", got.Data)
	}
}

// TestWriteModeWithDispatch pins the full tool list in write mode with
// dispatch enabled: all 27 curated tools plus ops_lookup and ops_call.
func TestWriteModeWithDispatch(t *testing.T) {
	srv, _ := buildServer(t, map[string]string{
		config.EnvReadOnly:       "false",
		config.EnvEnableDispatch: "true",
	})
	cs := newSession(t, srv)
	wantExactly(t, toolNames(t, cs), curatedTools, dispatchTools)
}

// TestReadOnlyMode pins read-only mode: the nine mutating tools are absent
// while the dispatch pair stays present: 18 read tools + ops_lookup +
// ops_call.
func TestReadOnlyMode(t *testing.T) {
	srv, _ := buildServer(t, map[string]string{
		config.EnvEnableDispatch: "true",
	})
	cs := newSession(t, srv)
	wantExactly(t, toolNames(t, cs), subtract(curatedTools, mutatingTools), dispatchTools)

	names := toolNames(t, cs)
	for _, m := range mutatingTools {
		if contains(names, m) {
			t.Errorf("read-only: mutating tool %s is listed: %v", m, names)
		}
	}
}

// TestDispatchDisabled pins dispatch disabled: the curated catalog is
// complete and no ops_* tool exists.
func TestDispatchDisabled(t *testing.T) {
	srv, _ := buildServer(t, map[string]string{
		config.EnvReadOnly: "false",
	})
	cs := newSession(t, srv)
	wantExactly(t, toolNames(t, cs), curatedTools)

	names := toolNames(t, cs)
	for _, d := range dispatchTools {
		if contains(names, d) {
			t.Errorf("dispatch disabled: %s is listed: %v", d, names)
		}
	}
}

// TestToolGroupsFilter pins OMADA_TOOL_GROUPS=site: exactly the four site
// tools are exposed, nothing from any other group.
func TestToolGroupsFilter(t *testing.T) {
	srv, _ := buildServer(t, map[string]string{
		config.EnvReadOnly:   "false",
		config.EnvToolGroups: "site",
	})
	cs := newSession(t, srv)
	wantExactly(t, toolNames(t, cs), []string{
		"omada_site_list", "omada_site_get", "omada_site_create", "omada_site_delete",
	})
}

// TestSiteListEndToEnd pins a real end-to-end call of a curated tool
// through the full wire: env fixture → config → client (preflight) →
// wire.Build → session → fakectrl. The fake rows come back in the content.
func TestSiteListEndToEnd(t *testing.T) {
	srv, fake := buildServer(t, nil)
	addSiteListRoute(fake)
	cs := newSession(t, srv)

	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "omada_site_list"})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	text, isError := toolText(result)
	if isError {
		t.Fatalf("omada_site_list errored: %s", text)
	}
	assertSiteRows(t, text)
}

// TestOpsCallEndToEnd pins a real end-to-end ops_call through the full
// wire with dispatch enabled: the read-only getSiteList op goes through
// the client's auth transport to the fake and returns the fake rows.
func TestOpsCallEndToEnd(t *testing.T) {
	srv, fake := buildServer(t, map[string]string{
		config.EnvReadOnly:       "false",
		config.EnvEnableDispatch: "true",
	})
	addSiteListRoute(fake)
	cs := newSession(t, srv)

	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "ops_call",
		Arguments: map[string]any{"operation_id": "getSiteList"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	text, isError := toolText(result)
	if isError {
		t.Fatalf("ops_call errored: %s", text)
	}
	var got struct {
		Status int             `json:"status"`
		Result json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal([]byte(text), &got); err != nil {
		t.Fatalf("content %q is not JSON: %v", text, err)
	}
	if got.Status != 200 {
		t.Errorf("status = %d, want 200", got.Status)
	}
	assertSiteRows(t, string(got.Result))
}

// startHTTP serves srv the way mcpkit.Run serves the streamable HTTP
// transport: the same /mcp mount with the same handler, here on an
// httptest server bound to a random free port.
func startHTTP(t *testing.T, srv *mcpkit.Server) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.Handle("/mcp", mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return srv.MCPServer()
	}, nil))
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts
}

func connectHTTP(t *testing.T, ts *httptest.Server) *mcp.ClientSession {
	t.Helper()
	client := mcp.NewClient(&mcp.Implementation{Name: "wire-e2e-http-client", Version: "0.0.0"}, nil)
	cs, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{Endpoint: ts.URL + "/mcp"}, nil)
	if err != nil {
		t.Fatalf("client.Connect: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

// TestStreamableHTTPWholePath pins the whole-path test: env fixture with
// OMADA_TRANSPORT=streamable-http → config → client → wire.Build →
// mcpkit.Run-style handler on a random free port → real SDK client over
// HTTP. The read-only default lists the curated read set with no write and
// no dispatch tools; omada_site_list returns the fake rows; flipping the
// fixture to OMADA_ENABLE_DISPATCH=true makes the dispatch tools appear.
func TestStreamableHTTPWholePath(t *testing.T) {
	srv, fake := buildServer(t, map[string]string{
		config.EnvTransport: config.TransportStreamableHTTP,
	})
	addSiteListRoute(fake)

	cs := connectHTTP(t, startHTTP(t, srv))

	wantExactly(t, toolNames(t, cs), subtract(curatedTools, mutatingTools))
	for _, m := range mutatingTools {
		if contains(toolNames(t, cs), m) {
			t.Errorf("read-only default: mutating tool %s is listed", m)
		}
	}
	for _, d := range dispatchTools {
		if contains(toolNames(t, cs), d) {
			t.Errorf("dispatch not enabled: %s is listed", d)
		}
	}

	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "omada_site_list"})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	text, isError := toolText(result)
	if isError {
		t.Fatalf("omada_site_list errored: %s", text)
	}
	assertSiteRows(t, text)

	srv2, _ := buildServer(t, map[string]string{
		config.EnvTransport:      config.TransportStreamableHTTP,
		config.EnvEnableDispatch: "true",
	})
	cs2 := connectHTTP(t, startHTTP(t, srv2))
	wantExactly(t, toolNames(t, cs2), subtract(curatedTools, mutatingTools), dispatchTools)
}
