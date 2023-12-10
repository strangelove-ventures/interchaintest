package cosmos

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"path"
	"path/filepath"
	"strconv"

	govv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	govv1beta1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1beta1"
	paramsutils "github.com/cosmos/cosmos-sdk/x/params/client/utils"
	"github.com/strangelove-ventures/interchaintest/v8/internal/dockerutil"
)

// VoteOnProposal submits a vote for the specified proposal.
func (tn *ChainNode) VoteOnProposal(ctx context.Context, keyName string, proposalID string, vote string) error {
	_, err := tn.ExecTx(ctx, keyName,
		"gov", "vote",
		proposalID, vote, "--gas", "auto",
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

// UpgradeProposal submits a software-upgrade governance proposal to the chain.
func (tn *ChainNode) UpgradeProposal(ctx context.Context, keyName string, prop SoftwareUpgradeProposal) (string, error) {
	command := []string{
		"gov", "submit-proposal",
		"software-upgrade", prop.Name,
		"--upgrade-height", strconv.FormatUint(prop.Height, 10),
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

// QueryProposal returns the state and details of a v1beta1 governance proposal.
func (c *CosmosChain) QueryProposal(ctx context.Context, proposalID uint64) (*govv1beta1.Proposal, error) {
	res, err := govv1beta1.NewQueryClient(c.GetNode().GrpcConn).Proposal(ctx, &govv1beta1.QueryProposalRequest{ProposalId: proposalID})
	if err != nil {
		return nil, err
	}

	return &res.Proposal, nil
}

// QueryProposal returns the state and details of a v1 governance proposal.
func (c *CosmosChain) QueryProposalV1(ctx context.Context, proposalID uint64) (*govv1.Proposal, error) {
	res, err := govv1.NewQueryClient(c.GetNode().GrpcConn).Proposal(ctx, &govv1.QueryProposalRequest{ProposalId: proposalID})
	if err != nil {
		return nil, err
	}

	return res.Proposal, nil
}
