package features

import (
	"context"
	"fmt"
	"testing"

	tc "github.com/strangelove-ventures/interchaintest/v8/chain/thorchain"
	"github.com/strangelove-ventures/interchaintest/v8/chain/thorchain/common"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"github.com/stretchr/testify/require"
)

func DualLp(
	t *testing.T,
	ctx context.Context,
	thorchain *tc.Thorchain,
	exoChain ibc.Chain,
) (thorUser, exoUser ibc.Wallet) {
	fmt.Println("#### Dual Lper:", exoChain.Config().Name)
	users := GetAndFundTestUsers(t, ctx, fmt.Sprintf("%s-DualLper", exoChain.Config().Name), thorchain, exoChain)
	thorUser, exoUser = users[0], users[1]

	exoChainType, err := common.NewChain(exoChain.Config().Name)
	require.NoError(t, err)
	exoAsset := exoChainType.GetGasAsset()

	thorUserBalance, err := thorchain.GetBalance(ctx, thorUser.FormattedAddress(), thorchain.Config().Denom)
	require.NoError(t, err)
	memo := fmt.Sprintf("+:%s:%s", exoAsset, exoUser.FormattedAddress())
	err = thorchain.Deposit(ctx, thorUser.KeyName(), thorUserBalance.QuoRaw(100).MulRaw(90), thorchain.Config().Denom, memo)
	require.NoError(t, err)

	exoUserBalance, err := exoChain.GetBalance(ctx, exoUser.FormattedAddress(), exoChain.Config().Denom)
	require.NoError(t, err)
	memo = fmt.Sprintf("+:%s:%s", exoAsset, thorUser.FormattedAddress())
	exoInboundAddr, _, err := thorchain.ApiGetInboundAddress(exoChainType.String())
	require.NoError(t, err)
	_, err = exoChain.SendFundsWithNote(ctx, exoUser.KeyName(), ibc.WalletAmount{
		Address: exoInboundAddr,
		Denom: exoChain.Config().Denom,
		Amount: exoUserBalance.QuoRaw(100).MulRaw(90), // LP 90% of balance
	}, memo)
	require.NoError(t, err)

	err = PollForPool(ctx, thorchain, 30, exoAsset)
	require.NoError(t, err, fmt.Sprintf("%s pool did not get created in 30 blocks", exoAsset))

	return thorUser, exoUser
}