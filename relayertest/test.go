// Package relayertest exposes a TestRelayer function
// that can be imported into other packages' tests.
//
// The exported TestRelayer function is intended to be a stable API
// that calls the other TestRelayer_* functions, which are not guaranteed
// to remain a stable API.
//
// External packages that intend to run IBC tests against their relayer implementation
// should define their own implementation of ibc.RelayerFactory,
// and in most cases should use an instance of ibc.BuiltinChainFactory.
//
//     package myrelayer_test
//
//     import (
//       "testing"
//
//       "github.com/strangelove-ventures/ibc-test-framework/ibc"
//       "github.com/strangelove-ventures/ibc-test-framework/relayertest"
//     )
//
//     func TestMyRelayer(t *testing.T) {
//       relayertest.TestRelayer(t, ibc.NewBuiltinChainFactory([]ibc.BuiltinChainFactoryEntry{
//         {Name: ""},
//       }, MyRelayerFactory())
//     }
//
// Although the relayertest package is made available as a convenience for other projects,
// the ibc-test-framework project should be considered the canonical definition of tests and configuration.
package relayertest

import (
	"strings"
	"testing"

	transfertypes "github.com/cosmos/ibc-go/v3/modules/apps/transfer/types"
	"github.com/strangelove-ventures/ibc-test-framework/ibc"
	"github.com/stretchr/testify/require"
)

// TestRelayer is the stable API exposed by the relayertest package.
// This is intended to be used by Go unit tests.
func TestRelayer(t *testing.T, cf ibc.ChainFactory, rf ibc.RelayerFactory) {
	t.Run("relay packet", func(t *testing.T) {
		t.Parallel()

		TestRelayer_RelayPacket(t, cf, rf)
	})
}

func sanitizeTestNameForContainer(testName string) string {
	// Subtests have slashes.
	testName = strings.ReplaceAll(testName, "/", "_")

	// Constructed subtest names in ibctest may contain + or @.
	testName = strings.ReplaceAll(testName, "+", "_")
	testName = strings.ReplaceAll(testName, "@", "_")
	return testName
}

func TestRelayer_RelayPacket(t *testing.T, cf ibc.ChainFactory, rf ibc.RelayerFactory) {
	testName := sanitizeTestNameForContainer(t.Name())

	ctx, home, pool, network, cleanup, err := ibc.SetupTestRun(testName)
	if err != nil {
		require.FailNow(t, "failed to set up test run: %v", err)
	}
	defer cleanup()

	srcChain, dstChain, err := cf.Pair(testName)
	if err != nil {
		require.FailNow(t, "failed to get chain pair: %v", err)
	}

	// startup both chains and relayer
	// creates wallets in the relayer for src and dst chain
	// funds relayer src and dst wallets on respective chain in genesis
	// creates a user account on the src chain (separate fullnode)
	// funds user account on src chain in genesis
	_, channels, srcUser, dstUser, rlyCleanup, err := ibc.StartChainsAndRelayerFromFactory(testName, ctx, pool, network, home, srcChain, dstChain, rf, nil)
	if err != nil {
		require.FailNow(t, "failed to StartChainsAndRelayerFromFactory: %v", err)
	}
	defer rlyCleanup()

	// will test a user sending an ibc transfer from the src chain to the dst chain
	// denom will be src chain native denom
	testDenomSrc := srcChain.Config().Denom

	// query initial balance of user wallet for src chain native denom on the src chain
	srcInitialBalance, err := srcChain.GetBalance(ctx, srcUser.SrcChainAddress, testDenomSrc)
	if err != nil {
		require.FailNow(t, "failed to get balance from source chain %s: %v", srcChain.Config().Name, err)
	}

	// get ibc denom for test denom on dst chain
	denomTrace := transfertypes.ParseDenomTrace(transfertypes.GetPrefixedDenom(channels[0].Counterparty.PortID, channels[0].Counterparty.ChannelID, testDenomSrc))
	dstIbcDenom := denomTrace.IBCDenom()

	// query initial balance of user wallet for src chain native denom on the dst chain
	// don't care about error here, account does not exist on destination chain
	dstInitialBalance, _ := dstChain.GetBalance(ctx, srcUser.DstChainAddress, dstIbcDenom)

	t.Logf("Initial source balance: %d", srcInitialBalance)
	t.Logf("Initial dest balance: %d", dstInitialBalance)

	// test coin, address is recipient of ibc transfer on dst chain
	testCoinSrc := ibc.WalletAmount{
		Address: srcUser.DstChainAddress,
		Denom:   testDenomSrc,
		Amount:  1000000,
	}

	// send ibc transfer from the user wallet using its fullnode
	// timeout is nil so that it will use the default timeout
	srcTxHash, err := srcChain.SendIBCTransfer(ctx, channels[0].ChannelID, srcUser.KeyName, testCoinSrc, nil)
	if err != nil {
		require.FailNow(t, "failed to send ibc transfer: %v", err)
	}

	// wait for both chains to produce 10 blocks
	if err := ibc.WaitForBlocks(srcChain, dstChain, 10); err != nil {
		require.FailNow(t, "failed to wait for blocks: %v", err)
	}

	// fetch ibc transfer tx
	srcTx, err := srcChain.GetTransaction(ctx, srcTxHash)
	if err != nil {
		require.FailNow(t, "failed to get ibc transaction: %v", err)
	}

	t.Logf("Transaction: %v", srcTx)

	// query final balance of src user wallet for src chain native denom on the src chain
	srcFinalBalance, err := srcChain.GetBalance(ctx, srcUser.SrcChainAddress, testDenomSrc)
	if err != nil {
		require.FailNow(t, "failed to get balance from source chain: %v", err)
	}

	// query final balance of src user wallet for src chain native denom on the dst chain
	dstFinalBalance, err := dstChain.GetBalance(ctx, srcUser.DstChainAddress, dstIbcDenom)
	if err != nil {
		require.FailNow(t, "failed to get balance from dest chain: %v", err)
	}

	totalFees := srcChain.GetGasFeesInNativeDenom(srcTx.GasWanted)
	expectedDifference := testCoinSrc.Amount + totalFees

	require.Equal(t, srcFinalBalance, srcInitialBalance-expectedDifference)
	require.Equal(t, dstFinalBalance, dstInitialBalance+testCoinSrc.Amount)

	// Now relay from dst chain to src chain using dst user wallet

	// will test a user sending an ibc transfer from the dst chain to the src chain
	// denom will be dst chain native denom
	testDenomDst := dstChain.Config().Denom

	// query initial balance of dst user wallet for dst chain native denom on the dst chain
	dstInitialBalance, err = dstChain.GetBalance(ctx, dstUser.DstChainAddress, testDenomDst)
	if err != nil {
		require.FailNow(t, "failed to get balance from dest chain: %v", err)
	}

	// get ibc denom for test denom on src chain
	srcDenomTrace := transfertypes.ParseDenomTrace(transfertypes.GetPrefixedDenom(channels[0].PortID, channels[0].ChannelID, testDenomDst))
	srcIbcDenom := srcDenomTrace.IBCDenom()

	// query initial balance of user wallet for src chain native denom on the dst chain
	// don't care about error here, account does not exist on destination chain
	srcInitialBalance, _ = srcChain.GetBalance(ctx, dstUser.SrcChainAddress, srcIbcDenom)

	t.Logf("Initial balance on src chain: %d", srcInitialBalance)
	t.Logf("Initial balance on dst chain: %d", dstInitialBalance)

	// test coin, address is recipient of ibc transfer on src chain
	testCoinDst := ibc.WalletAmount{
		Address: dstUser.SrcChainAddress,
		Denom:   testDenomDst,
		Amount:  1000000,
	}

	// send ibc transfer from the dst user wallet using its fullnode
	// timeout is nil so that it will use the default timeout
	dstTxHash, err := dstChain.SendIBCTransfer(ctx, channels[0].Counterparty.ChannelID, dstUser.KeyName, testCoinDst, nil)
	if err != nil {
		require.FailNow(t, "failed to send ibc transfer: %v", err)
	}

	// wait for both chains to produce 10 blocks
	if err := ibc.WaitForBlocks(srcChain, dstChain, 10); err != nil {
		require.FailNow(t, "failed to wait for blocks: %v", err)
	}

	// fetch ibc transfer tx
	dstTx, err := dstChain.GetTransaction(ctx, dstTxHash)
	if err != nil {
		require.FailNow(t, "failed to get transaction: %v", err)
	}

	t.Logf("Transaction: %v", dstTx)

	// query final balance of dst user wallet for dst chain native denom on the dst chain
	dstFinalBalance, err = dstChain.GetBalance(ctx, dstUser.DstChainAddress, testDenomDst)
	if err != nil {
		require.FailNow(t, "failed to get dest balance: %v", err)
	}

	// query final balance of dst user wallet for dst chain native denom on the src chain
	srcFinalBalance, err = srcChain.GetBalance(ctx, dstUser.SrcChainAddress, srcIbcDenom)
	if err != nil {
		require.FailNow(t, "failed to get src balance: %v", err)
	}

	totalFeesDst := dstChain.GetGasFeesInNativeDenom(dstTx.GasWanted)
	expectedDifference = testCoinDst.Amount + totalFeesDst

	require.Equal(t, dstFinalBalance, dstInitialBalance-expectedDifference)

	require.Equal(t, srcFinalBalance, srcInitialBalance+testCoinDst.Amount)
}
