package features

import (
	"context"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	sdkmath "cosmossdk.io/math"
	"github.com/strangelove-ventures/interchaintest/v8"
	tc "github.com/strangelove-ventures/interchaintest/v8/chain/thorchain"
	"github.com/strangelove-ventures/interchaintest/v8/chain/thorchain/common"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"github.com/strangelove-ventures/interchaintest/v8/testutil"
	"golang.org/x/sync/errgroup"
)

func GetAndFundTestUsers(
	t *testing.T,
	ctx context.Context,
	keyNamePrefix string,
	chains ...ibc.Chain,
) ([]ibc.Wallet, error) {
	users := make([]ibc.Wallet, len(chains))
	eg, egCtx := errgroup.WithContext(ctx)
	for i, chain := range chains {
		i := i
		chain := chain
		oneCoin := sdkmath.NewInt(int64(math.Pow10(int(*chain.Config().CoinDecimals))))
		amount := oneCoin.MulRaw(1000) // thor, gaia
		switch chain.Config().CoinType {
		case "0", "60": // btc, eth
			amount = oneCoin.MulRaw(10)
		case "2", "145": // ltc, bch
			amount = oneCoin.MulRaw(100)
		case "3": // doge
			amount = oneCoin.MulRaw(10_000)
		}
		eg.Go(func() error {
			user := interchaintest.GetAndFundTestUsers(t, egCtx, keyNamePrefix, amount, chain)
			users[i] = user[0]

			userBalance, err := chain.GetBalance(egCtx, user[0].FormattedAddress(), chain.Config().Denom)
			if err != nil {
				return err
			}
			if !userBalance.Equal(amount) {
				return fmt.Errorf("user (%s) was not properly funded", user[0].KeyName())
			}

			return nil
		})
	}

	err := eg.Wait()

	return users, err
}

// PollForPool polls until the pool is found and funded
func PollForPool(ctx context.Context, thorchain *tc.Thorchain, deltaBlocks int64, asset common.Asset) error {
	h, err := thorchain.Height(ctx)
	if err != nil {
		return fmt.Errorf("failed to get height: %w", err)
	}
	doPoll := func(ctx context.Context, height int64) (any, error) {
		pool, err := thorchain.ApiGetPool(asset)
		if err != nil {
			time.Sleep(time.Second) // rate limit
			return nil, err
		}

		if pool.BalanceAsset == "0" {
			time.Sleep(time.Second) // rate limit
			return nil, fmt.Errorf("pool (%s) exists, but not asset balance in %d blocks", asset, deltaBlocks)
		}
		return nil, nil
	}
	time.Sleep(time.Second) // Limit how quickly Height() is called back to back per go routine
	bp := testutil.BlockPoller[any]{CurrentHeight: thorchain.Height, PollFunc: doPoll}
	_, err = bp.DoPoll(ctx, h, h+deltaBlocks)
	return err
}

// PollForSaver polls until the saver is found
func PollForSaver(ctx context.Context, thorchain *tc.Thorchain, deltaBlocks int64, asset common.Asset, exoUser ibc.Wallet) (tc.Saver, error) {
	h, err := thorchain.Height(ctx)
	if err != nil {
		return tc.Saver{}, fmt.Errorf("failed to get height: %w", err)
	}
	doPoll := func(ctx context.Context, height int64) (tc.Saver, error) {
		savers, err := thorchain.ApiGetSavers(asset)
		if err != nil {
			time.Sleep(time.Second) // rate limit
			return tc.Saver{}, err
		}
		for _, saver := range savers {
			if strings.EqualFold(saver.AssetAddress, exoUser.FormattedAddress()) {
				return saver, nil
			}

		}
		time.Sleep(time.Second) // rate limit
		return tc.Saver{}, fmt.Errorf("saver took longer than %d blocks to show", deltaBlocks)
	}
	time.Sleep(time.Second) // Limit how quickly Height() is called back to back per go routine
	bp := testutil.BlockPoller[tc.Saver]{CurrentHeight: thorchain.Height, PollFunc: doPoll}
	saver, err := bp.DoPoll(ctx, h, h+deltaBlocks)
	return saver, err
}

// PollForEjectedSaver polls until the saver no longer found
func PollForEjectedSaver(ctx context.Context, thorchain *tc.Thorchain, deltaBlocks int64, asset common.Asset, exoUser ibc.Wallet) (tc.Saver, error) {
	h, err := thorchain.Height(ctx)
	if err != nil {
		return tc.Saver{}, fmt.Errorf("failed to get height: %w", err)
	}
	doPoll := func(ctx context.Context, height int64) (tc.Saver, error) {
		savers, err := thorchain.ApiGetSavers(asset)
		if err != nil {
			time.Sleep(time.Second) // rate limit
			return tc.Saver{}, err
		}
		for _, saver := range savers {
			if strings.EqualFold(saver.AssetAddress, exoUser.FormattedAddress()) {
				time.Sleep(time.Second) // rate limit
				return saver, fmt.Errorf("saver took longer than %d blocks to eject", deltaBlocks)
			}

		}
		return tc.Saver{}, nil
	}
	time.Sleep(time.Second) // Limit how quickly Height() is called back to back per go routine
	bp := testutil.BlockPoller[tc.Saver]{CurrentHeight: thorchain.Height, PollFunc: doPoll}
	saver, err := bp.DoPoll(ctx, h, h+deltaBlocks)
	return saver, err
}

// PollSwapCompleted polls until the swap is completed
func PollSwapCompleted(ctx context.Context, thorchain *tc.Thorchain, deltaBlocks int64, txHash string) (any, error) {
	h, err := thorchain.Height(ctx)
	if err != nil {
		return tc.Saver{}, fmt.Errorf("failed to get height: %w", err)
	}
	doPoll := func(ctx context.Context, height int64) (any, error) {
		stages, err := thorchain.ApiGetTxStages(txHash)
		if err != nil {
			time.Sleep(time.Second) // rate limit
			return nil, err
		}
		if stages.SwapFinalised == nil || !stages.SwapFinalised.Completed {
			time.Sleep(time.Second) // rate limit
			return nil, fmt.Errorf("swap (tx: %s) didn't complete in %d blocks", txHash, deltaBlocks)
		}
		return nil, nil
	}
	time.Sleep(time.Second) // Limit how quickly Height() is called back to back per go routine
	bp := testutil.BlockPoller[any]{CurrentHeight: thorchain.Height, PollFunc: doPoll}
	saver, err := bp.DoPoll(ctx, h, h+deltaBlocks)
	return saver, err
}

// PollOutboundSigned polls until the swap is completed and outbound has been signed
func PollOutboundSigned(ctx context.Context, thorchain *tc.Thorchain, deltaBlocks int64, txHash string) (any, error) {
	h, err := thorchain.Height(ctx)
	if err != nil {
		return tc.Saver{}, fmt.Errorf("failed to get height: %w", err)
	}
	doPoll := func(ctx context.Context, height int64) (any, error) {
		stages, err := thorchain.ApiGetTxStages(txHash)
		if err != nil {
			time.Sleep(time.Second) // rate limit
			return nil, err
		}
		if stages.OutboundSigned == nil || !stages.OutboundSigned.Completed {
			time.Sleep(time.Second) // rate limit
			return nil, fmt.Errorf("swap (tx: %s) didn't outbound sign in %d blocks", txHash, deltaBlocks)
		}
		return nil, nil
	}
	time.Sleep(time.Second) // Limit how quickly Height() is called back to back per go routine
	bp := testutil.BlockPoller[any]{CurrentHeight: thorchain.Height, PollFunc: doPoll}
	saver, err := bp.DoPoll(ctx, h, h+deltaBlocks)
	return saver, err
}

// PollForBalanceChaqnge polls until the balance changes
func PollForBalanceChange(ctx context.Context, chain ibc.Chain, deltaBlocks int64, balance ibc.WalletAmount) error {
	h, err := chain.Height(ctx)
	if err != nil {
		return fmt.Errorf("failed to get height: %w", err)
	}
	doPoll := func(ctx context.Context, height int64) (any, error) {
		bal, err := chain.GetBalance(ctx, balance.Address, balance.Denom)
		if err != nil {
			time.Sleep(time.Second) // rate limit
			return nil, err
		}
		if balance.Amount.Equal(bal) {
			time.Sleep(time.Second) // rate limit
			return nil, fmt.Errorf("%s balance (%s) hasn't changed: (%s) in %d blocks", balance.Address, bal.String(), balance.Amount.String(), deltaBlocks)
		}
		return nil, nil
	}
	time.Sleep(time.Second) // Limit how quickly Height() is called back to back per go routine
	bp := testutil.BlockPoller[any]{CurrentHeight: chain.Height, PollFunc: doPoll}
	_, err = bp.DoPoll(ctx, h, h+deltaBlocks)
	return err
}

// PollForPoolSuspended polls until the pool is gone or suspended
func PollForPoolSuspended(ctx context.Context, thorchain *tc.Thorchain, deltaBlocks int64, exoAsset common.Asset) error {
	h, err := thorchain.Height(ctx)
	if err != nil {
		return fmt.Errorf("failed to get height: %w", err)
	}
	doPoll := func(ctx context.Context, height int64) (any, error) {
		pool, err := thorchain.ApiGetPool(exoAsset)
		if err != nil {
			time.Sleep(time.Second) // rate limit
			return nil, err
		}

		if pool.Status == "Suspended" {
			return nil, nil
		}

		time.Sleep(time.Second) // rate limit
		return nil, fmt.Errorf("pool (%s) did not suspend in %d blocks", exoAsset, deltaBlocks)
	}
	time.Sleep(time.Second) // Limit how quickly Height() is called back to back per go routine
	bp := testutil.BlockPoller[any]{CurrentHeight: thorchain.Height, PollFunc: doPoll}
	_, err = bp.DoPoll(ctx, h, h+deltaBlocks)
	return err
}

func AddAdminIfNecessary(ctx context.Context, thorchain *tc.Thorchain) error {
	_, err := thorchain.GetAddress(ctx, "admin")
	if err != nil {
		if err := thorchain.RecoverKey(ctx, "admin", strings.Repeat("master ", 23)+"notice"); err != nil {
			return err
		}
	}
	return nil
}
