package cosmos_test

import (
	"context"
	"fmt"
	"testing"

	sdkmath "cosmossdk.io/math"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/strangelove-ventures/interchaintest/v8"
	"github.com/strangelove-ventures/interchaintest/v8/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestCoreSDKCommands(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	numVals := 1
	numFullNodes := 0

	cosmos.SetSDKConfig("cosmos")

	denomMetadata := banktypes.Metadata{
		Description: "Denom metadata for TOK (token)",
		DenomUnits: []*banktypes.DenomUnit{
			{
				Denom:    "token",
				Exponent: 0,
				Aliases:  []string{},
			},
			{
				Denom:    "TOK",
				Exponent: 6,
				Aliases:  []string{},
			},
		},
		Base:    "token",
		Display: "TOK",
		Name:    "TOK",
		Symbol:  "TOK",
		URI:     "",
		URIHash: "",
	}

	sdk47Genesis := []cosmos.GenesisKV{
		cosmos.NewGenesisKV("app_state.gov.params.voting_period", "15s"),
		cosmos.NewGenesisKV("app_state.gov.params.max_deposit_period", "10s"),
		cosmos.NewGenesisKV("app_state.gov.params.min_deposit.0.denom", "token"),
		cosmos.NewGenesisKV("app_state.gov.params.min_deposit.0.amount", "1"),
		cosmos.NewGenesisKV("app_state.bank.denom_metadata", []banktypes.Metadata{denomMetadata}),
	}

	cf := interchaintest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*interchaintest.ChainSpec{
		{
			Name:      "ibc-go-simd",
			ChainName: "ibc-go-simd",
			Version:   "v8.0.0", // SDK v50
			ChainConfig: ibc.ChainConfig{
				Denom:         denomMetadata.Base,
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

	genesisAmt := sdkmath.NewInt(10_000_000_000)
	users := interchaintest.GetAndFundTestUsers(t, ctx, "default", genesisAmt, chain, chain, chain)

	testAuthz(ctx, t, chain, users)
	testBank(ctx, t, chain, users)
}

func testAuthz(ctx context.Context, t *testing.T, chain *cosmos.CosmosChain, users []ibc.Wallet) {
	granter := users[0].FormattedAddress()
	grantee := users[1].FormattedAddress()

	// Grant BankSend Authz
	// TODO: test other types as well (send is giving a NPE)
	txRes, _ := cosmos.AuthzGrant(ctx, chain, users[0], grantee, "generic", "--msg-type", "/cosmos.bank.v1beta1.MsgSend")
	require.EqualValues(t, 0, txRes.Code)

	grants, err := cosmos.AuthzQueryGrants(ctx, chain, granter, grantee, "")
	require.NoError(t, err)
	require.Len(t, grants.Grants, 1)
	require.EqualValues(t, grants.Grants[0].Authorization.Type, "cosmos-sdk/GenericAuthorization")
	require.EqualValues(t, grants.Grants[0].Authorization.Value.Msg, "/cosmos.bank.v1beta1.MsgSend")

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

	fmt.Printf("grants: %+v %+v %+v\n", grants, byGrantee, byGranter)

	// Perform BankSend tx via authz (make sure to put this )

	// before balance
	balanceBefore, err := chain.GetBalance(ctx, granter, chain.Config().Denom)
	require.NoError(t, err)
	fmt.Printf("balanceBefore: %+v\n", balanceBefore)

	sendAmt := 1234

	nestedCmd := []string{
		chain.Config().Bin,
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
	require.EqualValues(t, 0, resp.Code)

	// after balance
	balanceAfter, err := chain.GetBalance(ctx, granter, chain.Config().Denom)
	require.NoError(t, err)

	fmt.Printf("balanceAfter: %+v\n", balanceAfter)
	require.EqualValues(t, balanceBefore.SubRaw(int64(sendAmt)), balanceAfter)
}

func testBank(ctx context.Context, t *testing.T, chain *cosmos.CosmosChain, users []ibc.Wallet) {

	user0 := users[0].FormattedAddress()
	user1 := users[1].FormattedAddress()
	user2 := users[2].FormattedAddress()

	// Verify genesis amounts were set properly
	b1, err := chain.BankGetBalance(ctx, user0, chain.Config().Denom)
	require.NoError(t, err)
	b2, err := chain.BankGetBalance(ctx, user1, chain.Config().Denom)
	require.NoError(t, err)
	require.EqualValues(t, b1, b2)

	// send 1 token
	sendAmt := int64(1)
	_, err = sendTokens(ctx, chain, users[0], users[1], "", sendAmt)
	require.NoError(t, err)

	// send multiple
	err = chain.GetNode().BankMultiSend(ctx, users[0].KeyName(), []string{user1, user2}, sdkmath.NewInt(sendAmt), chain.Config().Denom)
	require.NoError(t, err)

	// == balances ==
	// sendAmt*2 because of the multisend as well
	b2New, err := chain.GetBalance(ctx, user1, chain.Config().Denom)
	require.NoError(t, err)
	require.Equal(t, b2.Add(sdkmath.NewInt(sendAmt*2)), b2New)

	b2All, err := chain.BankAllBalances(ctx, user1)
	require.NoError(t, err)
	require.Equal(t, b2New, b2All.AmountOf(chain.Config().Denom))

	// == spendable balances ==
	spendableBal, err := chain.BankQuerySpendableBalance(ctx, user0, chain.Config().Denom)
	require.NoError(t, err)

	spendableBals, err := chain.BankQuerySpendableBalances(ctx, user0)
	require.NoError(t, err)
	require.Equal(t, spendableBal.Amount, spendableBals.AmountOf(chain.Config().Denom))

	// == metadata ==
	meta, err := chain.BankDenomMetadata(ctx, chain.Config().Denom)
	require.NoError(t, err)

	meta2, err := chain.BankQueryDenomMetadataByQueryString(ctx, chain.Config().Denom)
	require.NoError(t, err)
	require.EqualValues(t, meta, meta2)

	allMeta, err := chain.BankQueryDenomsMetadata(ctx)
	require.NoError(t, err)
	require.Len(t, allMeta, 1)
	require.EqualValues(t, allMeta[0].Display, meta.Display)

	// == params ==
	params, err := chain.BankQueryParams(ctx)
	require.NoError(t, err)
	require.True(t, params.DefaultSendEnabled)

	sendEnabled, err := chain.BankQuerySendEnabled(ctx, []string{chain.Config().Denom})
	require.NoError(t, err)
	require.Len(t, sendEnabled, 0)

	// == supply ==
	supply, err := chain.BankQueryTotalSupply(ctx)
	require.NoError(t, err)

	supplyOf, err := chain.BankQueryTotalSupplyOf(ctx, chain.Config().Denom)
	require.NoError(t, err)
	require.EqualValues(t, supply.AmountOf(chain.Config().Denom), supplyOf.Amount)

	// == denom owner ==
	denomOwner, err := chain.BankQueryDenomOwners(ctx, chain.Config().Denom)
	require.NoError(t, err)

	found := false
	for _, owner := range denomOwner {
		if owner.Address == user0 {
			found = true
			break
		}
	}
	require.True(t, found)

}
