package xrp_test

import (
	"context"
	"encoding/hex"
	"fmt"
	"math"
	"strconv"
	"testing"
	//"time"

	sdkmath "cosmossdk.io/math"
	"github.com/strangelove-ventures/interchaintest/v8"
	"github.com/strangelove-ventures/interchaintest/v8/chain/xrp"
	xrpclient "github.com/strangelove-ventures/interchaintest/v8/chain/xrp/client"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestXrp(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	t.Parallel()

	client, network := interchaintest.DockerSetup(t)
	ctx := context.Background()

	cf := interchaintest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*interchaintest.ChainSpec{
		{ChainConfig: xrp.DefaultXrpChainConfig("xrptest")},
	})

	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)
	xrpChain := chains[0].(*xrp.XrpChain)

	ic := interchaintest.NewInterchain().
		AddChain(xrpChain)

	require.NoError(t, ic.Build(ctx, nil, interchaintest.InterchainBuildOptions{
		TestName:         t.Name(),
		Client:           client,
		NetworkID:        network,
		SkipPathCreation: true, // Skip path creation, so we can have granular control over the process
	}))
	t.Cleanup(func() {
		_ = ic.Close()
		xrpChain.Stop()
	})

	fmt.Println("chain up")
	// Create and fund a user using GetAndFundTestUsers
	// Fund 2 coins to user1 and user2
	fundAmount := sdkmath.NewInt(200_000_000)
	users := interchaintest.GetAndFundTestUsers(t, ctx, "user1", fundAmount, xrpChain)
	user1 := users[0]
	users = interchaintest.GetAndFundTestUsers(t, ctx, "user2", fundAmount, xrpChain)
	user2 := users[0]

	// Verify user1 balance
	balanceUser1, err := xrpChain.GetBalance(ctx, user1.FormattedAddress(), "")
	require.NoError(t, err)
	require.Equal(t, fundAmount, balanceUser1, fmt.Errorf("User (%s) balance (%s) is not expected (%s)", user1.KeyName(), balanceUser1, fundAmount))

	// Verify user2 balance
	balanceUser2, err := xrpChain.GetBalance(ctx, user2.FormattedAddress(), "")
	require.NoError(t, err)
	require.Equal(t, fundAmount, balanceUser2, fmt.Errorf("User (%s) balance (%s) is not expected (%s)", user2.KeyName(), balanceUser2, fundAmount))

	// Send 1 coin from user1 to user2 with a note/memo
	memo := fmt.Sprintf("+:%s:%s", "abc.abc", "rHb9CJAWyB4rj91VRWn96DkukG4bwdtyTh")
	transferAmount := sdkmath.NewInt(100_000_000)
	txHash, err := xrpChain.SendFundsWithNote(ctx, user1.KeyName(), ibc.WalletAmount{
		Address: user2.FormattedAddress(),
		Amount:  transferAmount,
	}, memo)
	require.NoError(t, err)

	// Verify user1 balance
	balanceUser1, err = xrpChain.GetBalance(ctx, user1.FormattedAddress(), "")
	require.NoError(t, err)
	fees, err := strconv.ParseFloat(xrpChain.Config().GasPrices, 64)
	require.NoError(t, err)
	feeScaled := fees * math.Pow10(int(*xrpChain.Config().CoinDecimals))
	expectedBalance := fundAmount.Sub(transferAmount).SubRaw(int64(feeScaled))
	require.Equal(t, expectedBalance, balanceUser1, fmt.Errorf("User (%s) balance (%s) is not expected (%s) (check2)", user1.KeyName(), balanceUser1, expectedBalance))

	// Verify user2 balance
	balanceUser2, err = xrpChain.GetBalance(ctx, user2.FormattedAddress(), "")
	require.NoError(t, err)
	expectedBalance = fundAmount.Add(transferAmount)
	require.Equal(t, expectedBalance, balanceUser2, fmt.Errorf("User (%s) balance (%s) is not expected (%s) (check2)", user2.KeyName(), balanceUser2, expectedBalance))

	xrpClient := xrpclient.NewXrpClient(xrpChain.GetHostRPCAddress())
	txResp, err := xrpClient.GetTx(txHash)
	require.NoError(t, err)
	memoData, err := hex.DecodeString(txResp.Memos[0].Memo.MemoData)
	require.NoError(t, err)
	require.Equal(t, memo, string(memoData))
	fmt.Println("Memo:", string(memoData))

	// fmt.Println("Staying up 2 min")
	// time.Sleep(time.Minute * 2)
}
