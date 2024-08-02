package features

import (
	"context"
	"fmt"
	"math"
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
