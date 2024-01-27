package cosmos_test

import (
	"context"
	"testing"

	"cosmossdk.io/math"
	"github.com/strangelove-ventures/interchaintest/v8"
	"github.com/strangelove-ventures/interchaintest/v8/chain/cosmos"
	interchaintestwasm "github.com/strangelove-ventures/interchaintest/v8/chain/cosmos/wasm"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

var (
	// cosmos.GenesisKV sets the genesis key-value state of the cosmos chain before startup.
	// To find the format of the genesis state, you can run `appd init <moniker>` on your local machine.
	// On Unix based systems, this will save the genesis file to `$HOME/.appd/config/genesis.json`
	sdk47Genesis = []cosmos.GenesisKV{
		cosmos.NewGenesisKV("app_state.gov.params.voting_period", "15s"),
		cosmos.NewGenesisKV("app_state.gov.params.max_deposit_period", "10s"),
		cosmos.NewGenesisKV("app_state.gov.params.min_deposit.0.denom", "ujuno"),
		cosmos.NewGenesisKV("app_state.gov.params.min_deposit.0.amount", "1"),
	}

	// The amount of token funds to send to each user when we request a faucet in the test
	genesisFundsAmt = math.NewInt(10_000_000_000)
)

func TestCometMock(t *testing.T) {
	// NewBuiltinChainFactory creates the base for a chain and its configuration.
	cf := interchaintest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*interchaintest.ChainSpec{
		{
			Name:      "juno",
			ChainName: "juno",
			Version:   "v16.0.0",
			ChainConfig: ibc.ChainConfig{
				Denom:          "ujuno",
				Bech32Prefix:   "juno",
				CoinType:       "118",
				ModifyGenesis:  cosmos.ModifyGenesis(sdk47Genesis),
				EncodingConfig: interchaintestwasm.WasmEncoding(),
				CometMockImage: []ibc.DockerImage{
					{
						// docker pull ghcr.io/informalsystems/cometmock:v0.37.x needs to be done automatically from my other PR (rollkit).
						Repository: "ghcr.io/informalsystems/cometmock",
						Version:    "v0.37.x",
						UidGid:     "1025:1025",
					},
				},
				HostPortOverride: map[int]int{ // debugging, remove
					26657: 26657,
					1317:  1317,
					9090:  9090,
					1234:  1234,
					26656: 26656,
				},
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

	// Faucet funds to a user
	users := interchaintest.GetAndFundTestUsers(t, ctx, "default", genesisFundsAmt, chain)
	user := users[0]

	// get the users balance
	balance, err := chain.GetBalance(ctx, user.FormattedAddress(), "ujuno")
	require.NoError(t, err)
	t.Logf("User balance: %s", balance)

	// send a tx
	err = chain.SendFunds(ctx, user.KeyName(), ibc.WalletAmount{
		Address: user.FormattedAddress(),
		Denom:   "ujuno",
		Amount:  math.NewInt(1),
	})
	require.NoError(t, err)
	err = chain.SendFunds(ctx, user.KeyName(), ibc.WalletAmount{
		Address: user.FormattedAddress(),
		Denom:   "ujuno",
		Amount:  math.NewInt(1),
	})
	require.NoError(t, err)
	err = chain.SendFunds(ctx, user.KeyName(), ibc.WalletAmount{
		Address: user.FormattedAddress(),
		Denom:   "ujuno",
		Amount:  math.NewInt(1),
	})
	require.NoError(t, err)
	err = chain.SendFunds(ctx, user.KeyName(), ibc.WalletAmount{
		Address: user.FormattedAddress(),
		Denom:   "ujuno",
		Amount:  math.NewInt(1),
	})
	require.NoError(t, err)
	err = chain.SendFunds(ctx, user.KeyName(), ibc.WalletAmount{
		Address: user.FormattedAddress(),
		Denom:   "ujuno",
		Amount:  math.NewInt(1),
	})
	require.NoError(t, err)
	err = chain.SendFunds(ctx, user.KeyName(), ibc.WalletAmount{
		Address: user.FormattedAddress(),
		Denom:   "ujuno",
		Amount:  math.NewInt(1),
	})
	require.NoError(t, err)
	err = chain.SendFunds(ctx, user.KeyName(), ibc.WalletAmount{
		Address: user.FormattedAddress(),
		Denom:   "ujuno",
		Amount:  math.NewInt(1),
	})
	require.NoError(t, err)
	err = chain.SendFunds(ctx, user.KeyName(), ibc.WalletAmount{
		Address: user.FormattedAddress(),
		Denom:   "ujuno",
		Amount:  math.NewInt(1),
	})
	require.NoError(t, err)
	err = chain.SendFunds(ctx, user.KeyName(), ibc.WalletAmount{
		Address: user.FormattedAddress(),
		Denom:   "ujuno",
		Amount:  math.NewInt(1),
	})
	require.NoError(t, err)
	err = chain.SendFunds(ctx, user.KeyName(), ibc.WalletAmount{
		Address: user.FormattedAddress(),
		Denom:   "ujuno",
		Amount:  math.NewInt(1),
	})
	require.NoError(t, err)
	err = chain.SendFunds(ctx, user.KeyName(), ibc.WalletAmount{
		Address: user.FormattedAddress(),
		Denom:   "ujuno",
		Amount:  math.NewInt(1),
	})
	require.NoError(t, err)
}
