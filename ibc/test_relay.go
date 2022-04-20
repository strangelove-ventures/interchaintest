package ibc

import (
	"fmt"
	"time"

	transfertypes "github.com/cosmos/ibc-go/v3/modules/apps/transfer/types"
)

// makes sure that a queued packet that is timed out (relative height timeout) will not be relayed
func (ibc IBCTestCase) RelayPacketTestHeightTimeout(testName string, cf ChainFactory, relayerImplementation RelayerImplementation) error {
	ctx, home, pool, network, cleanup, err := SetupTestRun(testName)
	if err != nil {
		return err
	}
	defer cleanup()

	srcChain, dstChain, err := cf.Pair(testName)
	if err != nil {
		return fmt.Errorf("failed to get chain pair: %w", err)
	}

	var srcInitialBalance, dstInitialBalance int64
	var txHash string
	testDenom := srcChain.Config().Denom
	var dstIbcDenom string

	// Query user account balances on both chains and send IBC transfer before starting the relayer
	preRelayerStart := func(channels []ChannelOutput, srcUser User, dstUser User) error {
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

		fmt.Printf("Initial balances: Src chain: %d\nDst chain: %d\n", srcInitialBalance, dstInitialBalance)

		testCoin := WalletAmount{
			Address: srcUser.DstChainAddress,
			Denom:   testDenom,
			Amount:  1000000,
		}

		// send ibc transfer with a timeout of 10 blocks from now on counterparty chain
		txHash, err = srcChain.SendIBCTransfer(ctx, channels[0].ChannelID, srcUser.KeyName, testCoin, &IBCTimeout{Height: 10})
		if err != nil {
			return err
		}

		// wait until counterparty chain has passed the timeout
		_, err = dstChain.WaitForBlocks(11)
		return err
	}

	// Startup both chains and relayer
	_, _, user, _, rlyCleanup, err := StartChainsAndRelayer(testName, ctx, pool, network, home, srcChain, dstChain, relayerImplementation, preRelayerStart)
	if err != nil {
		return err
	}
	defer rlyCleanup()

	// wait for both chains to produce 10 blocks
	if err := WaitForBlocks(srcChain, dstChain, 10); err != nil {
		return err
	}

	// fetch ibc transfer tx
	srcTx, err := srcChain.GetTransaction(ctx, txHash)
	if err != nil {
		return err
	}

	fmt.Printf("Transaction:\n%v\n", srcTx)

	srcFinalBalance, err := srcChain.GetBalance(ctx, user.SrcChainAddress, testDenom)
	if err != nil {
		return err
	}

	dstFinalBalance, err := dstChain.GetBalance(ctx, user.DstChainAddress, dstIbcDenom)
	if err != nil {
		return err
	}

	totalFees := srcChain.GetGasFeesInNativeDenom(srcTx.GasWanted)

	if srcFinalBalance != srcInitialBalance-totalFees {
		return fmt.Errorf("source balances do not match. expected: %d, actual: %d", srcInitialBalance-totalFees, srcFinalBalance)
	}

	if dstFinalBalance != dstInitialBalance {
		return fmt.Errorf("destination balances do not match. expected: %d, actual: %d", dstInitialBalance, dstFinalBalance)
	}

	return nil
}

// makes sure that a queued packet that is timed out (nanoseconds timeout) will not be relayed
func (ibc IBCTestCase) RelayPacketTestTimestampTimeout(testName string, cf ChainFactory, relayerImplementation RelayerImplementation) error {
	ctx, home, pool, network, cleanup, err := SetupTestRun(testName)
	if err != nil {
		return err
	}
	defer cleanup()

	srcChain, dstChain, err := cf.Pair(testName)
	if err != nil {
		return fmt.Errorf("failed to get chain pair: %w", err)
	}

	var srcInitialBalance, dstInitialBalance int64
	var txHash string

	testDenom := srcChain.Config().Denom
	var dstIbcDenom string

	// Query user account balances on both chains and send IBC transfer before starting the relayer
	preRelayerStart := func(channels []ChannelOutput, srcUser User, dstUser User) error {
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

		fmt.Printf("Initial balances: Src chain: %d\nDst chain: %d\n", srcInitialBalance, dstInitialBalance)

		testCoin := WalletAmount{
			Address: srcUser.DstChainAddress,
			Denom:   testDenom,
			Amount:  1000000,
		}

		// send ibc transfer with a timeout of 10 blocks from now on counterparty chain
		txHash, err = srcChain.SendIBCTransfer(ctx, channels[0].ChannelID, srcUser.KeyName, testCoin, &IBCTimeout{NanoSeconds: uint64((10 * time.Second).Nanoseconds())})
		if err != nil {
			return err
		}

		// wait until ibc transfer times out
		time.Sleep(15 * time.Second)

		return nil
	}

	// Startup both chains and relayer
	_, _, user, _, rlyCleanup, err := StartChainsAndRelayer(testName, ctx, pool, network, home, srcChain, dstChain, relayerImplementation, preRelayerStart)
	if err != nil {
		return err
	}
	defer rlyCleanup()

	// wait for both chains to produce 10 blocks
	if err := WaitForBlocks(srcChain, dstChain, 10); err != nil {
		return err
	}

	// fetch ibc transfer tx
	srcTx, err := srcChain.GetTransaction(ctx, txHash)
	if err != nil {
		return err
	}

	fmt.Printf("Transaction:\n%v\n", srcTx)

	srcFinalBalance, err := srcChain.GetBalance(ctx, user.SrcChainAddress, testDenom)
	if err != nil {
		return err
	}

	dstFinalBalance, err := dstChain.GetBalance(ctx, user.DstChainAddress, dstIbcDenom)
	if err != nil {
		return err
	}

	totalFees := srcChain.GetGasFeesInNativeDenom(srcTx.GasWanted)

	if srcFinalBalance != srcInitialBalance-totalFees {
		return fmt.Errorf("source balances do not match. expected: %d, actual: %d", srcInitialBalance-totalFees, srcFinalBalance)
	}

	if dstFinalBalance != dstInitialBalance {
		return fmt.Errorf("destination balances do not match. expected: %d, actual: %d", dstInitialBalance, dstFinalBalance)
	}

	return nil
}
