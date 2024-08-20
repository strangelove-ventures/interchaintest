package features

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"sync"
	"testing"

	sdkmath "cosmossdk.io/math"
	tc "github.com/strangelove-ventures/interchaintest/v8/chain/thorchain"
	"github.com/strangelove-ventures/interchaintest/v8/chain/thorchain/common"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
)

func SaverEject(
	t *testing.T,
	ctx context.Context,
	mimirLock *sync.Mutex, // Lock must be used across all chains testing saver eject in parallel
	thorchain *tc.Thorchain,
	exoChain ibc.Chain,
	exoSavers ...ibc.Wallet, // Savers that should not be ejected
) (exoUser ibc.Wallet, err error) {
	fmt.Println("#### Saver Eject:", exoChain.Config().Name)
	if err := AddAdminIfNecessary(ctx, thorchain); err != nil {
		return exoUser, err
	}

	users, err := GetAndFundTestUsers(t, ctx, fmt.Sprintf("%s-SaverEject", exoChain.Config().Name), exoChain)
	if err != nil {
		return exoUser, err
	}
	exoUser = users[0]

	// Reset mimirs
	mimirLock.Lock()
	mimirs, err := thorchain.ApiGetMimirs()
	if err != nil {
		mimirLock.Unlock()
		return exoUser, err
	}

	// Set max synth per pool depth to 100% of pool amount
	if mimir, ok := mimirs[strings.ToUpper("MaxSynthPerPoolDepth")]; ok && mimir != int64(5000) {
		if err = thorchain.SetMimir(ctx, "admin", "MaxSynthPerPoolDepth", "5000"); err != nil {
			mimirLock.Unlock()
			return exoUser, err
		}
	}

	// Disable saver ejection
	if mimir, ok := mimirs[strings.ToUpper("SaversEjectInterval")]; ok && mimir != int64(0) || !ok {
		if err = thorchain.SetMimir(ctx, "admin", "SaversEjectInterval", "0"); err != nil {
			mimirLock.Unlock()
			return exoUser, err
		}
	}

	exoChainType, err := common.NewChain(exoChain.Config().Name)
	if err != nil {
		mimirLock.Unlock()
		return exoUser, err
	}
	exoAsset := exoChainType.GetGasAsset()

	pool, err := thorchain.ApiGetPool(exoAsset)
	if err != nil {
		mimirLock.Unlock()
		return exoUser, err
	}
	saveAmount := sdkmath.NewUintFromString(pool.BalanceAsset).
		MulUint64(2000).QuoUint64(10_000)

	saverQuote, err := thorchain.ApiGetSaverDepositQuote(exoAsset, saveAmount)
	if err != nil {
		mimirLock.Unlock()
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
	if rand.Intn(2) == 0 || exoChainType.String() == common.GAIAChain.String() { // if gaia memo is empty, bifrost errors
		memo = fmt.Sprintf("+:%s", exoAsset.GetSyntheticAsset())
	}

	exoInboundAddr, _, err := thorchain.ApiGetInboundAddress(exoChainType.String())
	if err != nil {
		mimirLock.Unlock()
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
	} else {
		err = exoChain.SendFunds(ctx, exoUser.KeyName(), wallet)
	}
	if err != nil {
		mimirLock.Unlock()
		return exoUser, err
	}

	errMsgCommon := fmt.Sprintf("saver (%s - %s) of asset %s", exoUser.KeyName(), exoUser.FormattedAddress(), exoAsset)
	saver, err := PollForSaver(ctx, thorchain, 30, exoAsset, exoUser)
	if err != nil {
		mimirLock.Unlock()
		return exoUser, fmt.Errorf("%s not found, %w", errMsgCommon, err)
	}

	deposit := sdkmath.NewUintFromString(saver.AssetDepositValue)
	if deposit.LT(minExpectedSaver) {
		mimirLock.Unlock()
		return exoUser, fmt.Errorf("%s deposit: %s, min expected: %s", errMsgCommon, deposit, minExpectedSaver)
	}
	if deposit.GT(maxExpectedSaver) {
		mimirLock.Unlock()
		return exoUser, fmt.Errorf("%s deposit: %s, max expected: %s", errMsgCommon, deposit, maxExpectedSaver)
	}

	exoUserPreEjectBalance, err := exoChain.GetBalance(ctx, exoUser.FormattedAddress(), exoChain.Config().Denom)
	if err != nil {
		mimirLock.Unlock()
		return exoUser, err
	}

	exoSaversBalance := make([]sdkmath.Int, len(exoSavers))
	for i, exoSaver := range exoSavers {
		exoSaversBalance[i], err = exoChain.GetBalance(ctx, exoSaver.FormattedAddress(), exoChain.Config().Denom)
		if err != nil {
			mimirLock.Unlock()
			return exoUser, err
		}
	}

	mimirs, err = thorchain.ApiGetMimirs()
	if err != nil {
		mimirLock.Unlock()
		return exoUser, err
	}

	// Set mimirs
	if mimir, ok := mimirs[strings.ToUpper("MaxSynthPerPoolDepth")]; ok && mimir != int64(500) || !ok {
		if err := thorchain.SetMimir(ctx, "admin", "MaxSynthPerPoolDepth", "500"); err != nil {
			mimirLock.Unlock()
			return exoUser, err
		}
	}

	if mimir, ok := mimirs[strings.ToUpper("SaversEjectInterval")]; ok && mimir != int64(1) || !ok {
		if err := thorchain.SetMimir(ctx, "admin", "SaversEjectInterval", "1"); err != nil {
			mimirLock.Unlock()
			return exoUser, err
		}
	}

	_, err = PollForEjectedSaver(ctx, thorchain, 30, exoAsset, exoUser)
	if err != nil {
		mimirLock.Unlock()
		return exoUser, err
	}
	mimirLock.Unlock()

	if err = PollForBalanceChange(ctx, exoChain, 15, ibc.WalletAmount{
		Address: exoUser.FormattedAddress(),
		Denom:   exoChain.Config().Denom,
		Amount:  exoUserPreEjectBalance,
	}); err != nil {
		return exoUser, err
	}
	exoUserPostEjectBalance, err := exoChain.GetBalance(ctx, exoUser.FormattedAddress(), exoChain.Config().Denom)
	if err != nil {
		return exoUser, err
	}
	if exoUserPostEjectBalance.LTE(exoUserPreEjectBalance) {
		return exoUser, fmt.Errorf("User (%s) balance (%s) must be greater after ejection: %s", exoUser.KeyName(), exoUserPostEjectBalance, exoUserPreEjectBalance)
	}

	for i, exoSaver := range exoSavers {
		exoSaverPostBalance, err := exoChain.GetBalance(ctx, exoSaver.FormattedAddress(), exoChain.Config().Denom)
		if err != nil {
			return exoUser, err
		}
		if !exoSaverPostBalance.Equal(exoSaversBalance[i]) {
			return exoUser, fmt.Errorf("Saver's (%s) post balance (%s) should be the same as (%s)", exoSaver.KeyName(), exoSaverPostBalance, exoSaversBalance[i])
		}
	}

	return exoUser, nil
}
