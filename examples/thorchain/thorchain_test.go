package thorchain_test

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"testing"

	"github.com/strangelove-ventures/interchaintest/v9"
	"github.com/strangelove-ventures/interchaintest/v9/examples/thorchain/features"

	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
)

// go test -timeout 20m -v -run TestThorchain examples/thorchain/*.go -count 1
func TestThorchainSim(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	t.Parallel()

	ctx := context.Background()
	client, network := interchaintest.DockerSetup(t)

	// Start non-thorchain chains
	exoChains := StartExoChains(t, ctx, client, network)
	gaiaEg := SetupGaia(t, ctx, exoChains["GAIA"])
	ethRouterContractAddress, bscRouterContractAddress, err := SetupContracts(ctx, exoChains["ETH"], exoChains["BSC"])
	require.NoError(t, err)

	// Start thorchain
	thorchain := StartThorchain(t, ctx, client, network, exoChains, ethRouterContractAddress, bscRouterContractAddress)
	require.NoError(t, gaiaEg.Wait()) // Wait for 100 transactions before starting tests

	// --------------------------------------------------------
	// Bootstrap pool
	// --------------------------------------------------------
	eg, egCtx := errgroup.WithContext(ctx)
	for _, exoChain := range exoChains {
		exoChain := exoChain
		eg.Go(func() error {
			_, lper, err := features.DualLp(t, egCtx, thorchain, exoChain.chain)
			if err != nil {
				return err
			}
			exoChain.lpers = append(exoChain.lpers, lper)
			return nil
		})
	}
	require.NoError(t, eg.Wait())

	// --------------------------------------------------------
	// Savers
	// --------------------------------------------------------
	eg, egCtx = errgroup.WithContext(ctx)
	for _, exoChain := range exoChains {
		exoChain := exoChain
		eg.Go(func() error {
			saver, err := features.Saver(t, egCtx, thorchain, exoChain.chain)
			if err != nil {
				return err
			}
			exoChain.savers = append(exoChain.savers, saver)
			return nil
		})
	}
	require.NoError(t, eg.Wait())

	// --------------------------------------------------------
	// Arb
	// --------------------------------------------------------
	_, err = features.Arb(t, ctx, thorchain, exoChains.GetChains()...)
	require.NoError(t, err)

	// --------------------------------------------------------
	// Swap - only swaps non-rune assets for now
	// --------------------------------------------------------
	eg, egCtx = errgroup.WithContext(ctx)
	exoChainList := exoChains.GetChains()
	for i := range exoChainList {
		i := i
		fmt.Println("Chain:", i, "Name:", exoChainList[i].Config().Name)
		randomChain := rand.Intn(len(exoChainList))
		if i == randomChain && i == 0 {
			randomChain++
		} else if i == randomChain {
			randomChain--
		}
		eg.Go(func() error {
			return features.DualSwap(t, ctx, thorchain, exoChainList[i], exoChainList[randomChain])
		})
	}
	require.NoError(t, eg.Wait())

	// ------------------------------------------------------------
	// Saver Eject - must be done sequentially due to mimir states
	// ------------------------------------------------------------
	mimirLock := sync.Mutex{}
	eg, egCtx = errgroup.WithContext(ctx)
	for _, exoChain := range exoChains {
		exoChain := exoChain
		eg.Go(func() error {
			_, err = features.SaverEject(t, egCtx, &mimirLock, thorchain, exoChain.chain, exoChain.savers...)
			if err != nil {
				return err
			}
			return nil
		})
	}
	require.NoError(t, eg.Wait())

	// --------------------------------------------------------
	// Ragnarok
	// --------------------------------------------------------
	eg, egCtx = errgroup.WithContext(ctx)
	for _, exoChain := range exoChains {
		exoChain := exoChain
		eg.Go(func() error {
			refundWallets := append(exoChain.lpers, exoChain.savers...)
			return features.Ragnarok(t, egCtx, thorchain, exoChain.chain, refundWallets...)
		})
	}
	require.NoError(t, eg.Wait())

	// err = testutil.WaitForBlocks(ctx, 300, thorchain)
	// require.NoError(t, err, "thorchain failed to make blocks")
}
