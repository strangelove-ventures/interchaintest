package features

import (
	"context"
	"fmt"
	"testing"

	tc "github.com/strangelove-ventures/interchaintest/v8/chain/thorchain"
	"github.com/strangelove-ventures/interchaintest/v8/chain/thorchain/common"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
)

func DualLp(
	t *testing.T,
	ctx context.Context,
	thorchain *tc.Thorchain,
	exoChain ibc.Chain,
) (thorUser ibc.Wallet, exoUser ibc.Wallet, err error) {
	fmt.Println("#### Dual Lper:", exoChain.Config().Name)
	users, err := GetAndFundTestUsers(t, ctx, fmt.Sprintf("%s-DualLper", exoChain.Config().Name), thorchain, exoChain)
	if err != nil {
		return thorUser, exoUser, err
	}
	thorUser, exoUser = users[0], users[1]

	exoChainType, err := common.NewChain(exoChain.Config().Name)
	if err != nil {
		return thorUser, exoUser, err
	}
	exoAsset := exoChainType.GetGasAsset()

	thorUserBalance, err := thorchain.GetBalance(ctx, thorUser.FormattedAddress(), thorchain.Config().Denom)
	if err != nil {
		return thorUser, exoUser, err
	}
	memo := fmt.Sprintf("+:%s:%s", exoAsset, exoUser.FormattedAddress())
	err = thorchain.Deposit(ctx, thorUser.KeyName(), thorUserBalance.QuoRaw(100).MulRaw(90), thorchain.Config().Denom, memo)
	if err != nil {
		return thorUser, exoUser, err
	}

	exoUserBalance, err := exoChain.GetBalance(ctx, exoUser.FormattedAddress(), exoChain.Config().Denom)
	if err != nil {
		return thorUser, exoUser, err
	}
	memo = fmt.Sprintf("+:%s:%s", exoAsset, thorUser.FormattedAddress())
	exoInboundAddr, _, err := thorchain.ApiGetInboundAddress(exoChainType.String())
	if err != nil {
		return thorUser, exoUser, err
	}
	_, err = exoChain.SendFundsWithNote(ctx, exoUser.KeyName(), ibc.WalletAmount{
		Address: exoInboundAddr,
		Denom:   exoChain.Config().Denom,
		Amount:  exoUserBalance.QuoRaw(100).MulRaw(90), // LP 90% of balance
	}, memo)
	if err != nil {
		return thorUser, exoUser, err
	}

	err = PollForPool(ctx, thorchain, 60, exoAsset)
	if err != nil {
		return thorUser, exoUser, err
	}

	return thorUser, exoUser, err
}
