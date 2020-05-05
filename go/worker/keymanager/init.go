// Package keymanager implements the key manager worker.
package keymanager

import (
	"context"
	"fmt"

	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/oasislabs/oasis-core/go/common"
	"github.com/oasislabs/oasis-core/go/common/grpc/policy"
	"github.com/oasislabs/oasis-core/go/common/logging"
	"github.com/oasislabs/oasis-core/go/common/node"
	ias "github.com/oasislabs/oasis-core/go/ias/api"
	"github.com/oasislabs/oasis-core/go/keymanager/api"
	enclaverpc "github.com/oasislabs/oasis-core/go/runtime/enclaverpc/api"
	"github.com/oasislabs/oasis-core/go/runtime/localstorage"
	runtimeRegistry "github.com/oasislabs/oasis-core/go/runtime/registry"
	workerCommon "github.com/oasislabs/oasis-core/go/worker/common"
	"github.com/oasislabs/oasis-core/go/worker/common/host"
	"github.com/oasislabs/oasis-core/go/worker/registration"
)

const (
	// CfgEnabled enables the key manager worker.
	CfgEnabled = "worker.keymanager.enabled"

	// CfgTEEHardware configures the enclave TEE hardware.
	CfgTEEHardware = "worker.keymanager.tee_hardware"
	// CfgRuntimeLoader configures the runtime loader.
	CfgRuntimeLoader = "worker.keymanager.runtime.loader"
	// CfgRuntimeBinary configures the runtime binary.
	CfgRuntimeBinary = "worker.keymanager.runtime.binary"
	// CfgRuntimeID configures the runtime ID.
	CfgRuntimeID = "worker.keymanager.runtime.id"
	// CfgMayGenerate allows the enclave to generate a master secret.
	CfgMayGenerate = "worker.keymanager.may_generate"
)

// Flags has the configuration flags.
var Flags = flag.NewFlagSet("", flag.ContinueOnError)

// Enabled reads our enabled flag from viper.
func Enabled() bool {
	return viper.GetBool(CfgEnabled)
}

// New constructs a new key manager worker.
func New(
	dataDir string,
	commonWorker *workerCommon.Worker,
	ias ias.Endpoint,
	r *registration.Worker,
	backend api.Backend,
) (*Worker, error) {
	var teeHardware node.TEEHardware
	s := viper.GetString(CfgTEEHardware)
	if err := teeHardware.FromString(s); err != nil {
		return nil, fmt.Errorf("invalid TEE hardware: %s", s)
	}

	workerRuntimeLoaderBinary := viper.GetString(CfgRuntimeLoader)
	runtimeBinary := viper.GetString(CfgRuntimeBinary)

	ctx, cancelFn := context.WithCancel(context.Background())

	w := &Worker{
		logger:       logging.GetLogger("worker/keymanager"),
		ctx:          ctx,
		cancelCtx:    cancelFn,
		stopCh:       make(chan struct{}),
		quitCh:       make(chan struct{}),
		initCh:       make(chan struct{}),
		commonWorker: commonWorker,
		backend:      backend,
		grpcPolicy:   policy.NewDynamicRuntimePolicyChecker(enclaverpc.ServiceName, commonWorker.GrpcPolicyWatcher),
		enabled:      Enabled(),
		mayGenerate:  viper.GetBool(CfgMayGenerate),
	}

	if w.enabled {
		if !w.commonWorker.Enabled() {
			panic("common worker should have been enabled for key manager worker")
		}

		var runtimeID common.Namespace
		if err := runtimeID.UnmarshalHex(viper.GetString(CfgRuntimeID)); err != nil {
			return nil, fmt.Errorf("worker/keymanager: failed to parse runtime ID: %w", err)
		}

		if workerRuntimeLoaderBinary == "" {
			return nil, fmt.Errorf("worker/keymanager: worker runtime loader binary not configured")
		}
		if runtimeBinary == "" {
			return nil, fmt.Errorf("worker/keymanager: runtime binary not configured")
		}

		// Create local storage for the key manager.
		path, err := runtimeRegistry.EnsureRuntimeStateDir(dataDir, runtimeID)
		if err != nil {
			return nil, fmt.Errorf("worker/keymanager: failed to ensure runtime state directory: %w", err)
		}
		localStorage, err := localstorage.New(path, runtimeRegistry.LocalStorageFile, runtimeID)
		if err != nil {
			return nil, fmt.Errorf("worker/keymanager: cannot create local storage: %w", err)
		}

		w.roleProvider, err = r.NewRuntimeRoleProvider(node.RoleKeyManager, runtimeID)
		if err != nil {
			return nil, fmt.Errorf("worker/keymanager: failed to create role provider: %w", err)
		}

		w.runtime, err = commonWorker.RuntimeRegistry.NewUnmanagedRuntime(ctx, runtimeID)
		if err != nil {
			return nil, fmt.Errorf("worker/keymanager: failed to create runtime registry entry: %w", err)
		}

		w.workerHostCfg = host.Config{
			Role:           node.RoleKeyManager,
			ID:             runtimeID,
			WorkerBinary:   workerRuntimeLoaderBinary,
			RuntimeBinary:  runtimeBinary,
			TEEHardware:    teeHardware,
			IAS:            ias,
			MessageHandler: newHostHandler(w, commonWorker, localStorage),
		}

		// Register the Keymanager EnclaveRPC transport gRPC service.
		enclaverpc.RegisterService(w.commonWorker.Grpc.Server(), w)
	}

	return w, nil
}

func init() {
	emptyRoot.Empty()

	Flags.Bool(CfgEnabled, false, "Enable key manager worker")

	Flags.String(CfgTEEHardware, "", "TEE hardware to use for the key manager")
	Flags.String(CfgRuntimeLoader, "", "Path to key manager worker process binary")
	Flags.String(CfgRuntimeBinary, "", "Path to key manager runtime binary")
	Flags.String(CfgRuntimeID, "", "Key manager Runtime ID")
	Flags.Bool(CfgMayGenerate, false, "Key manager may generate new master secret")

	_ = viper.BindPFlags(Flags)
}
