// Command omada-mcp is the omada-mcp server binary. This main is a
// temporary scaffold demo (one omada_echo tool over both transports);
// the real tool wiring and configuration arrive in later tasks.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/RonaldPhilipsen/omada-mcp/internal/mcpkit"
)

func main() {
	transport := flag.String("transport", "streamable-http", `MCP transport: "streamable-http" (default) or "stdio"`)
	port := flag.Int("port", 8080, "port for the streamable-http transport")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	s := mcpkit.NewServer("omada", mcpkit.Options{ReadOnly: true})
	s.Register(mcpkit.Tool{
		Name:        "omada_echo",
		Description: "temporary demo tool: echoes the q argument back as JSON",
		Handler: func(_ context.Context, args *mcpkit.Args) (*mcp.CallToolResult, error) {
			in, err := mcpkit.OnArgs[struct {
				Q string `json:"q"`
			}](args)
			if err != nil {
				return nil, err
			}
			return mcpkit.JSONResult(map[string]string{"q": in.Q})
		},
	})

	if err := mcpkit.Run(ctx, s, *transport, *port); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
