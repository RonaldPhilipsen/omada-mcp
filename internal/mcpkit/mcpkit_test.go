// Copyright 2026 Ronald Philipsen. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package mcpkit_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/RonaldPhilipsen/omada-mcp/internal/mcpkit"
)

// newSession wires the real server under test to a real SDK client session
// through the SDK's in-memory transport pair, exercising the full stack
// (no mocks).
func newSession(t *testing.T, name string, opt mcpkit.Options, tools ...mcpkit.Tool) *mcp.ClientSession {
	t.Helper()

	s := mcpkit.NewServer(name, opt)
	for _, tool := range tools {
		s.Register(tool)
	}

	ctx := context.Background()
	srvT, cliT := mcp.NewInMemoryTransports()
	srvSession, err := s.MCPServer().Connect(ctx, srvT, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = srvSession.Close() })
	client := mcp.NewClient(&mcp.Implementation{Name: "mcpkit-test-client", Version: "0.0.0"}, nil)
	cs, err := client.Connect(ctx, cliT, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
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

// TestRegisterGating asserts read-only + group gating at registration:
// mutating tools are never listed when read-only, group filtering hides
// tools outside configured groups, and calling a hidden tool yields the
// tool-not-found shape instead of executing it.
func TestRegisterGating(t *testing.T) {
	tools := []mcpkit.Tool{
		{
			Name:        "omada_site_list",
			Description: "hidden: group site not configured",
			Group:       "site",
			Handler: func(context.Context, *mcpkit.Args) (*mcp.CallToolResult, error) {
				return mcpkit.JSONResult(map[string]any{"ok": true})
			},
		},
		{
			Name:        "omada_device_list",
			Description: "kept: read tool in configured group device",
			Group:       "device",
			Handler: func(context.Context, *mcpkit.Args) (*mcp.CallToolResult, error) {
				return mcpkit.JSONResult(map[string]any{"ok": true})
			},
		},
		{
			Name:        "omada_firmware",
			Description: "hidden: mutating while read-only, in kept group device",
			Group:       "device",
			Mutating:    true,
			Handler: func(context.Context, *mcpkit.Args) (*mcp.CallToolResult, error) {
				return mcpkit.JSONResult(map[string]any{"ok": true})
			},
		},
		{
			Name:        "omada_reboot",
			Description: "hidden: mutating and group site not configured",
			Group:       "site",
			Mutating:    true,
			Handler: func(context.Context, *mcpkit.Args) (*mcp.CallToolResult, error) {
				return mcpkit.JSONResult(map[string]any{"ok": true})
			},
		},
	}

	cs := newSession(t, "omada", mcpkit.Options{ReadOnly: true, ToolGroups: []string{"device"}}, tools...)

	listed, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, tl := range listed.Tools {
		names = append(names, tl.Name)
	}
	if len(names) != 1 || names[0] != "omada_device_list" {
		t.Fatalf("listed tools = %v, want exactly [omada_device_list]", names)
	}

	if _, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "omada_firmware"}); err == nil {
		t.Fatal("calling hidden tool omada_firmware: want error, got nil")
	} else if msg := err.Error(); !strings.Contains(msg, "unknown tool") {
		t.Errorf("hidden tool call error = %q, want tool-not-found shape", msg)
	}

	// A second registration with ReadOnly:false lists all tools incl. the
	// mutating ones: gating is per-Options, not global.
	cs2 := newSession(t, "omada", mcpkit.Options{ReadOnly: false}, tools...)
	listed2, err := cs2.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	seen := make(map[string]bool)
	for _, tl := range listed2.Tools {
		seen[tl.Name] = true
	}
	for _, want := range []string{"omada_site_list", "omada_device_list", "omada_firmware", "omada_reboot"} {
		if !seen[want] {
			t.Errorf("ToolGroups nil and ReadOnly false: %s not listed (got %v)", want, seen)
		}
	}
}

// TestOnArgsAndJSONResult exercises argument decoding and JSON results
// through a real session: valid args round-trip via the text content, and
// invalid args produce an error mentioning "invalid arguments".
func TestOnArgsAndJSONResult(t *testing.T) {
	cs := newSession(t, "omada", mcpkit.Options{ReadOnly: true}, mcpkit.Tool{
		Name:        "omada_echo",
		Description: "echoes the q argument",
		Handler: func(_ context.Context, args *mcpkit.Args) (*mcp.CallToolResult, error) {
			in, err := mcpkit.OnArgs[struct {
				Q int `json:"q"`
			}](args)
			if err != nil {
				return nil, err
			}
			return mcpkit.JSONResult(map[string]any{"q": in.Q})
		},
	})

	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "omada_echo",
		Arguments: map[string]any{"q": 42},
	})
	if err != nil {
		t.Fatal(err)
	}
	text, isError := toolText(result)
	if isError {
		t.Fatalf("omada_echo errored: %s", text)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(text), &got); err != nil {
		t.Fatalf("content %q is not JSON: %v", text, err)
	}
	if got["q"].(float64) != 42 {
		t.Errorf("round-tripped q = %v, want 42", got["q"])
	}

	_, err = cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "omada_echo",
		Arguments: map[string]any{"q": "not-a-number"},
	})
	if err == nil {
		t.Fatal("invalid args: want error, got nil")
	}
	if msg := err.Error(); !strings.Contains(msg, "invalid arguments") {
		t.Errorf(`invalid args error = %q, want it to mention "invalid arguments"`, msg)
	}
}

// TestNoSecrets establishes the scan pattern later tasks reuse for leakage
// checks: output embedding a secret marker marshals verbatim, so scanning
// the marshaled result catches leakage.
func TestNoSecrets(t *testing.T) {
	cs := newSession(t, "omada", mcpkit.Options{ReadOnly: true}, mcpkit.Tool{
		Name:        "omada_secret_speaker",
		Description: "embeds a secret marker in its output",
		Handler: func(context.Context, *mcpkit.Args) (*mcp.CallToolResult, error) {
			return mcpkit.JSONResult(map[string]any{"value": "OMADA_SECRET_TEST_MARKER"})
		},
	})

	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "omada_secret_speaker"})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "OMADA_SECRET_TEST_MARKER") {
		t.Errorf("marshaled session result %s does not contain the marker; scan pattern is broken", raw)
	}

	direct, err := mcpkit.JSONResult(map[string]any{"value": "OMADA_SECRET_TEST_MARKER"})
	if err != nil {
		t.Fatal(err)
	}
	rawDirect, err := json.Marshal(direct)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(rawDirect), "OMADA_SECRET_TEST_MARKER") {
		t.Errorf("JSONResult output %s does not contain the marker; scan pattern is broken", rawDirect)
	}
}
