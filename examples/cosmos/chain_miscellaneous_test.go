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
	// TODO: Convert to sim v0.50 RC 0
	CosmosChainTestMiscellaneous(t, "juno", "v16.0.0", true)
}

func CosmosChainTestMiscellaneous(t *testing.T, name, version string, useNewGenesisCmd bool) {
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

	// TODO: Get this from PR 760
	// BuildDependencies(ctx, t, chain)

	// TODO: Move ExportState here? (we need to run both for SDK 45 and 47+
	testWalletKeys(ctx, t, chain)
	testSendingTokens(ctx, t, chain, users)
	testFindTxs(ctx, t, chain, users)
	testPollForBalance(ctx, t, chain, users)
	testRangeBlockMessages(ctx, t, chain, users)
	testBroadcaster(ctx, t, chain, users)

	testQueryCmd(ctx, t, chain)
	testHasCommand(ctx, t, chain)

	// testProposal(ctx, t, chain, user) // broken param unmarshaling, requires v50 sim.
	// testCosmWasm(ctx, t, chain, user.KeyName(), "contracts/cw_template.wasm", `{"count":0}`) // requires wasmd v50

	testAddingNode(ctx, t, chain)
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

/*
func testProposal(ctx context.Context, t *testing.T, chain *cosmos.CosmosChain, user ibc.Wallet) {
	govAcc := "cosmos10d07y265gmmuvt4z0w9aw880jnsr700j6zn9kn"

	dp := govtypes.DefaultParams()
	dp.MinDeposit = sdk.NewCoins(sdk.NewCoin(chain.Config().Denom, sdkmath.NewInt(7)))

	// make sure to register the interface for this module's types.
	updateParams := []cosmosproto.Message{
		&govtypes.MsgUpdateParams{
			Authority: govAcc,
			Params:    dp,
		},
	}

	proposal, err := chain.BuildProposal(updateParams, "title", "summary", "ipfs://CID", fmt.Sprintf(`500000000%s`, chain.Config().Denom))
	require.NoError(t, err, "error building proposal")

	txProp, err := chain.SubmitProposal(ctx, user.KeyName(), proposal)
	require.NoError(t, err, "error submitting proposal")

	height, err := chain.Height(ctx)
	require.NoError(t, err, "error getting height")

	require.Equal(t, height, txProp.Height)
	require.Equal(t, "1", txProp.ProposalID)

	err = chain.VoteOnProposalAllValidators(ctx, txProp.ProposalID, cosmos.ProposalVoteYes)
	require.NoError(t, err, "failed to submit votes")

	_, err = cosmos.PollForProposalStatus(ctx, chain, height, height+haltHeightDelta, txProp.ProposalID, cosmos.ProposalStatusPassed)
	require.NoError(t, err, "proposal status did not change to passed in expected number of blocks")

	// verify the params updated
}

func testCosmWasm(ctx context.Context, t *testing.T, chain *cosmos.CosmosChain, keyname string, fileLoc string, message string) {
	codeId, err := chain.StoreContract(ctx, keyname, fileLoc)
	require.NoError(t, err)

	contractAddr, err := chain.InstantiateContract(ctx, keyname, codeId, message, true)
	require.NoError(t, err)
	require.NotEmpty(t, contractAddr)

	// helpers.ExecuteMsgWithFee(t, ctx, juno, user, contractAddr, "", "10000"+nativeDenom, `{"increment":{}}`)
	txHash, err := chain.ExecuteContract(ctx, keyname, contractAddr, `{"increment":{}}`)
	require.NoError(t, err)
	require.NotEmpty(t, txHash)

	// QueryContract
	type QueryMsg struct {
		GetConfig *struct{} `json:"get_config,omitempty"`
	}

	type IncrementResponse struct {
		Val uint32 `json:"val"`
	}

	var res IncrementResponse
	err = chain.QueryContract(ctx, contractAddr, QueryMsg{GetConfig: &struct{}{}}, &res)
	require.NoError(t, err)
	require.Equal(t, uint32(1), res.Val)

	// DumpContractState
	height, _ := chain.Height(ctx)
	resp, err := chain.DumpContractState(ctx, contractAddr, int64(height))
	require.NoError(t, err)
	require.NotEmpty(t, resp)

	fmt.Printf("resp.Modules: %+v\n", resp.Models)
}
*/

func testSidecar(ctx context.Context, t *testing.T, chain *cosmos.CosmosChain) {
	// NewSidecarProcess
	// StopAllSidecars
	// StartAllSidecars
	// StartAllValSidecars
	// and everythign in sidecar.go
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
