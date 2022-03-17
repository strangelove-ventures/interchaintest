package test

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/cosmos/cosmos-sdk/types"
	cosmosRelayer "github.com/cosmos/relayer/relayer"
	"github.com/ory/dockertest"
	"github.com/ory/dockertest/docker"
	"github.com/strangelove-ventures/ibc-test-framework/relayer"
	"github.com/stretchr/testify/require"
)

var (
	relayerImplementation = "cosmos/relayer" // TODO make dynamic
	chainTimeout          = 3 * time.Second
)

func startValidatorsAsync(
	t *testing.T,
	ctx context.Context,
	network *docker.Network,
	chainType *ChainType,
	validators TestNodes,
	wg *sync.WaitGroup,
) {
	StartNodeContainers(t, ctx, network, chainType, validators, []*TestNode{})
	validators.WaitForHeight(5)
	wg.Done()
}

func TestChainSpinUp(t *testing.T) {
	chainTypeSrc := getChain("gaia", "v6.0.4", "gaiad", "cosmos")
	chainTypeDst := getChain("osmosis", "v7.0.4", "osmosisd", "osmo")

	ctx, home, pool, network, validatorsSrc, validatorsDst := SetupTestRun(t, chainTypeSrc, chainTypeDst, 4)

	t.Cleanup(Cleanup(pool, t.Name(), home))

	// start validators and sentry nodes
	wg := sync.WaitGroup{}
	wg.Add(2)
	go startValidatorsAsync(t, ctx, network, chainTypeSrc, validatorsSrc, &wg)
	go startValidatorsAsync(t, ctx, network, chainTypeDst, validatorsDst, &wg)
	wg.Wait()

	// Both chains are started and ready for relaying

	srcRpc := fmt.Sprintf("http://%s", GetHostPort(validatorsSrc[0].Container, rpcPort))
	dstRpc := fmt.Sprintf("http://%s", GetHostPort(validatorsDst[0].Container, rpcPort))

	fmt.Printf("Src RPC: %s\nDst RPC: %s\n", srcRpc, dstRpc)

	var relayerImpl relayer.Relayer

	var srcChain, dstChain *cosmosRelayer.Chain

	if relayerImplementation == "cosmos/relayer" {
		srcChain = &cosmosRelayer.Chain{
			Key:            "testkey",
			ChainID:        chainTypeSrc.Name,
			AccountPrefix:  chainTypeSrc.Bech32Prefix,
			GasPrices:      "0.01uatom",
			GasAdjustment:  1.3,
			RPCAddr:        srcRpc,
			TrustingPeriod: "504h",
		}
		err := srcChain.Init("", chainTimeout, nil, true)
		require.NoError(t, err)

		dstChain = &cosmosRelayer.Chain{
			Key:            "testkey",
			ChainID:        chainTypeDst.Name,
			AccountPrefix:  chainTypeDst.Bech32Prefix,
			GasPrices:      "0uosmo",
			GasAdjustment:  1.3,
			RPCAddr:        dstRpc,
			TrustingPeriod: "336h",
		}
		err = dstChain.Init("", chainTimeout, nil, true)
		require.NoError(t, err)

		relayerImpl = relayer.NewCosmosRelayerFromChains(
			srcChain,
			dstChain,
		)

		// query initial balances to compare against at the end
		srcExpected, err := srcChain.QueryBalance(srcChain.Key)
		require.NoError(t, err)
		dstExpected, err := dstChain.QueryBalance(dstChain.Key)
		require.NoError(t, err)

		fmt.Printf("Src chain: %v\nDst chain: %v\n", srcExpected, dstExpected)
	}

	err := relayerImpl.StartRelayer()
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

	testDenom := "uatom"
	testAmount := types.NewInt(1000000)

	testCoin := types.Coin{
		Denom:  testDenom,
		Amount: testAmount,
	}

	//err = relayerImpl.RelayPacketFromSource(testCoin, validator0DstBech32Address)
	err = relayerImpl.RelayPacketFromSource(testCoin, dstChain.MustGetAddress())
	require.NoError(t, err)

	require.NoError(t, dstChain.WaitForNBlocks(6))

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

	if dstChain != nil {
		dstGot, err := dstChain.QueryBalance(dstChain.Key)
		require.NoError(t, err)
		fmt.Printf("dst balance: %v\n", dstGot)
		require.Equal(t, dstGot.AmountOf(testDenom).Int64(), testAmount.Int64())
	}
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
