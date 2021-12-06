package runtime

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	beacon "github.com/oasisprotocol/oasis-core/go/beacon/api"
	"github.com/oasisprotocol/oasis-core/go/common/quantity"
	consensus "github.com/oasisprotocol/oasis-core/go/consensus/api"
	"github.com/oasisprotocol/oasis-core/go/oasis-test-runner/env"
	"github.com/oasisprotocol/oasis-core/go/oasis-test-runner/log"
	"github.com/oasisprotocol/oasis-core/go/oasis-test-runner/oasis"
	"github.com/oasisprotocol/oasis-core/go/oasis-test-runner/oasis/cli"
	"github.com/oasisprotocol/oasis-core/go/oasis-test-runner/scenario"
	staking "github.com/oasisprotocol/oasis-core/go/staking/api"
)

// StorageEarlyStateSync is the scenario where a runtime is registered first and is not yet
// operational, then a while later an executor node uses consensus layer state sync to catch up but
// the runtime has already advanced some epoch transition rounds and is no longer at genesis.
var StorageEarlyStateSync scenario.Scenario = newStorageEarlyStateSyncImpl()

type storageEarlyStateSyncImpl struct {
	runtimeImpl

	epoch beacon.EpochTime
}

func newStorageEarlyStateSyncImpl() scenario.Scenario {
	return &storageEarlyStateSyncImpl{
		runtimeImpl: *newRuntimeImpl("storage-early-state-sync", nil),
	}
}

func (sc *storageEarlyStateSyncImpl) Clone() scenario.Scenario {
	return &storageEarlyStateSyncImpl{
		runtimeImpl: *sc.runtimeImpl.Clone().(*runtimeImpl),
		epoch:       sc.epoch,
	}
}

func (sc *storageEarlyStateSyncImpl) Fixture() (*oasis.NetworkFixture, error) {
	f, err := sc.runtimeImpl.Fixture()
	if err != nil {
		return nil, err
	}

	// Allocate stake and set runtime thresholds.
	f.Network.StakingGenesis = &staking.Genesis{
		Parameters: staking.ConsensusParameters{
			Thresholds: map[staking.ThresholdKind]quantity.Quantity{
				staking.KindEntity:            *quantity.NewFromUint64(0),
				staking.KindNodeValidator:     *quantity.NewFromUint64(0),
				staking.KindNodeCompute:       *quantity.NewFromUint64(0),
				staking.KindNodeKeyManager:    *quantity.NewFromUint64(0),
				staking.KindRuntimeCompute:    *quantity.NewFromUint64(1000),
				staking.KindRuntimeKeyManager: *quantity.NewFromUint64(1000),
			},
		},
	}
	// Avoid unexpected blocks.
	f.Network.SetMockEpoch()
	// Enable consensus layer checkpoints.
	f.Network.Consensus.Parameters.StateCheckpointInterval = 10
	f.Network.Consensus.Parameters.StateCheckpointNumKept = 2
	f.Network.Consensus.Parameters.StateCheckpointChunkSize = 1024 * 1024
	// Disable certificate rotation on validator nodes so we can more easily use them for sync.
	for i := range f.Validators {
		f.Validators[i].DisableCertRotation = true
	}
	// No need for key managers.
	f.Runtimes = f.Runtimes[1:]
	f.Runtimes[0].Keymanager = -1
	f.Keymanagers = nil
	f.KeymanagerPolicies = nil
	// No need for clients (the runtime will not actually fully work as we just want to make sure
	// initialization works correctly).
	f.Clients = nil
	// Exclude runtime from genesis as we will register those dynamically.
	f.Runtimes[0].ExcludeFromGenesis = true
	// Only one compute worker that will use state sync after the runtime is registered.
	f.ComputeWorkers = []oasis.ComputeWorkerFixture{{
		NodeFixture: oasis.NodeFixture{
			NoAutoStart: true,
		},
		Entity:                1,
		CheckpointSyncEnabled: true,
		LogWatcherHandlerFactories: []log.WatcherHandlerFactory{
			oasis.LogEventABCIStateSyncComplete(),
		},
		Runtimes: []int{0},
	}}

	return f, nil
}

func (sc *storageEarlyStateSyncImpl) epochTransition(ctx context.Context) error {
	sc.epoch++

	sc.Logger.Info("triggering epoch transition",
		"epoch", sc.epoch,
	)
	if err := sc.Net.Controller().SetEpoch(ctx, sc.epoch); err != nil {
		return fmt.Errorf("failed to set epoch: %w", err)
	}
	sc.Logger.Info("epoch transition done")
	return nil
}

func (sc *storageEarlyStateSyncImpl) Run(childEnv *env.Env) error { // nolint: gocyclo
	if err := sc.Net.Start(); err != nil {
		return err
	}

	ctx := context.Background()
	cli := cli.New(childEnv, sc.Net, sc.Logger)

	// Wait for validator nodes to register.
	sc.Logger.Info("waiting for validator nodes to initialize",
		"num_validators", len(sc.Net.Validators()),
	)
	for _, n := range sc.Net.Validators() {
		if err := n.WaitReady(ctx); err != nil {
			return fmt.Errorf("failed to wait for a validator: %w", err)
		}
	}

	// Perform an initial epoch transition.
	if err := sc.epochTransition(ctx); err != nil {
		return err
	}

	// Fetch current epoch.
	epoch, err := sc.Net.Controller().Beacon.GetEpoch(ctx, consensus.HeightLatest)
	if err != nil {
		return fmt.Errorf("failed to get current epoch: %w", err)
	}

	// Register a new compute runtime.
	sc.Logger.Info("registering a new compute runtime")
	compRt := sc.Net.Runtimes()[0]
	compRtDesc := compRt.ToRuntimeDescriptor()
	compRtDesc.Deployments[0].ValidFrom = epoch + 1
	txPath := filepath.Join(childEnv.Dir(), "register_compute_runtime.json")
	if grr := cli.Registry.GenerateRegisterRuntimeTx(childEnv.Dir(), compRtDesc, 0, txPath); grr != nil {
		return fmt.Errorf("failed to generate register compute runtime tx: %w", grr)
	}
	if grr := cli.Consensus.SubmitTx(txPath); grr != nil {
		return fmt.Errorf("failed to register compute runtime: %w", grr)
	}

	// Now that the runtime is registered, trigger some epoch transitions.
	for i := 0; i < 3; i++ {
		if grr := sc.epochTransition(ctx); grr != nil {
			return err
		}

		// Wait a bit after epoch transitions.
		time.Sleep(1 * time.Second)
	}

	// Let the network run for 50 blocks. This should generate some checkpoints.
	blockCh, blockSub, err := sc.Net.Controller().Consensus.WatchBlocks(ctx)
	if err != nil {
		return err
	}
	defer blockSub.Close()

	sc.Logger.Info("waiting for some blocks")
	var blk *consensus.Block
	for {
		select {
		case blk = <-blockCh:
			if blk.Height < 50 {
				continue
			}
		case <-time.After(30 * time.Second):
			return fmt.Errorf("timed out waiting for blocks")
		}

		break
	}

	sc.Logger.Info("got some blocks, starting compute node that needs to sync",
		"trust_height", blk.Height,
		"trust_hash", blk.Hash.Hex(),
	)

	// Configure state sync for the compute node.
	worker := sc.Net.ComputeWorkers()[0]
	worker.SetConsensusStateSync(&oasis.ConsensusStateSyncCfg{
		TrustHeight: uint64(blk.Height),
		TrustHash:   blk.Hash.Hex(),
	})

	if err := worker.Start(); err != nil {
		return fmt.Errorf("can't start compute worker: %w", err)
	}
	readyCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	if err := worker.WaitReady(readyCtx); err != nil {
		return fmt.Errorf("error waiting for compute worker to become ready: %w", err)
	}

	// Wait a bit to give the logger in the node time to sync; the message has already been
	// logged by this point, it just might not be on disk yet.
	<-time.After(1 * time.Second)

	return sc.Net.CheckLogWatchers()
}
