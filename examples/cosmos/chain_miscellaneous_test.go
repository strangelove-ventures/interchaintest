package cosmos_test

import (
	"context"
	"fmt"
	"testing"

	"cosmossdk.io/math"

	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/strangelove-ventures/interchaintest/v8"
	"github.com/strangelove-ventures/interchaintest/v8/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestICTestMiscellaneous(t *testing.T) {
	CosmosChainTestMiscellaneous(t, "juno", "v16.0.0")
}

func CosmosChainTestMiscellaneous(t *testing.T, name, version string) {
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
			Name:      name,
			ChainName: name,
			Version:   version,
			ChainConfig: ibc.ChainConfig{
				Denom:         "ujuno",
				Bech32Prefix:  "juno",
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

	users := interchaintest.GetAndFundTestUsers(t, ctx, "default", int64(10_000_000_000), chain, chain)

	testBuildDependencies(ctx, t, chain)
	testWalletKeys(ctx, t, chain)
	testSendingTokens(ctx, t, chain, users)
	testFindTxs(ctx, t, chain, users)
	testPollForBalance(ctx, t, chain, users)
	testRangeBlockMessages(ctx, t, chain, users)
	testBroadcaster(ctx, t, chain, users)
	testQueryCmd(ctx, t, chain)
	testHasCommand(ctx, t, chain)
	testTokenFactory(ctx, t, chain, users)
	testAddingNode(ctx, t, chain)
}

func testBuildDependencies(ctx context.Context, t *testing.T, chain *cosmos.CosmosChain) {
	deps := chain.Validators[0].GetBuildInformation(ctx)

	sdkVer := "v0.47.3"

	require.Equal(t, deps.Name, "juno")
	require.Equal(t, deps.ServerName, "junod")
	require.Equal(t, deps.Version, "v16.0.0")
	require.Equal(t, deps.CosmosSdkVersion, sdkVer)
	require.Equal(t, deps.Commit, "054796f6173a9f15d012b656e255f94a4ec1d2cd")
	require.Equal(t, deps.BuildTags, "netgo muslc,")

	for _, dep := range deps.BuildDeps {
		dep := dep

		// Verify specific examples
		if dep.Parent == "github.com/cosmos/cosmos-sdk" {
			require.Equal(t, dep.Version, sdkVer)
			require.Equal(t, dep.IsReplacement, false)
		} else if dep.Parent == "github.com/99designs/keyring" {
			require.Equal(t, dep.Version, "v1.2.2")
			require.Equal(t, dep.IsReplacement, true)
			require.Equal(t, dep.Replacement, "github.com/cosmos/keyring")
			require.Equal(t, dep.ReplacementVersion, "v1.2.0")

		}

		// Verify all replacement logic
		if dep.IsReplacement {
			require.GreaterOrEqual(t, len(dep.ReplacementVersion), 6, "ReplacementVersion: %s must be >=6 length (ex: vA.B.C)", dep.ReplacementVersion)
			require.Greater(t, len(dep.Replacement), 0, "Replacement: %s must be >0 length.", dep.Replacement)
		} else {
			require.Equal(t, len(dep.Replacement), 0, "Replacement: %s is not 0.", dep.Replacement)
			require.Equal(t, len(dep.ReplacementVersion), 0, "ReplacementVersion: %s is not 0.", dep.ReplacementVersion)
		}
	}
}

func testWalletKeys(ctx context.Context, t *testing.T, chain *cosmos.CosmosChain) {
	// create a general key
	randKey := "randkey123"
	err := chain.CreateKey(ctx, randKey)
	require.NoError(t, err)

	// verify key was created properly
	_, err = chain.GetAddress(ctx, randKey)
	require.NoError(t, err)

	// recover a key
	// juno1hj5fveer5cjtn4wd6wstzugjfdxzl0xps73ftl
	keyName := "key-abc"
	testMnemonic := "decorate bright ozone fork gallery riot bus exhaust worth way bone indoor calm squirrel merry zero scheme cotton until shop any excess stage laundry"
	wallet, err := chain.BuildWallet(ctx, keyName, testMnemonic)
	require.NoError(t, err)

	// verify
	addr, err := chain.GetAddress(ctx, keyName)
	require.NoError(t, err)
	require.Equal(t, wallet.Address(), addr)

	tn := chain.Validators[0]
	a, err := tn.KeyBech32(ctx, "key-abc", "val")
	require.NoError(t, err)
	require.Equal(t, a, "junovaloper1hj5fveer5cjtn4wd6wstzugjfdxzl0xp0r8xsx")

	a, err = tn.KeyBech32(ctx, "key-abc", "acc")
	require.NoError(t, err)
	require.Equal(t, a, wallet.FormattedAddress())

	a, err = tn.AccountKeyBech32(ctx, "key-abc")
	require.NoError(t, err)
	require.Equal(t, a, wallet.FormattedAddress())
}

func testSendingTokens(ctx context.Context, t *testing.T, chain *cosmos.CosmosChain, users []ibc.Wallet) {
	_, err := chain.GetBalance(ctx, users[0].FormattedAddress(), chain.Config().Denom)
	require.NoError(t, err)
	b2, err := chain.GetBalance(ctx, users[1].FormattedAddress(), chain.Config().Denom)
	require.NoError(t, err)

	sendAmt := int64(1)
	_, err = sendTokens(ctx, chain, users[0], users[1], "", sendAmt)
	require.NoError(t, err)

	b2New, err := chain.GetBalance(ctx, users[1].FormattedAddress(), chain.Config().Denom)
	require.NoError(t, err)

	require.Equal(t, b2.Add(math.NewInt(sendAmt)), b2New)
}

func testFindTxs(ctx context.Context, t *testing.T, chain *cosmos.CosmosChain, users []ibc.Wallet) {
	height, _ := chain.Height(ctx)

	_, err := sendTokens(ctx, chain, users[0], users[1], "", 1)
	require.NoError(t, err)

	txs, err := chain.FindTxs(ctx, height+1)
	require.NoError(t, err)
	require.NotEmpty(t, txs)
	require.Equal(t, txs[0].Events[0].Type, "coin_spent")
}

func testPollForBalance(ctx context.Context, t *testing.T, chain *cosmos.CosmosChain, users []ibc.Wallet) {
	bal2, err := chain.GetBalance(ctx, users[1].FormattedAddress(), chain.Config().Denom)
	require.NoError(t, err)

	amt := ibc.WalletAmount{
		Address: users[1].FormattedAddress(),
		Denom:   chain.Config().Denom,
		Amount:  math.NewInt(1),
	}

	delta := uint64(3)

	ch := make(chan error)
	go func() {
		new := amt
		new.Amount = bal2.Add(math.NewInt(1))
		ch <- cosmos.PollForBalance(ctx, chain, delta, new)
	}()

	err = chain.SendFunds(ctx, users[0].KeyName(), amt)
	require.NoError(t, err)
	require.NoError(t, <-ch)
}

func testRangeBlockMessages(ctx context.Context, t *testing.T, chain *cosmos.CosmosChain, users []ibc.Wallet) {
	height, _ := chain.Height(ctx)

	_, err := sendTokens(ctx, chain, users[0], users[1], "", 1)
	require.NoError(t, err)

	var bankMsgs []*banktypes.MsgSend
	err = cosmos.RangeBlockMessages(ctx, chain.Config().EncodingConfig.InterfaceRegistry, chain.Validators[0].Client, height+1, func(msg sdk.Msg) bool {
		found, ok := msg.(*banktypes.MsgSend)
		if ok {
			bankMsgs = append(bankMsgs, found)
		}
		return false
	})
	require.NoError(t, err)
}

func testAddingNode(ctx context.Context, t *testing.T, chain *cosmos.CosmosChain) {
	// This should be tested last or else Txs will fail on the new full node.
	nodesAmt := len(chain.Nodes())
	chain.AddFullNodes(ctx, nil, 1)
	require.Equal(t, nodesAmt+1, len(chain.Nodes()))
}

func testBroadcaster(ctx context.Context, t *testing.T, chain *cosmos.CosmosChain, users []ibc.Wallet) {
	from := users[0].FormattedAddress()
	addr1 := "juno190g5j8aszqhvtg7cprmev8xcxs6csra7xnk3n3"
	addr2 := "juno1a53udazy8ayufvy0s434pfwjcedzqv34q7p7vj"

	c1 := sdk.NewCoins(sdk.NewCoin(chain.Config().Denom, math.NewInt(1)))
	c2 := sdk.NewCoins(sdk.NewCoin(chain.Config().Denom, math.NewInt(2)))

	b := cosmos.NewBroadcaster(t, chain)

	in := banktypes.Input{
		Address: from,
		Coins:   c1.Add(c2[0]),
	}
	out := []banktypes.Output{
		{
			Address: addr1,
			Coins:   c1,
		},
		{
			Address: addr2,
			Coins:   c2,
		},
	}

	txResp, err := cosmos.BroadcastTx(
		ctx,
		b,
		users[0],
		banktypes.NewMsgMultiSend(in, out),
	)
	require.NoError(t, err)
	require.NotEmpty(t, txResp.TxHash)
	fmt.Printf("txResp: %+v\n", txResp)

	updatedBal1, err := chain.GetBalance(ctx, addr1, chain.Config().Denom)
	require.NoError(t, err)
	require.Equal(t, math.NewInt(1), updatedBal1)

	updatedBal2, err := chain.GetBalance(ctx, addr2, chain.Config().Denom)
	require.NoError(t, err)
	require.Equal(t, math.NewInt(2), updatedBal2)
}

func testQueryCmd(ctx context.Context, t *testing.T, chain *cosmos.CosmosChain) {
	tn := chain.Validators[0]
	stdout, stderr, err := tn.ExecQuery(ctx, "slashing", "params")
	require.NoError(t, err)
	require.NotEmpty(t, stdout)
	require.Empty(t, stderr)
}

func testHasCommand(ctx context.Context, t *testing.T, chain *cosmos.CosmosChain) {
	tn := chain.Validators[0]
	res := tn.HasCommand(ctx, "query")
	require.True(t, res)

	if tn.IsAboveSDK47(ctx) {
		require.True(t, tn.HasCommand(ctx, "genesis"))
	} else {
		// 45 does not have this
		require.False(t, tn.HasCommand(ctx, "genesis"))
	}

	require.True(t, tn.HasCommand(ctx, "tx", "ibc"))
	require.True(t, tn.HasCommand(ctx, "q", "ibc"))
	require.True(t, tn.HasCommand(ctx, "keys"))
	require.True(t, tn.HasCommand(ctx, "help"))
	require.True(t, tn.HasCommand(ctx, "tx", "bank", "send"))

	require.False(t, tn.HasCommand(ctx, "tx", "bank", "send2notrealcmd"))
	require.False(t, tn.HasCommand(ctx, "incorrectcmd"))
}

func testTokenFactory(ctx context.Context, t *testing.T, chain *cosmos.CosmosChain, users []ibc.Wallet) {
	user := users[0]
	user2 := users[1]

	subDenom := "ictest"
	tfDenom, _, err := cosmos.TokenFactoryCreateDenom(chain, ctx, user, subDenom, 2500000)
	require.NoError(t, err)
	require.Equal(t, tfDenom, "factory/"+user.FormattedAddress()+"/"+subDenom)

	// modify metadata
	stdout, err := cosmos.TokenFactoryMetadata(chain, ctx, user.KeyName(), tfDenom, "SYMBOL", "description here", 6)
	t.Log(stdout, err)
	require.NoError(t, err)

	// verify metadata
	md, err := chain.QueryBankMetadata(ctx, tfDenom)
	require.NoError(t, err)
	require.Equal(t, md.Metadata.Description, "description here")
	require.Equal(t, md.Metadata.Symbol, "SYMBOL")
	require.Equal(t, md.Metadata.DenomUnits[1].Exponent, 6)

	// mint tokens
	_, err = cosmos.TokenFactoryMintDenom(chain, ctx, user.KeyName(), tfDenom, 1)
	require.NoError(t, err)
	validateBalance(ctx, t, chain, user, tfDenom, 1)

	// mint-to
	_, err = cosmos.TokenFactoryMintDenomTo(chain, ctx, user.KeyName(), tfDenom, 3, user2.FormattedAddress())
	require.NoError(t, err)
	validateBalance(ctx, t, chain, user2, tfDenom, 3)

	// force transfer 1 from user2 (3) to user1 (1)
	_, err = cosmos.TokenFactoryForceTransferDenom(chain, ctx, user.KeyName(), tfDenom, 1, user2.FormattedAddress(), user.FormattedAddress())
	require.NoError(t, err)
	validateBalance(ctx, t, chain, user, tfDenom, 2)
	validateBalance(ctx, t, chain, user2, tfDenom, 2)

	// burn token from account
	_, err = cosmos.TokenFactoryBurnDenomFrom(chain, ctx, user.KeyName(), tfDenom, 1, user.FormattedAddress())
	require.NoError(t, err)
	validateBalance(ctx, t, chain, user, tfDenom, 1)

	prevAdmin, err := cosmos.TokenFactoryGetAdmin(chain, ctx, tfDenom)
	require.NoError(t, err)
	require.Equal(t, prevAdmin.AuthorityMetadata.Admin, user.FormattedAddress())

	// change admin, then we will burn the token-from
	_, err = cosmos.TokenFactoryChangeAdmin(chain, ctx, user.KeyName(), tfDenom, user2.FormattedAddress())
	require.NoError(t, err)

	// validate new admin is set
	tfAdmin, err := cosmos.TokenFactoryGetAdmin(chain, ctx, tfDenom)
	require.NoError(t, err)
	require.Equal(t, tfAdmin.AuthorityMetadata.Admin, user2.FormattedAddress())

	// burn
	_, err = cosmos.TokenFactoryBurnDenomFrom(chain, ctx, user2.KeyName(), tfDenom, 1, user.FormattedAddress())
	require.NoError(t, err)
	validateBalance(ctx, t, chain, user, tfDenom, 0)

}

// helpers
func sendTokens(ctx context.Context, chain *cosmos.CosmosChain, from, to ibc.Wallet, token string, amount int64) (ibc.WalletAmount, error) {
	if token == "" {
		token = chain.Config().Denom
	}

	sendAmt := ibc.WalletAmount{
		Address: to.FormattedAddress(),
		Denom:   token,
		Amount:  math.NewInt(amount),
	}
	err := chain.SendFunds(ctx, from.KeyName(), sendAmt)
	return sendAmt, err
}

func validateBalance(ctx context.Context, t *testing.T, chain *cosmos.CosmosChain, user ibc.Wallet, tfDenom string, expected int64) {
	balance, err := chain.GetBalance(ctx, user.FormattedAddress(), tfDenom)
	require.NoError(t, err)
	require.Equal(t, balance, math.NewInt(expected))
}
