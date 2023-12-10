package cosmos_test

import (
	"context"
	"fmt"
	"math"
	"testing"
	"time"

	sdkmath "cosmossdk.io/math"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/strangelove-ventures/interchaintest/v8"
	"github.com/strangelove-ventures/interchaintest/v8/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

func TestCoreSDKCommands(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	t.Parallel()

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
		// high signing rate limit, easy jailing (ref POA) with 4 vals
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

	t.Run("authz", func(t *testing.T) {
		users := interchaintest.GetAndFundTestUsers(t, ctx, "default", genesisAmt, chain, chain, chain)
		testAuthz(ctx, t, chain, users)
	})

	t.Run("bank", func(t *testing.T) {
		users := interchaintest.GetAndFundTestUsers(t, ctx, "default", genesisAmt, chain, chain, chain)
		testBank(ctx, t, chain, users)
	})

	t.Run("distribution", func(t *testing.T) {
		users := interchaintest.GetAndFundTestUsers(t, ctx, "default", genesisAmt, chain, chain, chain)
		testDistribution(ctx, t, chain, users)
	})

	t.Run("feegrant", func(t *testing.T) {
		users := interchaintest.GetAndFundTestUsers(t, ctx, "default", genesisAmt, chain, chain, chain, chain)
		testFeeGrant(ctx, t, chain, users)
	})

	t.Run("gov", func(t *testing.T) {
		users := interchaintest.GetAndFundTestUsers(t, ctx, "default", genesisAmt, chain, chain, chain)
		testGov(ctx, t, chain, users)
	})
}

func testAuthz(ctx context.Context, t *testing.T, chain *cosmos.CosmosChain, users []ibc.Wallet) {
	granter := users[0].FormattedAddress()
	grantee := users[1].FormattedAddress()

	// Grant BankSend Authz
	// TODO: test other types as well (send is giving a NPE) (or move to only generic types)
	txRes, _ := cosmos.AuthzGrant(ctx, chain, users[0], grantee, "generic", "--msg-type", "/cosmos.bank.v1beta1.MsgSend")
	require.EqualValues(t, 0, txRes.Code)

	grants, err := chain.AuthzQueryGrants(ctx, granter, grantee, "")
	require.NoError(t, err)
	require.Len(t, grants, 1)
	require.EqualValues(t, grants[0].Authorization.TypeUrl, "/cosmos.authz.v1beta1.GenericAuthorization")
	require.Contains(t, string(grants[0].Authorization.Value), "/cosmos.bank.v1beta1.MsgSend")

	byGrantee, err := chain.AuthzQueryGrantsByGrantee(ctx, grantee, "")
	require.NoError(t, err)
	require.Len(t, byGrantee, 1)
	require.EqualValues(t, byGrantee[0].Granter, granter)
	require.EqualValues(t, byGrantee[0].Grantee, grantee)

	byGranter, err := chain.AuthzQueryGrantsByGranter(ctx, granter, "")
	require.NoError(t, err)
	require.Len(t, byGranter, 1)
	require.EqualValues(t, byGranter[0].Granter, granter)
	require.EqualValues(t, byGranter[0].Grantee, grantee)

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

	b2, err := chain.BankGetBalance(ctx, user1, chain.Config().Denom)
	require.NoError(t, err)

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

func testDistribution(ctx context.Context, t *testing.T, chain *cosmos.CosmosChain, users []ibc.Wallet) {
	var err error
	node := chain.GetNode()
	acc := authtypes.NewModuleAddress("distribution")
	require := require.New(t)

	vals, err := chain.StakingGetValidators(ctx, stakingtypes.Bonded.String())
	require.NoError(err)
	fmt.Printf("validators: %+v\n", vals)

	del, err := chain.StakingGetDelegationsTo(ctx, vals[0].OperatorAddress)
	require.NoError(err)

	delAddr := del[0].Delegation.DelegatorAddress
	valAddr := del[0].Delegation.ValidatorAddress

	newWithdrawAddr := "cosmos1hj83l3auyqgy5qcp52l6sp2e67xwq9xx80alju"

	t.Run("misc queries", func(t *testing.T) {
		slashes, err := chain.DistributionValidatorSlashes(ctx, valAddr)
		require.NoError(err)
		require.EqualValues(0, len(slashes))

		valDistInfo, err := chain.DistributionValidatorDistributionInfo(ctx, valAddr)
		require.NoError(err)
		fmt.Printf("valDistInfo: %+v\n", valDistInfo)
		require.EqualValues(1, valDistInfo.Commission.Len())

		valOutRewards, err := chain.DistributionValidatorOutstandingRewards(ctx, valAddr)
		require.NoError(err)
		require.EqualValues(1, valOutRewards.Rewards.Len())

		params, err := chain.DistributionParams(ctx)
		require.NoError(err)
		require.True(params.WithdrawAddrEnabled)

		comm, err := chain.DistributionCommission(ctx, valAddr)
		require.NoError(err)
		require.EqualValues(chain.Config().Denom, comm.Commission[0].Denom)
	})

	t.Run("withdraw-all-rewards", func(t *testing.T) {
		err = node.StakingDelegate(ctx, users[2].KeyName(), valAddr, fmt.Sprintf("%d%s", uint64(100*math.Pow10(6)), chain.Config().Denom))
		require.NoError(err)

		before, err := chain.BankGetBalance(ctx, acc.String(), chain.Config().Denom)
		require.NoError(err)
		fmt.Printf("before: %+v\n", before)

		err = node.DistributionWithdrawAllRewards(ctx, users[2].KeyName())
		require.NoError(err)

		after, err := chain.BankGetBalance(ctx, acc.String(), chain.Config().Denom)
		require.NoError(err)
		fmt.Printf("after: %+v\n", after)
		require.True(after.GT(before))
	})

	// fund pools
	t.Run("fund-pools", func(t *testing.T) {
		bal, err := chain.BankGetBalance(ctx, acc.String(), chain.Config().Denom)
		require.NoError(err)
		fmt.Printf("CP balance: %+v\n", bal)

		amount := uint64(9_000 * math.Pow10(6))

		err = node.DistributionFundCommunityPool(ctx, users[0].KeyName(), fmt.Sprintf("%d%s", amount, chain.Config().Denom))
		require.NoError(err)

		err = node.DistributionFundValidatorRewardsPool(ctx, users[0].KeyName(), valAddr, fmt.Sprintf("%d%s", uint64(100*math.Pow10(6)), chain.Config().Denom))
		require.NoError(err)

		bal2, err := chain.BankGetBalance(ctx, acc.String(), chain.Config().Denom)
		require.NoError(err)
		fmt.Printf("New CP balance: %+v\n", bal2) // 9147579661

		require.True(bal2.Sub(bal).GT(sdkmath.NewInt(int64(amount))))

		// queries
		coins, err := chain.DistributionCommunityPool(ctx)
		require.NoError(err)
		require.True(coins.AmountOf(chain.Config().Denom).GT(sdkmath.LegacyNewDec(int64(amount))))
	})

	t.Run("withdraw-address", func(t *testing.T) {
		// set custom withdraw address
		err = node.DistributionSetWithdrawAddr(ctx, users[0].KeyName(), newWithdrawAddr)
		require.NoError(err)

		withdrawAddr, err := chain.DistributionDelegatorWithdrawAddress(ctx, users[0].FormattedAddress())
		require.NoError(err)
		require.EqualValues(withdrawAddr, newWithdrawAddr)
	})

	t.Run("delegator", func(t *testing.T) {
		delRewards, err := chain.DistributionDelegationTotalRewards(ctx, delAddr)
		require.NoError(err)
		r := delRewards.Rewards[0]
		require.EqualValues(valAddr, r.ValidatorAddress)
		require.EqualValues(chain.Config().Denom, r.Reward[0].Denom)

		// DistributionDelegatorValidators
		delegatorVals, err := chain.DistributionDelegatorValidators(ctx, delAddr)
		require.NoError(err)
		require.EqualValues(valAddr, delegatorVals.Validators[0])

		rewards, err := chain.DistributionRewards(ctx, delAddr, valAddr)
		require.NoError(err)
		require.EqualValues(1, rewards.Len())
	})
}

func testFeeGrant(ctx context.Context, t *testing.T, chain *cosmos.CosmosChain, users []ibc.Wallet) {
	var err error
	node := chain.GetNode()

	denom := chain.Config().Denom

	// t.Run("successful grant and queries", func(t *testing.T) {
	// 	granter := users[0]
	// 	grantee := users[1]

	// 	err = node.FeeGrant(ctx, granter.KeyName(), grantee.FormattedAddress(), fmt.Sprintf("%d%s", 1000, chain.Config().Denom), []string{"/cosmos.bank.v1beta1.MsgSend"}, time.Now().Add(time.Hour*24*365))
	// 	require.NoError(t, err)

	// 	g, err := chain.FeeGrantGetAllowance(ctx, granter.FormattedAddress(), grantee.FormattedAddress())
	// 	require.NoError(t, err)
	// 	fmt.Printf("g: %+v\n", g)
	// 	require.EqualValues(t, granter.FormattedAddress(), g.Granter)
	// 	require.EqualValues(t, grantee.FormattedAddress(), g.Grantee)
	// 	require.EqualValues(t, "/cosmos.feegrant.v1beta1.BasicAllowance", g.Allowance.TypeUrl)
	// 	require.Contains(t, string(g.Allowance.Value), "/cosmos.bank.v1beta1.MsgSend")

	// 	all, err := chain.FeeGrantGetAllowances(ctx, grantee.FormattedAddress())
	// 	require.NoError(t, err)
	// 	require.Len(t, all, 1)
	// 	require.EqualValues(t, granter.FormattedAddress(), all[0].Granter)

	// 	all2, err := chain.FeeGrantGetAllowancesByGranter(ctx, granter.FormattedAddress())
	// 	require.NoError(t, err)
	// 	require.Len(t, all2, 1)
	// 	require.EqualValues(t, grantee.FormattedAddress(), all2[0].Grantee)
	// })

	t.Run("successful execution and a revoke", func(t *testing.T) {
		granter2 := users[2]
		grantee2 := users[3]

		err = node.FeeGrant(ctx, granter2.KeyName(), grantee2.FormattedAddress(), fmt.Sprintf("%d%s", 100_000, denom), nil, time.Unix(0, 0))
		require.NoError(t, err)

		bal, err := chain.BankGetBalance(ctx, granter2.FormattedAddress(), denom)
		require.NoError(t, err)

		fee := 500
		sendAmt := 501
		sendCoin := fmt.Sprintf("%d%s", sendAmt, denom)
		feeCoin := fmt.Sprintf("%d%s", fee, denom)

		// use the feegrant and validate the granter balance decreases by fee then add the sendAmt back
		_, err = node.ExecTx(ctx,
			grantee2.KeyName(), "bank", "send", grantee2.KeyName(), granter2.FormattedAddress(), sendCoin,
			"--fees", feeCoin, "--fee-granter", granter2.FormattedAddress(),
		)
		require.NoError(t, err)

		newBal, err := chain.BankGetBalance(ctx, granter2.FormattedAddress(), denom)
		require.NoError(t, err)
		require.EqualValues(t, bal.AddRaw(int64(sendAmt-fee)), newBal)

		// TODO: FeeGrantRevoke does not work
		//           	exit code 1:  [90m12:16AM[0m [1m[31mERR[0m[0m failure when running app [36merr=[0m"key with address cosmos1wycej8y4w7el8kghq69ah84kslgj9akjghpern not found: key not found"
		//  Test:       	TestCoreSDKCommands/feegrant/successful_execution_and_a_revoke
		//
		// revoke the grant
		// err = node.FeeGrantRevoke(ctx, granter2.KeyName(), granter2.FormattedAddress(), grantee2.FormattedAddress())
		// require.NoError(t, err)

		// // fail; try to execute the above logic again
		// _, err = node.ExecTx(ctx,
		// 	grantee2.KeyName(), "bank", "send", grantee2.KeyName(), granter2.FormattedAddress(), sendCoin,
		// 	"--fees", feeCoin, "--fee-granter", granter2.FormattedAddress(),
		// )
		// // TODO: actually want an error here
		// require.NoError(t, err)

		// postRevokeBal, err := chain.BankGetBalance(ctx, granter2.FormattedAddress(), denom)
		// require.NoError(t, err)
		// require.EqualValues(t, newBal, postRevokeBal)
	})
}

func testGov(ctx context.Context, t *testing.T, chain *cosmos.CosmosChain, users []ibc.Wallet) {}

// func testSlashing(ctx context.Context, t *testing.T, chain *cosmos.CosmosChain, users []ibc.Wallet) {}

// func testStaking(ctx context.Context, t *testing.T, chain *cosmos.CosmosChain, users []ibc.Wallet) {}

// func testUpgrade(ctx context.Context, t *testing.T, chain *cosmos.CosmosChain, users []ibc.Wallet) {}
