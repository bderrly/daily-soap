package server

import (
	"testing"

	"github.com/bderrly/daily-soap/internal/store"
)

// newTestApplication creates an Application for use in tests with the given
// store implementation.
func newTestApplication(t *testing.T, s store.Store) *application {
	t.Helper()

	app, err := NewApplication(s)
	if err != nil {
		t.Fatalf("failed to create test application: %v", err)
	}

	return app
}
