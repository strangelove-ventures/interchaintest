package cosmos_test

import (
	"context"
	"testing"

	"cosmossdk.io/math"
	"github.com/strangelove-ventures/interchaintest/v8"
	"github.com/strangelove-ventures/interchaintest/v8/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestCoreSDKCommands(t *testing.T) {
	// TODO: simplify this test to the basics
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	numVals := 1
	numFullNodes := 0

	cosmos.SetSDKConfig("juno")

	sdk47Genesis := []cosmos.GenesisKV{
		cosmos.NewGenesisKV("app_state.gov.params.voting_period", "15s"),
		cosmos.NewGenesisKV("app_state.gov.params.max_deposit_period", "10s"),
		cosmos.NewGenesisKV("app_state.gov.params.min_deposit.0.denom", "ujuno"),
		cosmos.NewGenesisKV("app_state.gov.params.min_deposit.0.amount", "1"),
	}

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
				EncodingConfig: wasmEncoding(),
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

	users := interchaintest.GetAndFundTestUsers(t, ctx, "default", math.NewInt(10_000_000_000), chain, chain)

	testAuthz(ctx, t, chain, users)
}

func testAuthz(ctx context.Context, t *testing.T, chain *cosmos.CosmosChain, users []ibc.Wallet) {
	// Grant BankSend Authz
	txRes, _ := cosmos.AuthzGrant(ctx, chain, users[0], users[1].FormattedAddress(), "/cosmos.bank.v1beta1.MsgSend")
	require.EqualValues(t, 0, txRes.Code)

	granter := users[0].FormattedAddress()
	grantee := users[1].FormattedAddress()

	grants, err := cosmos.AuthzQueryGrants(ctx, chain, granter, grantee, "")
	require.NoError(t, err)
	require.Len(t, grants.Grants, 1)
	require.EqualValues(t, grants.Grants[0].Authorization.Msg, "/cosmos.bank.v1beta1.MsgSend")

	byGrantee, err := cosmos.AuthzQueryGrantsByGrantee(ctx, chain, grantee, "")
	require.NoError(t, err)
	require.Len(t, byGrantee.Grants, 1)
	require.EqualValues(t, byGrantee.Grants[0].Granter, granter)
	require.EqualValues(t, byGrantee.Grants[0].Grantee, grantee)

	byGranter, err := cosmos.AuthzQueryGrantsByGranter(ctx, chain, granter, "")
	require.NoError(t, err)
	require.Len(t, byGranter.Grants, 1)
	require.EqualValues(t, byGranter.Grants[0].Granter, granter)
	require.EqualValues(t, byGranter.Grants[0].Grantee, grantee)

	// TODO: Perform bank send action here on behalf of granter
}
