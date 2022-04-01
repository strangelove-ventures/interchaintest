package test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/cosmos/cosmos-sdk/types"
	transfertypes "github.com/cosmos/ibc-go/v3/modules/apps/transfer/types"
	"github.com/strangelove-ventures/ibc-test-framework/ibc"
	"github.com/stretchr/testify/require"
)

var (
	relayerImplementation = "cosmos/relayer" // TODO make dynamic

	srcAccountKeyName  = "src-chain"
	dstAccountKeyName  = "dst-chain"
	userAccountKeyName = "user"
	testPathName       = "test-path"
)

func TestRelayPacket(t *testing.T) {
	numValidatorsPerChain := 4
	numFullNodesPerChain := 1

	ctx, home, pool, network := ibc.SetupTestRun(t)

	// TODO make chain configuration an input
	srcChain := ibc.NewCosmosChain(t, pool, home, network.ID, "gaia", "cosmoshub-1004", "v6.0.4", "gaiad", "cosmos", "uatom", "0.01uatom", 1.3, "504h", numValidatorsPerChain, numFullNodesPerChain)
	dstChain := ibc.NewCosmosChain(t, pool, home, network.ID, "osmosis", "osmosis-1001", "v7.0.4", "osmosisd", "osmo", "uosmo", "0.0uosmo", 1.3, "336h", numValidatorsPerChain, numFullNodesPerChain)

	srcChainCfg := srcChain.Config()
	dstChainCfg := dstChain.Config()

	var relayerImpl ibc.Relayer

	if relayerImplementation == "cosmos/relayer" {
		relayerImpl = ibc.NewCosmosRelayerFromChains(
			t,
			srcChain,
			dstChain,
			pool,
			network.ID,
			home,
		)
	}

	err := relayerImpl.AddChainConfiguration(ctx, srcChainCfg, srcAccountKeyName,
		srcChain.GetRPCAddress(), srcChain.GetGRPCAddress())
	require.NoError(t, err)

	err = relayerImpl.AddChainConfiguration(ctx, dstChainCfg, dstAccountKeyName,
		dstChain.GetRPCAddress(), dstChain.GetGRPCAddress())
	require.NoError(t, err)

	srcRelayerWallet, err := relayerImpl.AddKey(ctx, srcChain.Config().ChainID, srcAccountKeyName)
	require.NoError(t, err)
	dstRelayerWallet, err := relayerImpl.AddKey(ctx, dstChain.Config().ChainID, dstAccountKeyName)
	require.NoError(t, err)

	srcAccount := srcRelayerWallet.Address
	dstAccount := dstRelayerWallet.Address

	err = relayerImpl.GeneratePath(ctx, srcChainCfg.ChainID, dstChainCfg.ChainID, testPathName)
	require.NoError(t, err)

	// Fund relayer account on src chain
	srcWallet := ibc.WalletAmount{
		Address: srcAccount,
		Denom:   srcChainCfg.Denom,
		Amount:  10000000,
	}

	// Fund relayer account on dst chain
	dstWallet := ibc.WalletAmount{
		Address: dstAccount,
		Denom:   dstChainCfg.Denom,
		Amount:  10000000,
	}

	// Generate key to be used for "user" that will execute IBC transaction
	err = srcChain.CreateKey(ctx, userAccountKeyName)
	require.NoError(t, err)
	userAccountAddressBytes, err := srcChain.GetAddress(userAccountKeyName)
	require.NoError(t, err)

	userAccountSrc, err := types.Bech32ifyAddressBytes(srcChainCfg.Bech32Prefix, userAccountAddressBytes)
	require.NoError(t, err)

	userAccountDst, err := types.Bech32ifyAddressBytes(dstChainCfg.Bech32Prefix, userAccountAddressBytes)
	require.NoError(t, err)

	// Fund user account on src chain in order to relay from src to dst
	userWalletSrc := ibc.WalletAmount{
		Address: userAccountSrc,
		Denom:   srcChainCfg.Denom,
		Amount:  100000000,
	}

	// start chains from genesis, wait until they are producing blocks
	chainsGenesisWaitGroup := sync.WaitGroup{}
	chainsGenesisWaitGroup.Add(2)
	go func() {
		srcChain.Start(t, ctx, []ibc.WalletAmount{srcWallet, userWalletSrc})
		chainsGenesisWaitGroup.Done()
	}()
	go func() {
		dstChain.Start(t, ctx, []ibc.WalletAmount{dstWallet})
		chainsGenesisWaitGroup.Done()
	}()
	chainsGenesisWaitGroup.Wait()

	// Both chains are producing blocks

	testDenom := srcChainCfg.Denom

	srcInitialBalance, err := srcChain.GetBalance(ctx, userAccountSrc, testDenom)
	require.NoError(t, err)

	// don't care about error here, account does not exist on destination chain
	dstInitialBalance, _ := dstChain.GetBalance(ctx, userAccountDst, testDenom)

	fmt.Printf("Src chain: %v\nDst chain: %v\n", srcInitialBalance, dstInitialBalance)

	require.NoError(t, relayerImpl.LinkPath(ctx, testPathName))

	channels, err := relayerImpl.GetChannels(ctx, srcChainCfg.ChainID)
	require.NoError(t, err)
	require.Equal(t, len(channels), 1)

	require.NoError(t, relayerImpl.StartRelayer(ctx, testPathName))

	t.Cleanup(func() { _ = relayerImpl.StopRelayer(ctx) })

	// wait for relayer to start up
	time.Sleep(5 * time.Second)

	testCoin := ibc.WalletAmount{
		Address: userAccountDst,
		Denom:   testDenom,
		Amount:  1000000,
	}

	txHash, err := srcChain.SendIBCTransfer(ctx, channels[0].ChannelID, userAccountKeyName, testCoin, nil)
	require.NoError(t, err)

	chainsConsecutiveBlocksWaitGroup := sync.WaitGroup{}
	chainsConsecutiveBlocksWaitGroup.Add(2)
	go func() {
		srcChain.WaitForBlocks(10)
		chainsConsecutiveBlocksWaitGroup.Done()
	}()
	go func() {
		dstChain.WaitForBlocks(10)
		chainsConsecutiveBlocksWaitGroup.Done()
	}()
	chainsConsecutiveBlocksWaitGroup.Wait()

	srcTx, err := srcChain.GetTransaction(ctx, txHash)
	require.NoError(t, err)

	fmt.Printf("Transaction:\n%v\n", srcTx)

	srcFinalBalance, err := srcChain.GetBalance(ctx, userAccountSrc, testDenom)
	require.NoError(t, err)

	// get ibc denom for test denom on dst chain
	denomTrace := transfertypes.ParseDenomTrace(transfertypes.GetPrefixedDenom(channels[0].Counterparty.PortID, channels[0].Counterparty.ChannelID, testDenom))
	dstIbcDenom := denomTrace.IBCDenom()

	dstFinalBalance, err := dstChain.GetBalance(ctx, userAccountDst, dstIbcDenom)
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
	srcChain := ibc.NewCosmosChain(t, pool, home, network.ID, "gaia", "cosmoshub-1004", "v6.0.4", "gaiad", "cosmos", "uatom", "0.01uatom", 1.3, "504h", numValidatorsPerChain, numFullNodesPerChain)
	dstChain := ibc.NewCosmosChain(t, pool, home, network.ID, "osmosis", "osmosis-1001", "v7.0.4", "osmosisd", "osmo", "uosmo", "0.0uosmo", 1.3, "336h", numValidatorsPerChain, numFullNodesPerChain)

	srcChainCfg := srcChain.Config()
	dstChainCfg := dstChain.Config()

	var relayerImpl ibc.Relayer

	if relayerImplementation == "cosmos/relayer" {
		relayerImpl = ibc.NewCosmosRelayerFromChains(
			t,
			srcChain,
			dstChain,
			pool,
			network.ID,
			home,
		)
	}

	err := relayerImpl.AddChainConfiguration(ctx, srcChainCfg, srcAccountKeyName,
		srcChain.GetRPCAddress(), srcChain.GetGRPCAddress())
	require.NoError(t, err)

	err = relayerImpl.AddChainConfiguration(ctx, dstChainCfg, dstAccountKeyName,
		dstChain.GetRPCAddress(), dstChain.GetGRPCAddress())
	require.NoError(t, err)

	srcRelayerWallet, err := relayerImpl.AddKey(ctx, srcChain.Config().ChainID, srcAccountKeyName)
	require.NoError(t, err)
	dstRelayerWallet, err := relayerImpl.AddKey(ctx, dstChain.Config().ChainID, dstAccountKeyName)
	require.NoError(t, err)

	srcAccount := srcRelayerWallet.Address
	dstAccount := dstRelayerWallet.Address

	err = relayerImpl.GeneratePath(ctx, srcChainCfg.ChainID, dstChainCfg.ChainID, testPathName)
	require.NoError(t, err)

	// Fund relayer account on src chain
	srcWallet := ibc.WalletAmount{
		Address: srcAccount,
		Denom:   srcChainCfg.Denom,
		Amount:  10000000,
	}

	// Fund relayer account on dst chain
	dstWallet := ibc.WalletAmount{
		Address: dstAccount,
		Denom:   dstChainCfg.Denom,
		Amount:  10000000,
	}

	// Generate key to be used for "user" that will execute IBC transaction
	err = srcChain.CreateKey(ctx, userAccountKeyName)
	require.NoError(t, err)
	userAccountAddressBytes, err := srcChain.GetAddress(userAccountKeyName)
	require.NoError(t, err)

	userAccountSrc, err := types.Bech32ifyAddressBytes(srcChainCfg.Bech32Prefix, userAccountAddressBytes)
	require.NoError(t, err)

	userAccountDst, err := types.Bech32ifyAddressBytes(dstChainCfg.Bech32Prefix, userAccountAddressBytes)
	require.NoError(t, err)

	// Fund user account on src chain in order to relay from src to dst
	userWalletSrc := ibc.WalletAmount{
		Address: userAccountSrc,
		Denom:   srcChainCfg.Denom,
		Amount:  100000000,
	}

	// start chains from genesis, wait until they are producing blocks
	chainsGenesisWaitGroup := sync.WaitGroup{}
	chainsGenesisWaitGroup.Add(2)
	go func() {
		srcChain.Start(t, ctx, []ibc.WalletAmount{srcWallet, userWalletSrc})
		chainsGenesisWaitGroup.Done()
	}()
	go func() {
		dstChain.Start(t, ctx, []ibc.WalletAmount{dstWallet})
		chainsGenesisWaitGroup.Done()
	}()
	chainsGenesisWaitGroup.Wait()

	// Both chains are producing blocks

	testDenom := srcChainCfg.Denom

	srcInitialBalance, err := srcChain.GetBalance(ctx, userAccountSrc, testDenom)
	require.NoError(t, err)

	// don't care about error here, account does not exist on destination chain
	dstInitialBalance, _ := dstChain.GetBalance(ctx, userAccountDst, testDenom)

	fmt.Printf("Src chain: %v\nDst chain: %v\n", srcInitialBalance, dstInitialBalance)

	require.NoError(t, relayerImpl.LinkPath(ctx, testPathName))

	channels, err := relayerImpl.GetChannels(ctx, srcChainCfg.ChainID)
	require.NoError(t, err)
	require.Equal(t, len(channels), 1)

	testCoin := ibc.WalletAmount{
		Address: userAccountDst,
		Denom:   testDenom,
		Amount:  1000000,
	}

	// send ibc transfer with a timeout of 10 blocks from now on counterparty chain
	txHash, err := srcChain.SendIBCTransfer(ctx, channels[0].ChannelID, userAccountKeyName, testCoin, &ibc.IBCTimeout{Height: 10})
	require.NoError(t, err)

	// wait until counterparty chain has passed the timeout
	dstChain.WaitForBlocks(11)

	// start the relayer
	require.NoError(t, relayerImpl.StartRelayer(ctx, testPathName))
	t.Cleanup(func() { _ = relayerImpl.StopRelayer(ctx) })

	// wait for relayer to start up
	time.Sleep(5 * time.Second)

	// wait 10 blocks to make sure any valid packets would be relayed successfully
	blocksToWait := int64(10)
	chainsConsecutiveBlocksWaitGroup := sync.WaitGroup{}
	chainsConsecutiveBlocksWaitGroup.Add(2)
	go func() {
		srcChain.WaitForBlocks(blocksToWait)
		chainsConsecutiveBlocksWaitGroup.Done()
	}()
	go func() {
		dstChain.WaitForBlocks(blocksToWait)
		chainsConsecutiveBlocksWaitGroup.Done()
	}()
	chainsConsecutiveBlocksWaitGroup.Wait()

	srcTx, err := srcChain.GetTransaction(ctx, txHash)
	require.NoError(t, err)

	fmt.Printf("Transaction:\n%v\n", srcTx)

	srcFinalBalance, err := srcChain.GetBalance(ctx, userAccountSrc, testDenom)
	require.NoError(t, err)

	// get ibc denom for test denom on dst chain
	denomTrace := transfertypes.ParseDenomTrace(transfertypes.GetPrefixedDenom(channels[0].Counterparty.PortID, channels[0].Counterparty.ChannelID, testDenom))
	dstIbcDenom := denomTrace.IBCDenom()

	dstFinalBalance, err := dstChain.GetBalance(ctx, userAccountDst, dstIbcDenom)
	require.NoError(t, err)

	require.Equal(t, srcFinalBalance, srcInitialBalance)
	require.Equal(t, dstFinalBalance, dstInitialBalance)

}

// makes sure that a queued packet that is timed out (nanoseconds timeout) will not be relayed
func TestRelayTimeoutT(t *testing.T) {
	numValidatorsPerChain := 4
	numFullNodesPerChain := 1

	ctx, home, pool, network := ibc.SetupTestRun(t)

	// TODO make chain configuration an input
	srcChain := ibc.NewCosmosChain(t, pool, home, network.ID, "gaia", "cosmoshub-1004", "v6.0.4", "gaiad", "cosmos", "uatom", "0.01uatom", 1.3, "504h", numValidatorsPerChain, numFullNodesPerChain)
	dstChain := ibc.NewCosmosChain(t, pool, home, network.ID, "osmosis", "osmosis-1001", "v7.0.4", "osmosisd", "osmo", "uosmo", "0.0uosmo", 1.3, "336h", numValidatorsPerChain, numFullNodesPerChain)

	srcChainCfg := srcChain.Config()
	dstChainCfg := dstChain.Config()

	var relayerImpl ibc.Relayer

	if relayerImplementation == "cosmos/relayer" {
		relayerImpl = ibc.NewCosmosRelayerFromChains(
			t,
			srcChain,
			dstChain,
			pool,
			network.ID,
			home,
		)
	}

	err := relayerImpl.AddChainConfiguration(ctx, srcChainCfg, srcAccountKeyName,
		srcChain.GetRPCAddress(), srcChain.GetGRPCAddress())
	require.NoError(t, err)

	err = relayerImpl.AddChainConfiguration(ctx, dstChainCfg, dstAccountKeyName,
		dstChain.GetRPCAddress(), dstChain.GetGRPCAddress())
	require.NoError(t, err)

	srcRelayerWallet, err := relayerImpl.AddKey(ctx, srcChain.Config().ChainID, srcAccountKeyName)
	require.NoError(t, err)
	dstRelayerWallet, err := relayerImpl.AddKey(ctx, dstChain.Config().ChainID, dstAccountKeyName)
	require.NoError(t, err)

	srcAccount := srcRelayerWallet.Address
	dstAccount := dstRelayerWallet.Address

	err = relayerImpl.GeneratePath(ctx, srcChainCfg.ChainID, dstChainCfg.ChainID, testPathName)
	require.NoError(t, err)

	// Fund relayer account on src chain
	srcWallet := ibc.WalletAmount{
		Address: srcAccount,
		Denom:   srcChainCfg.Denom,
		Amount:  10000000,
	}

	// Fund relayer account on dst chain
	dstWallet := ibc.WalletAmount{
		Address: dstAccount,
		Denom:   dstChainCfg.Denom,
		Amount:  10000000,
	}

	// Generate key to be used for "user" that will execute IBC transaction
	err = srcChain.CreateKey(ctx, userAccountKeyName)
	require.NoError(t, err)
	userAccountAddressBytes, err := srcChain.GetAddress(userAccountKeyName)
	require.NoError(t, err)

	userAccountSrc, err := types.Bech32ifyAddressBytes(srcChainCfg.Bech32Prefix, userAccountAddressBytes)
	require.NoError(t, err)

	userAccountDst, err := types.Bech32ifyAddressBytes(dstChainCfg.Bech32Prefix, userAccountAddressBytes)
	require.NoError(t, err)

	// Fund user account on src chain in order to relay from src to dst
	userWalletSrc := ibc.WalletAmount{
		Address: userAccountSrc,
		Denom:   srcChainCfg.Denom,
		Amount:  100000000,
	}

	// start chains from genesis, wait until they are producing blocks
	chainsGenesisWaitGroup := sync.WaitGroup{}
	chainsGenesisWaitGroup.Add(2)
	go func() {
		srcChain.Start(t, ctx, []ibc.WalletAmount{srcWallet, userWalletSrc})
		chainsGenesisWaitGroup.Done()
	}()
	go func() {
		dstChain.Start(t, ctx, []ibc.WalletAmount{dstWallet})
		chainsGenesisWaitGroup.Done()
	}()
	chainsGenesisWaitGroup.Wait()

	// Both chains are producing blocks

	testDenom := srcChainCfg.Denom

	srcInitialBalance, err := srcChain.GetBalance(ctx, userAccountSrc, testDenom)
	require.NoError(t, err)

	// don't care about error here, account does not exist on destination chain
	dstInitialBalance, _ := dstChain.GetBalance(ctx, userAccountDst, testDenom)

	fmt.Printf("Src chain: %v\nDst chain: %v\n", srcInitialBalance, dstInitialBalance)

	require.NoError(t, relayerImpl.LinkPath(ctx, testPathName))

	channels, err := relayerImpl.GetChannels(ctx, srcChainCfg.ChainID)
	require.NoError(t, err)
	require.Equal(t, len(channels), 1)

	testCoin := ibc.WalletAmount{
		Address: userAccountDst,
		Denom:   testDenom,
		Amount:  1000000,
	}

	// send ibc transfer with a timeout of 10 seconds from now
	txHash, err := srcChain.SendIBCTransfer(ctx, channels[0].ChannelID, userAccountKeyName, testCoin, &ibc.IBCTimeout{NanoSeconds: uint64((10 * time.Second).Nanoseconds())})
	require.NoError(t, err)

	// wait until ibc transfer times out
	time.Sleep(15 * time.Second)

	// start the relayer
	require.NoError(t, relayerImpl.StartRelayer(ctx, testPathName))
	t.Cleanup(func() { _ = relayerImpl.StopRelayer(ctx) })

	// wait for relayer to start up
	time.Sleep(5 * time.Second)

	// wait 10 blocks to make sure any valid packets would be relayed successfully
	blocksToWait := int64(10)
	chainsConsecutiveBlocksWaitGroup := sync.WaitGroup{}
	chainsConsecutiveBlocksWaitGroup.Add(2)
	go func() {
		srcChain.WaitForBlocks(blocksToWait)
		chainsConsecutiveBlocksWaitGroup.Done()
	}()
	go func() {
		dstChain.WaitForBlocks(blocksToWait)
		chainsConsecutiveBlocksWaitGroup.Done()
	}()
	chainsConsecutiveBlocksWaitGroup.Wait()

	srcTx, err := srcChain.GetTransaction(ctx, txHash)
	require.NoError(t, err)

	fmt.Printf("Transaction:\n%v\n", srcTx)

	srcFinalBalance, err := srcChain.GetBalance(ctx, userAccountSrc, testDenom)
	require.NoError(t, err)

	// get ibc denom for test denom on dst chain
	denomTrace := transfertypes.ParseDenomTrace(transfertypes.GetPrefixedDenom(channels[0].Counterparty.PortID, channels[0].Counterparty.ChannelID, testDenom))
	dstIbcDenom := denomTrace.IBCDenom()

	dstFinalBalance, err := dstChain.GetBalance(ctx, userAccountDst, dstIbcDenom)
	require.NoError(t, err)

	require.Equal(t, srcFinalBalance, srcInitialBalance)
	require.Equal(t, dstFinalBalance, dstInitialBalance)

}
