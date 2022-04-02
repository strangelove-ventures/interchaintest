package test

import (
	"fmt"
	"testing"
	"time"

	transfertypes "github.com/cosmos/ibc-go/v3/modules/apps/transfer/types"
	"github.com/strangelove-ventures/ibc-test-framework/ibc"
	"github.com/stretchr/testify/require"
)

func TestRelayPacket(t *testing.T) {
	numValidatorsPerChain := 4
	numFullNodesPerChain := 1

	ctx, home, pool, network := ibc.SetupTestRun(t)

	// TODO make chain configuration an input
	srcChain := ibc.GaiaChain(t, pool, home, network.ID, numValidatorsPerChain, numFullNodesPerChain)
	dstChain := ibc.OsmosisChain(t, pool, home, network.ID, numValidatorsPerChain, numFullNodesPerChain)

	// startup both chains and relayer
	// creates wallets in the relayer for src and dst chain
	// funds relayer src and dst wallets on respective chain in genesis
	// creates a user account on the src chain (separate fullnode)
	// funds user account on src chain in genesis
	channels, user := ibc.StartChainsAndRelayer(t, ctx, pool, network.ID, home, srcChain, dstChain, ibc.CosmosRly, nil)

	// will test a user sending an ibc transfer from the src chain to the dst chain
	// denom will be src chain native denom
	testDenom := srcChain.Config().Denom

	// query initial balance of user wallet for src chain native denom on the src chain
	srcInitialBalance, err := srcChain.GetBalance(ctx, user.SrcChainAddress, testDenom)
	require.NoError(t, err)

	// get ibc denom for test denom on dst chain
	denomTrace := transfertypes.ParseDenomTrace(transfertypes.GetPrefixedDenom(channels[0].Counterparty.PortID, channels[0].Counterparty.ChannelID, testDenom))
	dstIbcDenom := denomTrace.IBCDenom()

	// query initial balance of user wallet for src chain native denom on the dst chain
	// don't care about error here, account does not exist on destination chain
	dstInitialBalance, _ := dstChain.GetBalance(ctx, user.DstChainAddress, dstIbcDenom)

	fmt.Printf("Initial balances: Src chain: %d\nDst chain: %d\n", srcInitialBalance, dstInitialBalance)

	// test coin, address is recipient of ibc transfer on dst chain
	testCoin := ibc.WalletAmount{
		Address: user.DstChainAddress,
		Denom:   testDenom,
		Amount:  1000000,
	}

	// send ibc transfer from the user wallet using its fullnode
	// timeout is nil so that it will use the default timeout
	txHash, err := srcChain.SendIBCTransfer(ctx, channels[0].ChannelID, user.KeyName, testCoin, nil)
	require.NoError(t, err)

	// wait for both chains to produce 10 blocks
	ibc.WaitForBlocks(srcChain, dstChain, 10)

	// fetch ibc transfer tx
	srcTx, err := srcChain.GetTransaction(ctx, txHash)
	require.NoError(t, err)

	fmt.Printf("Transaction:\n%v\n", srcTx)

	// query final balance of user wallet for src chain native denom on the src chain
	srcFinalBalance, err := srcChain.GetBalance(ctx, user.SrcChainAddress, testDenom)
	require.NoError(t, err)

	// query final balance of user wallet for src chain native denom on the dst chain
	dstFinalBalance, err := dstChain.GetBalance(ctx, user.DstChainAddress, dstIbcDenom)
	require.NoError(t, err)

	fmt.Printf("Src chain final balance: %v\nDst chain final balance: %v\n", srcFinalBalance, dstFinalBalance)

	require.Equal(t, srcFinalBalance, srcInitialBalance-testCoin.Amount)
	require.Equal(t, dstFinalBalance, dstInitialBalance+testCoin.Amount)
}

// makes sure that a queued packet that is timed out (relative height timeout) will not be relayed
func TestRelayNoTimeout(t *testing.T) {
	numValidatorsPerChain := 4
	numFullNodesPerChain := 1

	ctx, home, pool, network := ibc.SetupTestRun(t)

	// TODO make chain configuration an input
	srcChain := ibc.GaiaChain(t, pool, home, network.ID, numValidatorsPerChain, numFullNodesPerChain)
	dstChain := ibc.OsmosisChain(t, pool, home, network.ID, numValidatorsPerChain, numFullNodesPerChain)

	var srcInitialBalance, dstInitialBalance int64
	var txHash string
	testDenom := srcChain.Config().Denom
	var dstIbcDenom string
	var testCoin ibc.WalletAmount

	// Query user account balances on both chains and send IBC transfer before starting the relayer
	preRelayerStart := func(channels []ibc.ChannelOutput, user ibc.User) {
		var err error
		srcInitialBalance, err = srcChain.GetBalance(ctx, user.SrcChainAddress, testDenom)
		require.NoError(t, err)

		// get ibc denom for test denom on dst chain
		denomTrace := transfertypes.ParseDenomTrace(transfertypes.GetPrefixedDenom(channels[0].Counterparty.PortID, channels[0].Counterparty.ChannelID, testDenom))
		dstIbcDenom = denomTrace.IBCDenom()

		// don't care about error here, account does not exist on destination chain
		dstInitialBalance, _ = dstChain.GetBalance(ctx, user.DstChainAddress, dstIbcDenom)

		fmt.Printf("Initial balances: Src chain: %d\nDst chain: %d\n", srcInitialBalance, dstInitialBalance)

		testCoin = ibc.WalletAmount{
			Address: user.DstChainAddress,
			Denom:   testDenom,
			Amount:  1000000,
		}

		// send ibc transfer with both timeouts disabled
		txHash, err = srcChain.SendIBCTransfer(ctx, channels[0].ChannelID, user.KeyName, testCoin, &ibc.IBCTimeout{Height: 0, NanoSeconds: 0})
		require.NoError(t, err)
	}

	// Startup both chains and relayer
	_, user := ibc.StartChainsAndRelayer(t, ctx, pool, network.ID, home, srcChain, dstChain, ibc.CosmosRly, preRelayerStart)

	// wait for both chains to produce 10 blocks
	ibc.WaitForBlocks(srcChain, dstChain, 10)

	// fetch ibc transfer tx
	srcTx, err := srcChain.GetTransaction(ctx, txHash)
	require.NoError(t, err)

	fmt.Printf("Transaction:\n%v\n", srcTx)

	srcFinalBalance, err := srcChain.GetBalance(ctx, user.SrcChainAddress, testDenom)
	require.NoError(t, err)

	dstFinalBalance, err := dstChain.GetBalance(ctx, user.DstChainAddress, dstIbcDenom)
	require.NoError(t, err)

	fmt.Printf("Src chain final balance: %v\nDst chain final balance: %v\n", srcFinalBalance, dstFinalBalance)

	require.Equal(t, srcFinalBalance, srcInitialBalance-testCoin.Amount)
	require.Equal(t, dstFinalBalance, dstInitialBalance+testCoin.Amount)
}

// makes sure that a queued packet that is timed out (relative height timeout) will not be relayed
func TestRelayTimeoutH(t *testing.T) {
	numValidatorsPerChain := 4
	numFullNodesPerChain := 1

	ctx, home, pool, network := ibc.SetupTestRun(t)

	// TODO make chain configuration an input
	srcChain := ibc.GaiaChain(t, pool, home, network.ID, numValidatorsPerChain, numFullNodesPerChain)
	dstChain := ibc.OsmosisChain(t, pool, home, network.ID, numValidatorsPerChain, numFullNodesPerChain)

	var srcInitialBalance, dstInitialBalance int64
	var txHash string
	testDenom := srcChain.Config().Denom
	var dstIbcDenom string

	// Query user account balances on both chains and send IBC transfer before starting the relayer
	preRelayerStart := func(channels []ibc.ChannelOutput, user ibc.User) {
		var err error
		srcInitialBalance, err = srcChain.GetBalance(ctx, user.SrcChainAddress, testDenom)
		require.NoError(t, err)

		// get ibc denom for test denom on dst chain
		denomTrace := transfertypes.ParseDenomTrace(transfertypes.GetPrefixedDenom(channels[0].Counterparty.PortID, channels[0].Counterparty.ChannelID, testDenom))
		dstIbcDenom = denomTrace.IBCDenom()

		// don't care about error here, account does not exist on destination chain
		dstInitialBalance, _ = dstChain.GetBalance(ctx, user.DstChainAddress, dstIbcDenom)

		fmt.Printf("Initial balances: Src chain: %d\nDst chain: %d\n", srcInitialBalance, dstInitialBalance)

		testCoin := ibc.WalletAmount{
			Address: user.DstChainAddress,
			Denom:   testDenom,
			Amount:  1000000,
		}

		// send ibc transfer with a timeout of 10 blocks from now on counterparty chain
		txHash, err = srcChain.SendIBCTransfer(ctx, channels[0].ChannelID, user.KeyName, testCoin, &ibc.IBCTimeout{Height: 10})
		require.NoError(t, err)

		// wait until counterparty chain has passed the timeout
		dstChain.WaitForBlocks(11)
	}

	// Startup both chains and relayer
	_, user := ibc.StartChainsAndRelayer(t, ctx, pool, network.ID, home, srcChain, dstChain, ibc.CosmosRly, preRelayerStart)

	// wait for both chains to produce 10 blocks
	ibc.WaitForBlocks(srcChain, dstChain, 10)

	// fetch ibc transfer tx
	srcTx, err := srcChain.GetTransaction(ctx, txHash)
	require.NoError(t, err)

	fmt.Printf("Transaction:\n%v\n", srcTx)

	srcFinalBalance, err := srcChain.GetBalance(ctx, user.SrcChainAddress, testDenom)
	require.NoError(t, err)

	dstFinalBalance, err := dstChain.GetBalance(ctx, user.DstChainAddress, dstIbcDenom)
	require.NoError(t, err)

	fmt.Printf("Src chain final balance: %v\nDst chain final balance: %v\n", srcFinalBalance, dstFinalBalance)

	require.Equal(t, srcFinalBalance, srcInitialBalance)
	require.Equal(t, dstFinalBalance, dstInitialBalance)
}

// makes sure that a queued packet that is timed out (nanoseconds timeout) will not be relayed
func TestRelayTimeoutT(t *testing.T) {
	t.Skip() // skipping for now until timestamp timeout is fixed in cosmos relayer
	numValidatorsPerChain := 4
	numFullNodesPerChain := 1

	ctx, home, pool, network := ibc.SetupTestRun(t)

	// TODO make chain configuration an input
	srcChain := ibc.GaiaChain(t, pool, home, network.ID, numValidatorsPerChain, numFullNodesPerChain)
	dstChain := ibc.OsmosisChain(t, pool, home, network.ID, numValidatorsPerChain, numFullNodesPerChain)

	var srcInitialBalance, dstInitialBalance int64
	var txHash string

	testDenom := srcChain.Config().Denom
	var dstIbcDenom string

	// Query user account balances on both chains and send IBC transfer before starting the relayer
	preRelayerStart := func(channels []ibc.ChannelOutput, user ibc.User) {
		var err error
		srcInitialBalance, err = srcChain.GetBalance(ctx, user.SrcChainAddress, testDenom)
		require.NoError(t, err)

		// get ibc denom for test denom on dst chain
		denomTrace := transfertypes.ParseDenomTrace(transfertypes.GetPrefixedDenom(channels[0].Counterparty.PortID, channels[0].Counterparty.ChannelID, testDenom))
		dstIbcDenom = denomTrace.IBCDenom()

		// don't care about error here, account does not exist on destination chain
		dstInitialBalance, _ = dstChain.GetBalance(ctx, user.DstChainAddress, dstIbcDenom)

		fmt.Printf("Initial balances: Src chain: %d\nDst chain: %d\n", srcInitialBalance, dstInitialBalance)

		testCoin := ibc.WalletAmount{
			Address: user.DstChainAddress,
			Denom:   testDenom,
			Amount:  1000000,
		}

		// send ibc transfer with a timeout of 10 blocks from now on counterparty chain
		txHash, err = srcChain.SendIBCTransfer(ctx, channels[0].ChannelID, user.KeyName, testCoin, &ibc.IBCTimeout{NanoSeconds: uint64((10 * time.Second).Nanoseconds())})
		require.NoError(t, err)

		// wait until ibc transfer times out
		time.Sleep(15 * time.Second)
	}

	// Startup both chains and relayer
	_, user := ibc.StartChainsAndRelayer(t, ctx, pool, network.ID, home, srcChain, dstChain, ibc.CosmosRly, preRelayerStart)

	// wait for both chains to produce 10 blocks
	ibc.WaitForBlocks(srcChain, dstChain, 10)

	// fetch ibc transfer tx
	srcTx, err := srcChain.GetTransaction(ctx, txHash)
	require.NoError(t, err)

	fmt.Printf("Transaction:\n%v\n", srcTx)

	srcFinalBalance, err := srcChain.GetBalance(ctx, user.SrcChainAddress, testDenom)
	require.NoError(t, err)

	dstFinalBalance, err := dstChain.GetBalance(ctx, user.DstChainAddress, dstIbcDenom)
	require.NoError(t, err)

	fmt.Printf("Src chain final balance: %v\nDst chain final balance: %v\n", srcFinalBalance, dstFinalBalance)

	require.Equal(t, srcFinalBalance, srcInitialBalance)
	require.Equal(t, dstFinalBalance, dstInitialBalance)
}
