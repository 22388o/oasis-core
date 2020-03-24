// Package staking implements the tendermint backed staking token backend.
package staking

import (
	"bytes"
	"context"

	abcitypes "github.com/tendermint/tendermint/abci/types"
	tmtypes "github.com/tendermint/tendermint/types"

	"github.com/oasislabs/oasis-core/go/common/cbor"
	"github.com/oasislabs/oasis-core/go/common/crypto/signature"
	"github.com/oasislabs/oasis-core/go/common/logging"
	"github.com/oasislabs/oasis-core/go/common/pubsub"
	"github.com/oasislabs/oasis-core/go/common/quantity"
	"github.com/oasislabs/oasis-core/go/consensus/tendermint/abci"
	app "github.com/oasislabs/oasis-core/go/consensus/tendermint/apps/staking"
	"github.com/oasislabs/oasis-core/go/consensus/tendermint/service"
	"github.com/oasislabs/oasis-core/go/staking/api"
)

var _ api.Backend = (*tendermintBackend)(nil)

type tendermintBackend struct {
	logger *logging.Logger

	service service.TendermintService
	querier *app.QueryFactory

	transferNotifier *pubsub.Broker
	approvalNotifier *pubsub.Broker
	burnNotifier     *pubsub.Broker
	escrowNotifier   *pubsub.Broker

	closedCh chan struct{}
}

func (tb *tendermintBackend) TotalSupply(ctx context.Context, height int64) (*quantity.Quantity, error) {
	q, err := tb.querier.QueryAt(ctx, height)
	if err != nil {
		return nil, err
	}

	return q.TotalSupply(ctx)
}

func (tb *tendermintBackend) CommonPool(ctx context.Context, height int64) (*quantity.Quantity, error) {
	q, err := tb.querier.QueryAt(ctx, height)
	if err != nil {
		return nil, err
	}

	return q.CommonPool(ctx)
}

func (tb *tendermintBackend) LastBlockFees(ctx context.Context, height int64) (*quantity.Quantity, error) {
	q, err := tb.querier.QueryAt(ctx, height)
	if err != nil {
		return nil, err
	}

	return q.LastBlockFees(ctx)
}

func (tb *tendermintBackend) Threshold(ctx context.Context, query *api.ThresholdQuery) (*quantity.Quantity, error) {
	q, err := tb.querier.QueryAt(ctx, query.Height)
	if err != nil {
		return nil, err
	}

	return q.Threshold(ctx, query.Kind)
}

func (tb *tendermintBackend) Accounts(ctx context.Context, height int64) ([]signature.PublicKey, error) {
	q, err := tb.querier.QueryAt(ctx, height)
	if err != nil {
		return nil, err
	}

	return q.Accounts(ctx)
}

func (tb *tendermintBackend) AccountInfo(ctx context.Context, query *api.OwnerQuery) (*api.Account, error) {
	q, err := tb.querier.QueryAt(ctx, query.Height)
	if err != nil {
		return nil, err
	}

	return q.AccountInfo(ctx, query.Owner)
}

func (tb *tendermintBackend) Delegations(ctx context.Context, query *api.OwnerQuery) (map[signature.PublicKey]*api.Delegation, error) {
	q, err := tb.querier.QueryAt(ctx, query.Height)
	if err != nil {
		return nil, err
	}

	return q.Delegations(ctx, query.Owner)
}

func (tb *tendermintBackend) DebondingDelegations(ctx context.Context, query *api.OwnerQuery) (map[signature.PublicKey][]*api.DebondingDelegation, error) {
	q, err := tb.querier.QueryAt(ctx, query.Height)
	if err != nil {
		return nil, err
	}

	return q.DebondingDelegations(ctx, query.Owner)
}

func (tb *tendermintBackend) WatchTransfers(ctx context.Context) (<-chan *api.TransferEvent, pubsub.ClosableSubscription, error) {
	typedCh := make(chan *api.TransferEvent)
	sub := tb.transferNotifier.Subscribe()
	sub.Unwrap(typedCh)

	return typedCh, sub, nil
}

func (tb *tendermintBackend) WatchBurns(ctx context.Context) (<-chan *api.BurnEvent, pubsub.ClosableSubscription, error) {
	typedCh := make(chan *api.BurnEvent)
	sub := tb.burnNotifier.Subscribe()
	sub.Unwrap(typedCh)

	return typedCh, sub, nil
}

func (tb *tendermintBackend) WatchEscrows(ctx context.Context) (<-chan *api.EscrowEvent, pubsub.ClosableSubscription, error) {
	typedCh := make(chan *api.EscrowEvent)
	sub := tb.escrowNotifier.Subscribe()
	sub.Unwrap(typedCh)

	return typedCh, sub, nil
}

func (tb *tendermintBackend) StateToGenesis(ctx context.Context, height int64) (*api.Genesis, error) {
	q, err := tb.querier.QueryAt(ctx, height)
	if err != nil {
		return nil, err
	}

	return q.Genesis(ctx)
}

func (tb *tendermintBackend) Cleanup() {
	<-tb.closedCh
}

func (tb *tendermintBackend) worker(ctx context.Context) {
	defer close(tb.closedCh)

	sub, err := tb.service.Subscribe("staking-worker", app.QueryApp)
	if err != nil {
		tb.logger.Error("failed to subscribe",
			"err", err,
		)
		return
	}
	defer tb.service.Unsubscribe("staking-worker", app.QueryApp) // nolint: errcheck

	for {
		var event interface{}

		select {
		case msg := <-sub.Out():
			event = msg.Data()
		case <-sub.Cancelled():
			tb.logger.Debug("worker: terminating, subscription closed")
			return
		case <-ctx.Done():
			return
		}

		switch ev := event.(type) {
		case tmtypes.EventDataNewBlock:
			tb.onEventDataNewBlock(ctx, ev)
		case tmtypes.EventDataTx:
			tb.onEventDataTx(ctx, ev)
		default:
		}
	}
}

func (tb *tendermintBackend) onEventDataNewBlock(ctx context.Context, ev tmtypes.EventDataNewBlock) {
	events := append([]abcitypes.Event{}, ev.ResultBeginBlock.GetEvents()...)
	events = append(events, ev.ResultEndBlock.GetEvents()...)

	tb.onABCIEvents(ctx, events, ev.Block.Header.Height)
}

func (tb *tendermintBackend) onABCIEvents(context context.Context, events []abcitypes.Event, height int64) {
	for _, tmEv := range events {
		if tmEv.GetType() != app.EventType {
			continue
		}

		for _, pair := range tmEv.GetAttributes() {
			if bytes.Equal(pair.GetKey(), app.KeyTakeEscrow) {
				var e api.TakeEscrowEvent
				if err := cbor.Unmarshal(pair.GetValue(), &e); err != nil {
					tb.logger.Error("worker: failed to get take escrow event from tag",
						"err", err,
					)
					continue
				}

				tb.escrowNotifier.Broadcast(&api.EscrowEvent{Take: &e})
			} else if bytes.Equal(pair.GetKey(), app.KeyTransfer) {
				var e api.TransferEvent
				if err := cbor.Unmarshal(pair.GetValue(), &e); err != nil {
					tb.logger.Error("worker: failed to get transfer event from tag",
						"err", err,
					)
					continue
				}

				tb.transferNotifier.Broadcast(&e)
			} else if bytes.Equal(pair.GetKey(), app.KeyReclaimEscrow) {
				var e api.ReclaimEscrowEvent
				if err := cbor.Unmarshal(pair.GetValue(), &e); err != nil {
					tb.logger.Error("worker: failed to get reclaim escrow event from tag",
						"err", err,
					)
					continue
				}

				tb.escrowNotifier.Broadcast(&api.EscrowEvent{Reclaim: &e})
			} else if bytes.Equal(pair.GetKey(), app.KeyAddEscrow) {
				var e api.AddEscrowEvent
				if err := cbor.Unmarshal(pair.GetValue(), &e); err != nil {
					tb.logger.Error("worker: failed to get escrow event from tag",
						"err", err,
					)
					continue
				}

				tb.escrowNotifier.Broadcast(&api.EscrowEvent{Add: &e})
			} else if bytes.Equal(pair.GetKey(), app.KeyBurn) {
				var e api.BurnEvent
				if err := cbor.Unmarshal(pair.GetValue(), &e); err != nil {
					tb.logger.Error("worker: failed to get burn event from tag",
						"err", err,
					)
					continue
				}

				tb.burnNotifier.Broadcast(&e)
			}
		}
	}
}

func (tb *tendermintBackend) onEventDataTx(ctx context.Context, tx tmtypes.EventDataTx) {
	tb.onABCIEvents(ctx, tx.Result.Events, tx.Height)
}

// New constructs a new tendermint backed staking Backend instance.
func New(ctx context.Context, service service.TendermintService) (api.Backend, error) {
	// Initialize and register the tendermint service component.
	a := app.New()
	if err := service.RegisterApplication(a); err != nil {
		return nil, err
	}

	// Configure the staking application as a fee handler.
	if err := service.SetTransactionAuthHandler(a.(abci.TransactionAuthHandler)); err != nil {
		return nil, err
	}

	tb := &tendermintBackend{
		logger:           logging.GetLogger("staking/tendermint"),
		service:          service,
		querier:          a.QueryFactory().(*app.QueryFactory),
		transferNotifier: pubsub.NewBroker(false),
		approvalNotifier: pubsub.NewBroker(false),
		burnNotifier:     pubsub.NewBroker(false),
		escrowNotifier:   pubsub.NewBroker(false),
		closedCh:         make(chan struct{}),
	}

	go tb.worker(ctx)

	return tb, nil
}
