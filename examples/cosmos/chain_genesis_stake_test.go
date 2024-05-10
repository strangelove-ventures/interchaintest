package cosmos_test

import (
	"context"
	"encoding/json"
	"strconv"
	"testing"

	types "github.com/cosmos/cosmos-sdk/types"
	"github.com/strangelove-ventures/interchaintest/v7"
	"github.com/strangelove-ventures/interchaintest/v7/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v7/ibc"
	"github.com/strangelove-ventures/interchaintest/v7/testreporter"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestChainGenesisUnequalStake(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	t.Parallel()
	const (
		denom      = "uatom"
		val1_stake = 1_000_000_000
		val2_stake = 2_000_000_000
		balance    = 1_000_000_000_000
	)
	validators := 2
	cf := interchaintest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*interchaintest.ChainSpec{
		{
			Name:          "gaia",
			ChainName:     "gaia",
			Version:       "v15.1.0",
			NumValidators: &validators,
			ChainConfig: ibc.ChainConfig{
				Denom: denom,
				ModifyGenesisAmounts: func(i int) (types.Coin, types.Coin) {
					if i == 0 {
						return types.NewCoin(denom, types.NewInt(balance)), types.NewCoin(denom, types.NewInt(val1_stake))
					}
					return types.NewCoin(denom, types.NewInt(balance)), types.NewCoin(denom, types.NewInt(val2_stake))
				},
			},
		},
	})

	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)

	client, network := interchaintest.DockerSetup(t)

	chain := chains[0].(*cosmos.CosmosChain)

	ic := interchaintest.NewInterchain().
		AddChain(chain)
	rep := testreporter.NewNopReporter()

	err = ic.Build(context.Background(), rep.RelayerExecReporter(t), interchaintest.InterchainBuildOptions{
		TestName:  t.Name(),
		Client:    client,
		NetworkID: network,
		// BlockDatabaseFile: interchaintest.DefaultBlockDatabaseFilepath(),
		SkipPathCreation: false,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = ic.Close()
	})

	stdout, _, err := chain.GetNode().ExecQuery(context.Background(), "staking", "validators")
	require.NoError(t, err)

	var validatorsResp map[string]interface{}
	require.NoError(t, json.Unmarshal(stdout, &validatorsResp))
	require.Contains(t, validatorsResp, "validators")
	validatorsList := validatorsResp["validators"].([]interface{})
	require.Len(t, validatorsList, 2)

	tokens1 := validatorsList[0].(map[string]interface{})["tokens"].(string)
	tokens2 := validatorsList[1].(map[string]interface{})["tokens"].(string)
	require.NotEmpty(t, tokens1)
	require.NotEmpty(t, tokens2)

	tokens1Int, err := strconv.Atoi(tokens1)
	require.NoError(t, err)
	tokens2Int, err := strconv.Atoi(tokens2)
	require.NoError(t, err)

	if tokens1Int > tokens2Int {
		require.Equal(t, val2_stake, tokens1Int)
		require.Equal(t, val1_stake, tokens2Int)
	} else {
		require.Equal(t, val1_stake, tokens1Int)
		require.Equal(t, val2_stake, tokens2Int)
	}

}
