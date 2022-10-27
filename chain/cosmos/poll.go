package cosmos

import (
	"context"
	"errors"
	"fmt"

	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/strangelove-ventures/ibctest/v6/test"
)

// PollForProposalStatus attempts to find a proposal with matching ID and status.
func PollForProposalStatus(ctx context.Context, chain *CosmosChain, startHeight, maxHeight uint64, proposalID string, status string) (ProposalResponse, error) {
	var zero ProposalResponse
	doPoll := func(ctx context.Context, height uint64) (ProposalResponse, error) {
		p, err := chain.QueryProposal(ctx, proposalID)
		if err != nil {
			return zero, err
		}
		if p.Status != status {
			return zero, fmt.Errorf("proposal status (%s) does not match expected: (%s)", p.Status, status)
		}
		return *p, nil
	}
	bp := test.BlockPoller[ProposalResponse]{CurrentHeight: chain.Height, PollFunc: doPoll}
	return bp.DoPoll(ctx, startHeight, maxHeight)
}

// PollForMessage searches every transaction for a message. Must pass a coded registry capable of decoding the cosmos transaction.
// fn is optional. Return true from the fn to stop polling and return the found message. If fn is nil, returns the first message to match type T.
func PollForMessage[T any](ctx context.Context, chain *CosmosChain, registry codectypes.InterfaceRegistry, startHeight, maxHeight uint64, fn func(found T) bool) (T, error) {
	var zero T
	if fn == nil {
		fn = func(T) bool { return true }
	}
	doPoll := func(ctx context.Context, height uint64) (T, error) {
		h := int64(height)
		block, err := chain.getFullNode().Client.Block(ctx, &h)
		if err != nil {
			return zero, err
		}
		for _, tx := range block.Block.Txs {
			sdkTx, err := decodeTX(registry, tx)
			if err != nil {
				return zero, err
			}
			for _, msg := range sdkTx.GetMsgs() {
				if found, ok := msg.(T); ok {
					if fn(found) {
						return found, nil
					}
				}
			}
		}
		return zero, errors.New("not found")
	}

	bp := test.BlockPoller[T]{CurrentHeight: chain.Height, PollFunc: doPoll}
	return bp.DoPoll(ctx, startHeight, maxHeight)
}
