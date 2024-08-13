package features

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"testing"

	sdkmath "cosmossdk.io/math"
	tc "github.com/strangelove-ventures/interchaintest/v8/chain/thorchain"
	"github.com/strangelove-ventures/interchaintest/v8/chain/thorchain/common"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
)

func Saver(
	t *testing.T,
	ctx context.Context,
	thorchain *tc.Thorchain,
	exoChain ibc.Chain,
) (exoUser ibc.Wallet, err error) {
	fmt.Println("#### Savers:", exoChain.Config().Name)
	users, err := GetAndFundTestUsers(t, ctx, fmt.Sprintf("%s-Saver", exoChain.Config().Name), exoChain)
	if err != nil {
		return exoUser, err
	}
	exoUser = users[0]

	exoChainType, err := common.NewChain(exoChain.Config().Name)
	if err != nil {
		return exoUser, err
	}
	exoAsset := exoChainType.GetGasAsset()

	pool, err := thorchain.ApiGetPool(exoAsset)
	if err != nil {
		return exoUser, err
	}
	saveAmount := sdkmath.NewUintFromString(pool.BalanceAsset).
		MulUint64(500).QuoUint64(10_000)

	saverQuote, err := thorchain.ApiGetSaverDepositQuote(exoAsset, saveAmount)
	if err != nil {
		return exoUser, err
	}

	// store expected range to fail if received amount is outside 5% tolerance
	quoteOut := sdkmath.NewUintFromString(saverQuote.ExpectedAmountDeposit)
	tolerance := quoteOut.QuoUint64(20)
	if saverQuote.Fees.Outbound != nil {
		outboundFee := sdkmath.NewUintFromString(*saverQuote.Fees.Outbound)
		quoteOut = quoteOut.Add(outboundFee)
	}
	minExpectedSaver := quoteOut.Sub(tolerance)
	maxExpectedSaver := quoteOut.Add(tolerance)

	// send random half as memoless saver
	memo := ""
	if rand.Intn(2) == 0 || exoChainType.String() == common.GAIAChain.String() { // if gaia memo is empty, bifrost errors, maybe benign
		memo = fmt.Sprintf("+:%s", exoAsset.GetSyntheticAsset())
	}

	exoInboundAddr, _, err := thorchain.ApiGetInboundAddress(exoChainType.String())
	if err != nil {
		return exoUser, err
	}

	wallet := ibc.WalletAmount{
		Address: exoInboundAddr,
		Denom:   exoChain.Config().Denom,
		Amount: sdkmath.Int(saveAmount).
			MulRaw(int64(math.Pow10(int(*exoChain.Config().CoinDecimals)))).
			QuoRaw(int64(math.Pow10(int(*thorchain.Config().CoinDecimals)))), // save amount is based on 8 dec
	}
	if memo != "" {
		_, err = exoChain.SendFundsWithNote(ctx, exoUser.KeyName(), wallet, memo)
		if err != nil {
			return exoUser, err
		}
	} else {
		if err := exoChain.SendFunds(ctx, exoUser.KeyName(), wallet); err != nil {
			return exoUser, err
		}
	}

	errMsgCommon := fmt.Sprintf("saver (%s - %s) of asset %s", exoUser.KeyName(), exoUser.FormattedAddress(), exoAsset)
	saver, err := PollForSaver(ctx, thorchain, 30, exoAsset, exoUser)
	if err != nil {
		return exoUser, fmt.Errorf("%s not found, %w", errMsgCommon, err)
	}

	deposit := sdkmath.NewUintFromString(saver.AssetDepositValue)
	if deposit.LT(minExpectedSaver) {
		return exoUser, fmt.Errorf("%s deposit: %s, min expected: %s", errMsgCommon, deposit, minExpectedSaver)
	}
	if deposit.GT(maxExpectedSaver) {
		return exoUser, fmt.Errorf("%s deposit: %s, max expected: %s", errMsgCommon, deposit, maxExpectedSaver)
	}

	return exoUser, nil
}
