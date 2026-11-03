// Package mcpkit is the omada-mcp MCP plumbing: server construction,
// tool registration with read-only and group gating, and arg/result
// helpers shared by every tool.
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

type Options struct {
	// ReadOnly hides every mutating tool from clients entirely
	// (gating happens at registration, not at call time).
	ReadOnly bool
	// ToolGroups limits the exposed tools to those whose Group is listed.
	// nil means all groups.
	ToolGroups []string
}

type HandlerFunc func(context.Context, *Args) (*mcp.CallToolResult, error)

// Tool describes an omada-mcp tool in an SDK-independent way.
type Tool struct {
	Name        string
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
	Handler  HandlerFunc
}

type Server struct {
	mcp  *mcp.Server
	opts Options
	// all holds every tool ever passed to Register, regardless of gating.
	all []Tool
}

func NewServer(name string, opt Options) *Server {
	return &Server{
		mcp:  mcp.NewServer(&mcp.Implementation{Name: name, Version: Version}, nil),
		opts: opt,
	}
}

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

func (s *Server) MCPServer() *mcp.Server {
	return s.mcp
}

type Args = json.RawMessage

func OnArgs[T any](a *Args) (*T, error) {
	var out T
	if err := json.Unmarshal(*a, &out); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}
	return &out, nil
}

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
