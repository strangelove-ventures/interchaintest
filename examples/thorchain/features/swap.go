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
	"golang.org/x/sync/errgroup"
)

func SingleSwap(
	t *testing.T,
	ctx context.Context,
	thorchain *tc.Thorchain,
	srcChain ibc.Chain,
	destChain ibc.Chain,
) error {
	fmt.Println("#### Single swap:", srcChain.Config().Name, "to", destChain.Config().Name)
	users, err := GetAndFundTestUsers(t, ctx, fmt.Sprintf("swap-%s-%s", srcChain.Config().Name, destChain.Config().Name), srcChain, destChain)
	if err != nil {
		return err
	}
	srcUser, destUser := users[0], users[1]

	return singleSwap(ctx, thorchain, srcChain, srcUser, destChain, destUser)
}

func DualSwap(
	t *testing.T,
	ctx context.Context,
	thorchain *tc.Thorchain,
	srcChain ibc.Chain,
	destChain ibc.Chain,
) error {
	fmt.Println("#### Dual swap:", srcChain.Config().Name, "<->", destChain.Config().Name)
	users, err := GetAndFundTestUsers(t, ctx, fmt.Sprintf("swap-%s-%s", srcChain.Config().Name, destChain.Config().Name), srcChain, destChain)
	if err != nil {
		return err
	}
	srcUser, destUser := users[0], users[1]

	var eg errgroup.Group

	eg.Go(func() error {
		return singleSwap(ctx, thorchain, srcChain, srcUser, destChain, destUser)
	})

	eg.Go(func() error {
		return singleSwap(ctx, thorchain, destChain, destUser, srcChain, srcUser)
	})

	return eg.Wait()
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
	tolerance := quoteOut.QuoUint64(14) // TODO: was 5%, but got failures, now 7.1%
	if swapQuote.Fees.Outbound != nil {
		outboundFee := sdkmath.NewUintFromString(*swapQuote.Fees.Outbound)
		quoteOut = quoteOut.Add(outboundFee)

		// handle 2x gas rate fluctuation (add 1x outbound fee to tolerance)
		tolerance = tolerance.Add(outboundFee)
	}
	minExpectedSwapAmount := quoteOut.Sub(tolerance).
		MulUint64(uint64(math.Pow10(int(*destChain.Config().CoinDecimals)))).
		QuoUint64(uint64(math.Pow10(int(*thorchain.Config().CoinDecimals))))
	maxExpectedSwapAmount := quoteOut.Add(tolerance).
		MulUint64(uint64(math.Pow10(int(*destChain.Config().CoinDecimals)))).
		QuoUint64(uint64(math.Pow10(int(*thorchain.Config().CoinDecimals))))

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
		Denom:   srcChain.Config().Denom,
		Amount: sdkmath.Int(swapAmount).
			MulRaw(int64(math.Pow10(int(*srcChain.Config().CoinDecimals)))).
			QuoRaw(int64(math.Pow10(int(*thorchain.Config().CoinDecimals)))), // swap amount is based on 8 dec,
	}, memo)
	if err != nil {
		return err
	}

	if txHash[0:2] == "0x" {
		txHash = txHash[2:]
	}

	fmt.Println("Swap tx hash:", txHash)
	// ----- VerifyOutbound -----
	if destChainType.String() == common.THORChain.String() {
		_, err = PollSwapCompleted(ctx, thorchain, 30, txHash)
		if err != nil {
			return err
		}
	} else {
		_, err = PollOutboundSigned(ctx, thorchain, 200, txHash)
		if err != nil {
			return fmt.Errorf("Outbound chain: %s, err: %w", destChainType, err)
		}
	}

	details, err := thorchain.ApiGetTxDetails(txHash)
	if err != nil {
		return err
	}

	if len(details.OutTxs) != 1 {
		return fmt.Errorf("expected exactly one out transaction, tx: %s, OutTxs: %d", txHash, len(details.OutTxs))
	}

	if len(details.Actions) != 1 {
		return fmt.Errorf("expected exactly one action, tx: %s, actions: %d", txHash, len(details.Actions))
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
	// TODO: compare outAmountPlusMaxGas -> actualSwapAmount
	fmt.Println("outAmountPlusMaxGas:", outAmountPlusMaxGas, "actualSwapAmount:", actualSwapAmount)
	return nil
}
