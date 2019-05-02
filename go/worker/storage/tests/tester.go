// Package tests is a collection of storage worker test cases.
package tests

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oasislabs/ekiden/go/worker/storage"
)

// WorkerImplementationTests runs the storage worker implementation tests.
func WorkerImplementationTests(
	t *testing.T,
	worker *storage.Storage,
) {
	// Wait for storage worker to start and register.
	<-worker.Initialized()

	// Assure storage worker is enabled.
	require.True(t, worker.Enabled())
}