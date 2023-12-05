package cosmos_test

import (
	"context"
	"encoding/json"
	"testing"

	sdkmath "cosmossdk.io/math"
	paramsutils "github.com/cosmos/cosmos-sdk/x/params/client/utils"
	"github.com/strangelove-ventures/interchaintest/v6"
	"github.com/strangelove-ventures/interchaintest/v6/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v6/ibc"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestJunoParamChange(t *testing.T) {
	CosmosChainParamChangeTest(t, "juno", "v13.0.1")
}

func CosmosChainParamChangeTest(t *testing.T, name, version string) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	t.Parallel()

	numVals := 1
	numFullNodes := 1

	// SDK v45 params for Juno genesis
	shortVoteGenesis := []cosmos.GenesisKV{
		{
			Key:   "app_state.gov.voting_params.voting_period",
			Value: votingPeriod,
		},
		{
			Key:   "app_state.gov.deposit_params.max_deposit_period",
			Value: maxDepositPeriod,
		},
		{
			Key:   "app_state.gov.deposit_params.min_deposit.0.denom",
			Value: "ujuno",
		},
	}

	cf := interchaintest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*interchaintest.ChainSpec{
		{
			Name:      name,
			ChainName: name,
			Version:   version,
			ChainConfig: ibc.ChainConfig{
				Denom:         "ujuno",
				ModifyGenesis: cosmos.ModifyGenesis(shortVoteGenesis),
			},
			NumValidators: &numVals,
			NumFullNodes:  &numFullNodes,
		},
	})

	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)

	chain := chains[0].(*cosmos.CosmosChain)

	ic := interchaintest.NewInterchain().
		AddChain(chain)

	ctx := context.Background()
	client, network := interchaintest.DockerSetup(t)

	require.NoError(t, ic.Build(ctx, nil, interchaintest.InterchainBuildOptions{
		TestName:         t.Name(),
		Client:           client,
		NetworkID:        network,
		SkipPathCreation: true,
	}))
	t.Cleanup(func() {
		_ = ic.Close()
	})

	var userFunds = sdkmath.NewInt(10_000_000_000)
	users := interchaintest.GetAndFundTestUsers(t, ctx, t.Name(), userFunds, chain)
	chainUser := users[0]

	param, _ := chain.QueryParam(ctx, "staking", "MaxValidators")
	require.Equal(t, "100", param.Value, "MaxValidators value is not 100")

	paramChangeValue := 110
	rawValue, err := json.Marshal(paramChangeValue)
	require.NoError(t, err)

	param_change := paramsutils.ParamChangeProposalJSON{
		Title:       "Increase validator set to 110",
		Description: ".",
		Changes: paramsutils.ParamChangesJSON{
			paramsutils.ParamChangeJSON{
				Subspace: "staking",
				Key:      "MaxValidators",
				Value:    rawValue,
			},
		},
		Deposit: "10000000ujuno",
	}

	paramTx, err := chain.ParamChangeProposal(ctx, chainUser.KeyName(), &param_change)
	require.NoError(t, err, "error submitting param change proposal tx")

	err = chain.VoteOnProposalAllValidators(ctx, paramTx.ProposalID, cosmos.ProposalVoteYes)
	require.NoError(t, err, "failed to submit votes")

	height, _ := chain.Height(ctx)
	_, err = cosmos.PollForProposalStatus(ctx, chain, height, height+10, paramTx.ProposalID, cosmos.ProposalStatusPassed)
	require.NoError(t, err, "proposal status did not change to passed in expected number of blocks")

	param, _ = chain.QueryParam(ctx, "staking", "MaxValidators")
	require.Equal(t, "110", param.Value, "MaxValidators value is not 110")
}
