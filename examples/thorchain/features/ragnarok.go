package features

import (
	"context"
	"fmt"
	"testing"

	sdkmath "cosmossdk.io/math"
	tc "github.com/strangelove-ventures/interchaintest/v9/chain/thorchain"
	"github.com/strangelove-ventures/interchaintest/v9/chain/thorchain/common"
	"github.com/strangelove-ventures/interchaintest/v9/ibc"
)

func Ragnarok(
	t *testing.T,
	ctx context.Context,
	thorchain *tc.Thorchain,
	exoChain ibc.Chain,
	exoUsers ...ibc.Wallet,
) error {
	fmt.Println("#### Ragnarok:", exoChain.Config().Name)
	if err := AddAdminIfNecessary(ctx, thorchain); err != nil {
		return err
	}

	exoChainType, err := common.NewChain(exoChain.Config().Name)
	if err != nil {
		return err
	}
	exoAsset := exoChainType.GetGasAsset()

	_, err = thorchain.ApiGetPool(exoAsset)
	if err != nil {
		return fmt.Errorf("pool (%s) not found to ragnarok, %w", exoChain.Config().Name, err)
	}

	exoUsersPreRagBalance := make([]sdkmath.Int, len(exoUsers))
	for i, exoUser := range exoUsers {
		exoUsersPreRagBalance[i], err = exoChain.GetBalance(ctx, exoUser.FormattedAddress(), exoChain.Config().Denom)
		if err != nil {
			return err
		}
	}

	if err := thorchain.SetMimir(ctx, "admin", fmt.Sprintf("RAGNAROK-%s", exoAsset.MimirString()), "1"); err != nil {
		return err
	}

	if err := PollForPoolSuspended(ctx, thorchain, 30, exoAsset); err != nil {
		return err
	}

	for i, exoUser := range exoUsers {
		if err := PollForBalanceChange(ctx, exoChain, 100, ibc.WalletAmount{
			Address: exoUser.FormattedAddress(),
			Denom:   exoChain.Config().Denom,
			Amount:  exoUsersPreRagBalance[i],
		}); err != nil {
			return err
		}
		exoUserPostRagBalance, err := exoChain.GetBalance(ctx, exoUser.FormattedAddress(), exoChain.Config().Denom)
		if err != nil {
			return err
		}
		if exoUserPostRagBalance.LTE(exoUsersPreRagBalance[i]) {
			return fmt.Errorf("user (%s) balance did not increase after %s ragnarok", exoUser.KeyName(), exoChainType)
		}
		fmt.Printf("\nUser (%s), pre: %s, post: %s\n", exoUser.KeyName(), exoUsersPreRagBalance[i], exoUserPostRagBalance)
	}

	return nil
}
