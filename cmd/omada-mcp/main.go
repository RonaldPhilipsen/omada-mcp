// Command omada-mcp is the omada-mcp server binary. All logging goes to
// stderr; stdout is reserved for the stdio transport.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/RonaldPhilipsen/omada-mcp/internal/config"
	"github.com/RonaldPhilipsen/omada-mcp/internal/mcpkit"
	"github.com/RonaldPhilipsen/omada-mcp/internal/omadaclient"
	"github.com/RonaldPhilipsen/omada-mcp/internal/wire"
)

func main() {
	showVersion := flag.Bool("version", false, "print the version and exit")
	flag.Parse()
	if *showVersion {
		fmt.Println(mcpkit.Version)
		return
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := config.FromEnv(os.Getenv)
	if err != nil {
		logger.Error("configuration failed", "error", err)
		os.Exit(1)
	}

	omc, err := omadaclient.New(cfg)
	if err != nil {
		logger.Error("omada client setup failed", "error", err)
		os.Exit(1)
	}

	srv, err := wire.Build(cfg, omc)
	if err != nil {
		logger.Error("tool assembly failed", "error", err)
		os.Exit(1)
	}

	logger.Info("omada-mcp starting",
		"transport", cfg.Transport,
		"port", cfg.Port,
		"read_only", cfg.ReadOnly,
		"enable_dispatch", cfg.EnableDispatch,
		"tool_groups", cfg.ToolGroups,
	)

	if err := mcpkit.Run(ctx, srv, cfg.Transport, cfg.Port); err != nil {
		logger.Error("server stopped with error", "error", err)
		os.Exit(1)
	}
	logger.Info("omada-mcp stopped")
}
