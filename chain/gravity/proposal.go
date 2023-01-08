package gravity

import (
	"context"
	"github.com/strangelove-ventures/ibctest/v6/chain/cosmos"
)

// UpgradeProposal submits a software-upgrade governance proposal to the chain.
func (g *GravityChain) UpgradeProposal(ctx context.Context, keyName string, prop cosmos.SoftwareUpgradeProposal) (tx cosmos.TxProposal, _ error) {
	return g.CosmosChain.UpgradeProposal(ctx, keyName, prop)
}

// TextProposal submits a text governance proposal to the chain.
func (g *GravityChain) TextProposal(ctx context.Context, keyName string, prop cosmos.TextProposal) (tx cosmos.TxProposal, _ error) {
	return g.CosmosChain.TextProposal(ctx, keyName, prop)
}

func (g *GravityChain) VoteOnProposalAllValidators(ctx context.Context, proposalID string, vote string) error {
	return g.CosmosChain.VoteOnProposalAllValidators(ctx, proposalID, vote)
}
