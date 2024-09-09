package cosmos_test

import (
	"context"
	"testing"

	"cosmossdk.io/math"
	"github.com/strangelove-ventures/interchaintest/v9"
	"github.com/strangelove-ventures/interchaintest/v9/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v9/ibc"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

var (
	sdk47Genesis = []cosmos.GenesisKV{
		cosmos.NewGenesisKV("app_state.gov.params.voting_period", "15s"),
		cosmos.NewGenesisKV("app_state.gov.params.max_deposit_period", "10s"),
		cosmos.NewGenesisKV("app_state.gov.params.min_deposit.0.denom", "ujuno"),
		cosmos.NewGenesisKV("app_state.gov.params.min_deposit.0.amount", "1"),
	}

	genesisFundsAmt = math.NewInt(10_000_000_000)
)

func TestCometMock(t *testing.T) {
	cf := interchaintest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*interchaintest.ChainSpec{
		{
			Name:      "juno",
			ChainName: "juno",
			// NOTE: To use CometMock you must use an SDK version with patch: https://github.com/cosmos/cosmos-sdk/issues/16277.
			// SDK v0.47.6+
			Version: "v19.0.0-alpha.3",
			ChainConfig: ibc.ChainConfig{
				Denom:         "ujuno",
				Bech32Prefix:  "juno",
				CoinType:      "118",
				ModifyGenesis: cosmos.ModifyGenesis(sdk47Genesis),
				CometMock: ibc.CometMockConfig{
					Image:       ibc.NewDockerImage("ghcr.io/informalsystems/cometmock", "v0.37.x", "1025:1025"),
					BlockTimeMs: 200,
				},
				GasPrices: "0ujuno",
			},
			NumValidators: &numValsOne,
			NumFullNodes:  &numFullNodesZero,
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

	// Faucet funds to a user
	users := interchaintest.GetAndFundTestUsers(t, ctx, "default", genesisFundsAmt, chain, chain)
	user := users[0]
	user2 := users[1]

	// get the users balance
	initBal, err := chain.GetBalance(ctx, user.FormattedAddress(), "ujuno")
	require.NoError(t, err)

	// Send many transactions in a row
	for i := 0; i < 10; i++ {
		require.NoError(t, chain.SendFunds(ctx, user.KeyName(), ibc.WalletAmount{
			Address: user2.FormattedAddress(),
			Denom:   "ujuno",
			Amount:  math.NewInt(1),
		}))
		require.NoError(t, chain.SendFunds(ctx, user2.KeyName(), ibc.WalletAmount{
			Address: user.FormattedAddress(),
			Denom:   "ujuno",
			Amount:  math.NewInt(1),
		}))
	}

	endBal, err := chain.GetBalance(ctx, user.FormattedAddress(), "ujuno")
	require.NoError(t, err)
	require.EqualValues(t, initBal, endBal)

}
