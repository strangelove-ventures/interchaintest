package cosmos_test

import (
	"context"
	"os"
	"testing"

	"github.com/strangelove-ventures/interchaintest/v8"
	"github.com/strangelove-ventures/interchaintest/v8/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

// go test -timeout 3000s -run ^TestChainGordian$ github.com/strangelove-ventures/interchaintest/v8/examples/cosmos -v
func TestChainGordian(t *testing.T) {
	// TODO: this test is only local for now. Will add CI in the future
	if os.Getenv("IS_LOCAL_TESTING_GORDIAN") == "" {
		t.Skip("skipping test; set IS_LOCAL_TESTING_GORDIAN to run this test")
	}

	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	t.Parallel()

	cosmos.SetSDKConfig(baseBech32)

	sdk47Genesis := []cosmos.GenesisKV{
		cosmos.NewGenesisKV("app_state.gov.params.voting_period", "15s"),
		cosmos.NewGenesisKV("app_state.gov.params.max_deposit_period", "10s"),
		cosmos.NewGenesisKV("app_state.gov.params.min_deposit.0.denom", "token"),
		cosmos.NewGenesisKV("app_state.gov.params.min_deposit.0.amount", "1"),
		cosmos.NewGenesisKV("app_state.bank.denom_metadata", []banktypes.Metadata{denomMetadata}),
	}

	decimals := int64(6)
	cf := interchaintest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*interchaintest.ChainSpec{
		{
			Name:      "gordianproject",
			ChainName: "gordianproject",
			Version:   "local", // spawn -> gordian, modify docker file, build
			ChainConfig: ibc.ChainConfig{
				Images: []ibc.DockerImage{
					{
						Repository: "gordianproject",
						Version:    "local",
						UidGid:     "1025:1025",
					},
				},
				Type:           "cosmos",
				Name:           "gordian",
				ChainID:        "gordian-1",
				GasPrices:      "0.0" + denomMetadata.Base,
				CoinDecimals:   &decimals,
				Bin:            "appd",
				TrustingPeriod: "330h",
				AdditionalStartArgs: []string{
					"--g-http-addr", ":26657",
					"--g-grpc-addr", ":9092", // gRPC 9090 is already used by the SDK.
				},
				Denom:         denomMetadata.Base,
				Bech32Prefix:  baseBech32,
				CoinType:      "118",
				ModifyGenesis: cosmos.ModifyGenesis(sdk47Genesis),
				GasAdjustment: 1.5,
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

	// TODO: gordian does not yet accept standard tx commands, it requires a manual broadcast of a generate only. Need to submit the raw bytes properly
	// users := interchaintest.GetAndFundTestUsers(t, ctx, "default", genesisAmt, chain, chain)
	// user1 := users[1].FormattedAddress()
	// fmt.Println("user1", user1, "yuh")

	// b2, err := chain.BankQueryBalance(ctx, user1, chain.Config().Denom)
	// require.NoError(t, err)

	// fmt.Println("b2", b2)

	// send 1 token
	// sendAmt := int64(1)
	// _, err = sendTokens(ctx, chain, users[0], users[1], "", sendAmt)
	// require.NoError(t, err)

	// // check balances
	// b2New, err := chain.GetBalance(ctx, user1, chain.Config().Denom)
	// require.NoError(t, err)
	// require.Equal(t, b2.Add(sdkmath.NewInt(sendAmt)), b2New)

}
