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
	"github.com/stretchr/testify/require"
)

func Saver(
	t *testing.T,
	ctx context.Context,
	thorchain *tc.Thorchain,
	exoChain ibc.Chain,
) (exoUser ibc.Wallet) {
	fmt.Println("#### Savers:", exoChain.Config().Name)
	users := GetAndFundTestUsers(t, ctx, fmt.Sprintf("%s-Saver", exoChain.Config().Name), exoChain)
	exoUser = users[0]

	exoChainType, err := common.NewChain(exoChain.Config().Name)
	require.NoError(t, err)
	exoAsset := exoChainType.GetGasAsset()

	pool, err := thorchain.ApiGetPool(exoAsset)
	require.NoError(t, err)
	saveAmount := sdkmath.NewUintFromString(pool.BalanceAsset).
		MulUint64(500).QuoUint64(10_000)

	saverQuote, err := thorchain.ApiGetSaverDepositQuote(exoAsset, saveAmount)
	require.NoError(t, err)
	
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
	if rand.Intn(2) == 0 || exoChainType.String() == common.GAIAChain.String() { // if gaia memo is empty, bifrost errors
		memo = fmt.Sprintf("+:%s", exoAsset.GetSyntheticAsset())
	}

	exoInboundAddr, _, err := thorchain.ApiGetInboundAddress(exoChainType.String())
	require.NoError(t, err)

	wallet := ibc.WalletAmount{
		Address: exoInboundAddr,
		Denom: exoChain.Config().Denom,
		Amount: sdkmath.Int(saveAmount).
			MulRaw(int64(math.Pow10(int(*exoChain.Config().CoinDecimals)))).
			QuoRaw(int64(math.Pow10(int(*thorchain.Config().CoinDecimals)))), // save amount is based on 8 dec
	}
	if memo != "" {
		_, err = exoChain.SendFundsWithNote(ctx, exoUser.KeyName(), wallet, memo)
	} else {
		err = exoChain.SendFunds(ctx, exoUser.KeyName(), wallet)
	}
	require.NoError(t, err)

	errMsgCommon := fmt.Sprintf("saver (%s - %s) of asset %s", exoUser.KeyName(), exoUser.FormattedAddress(), exoAsset)
	saver, err := PollForSaver(ctx, thorchain, 30, exoAsset, exoUser)
	require.NoError(t, err, fmt.Sprintf("%s not found", errMsgCommon))

	deposit := sdkmath.NewUintFromString(saver.AssetDepositValue)
	require.True(t, deposit.GTE(minExpectedSaver), fmt.Sprintf("%s deposit: %s, min expected: %s", errMsgCommon, deposit, minExpectedSaver))
	require.True(t, deposit.LTE(maxExpectedSaver), fmt.Sprintf("%s deposit: %s, max expected: %s", errMsgCommon, deposit, maxExpectedSaver))

	return exoUser
}