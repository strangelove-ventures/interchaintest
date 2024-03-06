package templates

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

	// Set the number of validators and full nodes in the network.
	// You must have at least 1 validator to operate a Cosmos chain.
	// By default, the first validator node (validator[0]) runs all queries.
	// Transactions can take place on any validator or full node.
	numVals      = 1
	numFullNodes = 0

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

// Run this test with:
// go test -timeout 3000s -run TestSingleCosmosChain github.com/strangelove-ventures/interchaintest/v8/templates -v
func TestSingleCosmosChain(t *testing.T) {
	// NewBuiltinChainFactory creates the base for a chain and its configuration.
	cf := interchaintest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*interchaintest.ChainSpec{
		{
			// The name and version of the network we want to test against is set here.
			// You can find a list of pre-configured networks at: https://github.com/strangelove-ventures/heighliner/blob/main/chains.yaml
			// Versions can be found at: https://github.com/orgs/strangelove-ventures/packages?repo_name=heighliner
			// If you want to test against your custom chain, you can view the `docs` directory in the root of this repository.
			Name:      "juno",
			ChainName: "juno",
			Version:   "v16.0.0",
			ChainConfig: ibc.ChainConfig{
				Denom:          "ujuno",
				Bech32Prefix:   "juno",
				CoinType:       "118",
				ModifyGenesis:  cosmos.ModifyGenesis(sdk47Genesis),
				EncodingConfig: interchaintestwasm.WasmEncoding(),
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
}

// Run this test with:
// go test -timeout 3000s -run TestSingleChainReduced github.com/strangelove-ventures/interchaintest/v8/templates -v
// TODO: EXPLAIN
func TestSingleChainReduced(t *testing.T) {
	validators := 1
	fullNodes := 0

	// Create a single chain
	chains := interchaintest.CreateChainWithConfig(t, validators, fullNodes, "juno", "v16.0.0", ibc.ChainConfig{
		ChainID:       "localjuno-1",
		Name:          "single-chain",
		GasPrices:     "0ujuno",
		ModifyGenesis: cosmos.ModifyGenesis(sdk47Genesis),
	})

	enableBlockDB := false
	ctx, _, _, _ := interchaintest.BuildInitialChain(t, chains, enableBlockDB)

	// grab the chain
	chain := chains[0].(*cosmos.CosmosChain)

	// Faucet funds to a user
	users := interchaintest.GetAndFundTestUsers(t, ctx, "default", genesisFundsAmt, chain)
	user := users[0]

	// get the users balance
	balance, err := chain.GetBalance(ctx, user.FormattedAddress(), "ujuno")
	require.NoError(t, err)
	t.Logf("User balance: %s", balance)
}
