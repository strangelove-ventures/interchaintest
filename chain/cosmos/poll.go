package cosmos

import (
	"context"
	"fmt"

	"github.com/strangelove-ventures/ibctest/v3/test"
)

// PollForProposalStatus attempts to find a proposal with matching ID and status.
func PollForProposalStatus(ctx context.Context, chain *CosmosChain, startHeight, maxHeight uint64, proposalID string, status string) (ProposalResponse, error) {
	var zero ProposalResponse
	doPoll := func(ctx context.Context, height uint64) (any, error) {
		p, err := chain.QueryProposal(ctx, proposalID)
		if err != nil {
			return zero, err
		}
		if p.Status != status {
			return zero, fmt.Errorf("proposal status (%s) does not match expected: (%s)", p.Status, status)
		}
		return *p, nil
	}
	bp := test.BlockPoller{CurrentHeight: chain.Height, PollFunc: doPoll}
	p, err := bp.DoPoll(ctx, startHeight, maxHeight)
	if err != nil {
		return zero, err
	}
	return p.(ProposalResponse), nil
}
