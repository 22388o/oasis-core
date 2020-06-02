package batching

import (
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"

	"github.com/oasisprotocol/oasis-core/go/worker/compute/txnscheduler/algorithm/tests"
)

func TestBatchingAlgorithm(t *testing.T) {
	viper.Set(cfgMaxQueueSize, 100)

	algo, err := New(10, 16*1024*1024)
	require.NoError(t, err, "New()")

	tests.AlgorithmImplementationTests(t, algo)
}
