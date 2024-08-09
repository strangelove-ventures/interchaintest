package features

import (
	"context"
	"fmt"
	"math"
	"strings"
	"sync"
	"testing"

	sdkmath "cosmossdk.io/math"
	"github.com/strangelove-ventures/interchaintest/v8"
	tc "github.com/strangelove-ventures/interchaintest/v8/chain/thorchain"
	"github.com/strangelove-ventures/interchaintest/v8/chain/thorchain/common"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"github.com/strangelove-ventures/interchaintest/v8/testutil"
	"github.com/stretchr/testify/require"
)

func GetAndFundTestUsers(
	t *testing.T,
	ctx context.Context,
	keyNamePrefix string,
	chains ...ibc.Chain,
) []ibc.Wallet {
	users := make([]ibc.Wallet, len(chains))
	wg := &sync.WaitGroup{}
	for i, chain := range chains {
		i := i
		chain := chain
		oneCoin := int64(math.Pow10(int(*chain.Config().CoinDecimals)))
		amount := sdkmath.NewInt(1000 * oneCoin) // thor, gaia
		switch chain.Config().CoinType {
		case "60":
			amount = sdkmath.NewInt(9 * oneCoin) // change once gwei is supported
		case "0": // btc
			amount = sdkmath.NewInt(10 * oneCoin)
		case "2", "145": // ltc, bch
			amount = sdkmath.NewInt(100 * oneCoin)
		case "3": // doge
			amount = sdkmath.NewInt(10000 * oneCoin)
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			user := interchaintest.GetAndFundTestUsers(t, ctx, keyNamePrefix, amount, chain)
			users[i] = user[0]

			userBalance, err := chain.GetBalance(ctx, user[0].FormattedAddress(), chain.Config().Denom)
			require.NoError(t, err)
			require.True(t, userBalance.Equal(amount), fmt.Sprintf("User (%s) was not properly funded", user[0].KeyName()))
		}()
	}
	wg.Wait()

	return users
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
			return nil, err
		}

		if pool.BalanceAsset == "0" {
			return nil, fmt.Errorf("Pool (%s) exists, but not asset balance", asset)
		}
		return nil, nil
	}
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
			return tc.Saver{}, err
		}
		for _, saver := range savers {
			if strings.ToLower(saver.AssetAddress) == strings.ToLower(exoUser.FormattedAddress()) {
				return saver, nil
			}

		}
		return tc.Saver{}, fmt.Errorf("saver took longer than %d blocks to show", deltaBlocks)
	}
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
			return tc.Saver{}, err
		}
		for _, saver := range savers {
			if strings.ToLower(saver.AssetAddress) == strings.ToLower(exoUser.FormattedAddress()) {
				return saver, fmt.Errorf("saver took longer than %d blocks to eject", deltaBlocks)
			}

		}
		return tc.Saver{}, nil
	}
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
			return nil, err
		}
		if stages.SwapFinalised == nil || !stages.SwapFinalised.Completed {
			return nil, fmt.Errorf("swap (tx: %s) didn't complete in %d blocks", txHash, deltaBlocks)
		}
		return nil, nil
	}
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
			return nil, err
		}
		if stages.OutboundSigned == nil || !stages.OutboundSigned.Completed {
			return nil, fmt.Errorf("swap (tx: %s) didn't outbound sign in %d blocks", txHash, deltaBlocks)
		}
		return nil, nil
	}
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
			return nil, err
		}
		if balance.Amount.Equal(bal) {
			return nil, fmt.Errorf("%s balance (%s) hasn't changed: (%s) in %d blocks", balance.Address, bal.String(), balance.Amount.String(), deltaBlocks)
		}
		return nil, nil
	}
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
			return nil, nil
		}

		if pool.Status == "Suspended" {
			return nil, nil
		}

		return nil, fmt.Errorf("Pool (%s) did not suspend in %d blocks", exoAsset, deltaBlocks)
	}
	bp := testutil.BlockPoller[any]{CurrentHeight: thorchain.Height, PollFunc: doPoll}
	_, err = bp.DoPoll(ctx, h, h+deltaBlocks)
	return err
}

func AddAdminIfNecessary(ctx context.Context, thorchain *tc.Thorchain) error {
	_, err := thorchain.GetAddress(ctx, "admin")
	if err != nil {
		if err := thorchain.RecoverKey(ctx, "admin", strings.Repeat("master ", 23) + "notice"); err != nil {
			return err
		}
	}
	return nil
}