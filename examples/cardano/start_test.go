package cardano

import (
	"context"
	"fmt"
	"strconv"
	"testing"

	sdkmath "cosmossdk.io/math"
	"github.com/strangelove-ventures/interchaintest/v8"
	ada "github.com/strangelove-ventures/interchaintest/v8/chain/cardano"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestCardano(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	t.Parallel()

	client, network := interchaintest.DockerSetup(t)
	ctx := context.Background()

	cf := interchaintest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*interchaintest.ChainSpec{
		{ChainConfig: ada.DefaultConfig("cardano")},
	})

	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)
	adaChain := chains[0].(*ada.AdaChain)
	err = adaChain.SetFaucet(ctx)
	require.NoError(t, err)

	ic := interchaintest.NewInterchain().AddChain(adaChain)

	require.NoError(t, ic.Build(ctx, nil, interchaintest.InterchainBuildOptions{
		TestName:         t.Name(),
		Client:           client,
		NetworkID:        network,
		SkipPathCreation: true, // Skip path creation, so we can have granular control over the process
	}))
	t.Cleanup(func() {
		_ = ic.Close()
		adaChain.Stop()
	})

	// Create and fund a user using GetAndFundTestUsers
	// Fund 2 coins to user1 and user2
	fundAmount := sdkmath.NewInt(200_000_000)
	users := interchaintest.GetAndFundTestUsers(t, ctx, "user1", fundAmount, adaChain)
	user1 := users[0]
	users = interchaintest.GetAndFundTestUsers(t, ctx, "user2", fundAmount, adaChain)
	user2 := users[0]

	// Verify user1 balance
	balanceUser1, err := adaChain.GetBalance(ctx, user1.FormattedAddress(), "")
	require.NoError(t, err)
	require.Equal(t, fundAmount, balanceUser1, fmt.Errorf("user (%s) balance (%s) is not expected (%s)", user1.KeyName(), balanceUser1, fundAmount))

	// Verify user2 balance
	balanceUser2, err := adaChain.GetBalance(ctx, user2.FormattedAddress(), "")
	require.NoError(t, err)
	require.Equal(t, fundAmount, balanceUser2, fmt.Errorf("user (%s) balance (%s) is not expected (%s)", user2.KeyName(), balanceUser2, fundAmount))

	// Send 1 coin from user1 to user2 with a note/memo
	memo := fmt.Sprintf("+:%s:%s", "abc.abc", "rHb9CJAWyB4rj91VRWn96DkukG4bwdtyTh")
	transferAmount := sdkmath.NewInt(100_000_000)
	_, err = adaChain.SendFundsWithNote(ctx, user1.KeyName(), ibc.WalletAmount{
		Address: user2.FormattedAddress(),
		Amount:  transferAmount,
	}, memo)
	require.NoError(t, err)

	// Verify user1 balance
	balanceUser1, err = adaChain.GetBalance(ctx, user1.FormattedAddress(), "")
	require.NoError(t, err)
	fees, err := strconv.ParseFloat(adaChain.Config().GasPrices, 64)
	require.NoError(t, err)
	expectedBalance := fundAmount.Sub(transferAmount).SubRaw(int64(fees))
	require.InDelta(
		t, expectedBalance.Int64(), balanceUser1.Int64(), 50_000,
		fmt.Errorf("user (%s) balance (%s) is not expected (%s) (check1)", user1.KeyName(), balanceUser1, expectedBalance))

	// Verify user2 balance
	balanceUser2, err = adaChain.GetBalance(ctx, user2.FormattedAddress(), "")
	require.NoError(t, err)
	expectedBalance = fundAmount.Add(transferAmount)
	require.Equal(
		t, expectedBalance, balanceUser2,
		fmt.Errorf("user (%s) balance (%s) is not expected (%s) (check2)", user2.KeyName(), balanceUser2, expectedBalance))
}
