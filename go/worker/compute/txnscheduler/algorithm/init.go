package algorithm

import (
	"fmt"

	flag "github.com/spf13/pflag"

	"github.com/oasisprotocol/oasis-core/go/worker/compute/txnscheduler/algorithm/api"
	"github.com/oasisprotocol/oasis-core/go/worker/compute/txnscheduler/algorithm/batching"
)

// Flags has the configuration flags.
var Flags = flag.NewFlagSet("", flag.ContinueOnError)

// New creates a new algorithm.
func New(name string, maxBatchSize, maxBatchSizeBytes uint64) (api.Algorithm, error) {
	switch name {
	case batching.Name:
		return batching.New(maxBatchSize, maxBatchSizeBytes)
	default:
		return nil, fmt.Errorf("invalid transaction scheduler algorithm: %s", name)
	}
}

func init() {
	Flags.AddFlagSet(batching.Flags)
}
