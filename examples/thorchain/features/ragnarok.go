package features

import (
	"context"
	"fmt"
	"testing"

	sdkmath "cosmossdk.io/math"
	tc "github.com/strangelove-ventures/interchaintest/v8/chain/thorchain"
	"github.com/strangelove-ventures/interchaintest/v8/chain/thorchain/common"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"github.com/stretchr/testify/require"
)

func Ragnarok(
	t *testing.T,
	ctx context.Context,
	thorchain *tc.Thorchain,
	exoChain ibc.Chain,
	exoUsers ...ibc.Wallet,
) (error) {
	err := AddAdminIfNecessary(ctx, thorchain)
	require.NoError(t, err)

	exoChainType, err := common.NewChain(exoChain.Config().Name)
	require.NoError(t, err)
	exoAsset := exoChainType.GetGasAsset()

	_, err = thorchain.ApiGetPool(exoAsset)
	require.NoError(t, err, fmt.Sprintf("pool (%s) not found to ragnarok", exoChain.Config().Name))

	exoUsersPreRagBalance := make([]sdkmath.Int, len(exoUsers))
	for i, exoUser := range exoUsers {
		exoUsersPreRagBalance[i], err = exoChain.GetBalance(ctx, exoUser.FormattedAddress(), exoChain.Config().Denom)
		require.NoError(t, err)
	}

	err = thorchain.SetMimir(ctx, "admin", fmt.Sprintf("RAGNAROK-%s", exoAsset.MimirString()), "1")
	require.NoError(t, err)

	err = PollForPoolSuspended(ctx, thorchain, 30, exoAsset)
	require.NoError(t, err)

	for i, exoUser := range exoUsers {
		err = PollForBalanceChange(ctx, exoChain, 100, ibc.WalletAmount{
			Address: exoUser.FormattedAddress(),
			Denom: exoChain.Config().Denom,
			Amount: exoUsersPreRagBalance[i],
		})
		require.NoError(t, err)
		exoUserPostRagBalance, err := exoChain.GetBalance(ctx, exoUser.FormattedAddress(), exoChain.Config().Denom)
		require.NoError(t, err)
		require.True(t, exoUserPostRagBalance.GT(exoUsersPreRagBalance[i]), fmt.Sprintf("User (%s) balance did not increase after %s ragnarok", exoUser.KeyName(), exoChainType))
		fmt.Printf("\nUser (%s), pre: %s, post: %s\n", exoUser.KeyName(), exoUsersPreRagBalance[i], exoUserPostRagBalance)
	}

	return nil
}