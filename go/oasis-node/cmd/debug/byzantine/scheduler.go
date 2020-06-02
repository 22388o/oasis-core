package byzantine

import (
	"bytes"
	"context"
	"fmt"

	tmtypes "github.com/tendermint/tendermint/types"

	"github.com/oasisprotocol/oasis-core/go/common"
	"github.com/oasisprotocol/oasis-core/go/common/cbor"
	"github.com/oasisprotocol/oasis-core/go/common/crypto/signature"
	"github.com/oasisprotocol/oasis-core/go/common/node"
	schedulerapp "github.com/oasisprotocol/oasis-core/go/consensus/tendermint/apps/scheduler"
	"github.com/oasisprotocol/oasis-core/go/consensus/tendermint/service"
	scheduler "github.com/oasisprotocol/oasis-core/go/scheduler/api"
	"github.com/oasisprotocol/oasis-core/go/worker/common/p2p"
)

func schedulerNextElectionHeight(svc service.TendermintService, kind scheduler.CommitteeKind) (int64, error) {
	sub, err := svc.Subscribe("script", schedulerapp.QueryApp)
	if err != nil {
		return 0, fmt.Errorf("Tendermint Subscribe error: %w", err)
	}
	defer svc.Unsubscribe("script", schedulerapp.QueryApp) // nolint: errcheck

	for {
		ev := (<-sub.Out()).Data().(tmtypes.EventDataNewBlock)
		for _, tmEv := range ev.ResultBeginBlock.GetEvents() {
			if tmEv.GetType() != schedulerapp.EventType {
				continue
			}

			for _, pair := range tmEv.GetAttributes() {
				if bytes.Equal(pair.GetKey(), schedulerapp.KeyElected) {
					var kinds []scheduler.CommitteeKind
					if err := cbor.Unmarshal(pair.GetValue(), &kinds); err != nil {
						return 0, fmt.Errorf("CBOR Unmarshal kinds error: %w", err)
					}

					for _, k := range kinds {
						if k == kind {
							return ev.Block.Header.Height, nil
						}
					}
				}
			}
		}
	}
}

func schedulerGetCommittee(ht *honestTendermint, height int64, kind scheduler.CommitteeKind, runtimeID common.Namespace) (*scheduler.Committee, error) {
	committees, err := ht.service.Scheduler().GetCommittees(context.Background(), &scheduler.GetCommitteesRequest{
		RuntimeID: runtimeID,
		Height:    height,
	})
	if err != nil {
		return nil, fmt.Errorf("Scheduler GetCommittees() error: %w", err)
	}

	for _, committee := range committees {
		if committee.Kind != kind {
			continue
		}

		if !committee.RuntimeID.Equal(&runtimeID) {
			continue
		}

		return committee, nil
	}
	return nil, fmt.Errorf("query didn't return a committee for our runtime")
}

func schedulerCheckScheduled(committee *scheduler.Committee, nodeID signature.PublicKey, role scheduler.Role) error {
	for _, member := range committee.Members {
		if !member.PublicKey.Equal(nodeID) {
			continue
		}

		if member.Role != role {
			return fmt.Errorf("we're scheduled as %s, expected %s", member.Role, role)
		}

		// All good.
		return nil
	}
	return fmt.Errorf("we're not scheduled")
}

func schedulerCheckNotScheduled(committee *scheduler.Committee, nodeID signature.PublicKey) error {
	for _, member := range committee.Members {
		if !member.PublicKey.Equal(nodeID) {
			continue
		}

		return fmt.Errorf("we're scheduled as %s", member.Role)
	}

	// All good.
	return nil
}

func schedulerForRoleInCommittee(ht *honestTendermint, height int64, committee *scheduler.Committee, role scheduler.Role, fn func(*node.Node) error) error {
	for _, member := range committee.Members {
		if member.Role != role {
			continue
		}

		n, err := registryGetNode(ht, height, member.PublicKey)
		if err != nil {
			return fmt.Errorf("registry get node %s error: %w", member.PublicKey, err)
		}

		if err = fn(n); err != nil {
			// Forward callback error to caller verbatim.
			return err
		}
	}

	return nil
}

func schedulerPublishToCommittee(ht *honestTendermint, height int64, committee *scheduler.Committee, role scheduler.Role, ph *p2pHandle, message *p2p.Message) error {
	if err := schedulerForRoleInCommittee(ht, height, committee, role, func(n *node.Node) error {
		ph.service.Publish(ph.context, n, message)

		return nil
	}); err != nil {
		return err
	}

	ph.service.Flush()

	return nil
}
