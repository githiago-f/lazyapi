package app

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	// Keys are not held in tests (finalScreen quits from outside, which
	// would drop them); the burst tests turn it back on.
	burstWindow = 0
	os.Exit(m.Run())
}
