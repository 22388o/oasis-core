// Package tests is a collection of beacon implementation test cases.
package tests

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oasisprotocol/oasis-core/go/beacon/api"
	consensus "github.com/oasisprotocol/oasis-core/go/consensus/api"
	epochtime "github.com/oasisprotocol/oasis-core/go/epochtime/api"
	epochtimeTests "github.com/oasisprotocol/oasis-core/go/epochtime/tests"
)

// BeaconImplementationTests exercises the basic functionality of a
// beacon backend.
func BeaconImplementationTests(t *testing.T, backend api.Backend, epochtime epochtime.SetableBackend) {
	require := require.New(t)

	beacon, err := backend.GetBeacon(context.Background(), consensus.HeightLatest)
	require.NoError(err, "GetBeacon")
	require.Len(beacon, api.BeaconSize, "GetBeacon - length")

	_ = epochtimeTests.MustAdvanceEpoch(t, epochtime, 1)

	newBeacon, err := backend.GetBeacon(context.Background(), consensus.HeightLatest)
	require.NoError(err, "GetBeacon")
	require.Len(newBeacon, api.BeaconSize, "GetBeacon - length")
	require.NotEqual(beacon, newBeacon, "After epoch transition, new beacon should be generated.")
}
