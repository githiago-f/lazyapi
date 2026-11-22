package runner

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/githiago-f/lazyapi/internal/store"
)

func TestRun_ParallelKeepsSpecOrder(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Earlier paths answer slower, so completion order is reversed.
		var n int
		_, _ = fmt.Sscanf(r.URL.Path, "/p%d", &n)
		time.Sleep(time.Duration(10-n) * 5 * time.Millisecond)
		w.WriteHeader(200)
	}))
	defer srv.Close()

	var b strings.Builder
	b.WriteString("openapi: 3.0.0\ninfo: {title: t, version: \"1\"}\npaths:\n")
	for i := 0; i < 8; i++ {
		fmt.Fprintf(&b, "  /p%d:\n    get:\n      operationId: op%d\n      responses: {\"200\": {description: OK}}\n", i, i)
	}
	path := filepath.Join(t.TempDir(), "spec.yml")
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	spec, err := store.ParseSpec(path)
	if err != nil {
		t.Fatal(err)
	}

	var order []string
	rep, err := Run(context.Background(), spec, path, Options{
		Server:   srv.URL,
		Parallel: 8,
		Filter:   Filter{Methods: SafeMethods},
		OnResult: func(r OpResult) { order = append(order, r.OperationID) },
	})
	if err != nil {
		t.Fatal(err)
	}
	want := "op0,op1,op2,op3,op4,op5,op6,op7"
	if strings.Join(order, ",") != want {
		t.Errorf("OnResult order = %v", order)
	}
	for i, op := range rep.Operations {
		if op.OperationID != fmt.Sprintf("op%d", i) || op.Outcome != Passed {
			t.Errorf("result %d = %+v", i, op)
		}
	}
	if rep.Summary.Passed != 8 || rep.ExitCode() != ExitOK {
		t.Errorf("summary = %+v", rep.Summary)
	}
}

func TestParseMethods(t *testing.T) {
	if got := ParseMethods(""); strings.Join(got, ",") != "GET,HEAD,OPTIONS" {
		t.Errorf("default = %v", got)
	}
	if ParseMethods("all") != nil || ParseMethods("*") != nil {
		t.Error("all should select every method")
	}
	if got := ParseMethods("get, post"); strings.Join(got, ",") != "GET,POST" {
		t.Errorf("list = %v", got)
	}
}

func TestExitCode(t *testing.T) {
	cases := []struct {
		s    Summary
		want int
	}{
		{Summary{Passed: 2}, ExitOK},
		{Summary{Passed: 1, Failed: 1}, ExitFailed},
		{Summary{Failed: 1, Errors: 1}, ExitErrored},
		{Summary{Skipped: 3}, ExitOK},
	}
	for _, c := range cases {
		if got := (Report{Summary: c.s}).ExitCode(); got != c.want {
			t.Errorf("%+v → %d, want %d", c.s, got, c.want)
		}
	}
}
