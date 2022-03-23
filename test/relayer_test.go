package test

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/cosmos/cosmos-sdk/types"
	"github.com/ory/dockertest"
	"github.com/ory/dockertest/docker"
	"github.com/strangelove-ventures/ibc-test-framework/ibc"
	"github.com/stretchr/testify/require"
)

var (
	relayerImplementation = "cosmos/relayer" // TODO make dynamic

	srcMnemonic       = "bind problem flower execute floor position aerobic still enlist south royal example wave element desert gadget sing game goddess dolphin aunt place tuition tennis"
	srcAccount        = "cosmos1a5d6rh8zcrsfdft8uqpkepllf78fqesdc0rx7z"
	srcAccountKeyName = "src-chain"

	dstMnemonic       = "push sample catch crystal jewel episode clip crane betray scrub kidney air glow whale faith business fade balance enhance alpha write such leopard alcohol"
	dstAccount        = "osmo1wjf74ee7whft9rdveceuuhjm22t6e54zr72pzc"
	dstAccountKeyName = "dst-chain"

	userAccountKeyName = "user"

	testPathName = "test-path"
)

func TestChainSpinUp(t *testing.T) {

	numValidatorsPerChain := 4
	numFullNodesPerChain := 1

	ctx, home, pool, network := ibc.SetupTestRun(t)

	srcChain := ibc.NewCosmosChain(t, pool, home, network.ID, "gaia", "cosmoshub-1004", "v6.0.4", "gaiad", "cosmos", "uatom", "0.01uatom", 1.3, "504h", numValidatorsPerChain, numFullNodesPerChain)
	dstChain := ibc.NewCosmosChain(t, pool, home, network.ID, "osmosis", "osmosis-1001", "v7.0.4", "osmosisd", "osmo", "uosmo", "0.0uosmo", 1.3, "336h", numValidatorsPerChain, numFullNodesPerChain)

	srcChainCfg := srcChain.Config()
	dstChainCfg := dstChain.Config()

	t.Cleanup(Cleanup(pool, t.Name(), home))

	var relayerImpl ibc.Relayer

	if relayerImplementation == "cosmos/relayer" {
		relayerImpl = ibc.NewCosmosRelayerFromChains(
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

	err = relayerImpl.RestoreKey(ctx, srcChain.Config().ChainID, srcAccountKeyName, srcMnemonic)
	require.NoError(t, err)
	err = relayerImpl.RestoreKey(ctx, srcChain.Config().ChainID, dstAccountKeyName, dstMnemonic)
	require.NoError(t, err)

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

	err = relayerImpl.StartRelayer(ctx)
	require.NoError(t, err)

	// wait for relayer to start up
	time.Sleep(5 * time.Second)

	t.Cleanup(func() {
		_ = relayerImpl.StopRelayer(ctx)
	})

	testCoin := ibc.WalletAmount{
		Address: userAccountDst,
		Denom:   testDenom,
		Amount:  1000000,
	}

	err = srcChain.SendIBCTransfer(ctx, testPathName, userAccountKeyName, testCoin)
	require.NoError(t, err)

	chainsConsecutiveBlocksWaitGroup := sync.WaitGroup{}
	chainsConsecutiveBlocksWaitGroup.Add(2)
	go func() {
		srcChain.WaitForBlocks(5)
		chainsConsecutiveBlocksWaitGroup.Done()
	}()
	go func() {
		dstChain.WaitForBlocks(5)
		chainsConsecutiveBlocksWaitGroup.Done()
	}()
	chainsConsecutiveBlocksWaitGroup.Wait()

	srcFinalBalance, err := srcChain.GetBalance(ctx, userAccountSrc, testDenom)
	require.NoError(t, err)

	dstFinalBalance, err := dstChain.GetBalance(ctx, userAccountDst, testDenom)
	require.NoError(t, err)

	fmt.Printf("Src chain: %v\nDst chain: %v\n", srcFinalBalance, dstFinalBalance)

	require.Equal(t, srcFinalBalance, srcInitialBalance-testCoin.Amount)
	require.Equal(t, dstFinalBalance, dstInitialBalance+testCoin.Amount)
}

// Cleanup will clean up Docker containers, networks, and the other various config files generated in testing
func Cleanup(pool *dockertest.Pool, testName, testDir string) func() {
	return func() {
		cont, _ := pool.Client.ListContainers(docker.ListContainersOptions{All: true})
		for _, c := range cont {
			for k, v := range c.Labels {
				if k == "ibc-test" && v == testName {
					_ = pool.Client.StopContainer(c.ID, 10)
					exitCode, err := pool.Client.WaitContainerWithContext(c.ID, context.Background())
					// remove containers without error
					if err == nil && exitCode == 0 {
						_ = pool.Client.RemoveContainer(docker.RemoveContainerOptions{ID: c.ID})
					}
				}
			}
		}
		nets, _ := pool.Client.ListNetworks()
		for _, n := range nets {
			for k, v := range n.Labels {
				if k == "ibc-test" && v == testName {
					_ = pool.Client.RemoveNetwork(n.ID)
				}
			}
		}
		_ = os.RemoveAll(testDir)
	}
}
