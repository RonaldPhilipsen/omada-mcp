// Package mcpkit is the omada-mcp MCP plumbing: server construction,
// tool registration with read-only and group gating, and arg/result
// helpers shared by every tool. It wraps the official
// github.com/modelcontextprotocol/go-sdk mcp package.
package mcpkit

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Version is the server version reported in MCP initialize responses.
// It may be overridden at build time via -ldflags
// ("-X github.com/RonaldPhilipsen/omada-mcp/internal/mcpkit.Version=...").
var Version = "0.1.0"

// Options gates which tools a Server exposes.
type Options struct {
	// ReadOnly hides every mutating tool from clients entirely
	// (gating happens at registration, not at call time).
	ReadOnly bool
	// ToolGroups limits the exposed tools to those whose Group is listed.
	// nil means all groups.
	ToolGroups []string
}

// HandlerFunc handles a tool call with the raw JSON arguments of the
// request.
type HandlerFunc func(context.Context, *Args) (*mcp.CallToolResult, error)

// Tool describes an omada-mcp tool in an SDK-independent way.
type Tool struct {
	// Name is the tool name, "omada_..." / "ops_...".
	Name string
	// Description is the human-readable tool description.
	Description string
	// Group matches config.ToolGroups entries; "" == always present.
	Group string
	// Mutating marks tools that change controller state. They are only
	// exposed when Options.ReadOnly is false.
	Mutating bool
	// Register, when non-nil, is called with the SDK Tool before it is
	// added to the server, for extra ToolOption building (schemas,
	// annotations). It must leave Name set.
	Register func(*mcp.Tool)
	// Handler handles tool calls.
	Handler HandlerFunc
}

// Server is the omada-mcp MCP server core.
type Server struct {
	mcp  *mcp.Server
	opts Options
	// all holds every tool ever passed to Register, regardless of gating.
	all []Tool
}

// NewServer creates a Server exposing tools according to opt.
func NewServer(name string, opt Options) *Server {
	return &Server{
		mcp:  mcp.NewServer(&mcp.Implementation{Name: name, Version: Version}, nil),
		opts: opt,
	}
}

// groupAllowed reports whether a tool with the given group may be exposed.
// The empty group is always present; nil groups means all groups.
func groupAllowed(group string, groups []string) bool {
	if group == "" || groups == nil {
		return true
	}
	for _, g := range groups {
		if g == group {
			return true
		}
	}
	return false
}

// Register applies read-only and group gating silently and, when the tool
// passes, adds it to the underlying SDK server. Every tool passed to
// Register, gated or not, is visible via Tools.
func (s *Server) Register(t Tool) {
	s.all = append(s.all, t)

	if t.Mutating && s.opts.ReadOnly {
		return
	}
	if !groupAllowed(t.Group, s.opts.ToolGroups) {
		return
	}

	tt := &mcp.Tool{
		Name:        t.Name,
		Description: t.Description,
	}
	if t.Register != nil {
		t.Register(tt)
	}
	if tt.InputSchema == nil {
		tt.InputSchema = map[string]any{"type": "object"}
	}
	s.mcp.AddTool(tt, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := Args(req.Params.Arguments)
		return t.Handler(ctx, &args)
	})
}

// Tools returns all registered tools regardless of gating.
func (s *Server) Tools() []Tool {
	return append([]Tool(nil), s.all...)
}

// MCPServer returns the underlying SDK server.
func (s *Server) MCPServer() *mcp.Server {
	return s.mcp
}

// Args is the raw JSON object of tool arguments.
type Args = json.RawMessage

// OnArgs decodes tool args JSON object into T.
func OnArgs[T any](a *Args) (*T, error) {
	var out T
	if err := json.Unmarshal(*a, &out); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}
	return &out, nil
}

// JSONResult returns a tool result with v marshaled as JSON text content.
func JSONResult(v any) (*mcp.CallToolResult, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(b)}},
	}, nil
}

// Run serves s until ctx is canceled: transport "stdio" speaks MCP over
// stdin/stdout; anything else is a streamable HTTP handler mounted at
// /mcp, listening on :port.
func Run(ctx context.Context, s *Server, transport string, port int) error {
	if transport == "stdio" {
		return s.mcp.Run(ctx, &mcp.StdioTransport{})
	}

	mux := http.NewServeMux()
	mux.Handle("/mcp", mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return s.mcp
	}, nil))
	httpSrv := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
		BaseContext: func(net.Listener) context.Context {
			return ctx
		},
	}

	errCh := make(chan error, 1)
	go func() { errCh <- httpSrv.ListenAndServe() }()

	select {
	case err := <-errCh:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return httpSrv.Shutdown(shutdownCtx)
	}
}
