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
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	transfertypes "github.com/cosmos/ibc-go/v3/modules/apps/transfer/types"
	"github.com/strangelove-ventures/ibc-test-framework/ibc"
	"github.com/strangelove-ventures/ibc-test-framework/ibctest"
	"github.com/strangelove-ventures/ibc-test-framework/relayer"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
)

const (
	userFaucetFund = int64(10000000000)
	testCoinAmount = int64(1000000)
)

type RelayerTestCase struct {
	Name string
	// which relayer capabilities are required to run this test
	RequiredRelayerCapabilities []relayer.Capability
	// function to run after the chains are started but before the relayer is started
	// e.g. send a transfer and wait for it to timeout so that the relayer will handle it once it is timed out
	PreRelayerStart func(*RelayerTestCase, *testing.T, context.Context, ibc.Chain, ibc.Chain, []ibc.ChannelOutput)
	// user on source chain
	SrcUser *ibctest.User
	// user on destination chain
	DstUser *ibctest.User
	// test after chains and relayers are started
	Test func(*RelayerTestCase, *testing.T, context.Context, ibc.Chain, ibc.Chain, []ibc.ChannelOutput)
	// temp storage in between test phases
	Cache []string
}

var relayerTestCases = [...]RelayerTestCase{
	{
		Name:            "relay packet",
		PreRelayerStart: preRelayerStart_RelayPacket,
		Test:            testPacketRelaySuccess,
	},
	{
		Name:            "no timeout",
		PreRelayerStart: preRelayerStart_NoTimeout,
		Test:            testPacketRelaySuccess,
	},
	{
		Name:                        "height timeout",
		RequiredRelayerCapabilities: []relayer.Capability{relayer.HeightTimeout},
		PreRelayerStart:             preRelayerStart_HeightTimeout,
		Test:                        testPacketRelayFail,
	},
	{
		Name:                        "timestamp timeout",
		RequiredRelayerCapabilities: []relayer.Capability{relayer.TimestampTimeout},
		PreRelayerStart:             preRelayerStart_TimestampTimeout,
		Test:                        testPacketRelayFail,
	},
}

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

func sendIBCTransfersFromBothChainsWithTimeout(
	testCase *RelayerTestCase,
	t *testing.T,
	ctx context.Context,
	srcChain ibc.Chain,
	dstChain ibc.Chain,
	channels []ibc.ChannelOutput,
	timeout *ibc.IBCTimeout,
) {
	srcUser := testCase.SrcUser
	dstUser := testCase.DstUser

	// will send ibc transfers from user wallet on both chains to their own respective wallet on the other chain
	testCoinSrcToDst := ibc.WalletAmount{
		Address: srcUser.CounterpartyChainAddress,
		Denom:   srcChain.Config().Denom,
		Amount:  testCoinAmount,
	}
	testCoinDstToSrc := ibc.WalletAmount{
		Address: dstUser.CounterpartyChainAddress,
		Denom:   dstChain.Config().Denom,
		Amount:  testCoinAmount,
	}

	txHashes := []string{}
	txHashLock := sync.Mutex{}

	var eg errgroup.Group

	eg.Go(func() error {
		txHashSrc, err := srcChain.SendIBCTransfer(ctx, channels[0].ChannelID, srcUser.KeyName, testCoinSrcToDst, timeout)
		if err != nil {
			return fmt.Errorf("failed to send ibc transfer from source: %v", err)
		}
		txHashLock.Lock()
		defer txHashLock.Unlock()
		txHashes = append(txHashes, txHashSrc)
		return nil
	})

	eg.Go(func() error {
		txHashDst, err := dstChain.SendIBCTransfer(ctx, channels[0].Counterparty.ChannelID, dstUser.KeyName, testCoinDstToSrc, timeout)
		if err != nil {
			return fmt.Errorf("failed to send ibc transfer from source: %v", err)
		}
		txHashLock.Lock()
		defer txHashLock.Unlock()
		txHashes = append(txHashes, txHashDst)
		return nil
	})

	require.NoError(t, eg.Wait())
	testCase.Cache = txHashes
}

// TestRelayer is the stable API exposed by the relayertest package.
// This is intended to be used by Go unit tests.
func TestRelayer(t *testing.T, cf ibctest.ChainFactory, rf ibctest.RelayerFactory) {
	ctx, home, pool, network, err := ibctest.SetupTestRun(t)
	require.NoError(t, err, "failed to set up test run")

	srcChain, dstChain, err := cf.Pair(t.Name())
	require.NoError(t, err, "failed to get chain pair")

	preRelayerStartFuncs := []func([]ibc.ChannelOutput){}

	for _, relayerTestCase := range relayerTestCases {
		relayerTestCase := relayerTestCase
		preRelayerStartFunc := func(channels []ibc.ChannelOutput) {
			// fund a user wallet on both chains, save on test case
			relayerTestCase.SrcUser, relayerTestCase.DstUser = ibctest.GetAndFundTestUsers(t, ctx, srcChain, dstChain, strings.ReplaceAll(relayerTestCase.Name, " ", "-"), userFaucetFund)
			// run test specific pre relayer start action
			relayerTestCase.PreRelayerStart(&relayerTestCase, t, ctx, srcChain, dstChain, channels)
		}
		preRelayerStartFuncs = append(preRelayerStartFuncs, preRelayerStartFunc)
	}

	// startup both chains and relayer
	// creates wallets in the relayer for src and dst chain
	// funds relayer src and dst wallets on respective chain in genesis
	// creates a faucet account on the both chains (separate fullnode)
	// funds faucet accounts in genesis
	_, channels, err := ibctest.StartChainsAndRelayerFromFactory(t, ctx, pool, network, home, srcChain, dstChain, rf, preRelayerStartFuncs)
	require.NoError(t, err, "failed to StartChainsAndRelayerFromFactory")

	// Wait for both chains to produce 20 blocks.
	// This is long to allow for intermittent retries inside the relayer.
	require.NoError(t, ibctest.WaitForBlocks(srcChain, dstChain, 20), "failed to wait for blocks")

	for _, relayerTestCase := range relayerTestCases {
		relayerTestCase := relayerTestCase
		t.Run(relayerTestCase.Name, func(t *testing.T) {
			for _, capability := range relayerTestCase.RequiredRelayerCapabilities {
				requireCapabilities(t, rf, capability)
			}
			t.Parallel()
			relayerTestCase.Test(&relayerTestCase, t, ctx, srcChain, dstChain, channels)
		})
	}
}

// PreRelayerStart methods for the RelayerTestCases

func preRelayerStart_RelayPacket(testCase *RelayerTestCase, t *testing.T, ctx context.Context, srcChain ibc.Chain, dstChain ibc.Chain, channels []ibc.ChannelOutput) {
	sendIBCTransfersFromBothChainsWithTimeout(testCase, t, ctx, srcChain, dstChain, channels, nil)
}

func preRelayerStart_NoTimeout(testCase *RelayerTestCase, t *testing.T, ctx context.Context, srcChain ibc.Chain, dstChain ibc.Chain, channels []ibc.ChannelOutput) {
	ibcTimeoutDisabled := ibc.IBCTimeout{Height: 0, NanoSeconds: 0}
	sendIBCTransfersFromBothChainsWithTimeout(testCase, t, ctx, srcChain, dstChain, channels, &ibcTimeoutDisabled)
	// TODO should we wait here to make sure it successfully relays a packet beyond the default timeout period?
	// would need to shorten the chain default timeouts somehow to make that a feasible test
}

func preRelayerStart_HeightTimeout(testCase *RelayerTestCase, t *testing.T, ctx context.Context, srcChain ibc.Chain, dstChain ibc.Chain, channels []ibc.ChannelOutput) {
	ibcTimeoutHeight := ibc.IBCTimeout{Height: 10}
	sendIBCTransfersFromBothChainsWithTimeout(testCase, t, ctx, srcChain, dstChain, channels, &ibcTimeoutHeight)
	// wait for both chains to produce 15 blocks to expire timeout
	require.NoError(t, ibctest.WaitForBlocks(srcChain, dstChain, 15), "failed to wait for blocks")
}

func preRelayerStart_TimestampTimeout(testCase *RelayerTestCase, t *testing.T, ctx context.Context, srcChain ibc.Chain, dstChain ibc.Chain, channels []ibc.ChannelOutput) {
	ibcTimeoutTimestamp := ibc.IBCTimeout{NanoSeconds: uint64((10 * time.Second).Nanoseconds())}
	sendIBCTransfersFromBothChainsWithTimeout(testCase, t, ctx, srcChain, dstChain, channels, &ibcTimeoutTimestamp)
	// wait for 15 seconds to expire timeout
	time.Sleep(15 * time.Second)
}

// Ensure that a queued packet is successfully relayed.
func testPacketRelaySuccess(
	testCase *RelayerTestCase,
	t *testing.T,
	ctx context.Context,
	srcChain ibc.Chain,
	dstChain ibc.Chain,
	channels []ibc.ChannelOutput,
) {
	srcUser := testCase.SrcUser
	srcDenom := srcChain.Config().Denom

	dstUser := testCase.DstUser
	dstDenom := srcChain.Config().Denom

	// [BEGIN] assert on source to destination transfer
	// Assuming these values since the ibc transfers were sent in PreRelayerStart, so balances may have already changed by now
	srcInitialBalance := userFaucetFund
	dstInitialBalance := int64(0)

	// fetch src ibc transfer tx
	srcTxHash := testCase.Cache[0]
	srcTx, err := srcChain.GetTransaction(ctx, srcTxHash)
	require.NoError(t, err, "failed to get ibc transaction")

	// get ibc denom for src denom on dst chain
	srcDenomTrace := transfertypes.ParseDenomTrace(transfertypes.GetPrefixedDenom(channels[0].Counterparty.PortID, channels[0].Counterparty.ChannelID, srcDenom))
	dstIbcDenom := srcDenomTrace.IBCDenom()

	srcFinalBalance, err := srcChain.GetBalance(ctx, srcUser.NativeChainAddress, srcDenom)
	require.NoError(t, err, "failed to get balance from source chain")

	dstFinalBalance, err := dstChain.GetBalance(ctx, srcUser.CounterpartyChainAddress, dstIbcDenom)
	require.NoError(t, err, "failed to get balance from dest chain")

	totalFees := srcChain.GetGasFeesInNativeDenom(srcTx.GasWanted)
	expectedDifference := testCoinAmount + totalFees

	require.Equal(t, srcInitialBalance-expectedDifference, srcFinalBalance)
	require.Equal(t, dstInitialBalance+testCoinAmount, dstFinalBalance)
	// [END] assert on source to destination transfer

	// [BEGIN] assert on destination to source transfer
	// Assuming these values since the ibc transfers were sent in PreRelayerStart, so balances may have already changed by now
	srcInitialBalance = int64(0)
	dstInitialBalance = userFaucetFund
	// fetch src ibc transfer tx
	dstTxHash := testCase.Cache[1]
	dstTx, err := dstChain.GetTransaction(ctx, dstTxHash)
	require.NoError(t, err, "failed to get ibc transaction")

	// get ibc denom for dst denom on src chain
	dstDenomTrace := transfertypes.ParseDenomTrace(transfertypes.GetPrefixedDenom(channels[0].PortID, channels[0].ChannelID, dstDenom))
	srcIbcDenom := dstDenomTrace.IBCDenom()

	srcFinalBalance, err = srcChain.GetBalance(ctx, dstUser.CounterpartyChainAddress, srcIbcDenom)
	require.NoError(t, err, "failed to get balance from source chain")

	dstFinalBalance, err = dstChain.GetBalance(ctx, dstUser.NativeChainAddress, dstDenom)
	require.NoError(t, err, "failed to get balance from dest chain")

	totalFees = srcChain.GetGasFeesInNativeDenom(dstTx.GasWanted)
	expectedDifference = testCoinAmount + totalFees

	require.Equal(t, srcInitialBalance+expectedDifference, srcFinalBalance)
	require.Equal(t, dstInitialBalance-testCoinAmount, dstFinalBalance)
	// [END] assert on destination to source transfer
}

// Ensure that a queued packet that should not be relayed is not relayed.
func testPacketRelayFail(
	testCase *RelayerTestCase,
	t *testing.T,
	ctx context.Context,
	srcChain ibc.Chain,
	dstChain ibc.Chain,
	channels []ibc.ChannelOutput,
) {
	srcUser := testCase.SrcUser
	srcDenom := srcChain.Config().Denom

	dstUser := testCase.DstUser
	dstDenom := srcChain.Config().Denom

	// [BEGIN] assert on source to destination transfer
	// Assuming these values since the ibc transfers were sent in PreRelayerStart, so balances may have already changed by now
	srcInitialBalance := userFaucetFund
	dstInitialBalance := int64(0)

	// fetch src ibc transfer tx
	srcTxHash := testCase.Cache[0]
	srcTx, err := srcChain.GetTransaction(ctx, srcTxHash)
	require.NoError(t, err, "failed to get ibc transaction")

	// get ibc denom for src denom on dst chain
	srcDenomTrace := transfertypes.ParseDenomTrace(transfertypes.GetPrefixedDenom(channels[0].Counterparty.PortID, channels[0].Counterparty.ChannelID, srcDenom))
	dstIbcDenom := srcDenomTrace.IBCDenom()

	srcFinalBalance, err := srcChain.GetBalance(ctx, srcUser.NativeChainAddress, srcDenom)
	require.NoError(t, err, "failed to get balance from source chain")

	dstFinalBalance, err := dstChain.GetBalance(ctx, srcUser.CounterpartyChainAddress, dstIbcDenom)
	require.NoError(t, err, "failed to get balance from dest chain")

	totalFees := srcChain.GetGasFeesInNativeDenom(srcTx.GasWanted)

	require.Equal(t, srcInitialBalance-totalFees, srcFinalBalance)
	require.Equal(t, dstInitialBalance, dstFinalBalance)
	// [END] assert on source to destination transfer

	// [BEGIN] assert on destination to source transfer
	// Assuming these values since the ibc transfers were sent in PreRelayerStart, so balances may have already changed by now
	srcInitialBalance = int64(0)
	dstInitialBalance = userFaucetFund
	// fetch src ibc transfer tx
	dstTxHash := testCase.Cache[1]
	dstTx, err := dstChain.GetTransaction(ctx, dstTxHash)
	require.NoError(t, err, "failed to get ibc transaction")

	// get ibc denom for dst denom on src chain
	dstDenomTrace := transfertypes.ParseDenomTrace(transfertypes.GetPrefixedDenom(channels[0].PortID, channels[0].ChannelID, dstDenom))
	srcIbcDenom := dstDenomTrace.IBCDenom()

	srcFinalBalance, err = srcChain.GetBalance(ctx, dstUser.CounterpartyChainAddress, srcIbcDenom)
	require.NoError(t, err, "failed to get balance from source chain")

	dstFinalBalance, err = dstChain.GetBalance(ctx, dstUser.NativeChainAddress, dstDenom)
	require.NoError(t, err, "failed to get balance from dest chain")

	totalFees = srcChain.GetGasFeesInNativeDenom(dstTx.GasWanted)

	require.Equal(t, srcInitialBalance, srcFinalBalance)
	require.Equal(t, dstInitialBalance-totalFees, dstFinalBalance)
	// [END] assert on destination to source transfer
}
