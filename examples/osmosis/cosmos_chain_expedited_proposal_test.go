package osmosis_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/icza/dyno"
	"github.com/strangelove-ventures/interchaintest/v4"
	"github.com/strangelove-ventures/interchaintest/v4/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v4/examples/osmosis"
	"github.com/strangelove-ventures/interchaintest/v4/ibc"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestOsmosisExpeditedProposal(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	t.Parallel()

	const (
		votingPeriod          = "1h"
		expeditedVotingPeriod = "10s"
		maxDepositPeriod      = "10s"
		minDeposit            = "10000000"
		minExpeditedDeposit   = "50000000"
	)
	chainSpec := &interchaintest.ChainSpec{
		Name:      "osmosis",
		ChainName: "osmosis",
		Version:   "main",
		ChainConfig: ibc.ChainConfig{
			ChainID:        "osmosis-1001", // hardcoded handling in osmosis binary for osmosis-1, so need to override to something different.
			ModifyGenesis:  modifyGenesisExpeditedProposals(votingPeriod, expeditedVotingPeriod, minDeposit, minExpeditedDeposit, maxDepositPeriod),
			EncodingConfig: osmosis.OsmosisEncoding(),
		},
	}

	cf := interchaintest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*interchaintest.ChainSpec{chainSpec})

	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)

	chain := chains[0].(*cosmos.CosmosChain)

	ic := interchaintest.NewInterchain().
		AddChain(chain)

	ctx := context.Background()
	client, network := interchaintest.DockerSetup(t)

	require.NoError(t, ic.Build(ctx, nil, interchaintest.InterchainBuildOptions{
		TestName:  t.Name(),
		Client:    client,
		NetworkID: network,
		// BlockDatabaseFile: interchaintest.DefaultBlockDatabaseFilepath(),
		SkipPathCreation: true,
	}))
	t.Cleanup(func() {
		_ = ic.Close()
	})

	const userFunds = int64(10_000_000_000)
	users := interchaintest.GetAndFundTestUsers(t, ctx, t.Name(), userFunds, chain)
	chainUser := users[0]

	// submit non-expedited proposal first. will make assertions last, since this one should still
	// be in voting period after the expedited proposal passes.
	nonExpeditedProposalTx, err := chain.TextProposal(ctx, chainUser.KeyName, cosmos.TextProposal{
		Deposit:     minDeposit + chain.Config().Denom,
		Title:       "Chain Proposal 1",
		Description: "Non-expedited chain governance proposal",
		Expedited:   false,
	})
	require.NoError(t, err, "error submitting text proposal tx")

	expeditedProposalTx, err := chain.TextProposal(ctx, chainUser.KeyName, cosmos.TextProposal{
		Deposit:     minExpeditedDeposit + chain.Config().Denom,
		Title:       "Chain Proposal 2",
		Description: "Expedited chain governance proposal",
		Expedited:   true,
	})
	require.NoError(t, err, "error submitting text proposal tx")

	postProposalHeight, err := chain.Height(ctx)
	require.NoError(t, err, "failed to fetch chain height after submitting proposals")

	err = chain.VoteOnProposalAllValidators(ctx, nonExpeditedProposalTx.ProposalID, cosmos.ProposalVoteYes)
	require.NoError(t, err, "failed to submit votes")

	err = chain.VoteOnProposalAllValidators(ctx, expeditedProposalTx.ProposalID, cosmos.ProposalVoteYes)
	require.NoError(t, err, "failed to submit votes")

	_, err = cosmos.PollForProposalStatus(ctx, chain, postProposalHeight, postProposalHeight+10, expeditedProposalTx.ProposalID, cosmos.ProposalStatusPassed)
	require.NoError(t, err, "expedited proposal did not change to passed status in expected timeframe")

	nonExpeditedProposal, err := chain.QueryProposal(ctx, nonExpeditedProposalTx.ProposalID)
	require.NoError(t, err, "failed to query non-expedited proposal status")
	require.Equal(t, cosmos.ProposalStatusVotingPeriod, nonExpeditedProposal.Status, "non-expedited proposal is not in voting period")
}

func modifyGenesisExpeditedProposals(
	votingPeriod, expeditedVotingPeriod string,
	minDepositAmount, minExpeditedDepositAmount string,
	maxDepositPeriod string,
) func(ibc.ChainConfig, []byte) ([]byte, error) {
	return func(chainConfig ibc.ChainConfig, genbz []byte) ([]byte, error) {
		g := make(map[string]interface{})
		if err := json.Unmarshal(genbz, &g); err != nil {
			return nil, fmt.Errorf("failed to unmarshal genesis file: %w", err)
		}
		if err := dyno.Set(g, votingPeriod, "app_state", "gov", "voting_params", "voting_period"); err != nil {
			return nil, fmt.Errorf("failed to set voting period in genesis json: %w", err)
		}
		if err := dyno.Set(g, expeditedVotingPeriod, "app_state", "gov", "voting_params", "expedited_voting_period"); err != nil {
			return nil, fmt.Errorf("failed to set voting period in genesis json: %w", err)
		}
		if err := dyno.Set(g, maxDepositPeriod, "app_state", "gov", "deposit_params", "max_deposit_period"); err != nil {
			return nil, fmt.Errorf("failed to set voting period in genesis json: %w", err)
		}
		if err := dyno.Set(g, chainConfig.Denom, "app_state", "gov", "deposit_params", "min_deposit", 0, "denom"); err != nil {
			return nil, fmt.Errorf("failed to set min deposit denom in genesis json: %w", err)
		}
		if err := dyno.Set(g, minDepositAmount, "app_state", "gov", "deposit_params", "min_deposit", 0, "amount"); err != nil {
			return nil, fmt.Errorf("failed to set min deposit amount in genesis json: %w", err)
		}
		if err := dyno.Set(g, chainConfig.Denom, "app_state", "gov", "deposit_params", "min_expedited_deposit", 0, "denom"); err != nil {
			return nil, fmt.Errorf("failed to set min deposit denom in genesis json: %w", err)
		}
		if err := dyno.Set(g, minExpeditedDepositAmount, "app_state", "gov", "deposit_params", "min_expedited_deposit", 0, "amount"); err != nil {
			return nil, fmt.Errorf("failed to set min expedited deposit amount in genesis json: %w", err)
		}
		out, err := json.Marshal(g)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal genesis bytes to json: %w", err)
		}
		return out, nil
	}
}
