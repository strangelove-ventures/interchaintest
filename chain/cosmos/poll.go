package cosmos

import (
	"context"
	"errors"
	"fmt"

	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"github.com/strangelove-ventures/interchaintest/v8/testutil"
)

var ProposalStatus_name = map[int32]string{
	0: "PROPOSAL_STATUS_UNSPECIFIED",
	1: "PROPOSAL_STATUS_DEPOSIT_PERIOD",
	2: "PROPOSAL_STATUS_VOTING_PERIOD",
	3: "PROPOSAL_STATUS_PASSED",
	4: "PROPOSAL_STATUS_REJECTED",
	5: "PROPOSAL_STATUS_FAILED",
}

var ProposalStatus_value = map[string]int32{
	"PROPOSAL_STATUS_UNSPECIFIED":    0,
	"PROPOSAL_STATUS_DEPOSIT_PERIOD": 1,
	"PROPOSAL_STATUS_VOTING_PERIOD":  2,
	"PROPOSAL_STATUS_PASSED":         3,
	"PROPOSAL_STATUS_REJECTED":       4,
	"PROPOSAL_STATUS_FAILED":         5,
}

func ConvertStatus(status string) int {
	return int(ProposalStatus_value[status])
}

func ConvertStatusInt(status int) string {
	return ProposalStatus_name[int32(status)]
}

// PollForProposalStatus attempts to find a proposal with matching ID and status.
func PollForProposalStatus(ctx context.Context, chain *CosmosChain, startHeight, maxHeight uint64, proposalID string, status string) (ProposalResponse, error) {
	var zero ProposalResponse
	doPoll := func(ctx context.Context, height uint64) (ProposalResponse, error) {
		p, err := chain.QueryProposal(ctx, proposalID)
		if err != nil {
			return zero, err
		}
		if p.Proposal.Status != ConvertStatus(status) {
			return zero, fmt.Errorf("proposal status (%s) does not match expected: (%s)", ConvertStatusInt(p.Proposal.Status), status)
		}
		return *p, nil
	}
	bp := testutil.BlockPoller[ProposalResponse]{CurrentHeight: chain.Height, PollFunc: doPoll}
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

	bp := testutil.BlockPoller[T]{CurrentHeight: chain.Height, PollFunc: doPoll}
	return bp.DoPoll(ctx, startHeight, maxHeight)
}

// PollForBalance polls until the balance matches
func PollForBalance(ctx context.Context, chain *CosmosChain, deltaBlocks uint64, balance ibc.WalletAmount) error {
	h, err := chain.Height(ctx)
	if err != nil {
		return fmt.Errorf("failed to get height: %w", err)
	}
	doPoll := func(ctx context.Context, height uint64) (any, error) {
		bal, err := chain.GetBalance(ctx, balance.Address, balance.Denom)
		if err != nil {
			return nil, err
		}
		if !balance.Amount.Equal(bal) {
			return nil, fmt.Errorf("balance (%s) does not match expected: (%s)", bal.String(), balance.Amount.String())
		}
		return nil, nil
	}
	bp := testutil.BlockPoller[any]{CurrentHeight: chain.Height, PollFunc: doPoll}
	_, err = bp.DoPoll(ctx, h, h+deltaBlocks)
	return err
}
