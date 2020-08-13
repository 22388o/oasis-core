// Package tests is a collection of worker test cases.
package tests

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oasisprotocol/oasis-core/go/common/crypto/hash"
	"github.com/oasisprotocol/oasis-core/go/runtime/transaction"
	"github.com/oasisprotocol/oasis-core/go/worker/compute/txnscheduler/algorithm/api"
)

type testDispatcher struct {
	ShouldFail        bool
	DispatchedBatches []transaction.RawBatch
}

func (t *testDispatcher) Clear() {
	t.DispatchedBatches = []transaction.RawBatch{}
}

func (t *testDispatcher) Dispatch(batch transaction.RawBatch) error {
	if t.ShouldFail {
		return errors.New("dispatch failed")
	}
	t.DispatchedBatches = append(t.DispatchedBatches, batch)
	return nil
}

// AlgorithmImplementationTests runs the txnscheduler algorithm implementation tests.
func AlgorithmImplementationTests(
	t *testing.T,
	algorithm api.Algorithm,
) {
	td := testDispatcher{ShouldFail: false}

	// Initialize Algorithm.
	err := algorithm.Initialize(&td)
	require.NoError(t, err, "Initialize(td)")

	// Run the test cases.
	t.Run("ScheduleTxs", func(t *testing.T) {
		testScheduleTransactions(t, &td, algorithm)
	})
}

func testScheduleTransactions(t *testing.T, td *testDispatcher, algorithm api.Algorithm) {
	require.Equal(t, 0, algorithm.UnscheduledSize(), "no transactions should be scheduled")

	// Test ScheduleTx.
	testTx := []byte("hello world")
	txBytes := hash.NewFromBytes(testTx)
	err := algorithm.ScheduleTx(testTx)
	require.NoError(t, err, "ScheduleTx(testTx)")
	require.True(t, algorithm.IsQueued(txBytes), "IsQueued(tx)")

	// Test FlushTx.
	err = algorithm.Flush()
	require.NoError(t, err, "Flush()")
	require.Equal(t, 0, algorithm.UnscheduledSize(), "no transactions should be scheduled after flushing a single tx")
	require.Equal(t, 1, len(td.DispatchedBatches), "one batch should be dispatched")
	require.EqualValues(t, transaction.RawBatch{testTx}, td.DispatchedBatches[0], "transaction should be dispatched")
	require.False(t, algorithm.IsQueued(txBytes), "IsQueued(tx)")

	// Test with a Failing Dispatcher.
	td.Clear()
	td.ShouldFail = true
	testTx2 := []byte("hello world2")
	tx2Bytes := hash.NewFromBytes(testTx2)

	err = algorithm.ScheduleTx(testTx2)
	require.NoError(t, err, "ScheduleTx(testTx2)")
	require.True(t, algorithm.IsQueued(tx2Bytes), "IsQueued(tx)")
	require.False(t, algorithm.IsQueued(txBytes), "IsQueued(tx)")

	err = algorithm.Flush()
	require.Error(t, err, "dispatch failed", "Flush()")
	require.Equal(t, 1, algorithm.UnscheduledSize(), "failed dispatch should return tx in the queue")

	// Retry failed transaction with a Working Dispatcher.
	td.ShouldFail = false
	err = algorithm.Flush()
	require.NoError(t, err, "Flush()")
	require.Equal(t, 0, algorithm.UnscheduledSize(), "no transactions after flushing a single tx")
	require.False(t, algorithm.IsQueued(tx2Bytes), "IsQueued(tx)")
	require.Equal(t, 1, len(td.DispatchedBatches), "one batch should be dispatched")
	require.EqualValues(t, transaction.RawBatch{testTx2}, td.DispatchedBatches[0], "transaction should be dispatched")

	td.Clear()
}
