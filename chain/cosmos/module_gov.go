package cosmos

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"path"
	"path/filepath"
	"strconv"

	upgradetypes "cosmossdk.io/x/upgrade/types"
	govv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	govv1beta1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1beta1"
	paramsutils "github.com/cosmos/cosmos-sdk/x/params/client/utils"

	"github.com/strangelove-ventures/interchaintest/v8/dockerutil"
)

// VoteOnProposal submits a vote for the specified proposal.
func (tn *ChainNode) VoteOnProposal(ctx context.Context, keyName string, proposalID uint64, vote string) error {
	_, err := tn.ExecTx(ctx, keyName,
		"gov", "vote",
		fmt.Sprintf("%d", proposalID), vote, "--gas", "auto",
	)
	return err
}

// SubmitProposal submits a gov v1 proposal to the chain.
func (tn *ChainNode) SubmitProposal(ctx context.Context, keyName string, prop TxProposalv1) (string, error) {
	file := "proposal.json"
	propJson, err := json.MarshalIndent(prop, "", " ")
	if err != nil {
		return "", err
	}

	fw := dockerutil.NewFileWriter(tn.logger(), tn.DockerClient, tn.TestName)
	if err := fw.WriteFile(ctx, tn.VolumeName, file, propJson); err != nil {
		return "", fmt.Errorf("writing contract file to docker volume: %w", err)
	}

	command := []string{
		"gov", "submit-proposal",
		path.Join(tn.HomeDir(), file), "--gas", "auto",
	}

	return tn.ExecTx(ctx, keyName, command...)
}

// GovSubmitProposal is an alias for SubmitProposal.
func (tn *ChainNode) GovSubmitProposal(ctx context.Context, keyName string, prop TxProposalv1) (string, error) {
	return tn.SubmitProposal(ctx, keyName, prop)
}

// UpgradeProposal submits a software-upgrade governance proposal to the chain.
func (tn *ChainNode) UpgradeProposal(ctx context.Context, keyName string, prop SoftwareUpgradeProposal) (string, error) {
	if tn.IsAboveSDK47(ctx) {
		cosmosChain := tn.Chain.(*CosmosChain)

		if prop.Authority == "" {
			authority, err := cosmosChain.GetGovernanceAddress(ctx)
			if err != nil {
				return "", err
			}
			prop.Authority = authority
		}

		msg := upgradetypes.MsgSoftwareUpgrade{
			Authority: prop.Authority,
			Plan: upgradetypes.Plan{
				Name:   prop.Name,
				Height: prop.Height,
				Info:   prop.Info,
			},
		}

		proposal, err := cosmosChain.BuildProposal([]ProtoMessage{&msg}, prop.Title, prop.Description, "", prop.Deposit, prop.Proposer, prop.Expedited)
		if err != nil {
			return "", err
		}
		return tn.SubmitProposal(ctx, keyName, proposal)
	}
	command := []string{
		"gov", "submit-proposal",
		"software-upgrade", prop.Name,
		"--upgrade-height", strconv.FormatInt(prop.Height, 10),
		"--title", prop.Title,
		"--description", prop.Description,
		"--deposit", prop.Deposit,
	}

	if prop.Info != "" {
		command = append(command, "--upgrade-info", prop.Info)
	}

	return tn.ExecTx(ctx, keyName, command...)
}

// TextProposal submits a text governance proposal to the chain.
func (tn *ChainNode) TextProposal(ctx context.Context, keyName string, prop TextProposal) (string, error) {
	command := []string{
		"gov", "submit-proposal",
		"--type", "text",
		"--title", prop.Title,
		"--description", prop.Description,
		"--deposit", prop.Deposit,
	}
	if prop.Expedited {
		command = append(command, "--is-expedited=true")
	}
	return tn.ExecTx(ctx, keyName, command...)
}

// ParamChangeProposal submits a param change proposal to the chain, signed by keyName.
func (tn *ChainNode) ParamChangeProposal(ctx context.Context, keyName string, prop *paramsutils.ParamChangeProposalJSON) (string, error) {
	content, err := json.Marshal(prop)
	if err != nil {
		return "", err
	}

	hash := sha256.Sum256(content)
	proposalFilename := fmt.Sprintf("%x.json", hash)
	err = tn.WriteFile(ctx, content, proposalFilename)
	if err != nil {
		return "", fmt.Errorf("writing param change proposal: %w", err)
	}

	proposalPath := filepath.Join(tn.HomeDir(), proposalFilename)

	command := []string{
		"gov", "submit-proposal",
		"param-change",
		proposalPath,
	}

	return tn.ExecTx(ctx, keyName, command...)
}

// Build a gov v1 proposal type.
//
// The proposer field should only be set for IBC-Go v8 / SDK v50 chains.
func (c *CosmosChain) BuildProposal(messages []ProtoMessage, title, summary, metadata, depositStr, proposer string, expedited bool) (TxProposalv1, error) {
	var propType TxProposalv1
	rawMsgs := make([]json.RawMessage, len(messages))

	for i, msg := range messages {
		msg, err := c.Config().EncodingConfig.Codec.MarshalInterfaceJSON(msg)
		if err != nil {
			return propType, err
		}
		rawMsgs[i] = msg
	}

	propType = TxProposalv1{
		Messages: rawMsgs,
		Metadata: metadata,
		Deposit:  depositStr,
		Title:    title,
		Summary:  summary,
	}

	// SDK v50 only
	if proposer != "" {
		propType.Proposer = proposer
		propType.Expedited = expedited
	}

	return propType, nil
}

// GovQueryProposal returns the state and details of a v1beta1 governance proposal.
func (c *CosmosChain) GovQueryProposal(ctx context.Context, proposalID uint64) (*govv1beta1.Proposal, error) {
	res, err := govv1beta1.NewQueryClient(c.GetNode().GrpcConn).Proposal(ctx, &govv1beta1.QueryProposalRequest{ProposalId: proposalID})
	if err != nil {
		return nil, err
	}

	return &res.Proposal, nil
}

// GovQueryProposalV1 returns the state and details of a v1 governance proposal.
func (c *CosmosChain) GovQueryProposalV1(ctx context.Context, proposalID uint64) (*govv1.Proposal, error) {
	res, err := govv1.NewQueryClient(c.GetNode().GrpcConn).Proposal(ctx, &govv1.QueryProposalRequest{ProposalId: proposalID})
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
