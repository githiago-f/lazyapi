package cli

import (
	"net/http/httptest"
	"testing"

	"github.com/githiago-f/lazyapi/internal/mock"
	"github.com/githiago-f/lazyapi/internal/store"
)

// TestExamplesPassAgainstMock keeps examples/ runnable: every operation and
// test script must pass against the spec's own mock.
func TestExamplesPassAgainstMock(t *testing.T) {
	const specPath = "../../examples/openapi.yml"
	spec, err := store.ParseSpec(specPath)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(mock.New(spec, specPath))
	defer srv.Close()

	out, err := captureStdout(t, func() error {
		return RunTests([]string{specPath, "--server", srv.URL, "--methods", "all", "--frozen-lockfile"})
	})
	if err != nil {
		t.Fatalf("examples failed: %v\n%s", err, out)
	}
	if code := ExitCode(run([]string{"validate", "spec", specPath})); code != 0 {
		t.Errorf("validate spec exit = %d", code)
	}
}
