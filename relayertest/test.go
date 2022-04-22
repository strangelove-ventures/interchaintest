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
//         {Name: "foo_bar" /* ... */},
//       }, MyRelayerFactory())
//     }
//
// Although the relayertest package is made available as a convenience for other projects,
// the ibc-test-framework project should be considered the canonical definition of tests and configuration.
package relayertest

import (
	"testing"
	"time"

	transfertypes "github.com/cosmos/ibc-go/v3/modules/apps/transfer/types"
	"github.com/strangelove-ventures/ibc-test-framework/ibc"
	"github.com/strangelove-ventures/ibc-test-framework/ibctest"
	"github.com/strangelove-ventures/ibc-test-framework/relayer"
	"github.com/stretchr/testify/require"
)

func requireCapabilities(t *testing.T, rf ibctest.RelayerFactory, reqCaps ...relayer.Capability) {
	t.Helper()

	if len(reqCaps) == 0 {
		panic("requireCapabilities called without any capabilities provided")
	}

	caps := rf.Capabilities()
	for _, c := range reqCaps {
		if !caps[c] {
			t.Skipf("skipping due to missing capability %s", c)
		}
	}
}

// TestRelayer is the stable API exposed by the relayertest package.
// This is intended to be used by Go unit tests.
func TestRelayer(t *testing.T, cf ibctest.ChainFactory, rf ibctest.RelayerFactory) {
	t.Run("relay packet", func(t *testing.T) {
		t.Parallel()

		TestRelayer_RelayPacket(t, cf, rf)
	})

	t.Run("no timeout", func(t *testing.T) {
		t.Parallel()

		TestRelayer_RelayPacketNoTimeout(t, cf, rf)
	})

	t.Run("height timeout", func(t *testing.T) {
		requireCapabilities(t, rf, relayer.HeightTimeout)

		t.Parallel()

		TestRelayer_RelayPacketHeightTimeout(t, cf, rf)
	})

	t.Run("timestamp timeout", func(t *testing.T) {
		requireCapabilities(t, rf, relayer.TimestampTimeout)

		t.Parallel()

		TestRelayer_RelayPacketTimestampTimeout(t, cf, rf)
	})
}

func TestRelayer_RelayPacket(t *testing.T, cf ibctest.ChainFactory, rf ibctest.RelayerFactory) {
	ctx, home, pool, network, err := ibctest.SetupTestRun(t)
	require.NoErrorf(t, err, "failed to set up test run")

	srcChain, dstChain, err := cf.Pair(t.Name())
	require.NoError(t, err, "failed to get chain pair")

	// startup both chains and relayer
	// creates wallets in the relayer for src and dst chain
	// funds relayer src and dst wallets on respective chain in genesis
	// creates a user account on the src chain (separate fullnode)
	// funds user account on src chain in genesis
	_, channels, srcUser, dstUser, err := ibctest.StartChainsAndRelayerFromFactory(t, ctx, pool, network, home, srcChain, dstChain, rf, nil)
	require.NoError(t, err, "failed to StartChainsAndRelayerFromFactory")

	// will test a user sending an ibc transfer from the src chain to the dst chain
	// denom will be src chain native denom
	testDenomSrc := srcChain.Config().Denom

	// query initial balance of user wallet for src chain native denom on the src chain
	srcInitialBalance, err := srcChain.GetBalance(ctx, srcUser.SrcChainAddress, testDenomSrc)
	require.NoErrorf(t, err, "failed to get balance from source chain %s", srcChain.Config().Name)

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
	require.NoError(t, err, "failed to send ibc transfer")

	// wait for both chains to produce 10 blocks
	require.NoError(t, ibctest.WaitForBlocks(srcChain, dstChain, 10), "failed to wait for blocks")

	// fetch ibc transfer tx
	srcTx, err := srcChain.GetTransaction(ctx, srcTxHash)
	require.NoError(t, err, "failed to get ibc transaction")

	t.Logf("Transaction: %v", srcTx)

	// query final balance of src user wallet for src chain native denom on the src chain
	srcFinalBalance, err := srcChain.GetBalance(ctx, srcUser.SrcChainAddress, testDenomSrc)
	require.NoError(t, err, "failed to get balance from source chain")

	// query final balance of src user wallet for src chain native denom on the dst chain
	dstFinalBalance, err := dstChain.GetBalance(ctx, srcUser.DstChainAddress, dstIbcDenom)
	require.NoError(t, err, "failed to get balance from dest chain")

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
	require.NoError(t, err, "failed to get balance from dest chain")

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
	require.NoError(t, err, "failed to send ibc transfer")

	// wait for both chains to produce 10 blocks
	require.NoError(t, ibctest.WaitForBlocks(srcChain, dstChain, 10), "failed to wait for blocks")

	// fetch ibc transfer tx
	dstTx, err := dstChain.GetTransaction(ctx, dstTxHash)
	require.NoError(t, err, "failed to get transaction")

	t.Logf("Transaction: %v", dstTx)

	// query final balance of dst user wallet for dst chain native denom on the dst chain
	dstFinalBalance, err = dstChain.GetBalance(ctx, dstUser.DstChainAddress, testDenomDst)
	require.NoError(t, err, "failed to get dest balance")

	// query final balance of dst user wallet for dst chain native denom on the src chain
	srcFinalBalance, err = srcChain.GetBalance(ctx, dstUser.SrcChainAddress, srcIbcDenom)
	require.NoError(t, err, "failed to get source balance")

	totalFeesDst := dstChain.GetGasFeesInNativeDenom(dstTx.GasWanted)
	expectedDifference = testCoinDst.Amount + totalFeesDst

	require.Equal(t, dstInitialBalance-expectedDifference, dstFinalBalance)
	require.Equal(t, srcInitialBalance+testCoinDst.Amount, srcFinalBalance)
}

// Ensure that a queued packet that is timed out (relative height timeout) will not be relayed.
func TestRelayer_RelayPacketNoTimeout(t *testing.T, cf ibctest.ChainFactory, rf ibctest.RelayerFactory) {
	ctx, home, pool, network, err := ibctest.SetupTestRun(t)
	require.NoErrorf(t, err, "failed to set up test run")

	srcChain, dstChain, err := cf.Pair(t.Name())
	require.NoError(t, err, "failed to get chain pair")

	var srcInitialBalance, dstInitialBalance int64
	var txHash string
	testDenom := srcChain.Config().Denom
	var dstIbcDenom string
	var testCoin ibc.WalletAmount

	// Query user account balances on both chains and send IBC transfer before starting the relayer
	preRelayerStart := func(channels []ibc.ChannelOutput, srcUser ibctest.User, dstUser ibctest.User) error {
		var err error
		srcInitialBalance, err = srcChain.GetBalance(ctx, srcUser.SrcChainAddress, testDenom)
		if err != nil {
			return err
		}

		// get ibc denom for test denom on dst chain
		denomTrace := transfertypes.ParseDenomTrace(transfertypes.GetPrefixedDenom(channels[0].Counterparty.PortID, channels[0].Counterparty.ChannelID, testDenom))
		dstIbcDenom = denomTrace.IBCDenom()

		// don't care about error here, account does not exist on destination chain
		dstInitialBalance, _ = dstChain.GetBalance(ctx, srcUser.DstChainAddress, dstIbcDenom)

		t.Logf("Initial source balance: %d", srcInitialBalance)
		t.Logf("Initial dest balance: %d", dstInitialBalance)

		testCoin = ibc.WalletAmount{
			Address: srcUser.DstChainAddress,
			Denom:   testDenom,
			Amount:  1000000,
		}

		// send ibc transfer with both timeouts disabled
		txHash, err = srcChain.SendIBCTransfer(ctx, channels[0].ChannelID, srcUser.KeyName, testCoin, &ibc.IBCTimeout{Height: 0, NanoSeconds: 0})
		return err
	}

	// Startup both chains and relayer
	_, _, user, _, err := ibctest.StartChainsAndRelayerFromFactory(t, ctx, pool, network, home, srcChain, dstChain, rf, preRelayerStart)
	require.NoError(t, err, "failed to StartChainsAndRelayerFromFactory")

	// wait for both chains to produce 10 blocks
	require.NoError(t, ibctest.WaitForBlocks(srcChain, dstChain, 10), "failed to wait for blocks")

	// fetch ibc transfer tx
	srcTx, err := srcChain.GetTransaction(ctx, txHash)
	require.NoError(t, err, "failed to get ibc transaction")

	t.Logf("Transaction: %v", srcTx)

	srcFinalBalance, err := srcChain.GetBalance(ctx, user.SrcChainAddress, testDenom)
	require.NoError(t, err, "failed to get balance from source chain")

	dstFinalBalance, err := dstChain.GetBalance(ctx, user.DstChainAddress, dstIbcDenom)
	require.NoError(t, err, "failed to get balance from dest chain")

	totalFees := srcChain.GetGasFeesInNativeDenom(srcTx.GasWanted)
	expectedDifference := testCoin.Amount + totalFees

	require.Equal(t, srcInitialBalance-expectedDifference, srcFinalBalance)
	require.Equal(t, dstInitialBalance+testCoin.Amount, dstFinalBalance)
}

// Ensure that a queued packet that is timed out (relative height timeout) will not be relayed.
func TestRelayer_RelayPacketHeightTimeout(t *testing.T, cf ibctest.ChainFactory, rf ibctest.RelayerFactory) {
	ctx, home, pool, network, err := ibctest.SetupTestRun(t)
	require.NoErrorf(t, err, "failed to set up test run")

	srcChain, dstChain, err := cf.Pair(t.Name())
	require.NoError(t, err, "failed to get chain pair")

	var srcInitialBalance, dstInitialBalance int64
	var txHash string
	testDenom := srcChain.Config().Denom
	var dstIbcDenom string

	// Query user account balances on both chains and send IBC transfer before starting the relayer
	preRelayerStart := func(channels []ibc.ChannelOutput, srcUser ibctest.User, dstUser ibctest.User) error {
		var err error
		srcInitialBalance, err = srcChain.GetBalance(ctx, srcUser.SrcChainAddress, testDenom)
		if err != nil {
			return err
		}

		// get ibc denom for test denom on dst chain
		denomTrace := transfertypes.ParseDenomTrace(transfertypes.GetPrefixedDenom(channels[0].Counterparty.PortID, channels[0].Counterparty.ChannelID, testDenom))
		dstIbcDenom = denomTrace.IBCDenom()

		// don't care about error here, account does not exist on destination chain
		dstInitialBalance, _ = dstChain.GetBalance(ctx, srcUser.DstChainAddress, dstIbcDenom)

		t.Logf("Initial source balance: %d", srcInitialBalance)
		t.Logf("Initial dest balance: %d", dstInitialBalance)

		testCoin := ibc.WalletAmount{
			Address: srcUser.DstChainAddress,
			Denom:   testDenom,
			Amount:  1000000,
		}

		// send ibc transfer with a timeout of 10 blocks from now on counterparty chain
		txHash, err = srcChain.SendIBCTransfer(ctx, channels[0].ChannelID, srcUser.KeyName, testCoin, &ibc.IBCTimeout{Height: 10})
		if err != nil {
			return err
		}

		// wait until counterparty chain has passed the timeout
		_, err = dstChain.WaitForBlocks(11)
		return err
	}

	// Startup both chains and relayer
	_, _, user, _, err := ibctest.StartChainsAndRelayerFromFactory(t, ctx, pool, network, home, srcChain, dstChain, rf, preRelayerStart)
	require.NoError(t, err, "failed to StartChainsAndRelayerFromFactory")

	// wait for both chains to produce 10 blocks
	require.NoError(t, ibctest.WaitForBlocks(srcChain, dstChain, 10), "failed to wait for blocks")

	// fetch ibc transfer tx
	srcTx, err := srcChain.GetTransaction(ctx, txHash)
	require.NoError(t, err, "failed to get ibc transaction")

	t.Logf("Transaction: %v", srcTx)

	srcFinalBalance, err := srcChain.GetBalance(ctx, user.SrcChainAddress, testDenom)
	require.NoError(t, err, "failed to get balance from source chain")

	dstFinalBalance, err := dstChain.GetBalance(ctx, user.DstChainAddress, dstIbcDenom)
	require.NoError(t, err, "failed to get balance from dest chain")

	totalFees := srcChain.GetGasFeesInNativeDenom(srcTx.GasWanted)

	require.Equal(t, srcInitialBalance-totalFees, srcFinalBalance)
	require.Equal(t, dstInitialBalance, dstFinalBalance)
}

func TestRelayer_RelayPacketTimestampTimeout(t *testing.T, cf ibctest.ChainFactory, rf ibctest.RelayerFactory) {
	ctx, home, pool, network, err := ibctest.SetupTestRun(t)
	require.NoErrorf(t, err, "failed to set up test run")

	srcChain, dstChain, err := cf.Pair(t.Name())
	require.NoError(t, err, "failed to get chain pair")

	var srcInitialBalance, dstInitialBalance int64
	var txHash string

	testDenom := srcChain.Config().Denom
	var dstIbcDenom string

	// Query user account balances on both chains and send IBC transfer before starting the relayer
	preRelayerStart := func(channels []ibc.ChannelOutput, srcUser ibctest.User, dstUser ibctest.User) error {
		var err error
		srcInitialBalance, err = srcChain.GetBalance(ctx, srcUser.SrcChainAddress, testDenom)
		if err != nil {
			return err
		}

		// get ibc denom for test denom on dst chain
		denomTrace := transfertypes.ParseDenomTrace(transfertypes.GetPrefixedDenom(channels[0].Counterparty.PortID, channels[0].Counterparty.ChannelID, testDenom))
		dstIbcDenom = denomTrace.IBCDenom()

		// don't care about error here, account does not exist on destination chain
		dstInitialBalance, _ = dstChain.GetBalance(ctx, srcUser.DstChainAddress, dstIbcDenom)

		t.Logf("Initial source balance: %d", srcInitialBalance)
		t.Logf("Initial dest balance: %d", dstInitialBalance)

		testCoin := ibc.WalletAmount{
			Address: srcUser.DstChainAddress,
			Denom:   testDenom,
			Amount:  1000000,
		}

		// send ibc transfer with a timeout of 10 blocks from now on counterparty chain
		txHash, err = srcChain.SendIBCTransfer(ctx, channels[0].ChannelID, srcUser.KeyName, testCoin, &ibc.IBCTimeout{NanoSeconds: uint64((10 * time.Second).Nanoseconds())})
		if err != nil {
			return err
		}

		// wait until ibc transfer times out
		time.Sleep(15 * time.Second)

		return nil
	}

	// Startup both chains and relayer
	_, _, user, _, err := ibctest.StartChainsAndRelayerFromFactory(t, ctx, pool, network, home, srcChain, dstChain, rf, preRelayerStart)
	require.NoError(t, err, "failed to StartChainsAndRelayerFromFactory")

	// wait for both chains to produce 10 blocks
	require.NoError(t, ibctest.WaitForBlocks(srcChain, dstChain, 10), "failed to wait for blocks")

	// fetch ibc transfer tx
	srcTx, err := srcChain.GetTransaction(ctx, txHash)
	require.NoError(t, err, "failed to get ibc transaction")

	t.Logf("Transaction: %v", srcTx)

	srcFinalBalance, err := srcChain.GetBalance(ctx, user.SrcChainAddress, testDenom)
	require.NoError(t, err, "failed to get balance from source chain")

	dstFinalBalance, err := dstChain.GetBalance(ctx, user.DstChainAddress, dstIbcDenom)
	require.NoError(t, err, "failed to get balance from dest chain")

	totalFees := srcChain.GetGasFeesInNativeDenom(srcTx.GasWanted)

	require.Equal(t, srcInitialBalance-totalFees, srcFinalBalance)
	require.Equal(t, dstInitialBalance, dstFinalBalance)
}
