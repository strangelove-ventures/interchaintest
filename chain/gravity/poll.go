package gravity

import (
	"context"
	"fmt"

	"github.com/strangelove-ventures/ibctest/v6/chain/cosmos"
	"github.com/strangelove-ventures/ibctest/v6/testutil"
)

// PollForProposalStatus attempts to find a proposal with matching ID and status.
func PollForProposalStatus(ctx context.Context, chain *GravityChain, startHeight, maxHeight uint64, proposalID string, status string) (cosmos.ProposalResponse, error) {
	var zero cosmos.ProposalResponse
	doPoll := func(ctx context.Context, height uint64) (cosmos.ProposalResponse, error) {
		p, err := chain.CosmosChain.QueryProposal(ctx, proposalID)
		if err != nil {
			return zero, err
		}
		if p.Status != status {
			return zero, fmt.Errorf("proposal status (%s) does not match expected: (%s)", p.Status, status)
		}
		return *p, nil
	}
	bp := testutil.BlockPoller[cosmos.ProposalResponse]{CurrentHeight: chain.Height, PollFunc: doPoll}
	return bp.DoPoll(ctx, startHeight, maxHeight)
}
