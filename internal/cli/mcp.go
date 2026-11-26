package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"slices"

	"github.com/githiago-f/lazyapi/internal/mcp"
	"github.com/githiago-f/lazyapi/internal/permit"
	"github.com/githiago-f/lazyapi/internal/store"
)

// Version is reported to MCP clients; overridden at build time.
var Version = "dev"

const mcpUsage = `usage: lazyapi mcp [--root dir] [--allow origin]... [--allow-writes origin]... [--permissions ask|off]

Serves LazyAPI over the Model Context Protocol on stdio. Spec, env and script
paths given by the client are resolved relative to --root (default: the
current directory) and may not leave it.

Requests only go to servers the user allowed. A request to any other server
is held while the user is asked through the client (MCP elicitation):
localhost once, other servers for one request, the session or always, reads
and writes separately. Clients that cannot ask get a refusal explaining how
the user allows it. Remembered answers: lazyapi allow --list.

  --allow origin         allow reads (GET, HEAD, OPTIONS) from an origin, e.g.
                         https://api.example.com; "localhost" allows local servers
  --allow-writes origin  allow every method on an origin
  --permissions off      send anywhere without asking (trusted automation only)`

// ServeMCP implements `lazyapi mcp`.
func ServeMCP(args []string) error {
	fs := newFlagSet("mcp")
	root := fs.String("root", ".", "directory the tools may read and write")
	var allow, allowWrites listFlag
	fs.Var(&allow, "allow", "origin allowed for reads (repeatable)")
	fs.Var(&allowWrites, "allow-writes", "origin allowed for every method (repeatable)")
	mode := fs.String("permissions", "ask", "ask (default) or off")
	pos, err := parseInterleaved(fs, args)
	if err != nil || len(pos) != 0 {
		return usageErr("%s", mcpUsage)
	}
	if st, err := os.Stat(*root); err != nil || !st.IsDir() {
		return usageErr("--root %s is not a directory", *root)
	}
	for _, o := range append(slices.Clone(allow), allowWrites...) {
		if _, err := permit.Normalize(o); err != nil {
			return usageErr("--allow: %v", err)
		}
	}
	var policy *permit.Policy
	switch *mode {
	case "ask":
		store, err := permit.DefaultStore()
		if err != nil {
			return runtimeErr(err)
		}
		policy = &permit.Policy{Store: store, Allow: allow, AllowWrites: allowWrites}
	case "off":
		fmt.Fprintln(os.Stderr, "lazyapi mcp: --permissions off: requests go to any server without asking")
	default:
		return usageErr("--permissions must be ask or off")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	srv := &mcp.Server{
		Name:    "lazyapi",
		Version: Version,
		Tools:   &mcp.Tools{Root: *root, FindSpecs: store.FindSpecFiles, Permit: policy},
	}
	if policy != nil {
		policy.Ask = srv.Asker
	}
	if err := srv.Serve(ctx, os.Stdin, os.Stdout); err != nil && ctx.Err() == nil {
		return runtimeErr(err)
	}
	return nil
}
