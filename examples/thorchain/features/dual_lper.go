package features

import (
	"context"
	"fmt"
	"testing"

	tc "github.com/strangelove-ventures/interchaintest/v8/chain/thorchain"
	"github.com/strangelove-ventures/interchaintest/v8/chain/thorchain/common"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	//"github.com/strangelove-ventures/interchaintest/v8/testutil"
	"github.com/stretchr/testify/require"
)

func DualLp(
	t *testing.T,
	ctx context.Context,
	thorchain *tc.Thorchain,
	otherChain ibc.Chain,
) (thorUser, otherUser ibc.Wallet) {
	users := GetAndFundTestUsers(t, ctx, fmt.Sprintf("%s-DualLper", otherChain.Config().Name), thorchain, otherChain)
	thorUser, otherUser = users[0], users[1]

	otherChainType, err := common.NewChain(otherChain.Config().Name)
	require.NoError(t, err)

	otherUserBalance, err := otherChain.GetBalance(ctx, otherUser.FormattedAddress(), otherChain.Config().Denom)
	require.NoError(t, err)
	otherAsset := otherChainType.GetGasAsset()
	memo := fmt.Sprintf("+:%s:%s", otherAsset, thorUser.FormattedAddress())
	otherInboundAddr, _, err := thorchain.ApiGetInboundAddress(otherChainType.String())
	require.NoError(t, err)
	otherChain.SendFundsWithNote(ctx, otherUser.KeyName(), ibc.WalletAmount{
		Address: otherInboundAddr,
		Denom: otherChain.Config().Denom,
		Amount: otherUserBalance.QuoRaw(100).MulRaw(90), // LP 90% of balance
	}, memo)

	thorUserBalance, err := thorchain.GetBalance(ctx, thorUser.FormattedAddress(), thorchain.Config().Denom)
	memo = fmt.Sprintf("+:%s:%s", otherAsset, otherUser.FormattedAddress())
	err = thorchain.Deposit(ctx, thorUser.KeyName(), thorUserBalance.QuoRaw(100).MulRaw(90), thorchain.Config().Denom, memo)

	/*_, err = thorchain.ApiGetPool(otherAsset)
	count := 0
	for err != nil {
		require.Less(t, count, 30, fmt.Sprintf("%s pool did not get created in 30 blocks"))
		count++
		err = testutil.WaitForBlocks(ctx, 1, thorchain)
		require.NoError(t, err)
		_, err = thorchain.ApiGetPool(otherAsset)
	}*/
	err = PollForPool(ctx, thorchain, 30, otherAsset)
	require.NoError(t, err, fmt.Sprintf("%s pool did not get created in 30 blocks", otherAsset))

	return thorUser, otherUser
}