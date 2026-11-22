package cli

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/githiago-f/lazyapi/internal/mock"
)

const mockUsage = `usage: lazyapi mock <spec.yml> [--port 4010] [--host 127.0.0.1] [--no-validate] [--quiet]

Serves the spec as a fake API: each operation answers with its declared
example (or a sample generated from the schema). Requests that do not match
the spec get 422 with the reason. Choose a response with the Prefer header:
  Prefer: code=404            a declared status
  Prefer: example=<name>      a named example`

// ServeMock implements `lazyapi mock`.
func ServeMock(args []string) error {
	fs := newFlagSet("mock")
	port := fs.Int("port", 4010, "port to listen on (0 picks a free port)")
	host := fs.String("host", "127.0.0.1", "address to bind (use 0.0.0.0 to expose it)")
	noValidate := fs.Bool("no-validate", false, "answer requests that do not match the spec")
	quiet := fs.Bool("quiet", false, "do not log requests")
	pos, err := parseInterleaved(fs, args)
	if err != nil || len(pos) != 1 {
		return usageErr("%s", mockUsage)
	}
	spec, err := loadSpec(pos[0])
	if err != nil {
		return err
	}

	m := mock.New(spec, pos[0])
	m.Validate = !*noValidate
	if !*quiet {
		m.Log = os.Stderr
	}

	ln, err := net.Listen("tcp", net.JoinHostPort(*host, strconv.Itoa(*port)))
	if err != nil {
		return runtimeErr(err)
	}
	fmt.Printf("Mock for %s: %d operations on http://%s\n", pos[0], m.Operations(), ln.Addr())
	fmt.Printf("Try: lazyapi run %s --server http://%s\n", pos[0], ln.Addr())

	srv := &http.Server{Handler: m, ReadHeaderTimeout: 10 * time.Second}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdown)
	}()
	if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return runtimeErr(err)
	}
	return nil
}
