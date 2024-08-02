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
		amount := sdkmath.NewInt(100 * oneCoin)
		switch chain.Config().Denom {
		case "btc", "wei":
			amount = sdkmath.NewInt(2 * oneCoin)
		case "doge":
			amount = sdkmath.NewInt(1000 * oneCoin)
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			user := interchaintest.GetAndFundTestUsers(t, ctx, keyNamePrefix, amount, chain)
			users[i] = user[0]
		}()
	}
	wg.Wait()

	return users
}

// PollForPool polls until the pool is found
func PollForPool(ctx context.Context, thorchain *tc.Thorchain, deltaBlocks int64, asset common.Asset) error {
	h, err := thorchain.Height(ctx)
	if err != nil {
		return fmt.Errorf("failed to get height: %w", err)
	}
	doPoll := func(ctx context.Context, height int64) (any, error) {
		_, err = thorchain.ApiGetPool(asset)
		if err != nil {
			return nil, err
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
		//if stages.OutboundSigned == nil || !stages.OutboundSigned.Completed { // Only for non-rune swaps
			return nil, fmt.Errorf("swap (tx: %s) didn't complete in %d blocks", txHash, deltaBlocks)
		}
		return nil, nil
	}
	bp := testutil.BlockPoller[any]{CurrentHeight: thorchain.Height, PollFunc: doPoll}
	saver, err := bp.DoPoll(ctx, h, h+deltaBlocks)
	return saver, err
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