package test

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	cosmosRelayer "github.com/cosmos/relayer/relayer"
	"github.com/ory/dockertest"
	"github.com/ory/dockertest/docker"
	"github.com/strangelove-ventures/ibc-test-framework/relayer"
	"github.com/stretchr/testify/require"
)

var (
	relayerImplementation = "cosmos/relayer" // TODO make dynamic
)

func startValidatorsAsync(
	t *testing.T,
	ctx context.Context,
	network *docker.Network,
	chainType *ChainType,
	validators TestNodes,
	additionalGenesisWallets []relayer.WalletAmount,
	wg *sync.WaitGroup,
) {
	StartNodeContainers(t, ctx, network, chainType, validators, []*TestNode{}, additionalGenesisWallets)
	validators.WaitForHeight(5)
	wg.Done()
}

func TestChainSpinUp(t *testing.T) {
	chainTypeSrc := getChain("gaia", "v6.0.4", "gaiad", "cosmos", "uatom", "0.01uatom", 1.3, "504h")
	chainTypeDst := getChain("osmosis", "v7.0.4", "osmosisd", "osmo", "uosmo", "0.0uosmo", 1.3, "336h")

	ctx, home, pool, network, validatorsSrc, validatorsDst := SetupTestRun(t, chainTypeSrc, chainTypeDst, 4)

	t.Cleanup(Cleanup(pool, t.Name(), home))

	var relayerImpl relayer.Relayer

	if relayerImplementation == "cosmos/relayer" {
		srcChain := &cosmosRelayer.Chain{
			Key:            "testkey",
			ChainID:        chainTypeSrc.Name,
			AccountPrefix:  chainTypeSrc.Bech32Prefix,
			GasPrices:      chainTypeSrc.GasPrices,
			GasAdjustment:  chainTypeSrc.GasAdjustment,
			TrustingPeriod: chainTypeSrc.TrustingPeriod,
		}
		err := srcChain.Init("", 0, nil, false)
		require.NoError(t, err)

		dstChain := &cosmosRelayer.Chain{
			Key:            "testkey",
			ChainID:        chainTypeDst.Name,
			AccountPrefix:  chainTypeDst.Bech32Prefix,
			GasPrices:      chainTypeDst.GasPrices,
			GasAdjustment:  chainTypeDst.GasAdjustment,
			TrustingPeriod: chainTypeDst.TrustingPeriod,
		}
		err = dstChain.Init("", 0, nil, false)
		require.NoError(t, err)

		relayerImpl = relayer.NewCosmosRelayerFromChains(
			srcChain,
			dstChain,
			t,
		)
	}

	srcWallet, err := relayerImpl.InitializeSourceWallet()
	require.NoError(t, err)

	dstWallet, err := relayerImpl.InitializeDestinationWallet()
	require.NoError(t, err)

	srcWallet.Denom = chainTypeSrc.Denom
	srcWallet.Amount = 10000000

	dstWallet.Denom = chainTypeDst.Denom
	dstWallet.Amount = 10000000

	// start validators and sentry nodes
	chainsGenesisWaitGroup := sync.WaitGroup{}
	chainsGenesisWaitGroup.Add(2)
	go startValidatorsAsync(t, ctx, network, chainTypeSrc, validatorsSrc, []relayer.WalletAmount{srcWallet}, &chainsGenesisWaitGroup)
	go startValidatorsAsync(t, ctx, network, chainTypeDst, validatorsDst, []relayer.WalletAmount{dstWallet}, &chainsGenesisWaitGroup)
	chainsGenesisWaitGroup.Wait()

	// Both chains are producing blocks

	srcRpc := fmt.Sprintf("http://%s", GetHostPort(validatorsSrc[0].Container, rpcPort))
	dstRpc := fmt.Sprintf("http://%s", GetHostPort(validatorsDst[0].Container, rpcPort))

	err = relayerImpl.SetSourceRPC(srcRpc)
	require.NoError(t, err)
	err = relayerImpl.SetDestinationRPC(dstRpc)
	require.NoError(t, err)

	fmt.Printf("Src RPC: %s\nDst RPC: %s\n", srcRpc, dstRpc)

	testDenom := chainTypeSrc.Denom

	// query initial balances to compare against at the end, both for src denom
	srcInitial, err := relayerImpl.GetSourceBalance(testDenom)
	require.NoError(t, err)
	dstInitial, err := relayerImpl.GetDestinationBalance(testDenom)
	require.NoError(t, err)

	fmt.Printf("Src chain: %v\nDst chain: %v\n", srcInitial, dstInitial)

	err = relayerImpl.StartRelayer()
	require.NoError(t, err)

	// wait for relayer to start up
	time.Sleep(5 * time.Second)

	t.Cleanup(func() {
		_ = relayerImpl.StopRelayer()
	})

	// send 1 atom to source validator 0's osmosis wallet
	// validator0Key, err := validatorsSrc[0].GetKey(valKey)
	// require.NoError(t, err)

	// validator0SrcBech32Address, err := types.Bech32ifyAddressBytes(chainTypeSrc.Bech32Prefix, validator0Key.GetAddress().Bytes())
	// require.NoError(t, err)

	// validator0DstBech32Address, err := types.Bech32ifyAddressBytes(chainTypeDst.Bech32Prefix, validator0Key.GetAddress().Bytes())
	// require.NoError(t, err)

	//testAmount := types.NewInt(1000000)

	testCoin := relayer.WalletAmount{
		Denom:  testDenom,
		Amount: 1000000,
	}

	//err = relayerImpl.RelayPacketFromSource(testCoin, validator0DstBech32Address)
	err = relayerImpl.RelayPacketFromSource(testCoin)
	require.NoError(t, err)

	chainsConsecutiveBlocksWaitGroup := sync.WaitGroup{}
	chainsConsecutiveBlocksWaitGroup.Add(2)
	go func() {
		validatorsSrc[0].WaitForConsecutiveBlocks(5)
		chainsConsecutiveBlocksWaitGroup.Done()
	}()
	go func() {
		validatorsDst[0].WaitForConsecutiveBlocks(5)
		chainsConsecutiveBlocksWaitGroup.Done()
	}()
	chainsConsecutiveBlocksWaitGroup.Wait()

	// if srcChain != nil {
	// 	srcGot, err := srcChain.QueryBalanceWithAddress(validator0SrcBech32Address)
	// 	require.NoError(t, err)
	// 	fmt.Printf("src balance: %v\n", srcGot)
	// }

	// if dstChain != nil {
	// 	dstGot, err := dstChain.QueryBalanceWithAddress(validator0DstBech32Address)
	// 	require.NoError(t, err)
	// 	fmt.Printf("dst balance: %v\n", dstGot)
	// 	require.Equal(t, dstGot.AmountOf(testDenom).Int64(), testAmount.Int64())
	// }

	srcActual, err := relayerImpl.GetSourceBalance(testDenom)
	require.NoError(t, err)

	dstActual, err := relayerImpl.GetDestinationBalance(testDenom)
	require.NoError(t, err)

	require.Equal(t, srcActual.Amount, srcInitial.Amount-testCoin.Amount)
	require.Equal(t, dstActual.Amount, dstInitial.Amount+testCoin.Amount)
}

// Cleanup will clean up Docker containers, networks, and the other various config files generated in testing
func Cleanup(pool *dockertest.Pool, testName, testDir string) func() {
	return func() {
		cont, _ := pool.Client.ListContainers(docker.ListContainersOptions{All: true})
		for _, c := range cont {
			for k, v := range c.Labels {
				if k == "ibc-test" && v == testName {
					_ = pool.Client.StopContainer(c.ID, 10)
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
