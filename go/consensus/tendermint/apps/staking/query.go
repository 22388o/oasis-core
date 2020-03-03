package staking

import (
	"context"

	"github.com/oasislabs/oasis-core/go/common/crypto/signature"
	"github.com/oasislabs/oasis-core/go/common/quantity"
	"github.com/oasislabs/oasis-core/go/consensus/tendermint/abci"
	stakingState "github.com/oasislabs/oasis-core/go/consensus/tendermint/apps/staking/state"
	epochtime "github.com/oasislabs/oasis-core/go/epochtime/api"
	staking "github.com/oasislabs/oasis-core/go/staking/api"
)

// Query is the staking query interface.
type Query interface {
	TotalSupply(context.Context) (*quantity.Quantity, error)
	CommonPool(context.Context) (*quantity.Quantity, error)
	Threshold(context.Context, staking.ThresholdKind) (*quantity.Quantity, error)
	DebondingInterval(context.Context) (epochtime.EpochTime, error)
	Accounts(context.Context) ([]signature.PublicKey, error)
	AccountInfo(context.Context, signature.PublicKey) (*staking.Account, error)
	Delegations(context.Context, signature.PublicKey) (map[signature.PublicKey]*staking.Delegation, error)
	DebondingDelegations(context.Context, signature.PublicKey) (map[signature.PublicKey][]*staking.DebondingDelegation, error)
	Genesis(context.Context) (*staking.Genesis, error)
}

// QueryFactory is the staking query factory.
type QueryFactory struct {
	app *stakingApplication
}

// QueryAt returns the staking query interface for a specific height.
func (sf *QueryFactory) QueryAt(ctx context.Context, height int64) (Query, error) {
	var state *stakingState.ImmutableState
	var err error
	abciCtx := abci.FromCtx(ctx)

	// If this request was made from InitChain, no blocks and states have been
	// submitted yet, so we use the existing state instead.
	if abciCtx != nil && abciCtx.IsInitChain() {
		state = stakingState.NewMutableState(abciCtx.State()).ImmutableState
	} else {
		state, err = stakingState.NewImmutableState(sf.app.state, height)
		if err != nil {
			return nil, err
		}
	}

	// If this request was made from an ABCI app, make sure to use the associated
	// context for querying state instead of the default one.
	if abciCtx != nil && height == abciCtx.BlockHeight()+1 {
		state.Snapshot = abciCtx.State().ImmutableTree
	}

	return &stakingQuerier{state}, nil
}

type stakingQuerier struct {
	state *stakingState.ImmutableState
}

func (sq *stakingQuerier) TotalSupply(ctx context.Context) (*quantity.Quantity, error) {
	return sq.state.TotalSupply()
}

func (sq *stakingQuerier) CommonPool(ctx context.Context) (*quantity.Quantity, error) {
	return sq.state.CommonPool()
}

func (sq *stakingQuerier) Threshold(ctx context.Context, kind staking.ThresholdKind) (*quantity.Quantity, error) {
	thresholds, err := sq.state.Thresholds()
	if err != nil {
		return nil, err
	}

	threshold, ok := thresholds[kind]
	if !ok {
		return nil, staking.ErrInvalidThreshold
	}
	return &threshold, nil
}

func (sq *stakingQuerier) DebondingInterval(ctx context.Context) (epochtime.EpochTime, error) {
	return sq.state.DebondingInterval()
}

func (sq *stakingQuerier) Accounts(ctx context.Context) ([]signature.PublicKey, error) {
	return sq.state.Accounts()
}

func (sq *stakingQuerier) AccountInfo(ctx context.Context, id signature.PublicKey) (*staking.Account, error) {
	return sq.state.Account(id), nil
}

func (sq *stakingQuerier) Delegations(ctx context.Context, id signature.PublicKey) (map[signature.PublicKey]*staking.Delegation, error) {
	return sq.state.DelegationsFor(id)
}

func (sq *stakingQuerier) DebondingDelegations(ctx context.Context, id signature.PublicKey) (map[signature.PublicKey][]*staking.DebondingDelegation, error) {
	return sq.state.DebondingDelegationsFor(id)
}

func (app *stakingApplication) QueryFactory() interface{} {
	return &QueryFactory{app}
}
