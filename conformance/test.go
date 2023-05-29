// Package conformance exposes a Test function
// that can be imported into other packages' tests.
//
// The exported Test function is intended to be a stable API
// that calls other Test* functions, which are not guaranteed
// to remain a stable API.
//
// External packages that intend to run IBC tests against their relayer implementation
// should define their own implementation of ibc.RelayerFactory,
// and in most cases should use an instance of ibc.BuiltinChainFactory.
//
//	package myrelayer_test
//
//	import (
//	  "testing"
//
//	  "github.com/strangelove-ventures/interchaintest/v7/conformance"
//	  "github.com/strangelove-ventures/interchaintest/v7/ibc"
//	)
//
//	func TestMyRelayer(t *testing.T) {
//	  conformance.Test(t, ibc.NewBuiltinChainFactory([]ibc.BuiltinChainFactoryEntry{
//	    {Name: "foo_bar" /* ... */},
//	  }, MyRelayerFactory(), getTestReporter())
//	}
//
// Although the conformance package is made available as a convenience for other projects,
// the interchaintest project should be considered the canonical definition of tests and configuration.
package conformance

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	transfertypes "github.com/cosmos/ibc-go/v7/modules/apps/transfer/types"
	"github.com/docker/docker/client"
	interchaintest "github.com/strangelove-ventures/interchaintest/v7"
	"github.com/strangelove-ventures/interchaintest/v7/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v7/ibc"
	"github.com/strangelove-ventures/interchaintest/v7/internal/dockerutil"
	"github.com/strangelove-ventures/interchaintest/v7/relayer"
	"github.com/strangelove-ventures/interchaintest/v7/testreporter"
	"github.com/strangelove-ventures/interchaintest/v7/testutil"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
)

const (
	userFaucetFund = int64(10_000_000_000)
	testCoinAmount = int64(1_000_000)
	pollHeightMax  = uint64(50)
)

type TxCache struct {
	Src []ibc.Tx
	Dst []ibc.Tx
}

type RelayerTestCase struct {
	Config RelayerTestCaseConfig
	// user on source chain
	Users []ibc.Wallet
	// temp storage in between test phases
	TxCache TxCache
}

type RelayerTestCaseConfig struct {
	Name string
	// which relayer capabilities are required to run this test
	RequiredRelayerCapabilities []relayer.Capability
	// function to run after the chains are started but before the relayer is started
	// e.g. send a transfer and wait for it to timeout so that the relayer will handle it once it is timed out
	PreRelayerStart func(context.Context, *testing.T, *RelayerTestCase, ibc.Chain, ibc.Chain, []ibc.ChannelOutput)
	// test after chains and relayers are started
	Test func(context.Context, *testing.T, *RelayerTestCase, *testreporter.Reporter, ibc.Chain, ibc.Chain, []ibc.ChannelOutput)
}

var relayerTestCaseConfigs = [...]RelayerTestCaseConfig{
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

// requireCapabilities tracks skipping t, if the relayer factory cannot satisfy the required capabilities.
func requireCapabilities(t *testing.T, rep *testreporter.Reporter, rf interchaintest.RelayerFactory, reqCaps ...relayer.Capability) {
	t.Helper()

	missing := missingCapabilities(rf, reqCaps...)

	if len(missing) > 0 {
		rep.TrackSkip(t, "skipping due to missing relayer capabilities +%s", missing)
	}
}

func missingCapabilities(rf interchaintest.RelayerFactory, reqCaps ...relayer.Capability) []relayer.Capability {
	caps := rf.Capabilities()
	var missing []relayer.Capability
	for _, c := range reqCaps {
		if !caps[c] {
			missing = append(missing, c)
		}
	}
	return missing
}

func sendIBCTransfersFromBothChainsWithTimeout(
	ctx context.Context,
	t *testing.T,
	testCase *RelayerTestCase,
	srcChain ibc.Chain,
	dstChain ibc.Chain,
	channels []ibc.ChannelOutput,
	timeout *ibc.IBCTimeout,
) {
	srcChainCfg := srcChain.Config()
	srcUser := testCase.Users[0]

	dstChainCfg := dstChain.Config()
	dstUser := testCase.Users[1]

	// will send ibc transfers from user wallet on both chains to their own respective wallet on the other chain

	testCoinSrcToDst := ibc.WalletAmount{
		Address: srcUser.(*cosmos.CosmosWallet).FormattedAddressWithPrefix(dstChainCfg.Bech32Prefix),
		Denom:   srcChainCfg.Denom,
		Amount:  testCoinAmount,
	}
	testCoinDstToSrc := ibc.WalletAmount{
		Address: dstUser.(*cosmos.CosmosWallet).FormattedAddressWithPrefix(srcChainCfg.Bech32Prefix),
		Denom:   dstChainCfg.Denom,
		Amount:  testCoinAmount,
	}

	var eg errgroup.Group
	srcTxs := make([]ibc.Tx, len(channels))
	dstTxs := make([]ibc.Tx, len(channels))

	eg.Go(func() (err error) {
		for i, channel := range channels {
			srcChannelID := channel.ChannelID
			srcTxs[i], err = srcChain.SendIBCTransfer(ctx, srcChannelID, srcUser.KeyName(), testCoinSrcToDst, ibc.TransferOptions{Timeout: timeout})
			if err != nil {
				return fmt.Errorf("failed to send ibc transfer from source: %w", err)
			}
			if err := testutil.WaitForBlocks(ctx, 1, srcChain); err != nil {
				return err
			}
		}
		return nil
	})

	eg.Go(func() (err error) {
		for i, channel := range channels {
			dstChannelID := channel.Counterparty.ChannelID
			dstTxs[i], err = dstChain.SendIBCTransfer(ctx, dstChannelID, dstUser.KeyName(), testCoinDstToSrc, ibc.TransferOptions{Timeout: timeout})
			if err != nil {
				return fmt.Errorf("failed to send ibc transfer from destination: %w", err)
			}
			if err := testutil.WaitForBlocks(ctx, 1, dstChain); err != nil {
				return err
			}
		}
		return nil
	})

	require.NoError(t, eg.Wait())
	for _, srcTx := range srcTxs {
		require.NoError(t, srcTx.Validate(), "source ibc transfer tx is invalid")
	}
	for _, dstTx := range dstTxs {
		require.NoError(t, dstTx.Validate(), "destination ibc transfer tx is invalid")
	}

	testCase.TxCache = TxCache{
		Src: srcTxs,
		Dst: dstTxs,
	}
}

// Test is the stable API exposed by the conformance package.
// This is intended to be used by Go unit tests.
//
// This function accepts the full set of chain factories and relayer factories to use,
// so that it can properly group subtests in a single invocation.
// If the subtest configuration does not meet your needs,
// you can directly call one of the other exported Test functions, such as TestChainPair.
func Test(t *testing.T, ctx context.Context, cfs []interchaintest.ChainFactory, rfs []interchaintest.RelayerFactory, rep *testreporter.Reporter) {
	// Validate chain factory counts up front.
	counts := make(map[int]bool)
	for _, cf := range cfs {
		switch count := cf.Count(); count {
		case 2:
			counts[count] = true
		default:
			panic(fmt.Errorf("cannot accept chain factory with count=%d", cf.Count()))
		}
	}

	// Any chain pairs present?
	if counts[2] {
		t.Run("chain pairs", func(t *testing.T) {
			for _, cf := range cfs {
				cf := cf
				if cf.Count() != 2 {
					continue
				}

				t.Run(cf.Name(), func(t *testing.T) {
					for _, rf := range rfs {
						rf := rf

						t.Run(rf.Name(), func(t *testing.T) {
							// Record the labels for this nested test.
							rep.TrackTest(t)
							rep.TrackParallel(t)

							t.Run("relayer setup", func(t *testing.T) {
								rep.TrackTest(t)
								rep.TrackParallel(t)

								TestRelayerSetup(t, ctx, cf, rf, rep)
							})

							t.Run("conformance", func(t *testing.T) {
								rep.TrackTest(t)
								rep.TrackParallel(t)

								chains, err := cf.Chains(t.Name())
								if err != nil {
									panic(fmt.Errorf("failed to get chains: %v", err))
								}

								client, network := interchaintest.DockerSetup(t)
								TestChainPair(t, ctx, client, network, chains[0], chains[1], rf, rep, nil)
							})

							t.Run("flushing", func(t *testing.T) {
								rep.TrackTest(t)
								rep.TrackParallel(t)

								TestRelayerFlushing(t, ctx, cf, rf, rep)
							})
						})
					}
				})
			}
		})
	}
}

// TestChainPair runs the conformance tests for two chains and one relayer.
// This test asserts bidirectional behavior between both chains.
//
// Given 2 chains, Chain A and Chain B, this test asserts:
// 1. Successful IBC transfer from A -> B and B -> A.
// 2. Proper handling of no timeout from A -> B and B -> A.
// 3. Proper handling of height timeout from A -> B and B -> A.
// 4. Proper handling of timestamp timeout from A -> B and B -> A.
// If a non-nil relayerImpl is passed, it is assumed that the chains are already started.
func TestChainPair(
	t *testing.T,
	ctx context.Context,
	client *client.Client,
	network string,
	srcChain, dstChain ibc.Chain,
	rf interchaintest.RelayerFactory,
	rep *testreporter.Reporter,
	relayerImpl ibc.Relayer,
	pathNames ...string,
) {
	req := require.New(rep.TestifyT(t))

	var (
		preRelayerStartFuncs []func([]ibc.ChannelOutput)
		testCases            []*RelayerTestCase
		err                  error
	)

	randomSuffix := dockerutil.RandLowerCaseLetterString(4)

	for _, testCaseConfig := range relayerTestCaseConfigs {
		testCase := RelayerTestCase{
			Config: testCaseConfig,
		}
		testCases = append(testCases, &testCase)

		if len(missingCapabilities(rf, testCaseConfig.RequiredRelayerCapabilities...)) > 0 {
			// Do not add preRelayerStartFunc if capability missing.
			// Adding all preRelayerStartFuncs appears to cause test pollution which is why this step is necessary.
			continue
		}
		preRelayerStartFunc := func(channels []ibc.ChannelOutput) {
			// fund a user wallet on both chains, save on test case
			testCase.Users = interchaintest.GetAndFundTestUsers(t, ctx, strings.ReplaceAll(testCase.Config.Name, " ", "-")+"-"+randomSuffix, userFaucetFund, srcChain, dstChain)
			// run test specific pre relayer start action
			testCase.Config.PreRelayerStart(ctx, t, &testCase, srcChain, dstChain, channels)
		}
		preRelayerStartFuncs = append(preRelayerStartFuncs, preRelayerStartFunc)
	}

	if relayerImpl == nil {
		t.Logf("creating relayer: %s", rf.Name())
		// startup both chains.
		// creates wallets in the relayer for src and dst chain.
		// funds relayer src and dst wallets on respective chain in genesis.
		// creates a faucet account on the both chains (separate fullnode).
		// funds faucet accounts in genesis.
		relayerImpl, err = interchaintest.StartChainPair(t, ctx, rep, client, network, srcChain, dstChain, rf, preRelayerStartFuncs)
		req.NoError(err, "failed to StartChainPair")
	}

	// execute the pre relayer start functions, then start the relayer.
	channels, err := interchaintest.StopStartRelayerWithPreStartFuncs(
		t,
		ctx,
		srcChain.Config().ChainID,
		relayerImpl,
		rep.RelayerExecReporter(t),
		preRelayerStartFuncs,
		pathNames...,
	)
	req.NoError(err, "failed to StopStartRelayerWithPreStartFuncs")

	t.Run("post_relayer_start", func(t *testing.T) {
		for _, testCase := range testCases {
			testCase := testCase
			t.Run(testCase.Config.Name, func(t *testing.T) {
				rep.TrackTest(t)
				requireCapabilities(t, rep, rf, testCase.Config.RequiredRelayerCapabilities...)
				rep.TrackParallel(t)
				testCase.Config.Test(ctx, t, testCase, rep, srcChain, dstChain, channels)
			})
		}
	})
}

// PreRelayerStart methods for the RelayerTestCases

func preRelayerStart_RelayPacket(ctx context.Context, t *testing.T, testCase *RelayerTestCase, srcChain ibc.Chain, dstChain ibc.Chain, channels []ibc.ChannelOutput) {
	sendIBCTransfersFromBothChainsWithTimeout(ctx, t, testCase, srcChain, dstChain, channels, nil)
}

func preRelayerStart_NoTimeout(ctx context.Context, t *testing.T, testCase *RelayerTestCase, srcChain ibc.Chain, dstChain ibc.Chain, channels []ibc.ChannelOutput) {
	ibcTimeoutDisabled := ibc.IBCTimeout{Height: 0, NanoSeconds: 0}
	sendIBCTransfersFromBothChainsWithTimeout(ctx, t, testCase, srcChain, dstChain, channels, &ibcTimeoutDisabled)
	// TODO should we wait here to make sure it successfully relays a packet beyond the default timeout period?
	// would need to shorten the chain default timeouts somehow to make that a feasible test
}

func preRelayerStart_HeightTimeout(ctx context.Context, t *testing.T, testCase *RelayerTestCase, srcChain ibc.Chain, dstChain ibc.Chain, channels []ibc.ChannelOutput) {
	ibcTimeoutHeight := ibc.IBCTimeout{Height: 10}
	sendIBCTransfersFromBothChainsWithTimeout(ctx, t, testCase, srcChain, dstChain, channels, &ibcTimeoutHeight)
	// wait for both chains to produce 15 blocks to expire timeout
	require.NoError(t, testutil.WaitForBlocks(ctx, 15, srcChain, dstChain), "failed to wait for blocks")
}

func preRelayerStart_TimestampTimeout(ctx context.Context, t *testing.T, testCase *RelayerTestCase, srcChain ibc.Chain, dstChain ibc.Chain, channels []ibc.ChannelOutput) {
	ibcTimeoutTimestamp := ibc.IBCTimeout{NanoSeconds: uint64((1 * time.Second).Nanoseconds())}
	sendIBCTransfersFromBothChainsWithTimeout(ctx, t, testCase, srcChain, dstChain, channels, &ibcTimeoutTimestamp)
	// wait for 15 seconds to expire timeout
	time.Sleep(15 * time.Second)
}

// Ensure that a queued packet is successfully relayed.
func testPacketRelaySuccess(
	ctx context.Context,
	t *testing.T,
	testCase *RelayerTestCase,
	rep *testreporter.Reporter,
	srcChain ibc.Chain,
	dstChain ibc.Chain,
	channels []ibc.ChannelOutput,
) {
	req := require.New(rep.TestifyT(t))

	srcChainCfg := srcChain.Config()
	srcUser := testCase.Users[0]
	srcDenom := srcChainCfg.Denom

	dstChainCfg := dstChain.Config()

	// [BEGIN] assert on source to destination transfer
	for i, srcTx := range testCase.TxCache.Src {
		t.Logf("Asserting %s to %s transfer", srcChainCfg.ChainID, dstChainCfg.ChainID)
		// Assuming these values since the ibc transfers were sent in PreRelayerStart, so balances may have already changed by now
		srcInitialBalance := userFaucetFund
		dstInitialBalance := int64(0)

		srcAck, err := testutil.PollForAck(ctx, srcChain, srcTx.Height, srcTx.Height+pollHeightMax, srcTx.Packet)
		req.NoError(err, "failed to get acknowledgement on source chain")
		req.NoError(srcAck.Validate(), "invalid acknowledgement on source chain")

		// get ibc denom for src denom on dst chain
		srcDenomTrace := transfertypes.ParseDenomTrace(transfertypes.GetPrefixedDenom(channels[i].Counterparty.PortID, channels[i].Counterparty.ChannelID, srcDenom))
		dstIbcDenom := srcDenomTrace.IBCDenom()

		srcFinalBalance, err := srcChain.GetBalance(ctx, srcUser.(*cosmos.CosmosWallet).FormattedAddressWithPrefix(srcChainCfg.Bech32Prefix), srcDenom)
		req.NoError(err, "failed to get balance from source chain")

		dstFinalBalance, err := dstChain.GetBalance(ctx, srcUser.(*cosmos.CosmosWallet).FormattedAddressWithPrefix(dstChainCfg.Bech32Prefix), dstIbcDenom)
		req.NoError(err, "failed to get balance from dest chain")

		totalFees := srcChain.GetGasFeesInNativeDenom(srcTx.GasSpent)
		expectedDifference := testCoinAmount + totalFees

		req.Equal(srcInitialBalance-expectedDifference, srcFinalBalance)
		req.Equal(dstInitialBalance+testCoinAmount, dstFinalBalance)
	}

	// [END] assert on source to destination transfer

	// [BEGIN] assert on destination to source transfer
	for i, dstTx := range testCase.TxCache.Dst {
		t.Logf("Asserting %s to %s transfer", dstChainCfg.ChainID, srcChainCfg.ChainID)
		dstUser := testCase.Users[1]
		dstDenom := dstChainCfg.Denom
		// Assuming these values since the ibc transfers were sent in PreRelayerStart, so balances may have already changed by now
		srcInitialBalance := int64(0)
		dstInitialBalance := userFaucetFund

		dstAck, err := testutil.PollForAck(ctx, dstChain, dstTx.Height, dstTx.Height+pollHeightMax, dstTx.Packet)
		req.NoError(err, "failed to get acknowledgement on destination chain")
		req.NoError(dstAck.Validate(), "invalid acknowledgement on destination chain")

		// get ibc denom for dst denom on src chain
		dstDenomTrace := transfertypes.ParseDenomTrace(transfertypes.GetPrefixedDenom(channels[i].PortID, channels[i].ChannelID, dstDenom))
		srcIbcDenom := dstDenomTrace.IBCDenom()

		srcFinalBalance, err := srcChain.GetBalance(ctx, dstUser.(*cosmos.CosmosWallet).FormattedAddressWithPrefix(srcChainCfg.Bech32Prefix), srcIbcDenom)
		req.NoError(err, "failed to get balance from source chain")

		dstFinalBalance, err := dstChain.GetBalance(ctx, dstUser.(*cosmos.CosmosWallet).FormattedAddressWithPrefix(dstChainCfg.Bech32Prefix), dstDenom)
		req.NoError(err, "failed to get balance from dest chain")

		totalFees := dstChain.GetGasFeesInNativeDenom(dstTx.GasSpent)
		expectedDifference := testCoinAmount + totalFees

		req.Equal(srcInitialBalance+testCoinAmount, srcFinalBalance)
		req.Equal(dstInitialBalance-expectedDifference, dstFinalBalance)
	}
	//[END] assert on destination to source transfer
}

// Ensure that a queued packet that should not be relayed is not relayed.
func testPacketRelayFail(
	ctx context.Context,
	t *testing.T,
	testCase *RelayerTestCase,
	rep *testreporter.Reporter,
	srcChain ibc.Chain,
	dstChain ibc.Chain,
	channels []ibc.ChannelOutput,
) {
	req := require.New(rep.TestifyT(t))

	srcChainCfg := srcChain.Config()
	srcUser := testCase.Users[0]
	srcDenom := srcChainCfg.Denom

	dstChainCfg := dstChain.Config()
	dstUser := testCase.Users[1]
	dstDenom := dstChainCfg.Denom

	// [BEGIN] assert on source to destination transfer
	for i, srcTx := range testCase.TxCache.Src {
		// Assuming these values since the ibc transfers were sent in PreRelayerStart, so balances may have already changed by now
		srcInitialBalance := userFaucetFund
		dstInitialBalance := int64(0)

		timeout, err := testutil.PollForTimeout(ctx, srcChain, srcTx.Height, srcTx.Height+pollHeightMax, srcTx.Packet)
		req.NoError(err, "failed to get timeout packet on source chain")
		req.NoError(timeout.Validate(), "invalid timeout packet on source chain")

		// Even though we poll for the timeout, there may be timing issues where balances are not fully reconciled yet.
		// So we have a small buffer here.
		require.NoError(t, testutil.WaitForBlocks(ctx, 2, srcChain, dstChain))

		// get ibc denom for src denom on dst chain
		srcDenomTrace := transfertypes.ParseDenomTrace(transfertypes.GetPrefixedDenom(channels[i].Counterparty.PortID, channels[i].Counterparty.ChannelID, srcDenom))
		dstIbcDenom := srcDenomTrace.IBCDenom()

		srcFinalBalance, err := srcChain.GetBalance(ctx, srcUser.(*cosmos.CosmosWallet).FormattedAddressWithPrefix(srcChainCfg.Bech32Prefix), srcDenom)
		req.NoError(err, "failed to get balance from source chain")

		dstFinalBalance, err := dstChain.GetBalance(ctx, srcUser.(*cosmos.CosmosWallet).FormattedAddressWithPrefix(dstChainCfg.Bech32Prefix), dstIbcDenom)
		req.NoError(err, "failed to get balance from destination chain")

		totalFees := srcChain.GetGasFeesInNativeDenom(srcTx.GasSpent)

		req.Equal(srcInitialBalance-totalFees, srcFinalBalance)
		req.Equal(dstInitialBalance, dstFinalBalance)
	}
	// [END] assert on source to destination transfer

	// [BEGIN] assert on destination to source transfer
	for i, dstTx := range testCase.TxCache.Dst {
		// Assuming these values since the ibc transfers were sent in PreRelayerStart, so balances may have already changed by now
		srcInitialBalance := int64(0)
		dstInitialBalance := userFaucetFund

		timeout, err := testutil.PollForTimeout(ctx, dstChain, dstTx.Height, dstTx.Height+pollHeightMax, dstTx.Packet)
		req.NoError(err, "failed to get timeout packet on destination chain")
		req.NoError(timeout.Validate(), "invalid timeout packet on destination chain")

		// get ibc denom for dst denom on src chain
		dstDenomTrace := transfertypes.ParseDenomTrace(transfertypes.GetPrefixedDenom(channels[i].PortID, channels[i].ChannelID, dstDenom))
		srcIbcDenom := dstDenomTrace.IBCDenom()

		srcFinalBalance, err := srcChain.GetBalance(ctx, dstUser.(*cosmos.CosmosWallet).FormattedAddressWithPrefix(srcChainCfg.Bech32Prefix), srcIbcDenom)
		req.NoError(err, "failed to get balance from source chain")

		dstFinalBalance, err := dstChain.GetBalance(ctx, dstUser.(*cosmos.CosmosWallet).FormattedAddressWithPrefix(dstChainCfg.Bech32Prefix), dstDenom)
		req.NoError(err, "failed to get balance from destination chain")

		totalFees := dstChain.GetGasFeesInNativeDenom(dstTx.GasSpent)

		req.Equal(srcInitialBalance, srcFinalBalance)
		req.Equal(dstInitialBalance-totalFees, dstFinalBalance)
	}
	// [END] assert on destination to source transfer
}
