package features

import (
	"context"
	"fmt"
	"math"
	"testing"

	sdkmath "cosmossdk.io/math"
	tc "github.com/strangelove-ventures/interchaintest/v8/chain/thorchain"
	"github.com/strangelove-ventures/interchaintest/v8/chain/thorchain/common"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
)

func SingleSwap(
	t *testing.T,
	ctx context.Context,
	thorchain *tc.Thorchain,
	srcChain ibc.Chain,
	destChain ibc.Chain,
) error {
	users := GetAndFundTestUsers(t, ctx, fmt.Sprintf("swap-%s-%s", srcChain.Config().Name, destChain.Config().Name), srcChain, destChain)
	srcUser, destUser := users[0], users[1]

	return singleSwap(ctx, thorchain, srcChain, srcUser, destChain, destUser)
}

// swap 0.5% of pool depth
func singleSwap(
	ctx context.Context,
	thorchain *tc.Thorchain,
	srcChain ibc.Chain,
	srcUser ibc.Wallet,
	destChain ibc.Chain,
	destUser ibc.Wallet,
) error {
	srcChainType, err := common.NewChain(srcChain.Config().Name)
	if err != nil {
		return fmt.Errorf("srcChainType assignment, %w", err)
	}
	destChainType, err := common.NewChain(destChain.Config().Name)
	if err != nil {
		return fmt.Errorf("destChainType assignment, %w", err)
	}

	srcChainAsset := srcChainType.GetGasAsset()
	destChainAsset := destChainType.GetGasAsset()
	pool, err := thorchain.ApiGetPool(srcChainAsset)
	if err != nil {
		return fmt.Errorf("getting srcChain pool, %w", err)
	}

	swapAmount := sdkmath.NewUintFromString(pool.BalanceAsset).QuoUint64(200)
	swapQuote, err := thorchain.ApiGetSwapQuote(srcChainAsset, destChainAsset, swapAmount)
	if err != nil {
		return fmt.Errorf("get swap quote, %w", err)
	}
	
	// store expected range to fail if received amount is outside 5% tolerance
	quoteOut := sdkmath.NewUintFromString(swapQuote.ExpectedAmountOut)
	tolerance := quoteOut.QuoUint64(20)
	if swapQuote.Fees.Outbound != nil {
		outboundFee := sdkmath.NewUintFromString(*swapQuote.Fees.Outbound)
		quoteOut = quoteOut.Add(outboundFee)

		// handle 2x gas rate fluctuation (add 1x outbound fee to tolerance)
		tolerance = tolerance.Add(outboundFee)
	}
	minExpectedSwapAmount := quoteOut.Sub(tolerance)
	maxExpectedSwapAmount := quoteOut.Add(tolerance)

	destUserBalancePreSwap, err := destChain.GetBalance(ctx, destUser.FormattedAddress(), destChain.Config().Denom)
	if err != nil {
		return err
	}

	srcChainInboundAddr, _, err := thorchain.ApiGetInboundAddress(srcChainType.String())
	if err != nil {
		return fmt.Errorf("get srcChain inbound address: %w", err)
	}
	memo := fmt.Sprintf("=:%s:%s", destChainAsset, destUser.FormattedAddress())
	txHash, err := srcChain.SendFundsWithNote(ctx, srcUser.KeyName(), ibc.WalletAmount{
		Address: srcChainInboundAddr,
		Denom: srcChain.Config().Denom,
		Amount: sdkmath.Int(swapAmount).
			MulRaw(int64(math.Pow10(int(*srcChain.Config().CoinDecimals)))).
			QuoRaw(int64(math.Pow10(int(*thorchain.Config().CoinDecimals)))), // swap amount is based on 8 dec,
	}, memo)
	if err != nil {
		return err
	}

	// ----- VerifyOutbound -----
	_, err = PollSwapCompleted(ctx, thorchain, 30, txHash)
	if err != nil {
		return err
	}
	/*stages, err := thorchain.ApiGetTxStages(txHash)
	if err != nil {
		return err
	}
	count := 0
	for stages.SwapFinalised == nil || !stages.SwapFinalised.Completed {
	//for stages.OutboundSigned == nil || !stages.OutboundSigned.Completed { // Only for non-rune swaps
		time.Sleep(time.Second)
		stages, err = thorchain.ApiGetTxStages(txHash)
		if err != nil {
			return err
		}
		count++
		require.Less(t, count, 60, "swap didn't complete in 60 seconds")
	}*/

	details, err := thorchain.ApiGetTxDetails(txHash)
	if err != nil {
		return err
	}

	if len(details.OutTxs) != 1 {
		return fmt.Errorf("expected exactly one out transaction, tx: %s", txHash)
	}

	if len(details.Actions) != 1 {
		return fmt.Errorf("expected exactly one action, tx: %s", txHash)
	}

	// verify outbound amount + max gas within expected range
	action := details.Actions[0]
	out := details.OutTxs[0]
	outAmountPlusMaxGas := sdkmath.NewUintFromString(out.Coins[0].Amount)
	maxGas := action.MaxGas[0]
	if maxGas.Asset == destChainAsset.String() {
		outAmountPlusMaxGas = outAmountPlusMaxGas.Add(sdkmath.NewUintFromString(maxGas.Amount))
	} else { // shouldn't enter here for atom -> rune
		var maxGasAssetValue sdkmath.Uint
		maxGasAssetValue, err = thorchain.ConvertAssetAmount(maxGas, destChainAsset.String())
		if err != nil {
			return fmt.Errorf("failed to convert asset, %w", err)
		}
		outAmountPlusMaxGas = outAmountPlusMaxGas.Add(maxGasAssetValue)
	}

	destUserBalancePostSwap, err := destChain.GetBalance(ctx, destUser.FormattedAddress(), destChain.Config().Denom)
	if err != nil {
		return err
	}
	actualSwapAmount := destUserBalancePostSwap.Sub(destUserBalancePreSwap)
	if actualSwapAmount.LT(sdkmath.Int(minExpectedSwapAmount)) {
		return fmt.Errorf("Actual swap amount: %s %s, min expected: %s", actualSwapAmount, destChain.Config().Denom, minExpectedSwapAmount)
	}
	if actualSwapAmount.GT(sdkmath.Int(maxExpectedSwapAmount)) {
		return fmt.Errorf("Actual swap amount: %s %s, max expected: %s", actualSwapAmount, destChain.Config().Denom, maxExpectedSwapAmount)
	}
	return nil
}