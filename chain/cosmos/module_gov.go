package cosmos

import (
	"context"

	govv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	govv1beta1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1beta1"
)

// GovQueryProposal returns the state and details of a v1beta1 governance proposal.
func (c *CosmosChain) GovQueryProposal(ctx context.Context, proposalID int64) (*govv1beta1.Proposal, error) {
	res, err := govv1beta1.NewQueryClient(c.GetNode().GrpcConn).Proposal(ctx, &govv1beta1.QueryProposalRequest{ProposalId: uint64(proposalID)})
	if err != nil {
		return nil, err
	}

	return &res.Proposal, nil
}

// GovQueryProposalV1 returns the state and details of a v1 governance proposal.
func (c *CosmosChain) GovQueryProposalV1(ctx context.Context, proposalID int64) (*govv1.Proposal, error) {
	res, err := govv1.NewQueryClient(c.GetNode().GrpcConn).Proposal(ctx, &govv1.QueryProposalRequest{ProposalId: uint64(proposalID)})
	if err != nil {
		return nil, err
	}

	return res.Proposal, nil
}

// GovQueryProposalsV1 returns all proposals with a given status.
func (c *CosmosChain) GovQueryProposalsV1(ctx context.Context, status govv1.ProposalStatus) ([]*govv1.Proposal, error) {
	res, err := govv1.NewQueryClient(c.GetNode().GrpcConn).Proposals(ctx, &govv1.QueryProposalsRequest{
		ProposalStatus: status,
	})
	if err != nil {
		return nil, err
	}

	return res.Proposals, nil
}

// GovQueryVote returns the vote for a proposal from a specific voter.
func (c *CosmosChain) GovQueryVote(ctx context.Context, proposalID uint64, voter string) (*govv1.Vote, error) {
	res, err := govv1.NewQueryClient(c.GetNode().GrpcConn).Vote(ctx, &govv1.QueryVoteRequest{
		ProposalId: proposalID,
		Voter:      voter,
	})
	if err != nil {
		return nil, err
	}

	return res.Vote, nil
}

// GovQueryVotes returns all votes for a proposal.
func (c *CosmosChain) GovQueryVotes(ctx context.Context, proposalID uint64) ([]*govv1.Vote, error) {
	res, err := govv1.NewQueryClient(c.GetNode().GrpcConn).Votes(ctx, &govv1.QueryVotesRequest{
		ProposalId: proposalID,
	})
	if err != nil {
		return nil, err
	}

	return res.Votes, nil
}

// GovQueryParams returns the current governance parameters.
func (c *CosmosChain) GovQueryParams(ctx context.Context, paramsType string) (*govv1.Params, error) {
	res, err := govv1.NewQueryClient(c.GetNode().GrpcConn).Params(ctx, &govv1.QueryParamsRequest{
		ParamsType: paramsType,
	})
	if err != nil {
		return nil, err
	}

	return res.Params, nil
}
