package thorchain_test

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"sync"
	"testing"

	"cosmossdk.io/math"
	"github.com/strangelove-ventures/interchaintest/v8"
	"github.com/strangelove-ventures/interchaintest/v8/examples/thorchain/features"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"github.com/strangelove-ventures/interchaintest/v8/testutil"

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

	//err = testutil.WaitForBlocks(ctx, 300, thorchain)
	//require.NoError(t, err, "thorchain failed to make blocks")
}

// go test -timeout 20m -v -run TestThorchainMsgSend examples/thorchain/*.go -count 1
func TestThorchainMsgSend(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	t.Parallel()

	os.Setenv("ICTEST_SKIP_FAILURE_CLEANUP", "true")
	interchaintest.KeepDockerVolumesOnFailure(true)
	//testDir := interchaintest.TempDir(t)

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

	users, err := features.GetAndFundTestUsers(t, ctx, "thorusr1", thorchain)
	require.NoError(t, err)
	thorUsr1 := users[0]

	users2, err := features.GetAndFundTestUsers(t, ctx, "thorusr2", thorchain)
	require.NoError(t, err)
	thorUsr2 := users2[0]

	usr1BalBefore, err := thorchain.GetBalance(ctx, thorUsr1.FormattedAddress(), thorchain.Config().Denom)
	require.NoError(t, err)

	sendableAmount := usr1BalBefore.Quo(math.NewInt(10))

	usr2BalBefore, err := thorchain.GetBalance(ctx, thorUsr2.FormattedAddress(), thorchain.Config().Denom)
	require.NoError(t, err)

	amount := ibc.WalletAmount{
		Address: thorUsr2.FormattedAddress(),
		Denom:   thorchain.Config().Denom,
		Amount:  sendableAmount,
	}

	// No error verifies that the route is enabled for a normal bank send
	err = thorchain.CosmosBankSendWithNote(ctx, thorUsr1.KeyName(), amount, "THOR MSGSEND TEST")
	require.NoError(t, err)

	err = testutil.WaitForBlocks(ctx, 1, thorchain)
	require.NoError(t, err)

	usr1BalAfter, err := thorchain.GetBalance(ctx, thorUsr1.FormattedAddress(), thorchain.Config().Denom)
	require.NoError(t, err)
	usr2BalAfter, err := thorchain.GetBalance(ctx, thorUsr2.FormattedAddress(), thorchain.Config().Denom)
	require.NoError(t, err)
	usr2BalExpected := usr2BalBefore.Add(sendableAmount)

	require.Greater(t, usr1BalBefore.Int64(), usr1BalAfter.Int64())
	require.Equal(t, usr2BalExpected.Int64(), usr2BalAfter.Int64())
}
