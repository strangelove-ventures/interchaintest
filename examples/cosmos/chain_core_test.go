package cosmos_test

import (
	"context"
	"fmt"
	"testing"

	"cosmossdk.io/math"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/strangelove-ventures/interchaintest/v8"
	"github.com/strangelove-ventures/interchaintest/v8/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestCoreSDKCommands(t *testing.T) {
	// TODO: simplify this test to the basics and convert to SDK v50
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	numVals := 1
	numFullNodes := 0

	cosmos.SetSDKConfig("cosmos")

	sdk47Genesis := []cosmos.GenesisKV{
		cosmos.NewGenesisKV("app_state.gov.params.voting_period", "15s"),
		cosmos.NewGenesisKV("app_state.gov.params.max_deposit_period", "10s"),
		cosmos.NewGenesisKV("app_state.gov.params.min_deposit.0.denom", "token"),
		cosmos.NewGenesisKV("app_state.gov.params.min_deposit.0.amount", "1"),
	}

	cf := interchaintest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*interchaintest.ChainSpec{
		{
			Name:      "ibc-go-simd",
			ChainName: "ibc-go-simd",
			Version:   "v8.0.0", // SDK v50
			ChainConfig: ibc.ChainConfig{
				Denom:         "token",
				Bech32Prefix:  "cosmos",
				CoinType:      "118",
				ModifyGenesis: cosmos.ModifyGenesis(sdk47Genesis),
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
	testBank(ctx, t, chain, users)
}

func testAuthz(ctx context.Context, t *testing.T, chain *cosmos.CosmosChain, users []ibc.Wallet) {
	granter := users[0].FormattedAddress()
	grantee := users[1].FormattedAddress()

	// Grant BankSend Authz
	txRes, _ := cosmos.AuthzGrant(ctx, chain, users[0], grantee, "/cosmos.bank.v1beta1.MsgSend")
	require.EqualValues(t, 0, txRes.Code)

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

	// Perform BankSend tx via authz

	// before balance
	balanceBefore, err := chain.GetBalance(ctx, granter, chain.Config().Denom)
	require.NoError(t, err)
	fmt.Printf("balanceBefore: %+v\n", balanceBefore)

	sendAmt := 1234

	nestedCmd := []string{
		"tx", "bank", "send", granter, grantee, fmt.Sprintf("%d%s", sendAmt, chain.Config().Denom),
		"--from", granter, "--generate-only",
		"--chain-id", chain.GetNode().Chain.Config().ChainID,
		"--node", chain.GetNode().Chain.GetRPCAddress(),
		"--home", chain.GetNode().HomeDir(),
		"--keyring-backend", keyring.BackendTest,
		"--output", "json",
		"--yes",
	}

	resp, err := cosmos.AuthzExec(ctx, chain, users[1], nestedCmd)
	require.NoError(t, err)
	fmt.Printf("resp: %+v\n", resp)

	// after balance
	balanceAfter, err := chain.GetBalance(ctx, granter, chain.Config().Denom)
	require.NoError(t, err)

	fmt.Printf("balanceAfter: %+v\n", balanceAfter)

	require.EqualValues(t, balanceBefore.SubRaw(int64(sendAmt)), balanceAfter)
}

func testBank(ctx context.Context, t *testing.T, chain *cosmos.CosmosChain, users []ibc.Wallet) {
	b1, err := chain.BankGetBalance(ctx, users[0].FormattedAddress(), chain.Config().Denom)
	require.NoError(t, err)

	b2, err := chain.BankGetBalance(ctx, users[1].FormattedAddress(), chain.Config().Denom)
	require.NoError(t, err)

	fmt.Printf("b1: %+v\n", b1)
	fmt.Printf("b2: %+v\n", b2)

	sendAmt := int64(1)
	_, err = sendTokens(ctx, chain, users[0], users[1], "", sendAmt)
	require.NoError(t, err)

	b2New, err := chain.GetBalance(ctx, users[1].FormattedAddress(), chain.Config().Denom)
	require.NoError(t, err)
	require.Equal(t, b2.Add(math.NewInt(sendAmt)), b2New)

	// other chain query functions
	// res1, err :+ chain.BankQuery...
}
