package oasis

import (
	"encoding/hex"
	"path/filepath"
	"strconv"

	"github.com/oasislabs/oasis-core/go/client"
	"github.com/oasislabs/oasis-core/go/common/crypto/signature"
	commonGrpc "github.com/oasislabs/oasis-core/go/common/grpc"
	"github.com/oasislabs/oasis-core/go/common/node"
	"github.com/oasislabs/oasis-core/go/common/sgx"
	"github.com/oasislabs/oasis-core/go/consensus/tendermint"
	roothashTm "github.com/oasislabs/oasis-core/go/consensus/tendermint/roothash"
	"github.com/oasislabs/oasis-core/go/ias"
	cmdCommon "github.com/oasislabs/oasis-core/go/oasis-node/cmd/common"
	"github.com/oasislabs/oasis-core/go/oasis-node/cmd/common/flags"
	"github.com/oasislabs/oasis-core/go/oasis-node/cmd/common/grpc"
	"github.com/oasislabs/oasis-core/go/oasis-node/cmd/debug/byzantine"
	"github.com/oasislabs/oasis-core/go/storage"
	workerCommon "github.com/oasislabs/oasis-core/go/worker/common"
	"github.com/oasislabs/oasis-core/go/worker/common/p2p"
	"github.com/oasislabs/oasis-core/go/worker/compute"
	"github.com/oasislabs/oasis-core/go/worker/keymanager"
	"github.com/oasislabs/oasis-core/go/worker/merge"
	"github.com/oasislabs/oasis-core/go/worker/registration"
	workerStorage "github.com/oasislabs/oasis-core/go/worker/storage"
	"github.com/oasislabs/oasis-core/go/worker/txnscheduler"
)

type argBuilder struct {
	vec []string
}

func (args *argBuilder) debugAllowTestKeys() *argBuilder {
	args.vec = append(args.vec, "--"+cmdCommon.CfgDebugAllowTestKeys)
	return args
}

func (args *argBuilder) grpcDebugPort(port uint16) *argBuilder {
	args.vec = append(args.vec, []string{
		"--" + grpc.CfgDebugPort, strconv.Itoa(int(port)),
	}...)
	return args
}

func (args *argBuilder) grpcServerPort(port uint16) *argBuilder {
	args.vec = append(args.vec, []string{
		"--" + grpc.CfgServerPort, strconv.Itoa(int(port)),
	}...)
	return args
}

func (args *argBuilder) grpcVerboseDebug() *argBuilder {
	args.vec = append(args.vec, "--"+commonGrpc.CfgGRPCVerboseDebug)
	return args
}

func (args *argBuilder) consensusValidator() *argBuilder {
	args.vec = append(args.vec, []string{
		"--" + flags.CfgConsensusValidator,
	}...)
	return args
}

func (args *argBuilder) tendermintMinGasPrice(price uint64) *argBuilder {
	args.vec = append(args.vec, []string{
		"--" + tendermint.CfgConsensusMinGasPrice, strconv.Itoa(int(price)),
	}...)
	return args
}

func (args *argBuilder) tendermintCoreListenAddress(port uint16) *argBuilder {
	args.vec = append(args.vec, []string{
		"--" + tendermint.CfgCoreListenAddress, "tcp://0.0.0.0:" + strconv.Itoa(int(port)),
	}...)
	return args
}

func (args *argBuilder) tendermintSeedMode() *argBuilder {
	args.vec = append(args.vec, "--"+tendermint.CfgP2PSeedMode)
	return args
}

func (args *argBuilder) tendermintDebugAddrBookLenient() *argBuilder {
	args.vec = append(args.vec, "--"+tendermint.CfgDebugP2PAddrBookLenient)
	return args
}

func (args *argBuilder) roothashTendermintIndexBlocks() *argBuilder {
	args.vec = append(args.vec, "--"+roothashTm.CfgIndexBlocks)
	return args
}

func (args *argBuilder) storageBackend(backend string) *argBuilder {
	args.vec = append(args.vec, []string{
		"--" + storage.CfgBackend, backend,
	}...)
	return args
}

func (args *argBuilder) clientIndexRuntimes(id signature.PublicKey) *argBuilder {
	args.vec = append(args.vec, []string{
		"--" + client.CfgIndexRuntimes, id.String(),
	}...)
	return args
}

func (args *argBuilder) workerClientPort(port uint16) *argBuilder {
	args.vec = append(args.vec, []string{
		"--" + workerCommon.CfgClientPort, strconv.Itoa(int(port)),
	}...)
	return args
}

func (args *argBuilder) workerP2pPort(port uint16) *argBuilder {
	args.vec = append(args.vec, []string{
		"--" + p2p.CfgP2pPort, strconv.Itoa(int(port)),
	}...)
	return args
}

func (args *argBuilder) workerRuntimeID(id signature.PublicKey) *argBuilder {
	args.vec = append(args.vec, []string{
		"--" + workerCommon.CfgRuntimeID, id.String(),
	}...)
	return args
}

func (args *argBuilder) workerRuntimeBackend(backend string) *argBuilder {
	args.vec = append(args.vec, []string{
		"--" + workerCommon.CfgRuntimeBackend, backend,
	}...)
	return args
}

func (args *argBuilder) workerRuntimeLoader(fn string) *argBuilder {
	args.vec = append(args.vec, []string{
		"--" + workerCommon.CfgRuntimeLoader, fn,
	}...)
	return args
}

func (args *argBuilder) workerRuntimeBinary(fn string) *argBuilder {
	args.vec = append(args.vec, []string{
		"--" + workerCommon.CfgRuntimeBinary, fn,
	}...)
	return args
}

func (args *argBuilder) workerComputeEnabled() *argBuilder {
	args.vec = append(args.vec, "--"+compute.CfgWorkerEnabled)
	return args
}

func (args *argBuilder) workerKeymangerEnabled() *argBuilder {
	args.vec = append(args.vec, "--"+keymanager.CfgEnabled)
	return args
}

func (args *argBuilder) workerKeymanagerRuntimeBinary(fn string) *argBuilder {
	args.vec = append(args.vec, []string{
		"--" + keymanager.CfgRuntimeBinary, fn,
	}...)
	return args
}

func (args *argBuilder) workerKeymanagerRuntimeLoader(fn string) *argBuilder {
	args.vec = append(args.vec, []string{
		"--" + keymanager.CfgRuntimeLoader, fn,
	}...)
	return args
}

func (args *argBuilder) workerKeymanagerTEEHardware(hw node.TEEHardware) *argBuilder {
	args.vec = append(args.vec, []string{
		"--" + keymanager.CfgTEEHardware, hw.String(),
	}...)
	return args
}

func (args *argBuilder) workerKeymanagerRuntimeID(id signature.PublicKey) *argBuilder {
	args.vec = append(args.vec, []string{
		"--" + keymanager.CfgRuntimeID, id.String(),
	}...)
	return args
}

func (args *argBuilder) workerKeymanagerMayGenerate() *argBuilder {
	args.vec = append(args.vec, "--"+keymanager.CfgMayGenerate)
	return args
}

func (args *argBuilder) workerMergeEnabled() *argBuilder {
	args.vec = append(args.vec, "--"+merge.CfgWorkerEnabled)
	return args
}

func (args *argBuilder) workerStorageEnabled() *argBuilder {
	args.vec = append(args.vec, "--"+workerStorage.CfgWorkerEnabled)
	return args
}

func (args *argBuilder) workerStorageDebugIgnoreApplies(ignore bool) *argBuilder {
	if ignore {
		args.vec = append(args.vec, "--"+workerStorage.CfgWorkerDebugIgnoreApply)
	}
	return args
}

func (args *argBuilder) workerTxnschedulerEnabled() *argBuilder {
	args.vec = append(args.vec, "--"+txnscheduler.CfgWorkerEnabled)
	return args
}

func (args *argBuilder) iasUseGenesis() *argBuilder {
	args.vec = append(args.vec, "--ias.use_genesis")
	return args
}

func (args *argBuilder) iasDebugMock() *argBuilder {
	args.vec = append(args.vec, "--ias.debug.mock")
	return args
}

func (args *argBuilder) iasSPID(spid []byte) *argBuilder {
	args.vec = append(args.vec, []string{
		"--ias.spid", hex.EncodeToString(spid),
	}...)
	return args
}

func (args *argBuilder) appendSeedNodes(net *Network) *argBuilder {
	if seed := net.seedNode; seed != nil {
		args.vec = append(args.vec, []string{
			"--" + tendermint.CfgP2PSeeds, seed.tmAddress + "@127.0.0.1:" + strconv.Itoa(int(seed.consensusPort)),
		}...)
	}

	return args
}

func (args *argBuilder) appendNetwork(net *Network) *argBuilder {
	args = args.grpcVerboseDebug().
		appendSeedNodes(net)
	return args
}

func (args *argBuilder) appendComputeNodeRuntime(rt *Runtime) *argBuilder {
	args = args.workerRuntimeID(rt.id).
		workerRuntimeBinary(rt.binary)
	return args
}

func (args *argBuilder) appendEntity(ent *Entity) *argBuilder {
	if ent.dir != nil {
		dir := ent.dir.String()
		args.vec = append(args.vec, []string{
			"--" + registration.CfgRegistrationEntity, filepath.Join(dir, "entity.json"),
			"--" + registration.CfgRegistrationPrivateKey, filepath.Join(dir, "entity.pem"),
		}...)
	} else if ent.isDebugTestEntity {
		args.vec = append(args.vec, "--"+flags.CfgDebugTestEntity)
	}
	return args
}

func (args *argBuilder) appendIASProxy(iasProxy *iasProxy) *argBuilder {
	if iasProxy != nil {
		args.vec = append(args.vec, []string{
			"--" + ias.CfgProxyAddress, "127.0.0.1:" + strconv.Itoa(int(iasProxy.grpcPort)),
			"--" + ias.CfgTLSCertFile, iasProxy.tlsCertPath(),
			"--" + ias.CfgDebugSkipVerify,
			"--" + ias.CfgAllowDebugEnclaves,
		}...)
	}
	return args
}

func (args *argBuilder) byzantineFakeSGX() *argBuilder {
	args.vec = append(args.vec, "--"+byzantine.CfgFakeSGX)
	return args
}

func (args *argBuilder) byzantineVersionFakeEnclaveID(rt *Runtime) *argBuilder {
	eid := sgx.EnclaveIdentity{
		MrEnclave: *rt.mrEnclave,
		MrSigner:  *rt.mrSigner,
	}
	args.vec = append(args.vec, "--"+byzantine.CfgVersionFakeEnclaveID, eid.String())
	return args
}

func newArgBuilder() *argBuilder {
	return &argBuilder{}
}