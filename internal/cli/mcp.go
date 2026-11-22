package cli

import (
	"context"
	"os"
	"os/signal"

	"github.com/githiago-f/lazyapi/internal/mcp"
	"github.com/githiago-f/lazyapi/internal/store"
)

// Version is reported to MCP clients; overridden at build time.
var Version = "dev"

const mcpUsage = `usage: lazyapi mcp [--root dir]

Serves LazyAPI over the Model Context Protocol on stdio. Spec, env and script
paths given by the client are resolved relative to --root (default: the
current directory) and may not leave it.`

// ServeMCP implements `lazyapi mcp`.
func ServeMCP(args []string) error {
	fs := newFlagSet("mcp")
	root := fs.String("root", ".", "directory the tools may read and write")
	pos, err := parseInterleaved(fs, args)
	if err != nil || len(pos) != 0 {
		return usageErr("%s", mcpUsage)
	}
	if st, err := os.Stat(*root); err != nil || !st.IsDir() {
		return usageErr("--root %s is not a directory", *root)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	srv := &mcp.Server{
		Name:    "lazyapi",
		Version: Version,
		Tools:   &mcp.Tools{Root: *root, FindSpecs: store.FindSpecFiles},
	}
	if err := srv.Serve(ctx, os.Stdin, os.Stdout); err != nil && ctx.Err() == nil {
		return runtimeErr(err)
	}
	return nil
}
