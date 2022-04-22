//go:build exclude

package trophies

import (
	"encoding/base64"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/cosmos/cosmos-sdk/types"
	transfertypes "github.com/cosmos/ibc-go/v3/modules/apps/transfer/types"
	"github.com/ory/dockertest/v3/docker"
)

func (ibc IBCTestCase) JunoHaltTest(testName string, srcChain Chain, dstChain Chain, relayerImplementation RelayerImplementation) error {
	ctx, home, pool, network, cleanup, err := SetupTestRun(testName)
	if err != nil {
		return err
	}
	defer cleanup()

	if err := srcChain.Initialize(testName, home, pool, network); err != nil {
		return err
	}

	srcChainCfg := srcChain.Config()

	// Generate key to be used for "user" that will execute transactions
	if err := srcChain.CreateKey(ctx, userAccountKeyName); err != nil {
		return err
	}

	userAccountAddressBytes, err := srcChain.GetAddress(userAccountKeyName)
	if err != nil {
		return err
	}

	userAccountSrc, err := types.Bech32ifyAddressBytes(srcChainCfg.Bech32Prefix, userAccountAddressBytes)
	if err != nil {
		return err
	}

	// Fund user account on src chain that will be used to instantiate and execute contract
	userWalletSrc := WalletAmount{
		Address: userAccountSrc,
		Denom:   srcChainCfg.Denom,
		Amount:  100000000000,
	}

	if err := srcChain.Start(testName, ctx, []WalletAmount{userWalletSrc}); err != nil {
		return err
	}

	executablePath, err := os.Executable()
	if err != nil {
		return err
	}
	rootPath := filepath.Dir(executablePath)
	contractPath := path.Join(rootPath, "assets", "badcontract.wasm")

	contractAddress, err := srcChain.InstantiateContract(ctx, userAccountKeyName, WalletAmount{Amount: 100, Denom: srcChain.Config().Denom}, contractPath, "{\"count\":0}", srcChainCfg.Version == "v2.3.0")
	if err != nil {
		return err
	}

	resets := []int{0, 15, 84, 0, 84, 42, 55, 42, 15, 84, 42}

	for _, resetCount := range resets {
		// run reset
		if err := srcChain.ExecuteContract(ctx, userAccountKeyName, contractAddress, fmt.Sprintf("{\"reset\":{\"count\": %d}}", resetCount)); err != nil {
			return err
		}
		latestHeight, err := srcChain.WaitForBlocks(5)
		if err != nil {
			return err
		}

		// dump current contract state
		res, err := srcChain.DumpContractState(ctx, contractAddress, latestHeight)
		if err != nil {
			return err
		}
		contractData, err := base64.StdEncoding.DecodeString(res.Models[1].Value)
		if err != nil {
			return err
		}
		fmt.Printf("Contract data: %s\n", contractData)

		// run increment a bunch of times
		for i := 0; i < 5; i++ {
			if err := srcChain.ExecuteContract(ctx, userAccountKeyName, contractAddress, "{\"increment\":{}}"); err != nil {
				return err
			}
			if _, err := srcChain.WaitForBlocks(1); err != nil {
				return err
			}
		}
	}

	return nil
}

func (ibc IBCTestCase) JunoPostHaltGenesis(testName string, srcChain Chain, dstChain Chain, relayerImplementation RelayerImplementation) error {
	ctx, home, pool, network, cleanup, err := SetupTestRun(testName)
	if err != nil {
		return err
	}
	defer cleanup()

	if err := srcChain.Initialize(testName, home, pool, network); err != nil {
		return err
	}

	executablePath, err := os.Executable()
	if err != nil {
		return err
	}
	rootPath := filepath.Dir(executablePath)
	genesisFilePath := path.Join(rootPath, "assets", "juno-1-96.json")

	if err := srcChain.StartWithGenesisFile(testName, ctx, home, pool, network, genesisFilePath); err != nil {
		return err
	}

	_, err = srcChain.WaitForBlocks(20)
	return err
}

func (ibc IBCTestCase) JunoHaltNewGenesis(testName string, _ Chain, _ Chain, relayerImplementation RelayerImplementation) error {
	ctx, home, pool, network, cleanup, err := SetupTestRun(testName)
	if err != nil {
		return err
	}
	defer cleanup()

	// overriding input vars
	srcChain, err := GetChain(testName, "juno", "v2.1.0", "juno-1", 10, 1)
	if err != nil {
		return err
	}

	dstChain, err := GetChain(testName, "osmosis", "v7.1.0", "osmosis-1", 4, 0)
	if err != nil {
		return err
	}

	// startup both chains and relayer
	// creates wallets in the relayer for src and dst chain
	// funds relayer src and dst wallets on respective chain in genesis
	// creates a user account on the src chain (separate fullnode)
	// funds user account on src chain in genesis
	relayer, channels, user, rlyCleanup, err := StartChainsAndRelayer(testName, ctx, pool, network, home, srcChain, dstChain, relayerImplementation, nil)
	if err != nil {
		return err
	}
	defer rlyCleanup()

	// will test a user sending an ibc transfer from the src chain to the dst chain
	// denom will be src chain native denom
	testDenom := srcChain.Config().Denom

	// query initial balance of user wallet for src chain native denom on the src chain
	srcInitialBalance, err := srcChain.GetBalance(ctx, user.SrcChainAddress, testDenom)
	if err != nil {
		return err
	}

	// get ibc denom for test denom on dst chain
	denomTrace := transfertypes.ParseDenomTrace(transfertypes.GetPrefixedDenom(channels[0].Counterparty.PortID, channels[0].Counterparty.ChannelID, testDenom))
	dstIbcDenom := denomTrace.IBCDenom()

	// query initial balance of user wallet for src chain native denom on the dst chain
	// don't care about error here, account does not exist on destination chain
	dstInitialBalance, _ := dstChain.GetBalance(ctx, user.DstChainAddress, dstIbcDenom)

	fmt.Printf("Initial balances: Src chain: %d\nDst chain: %d\n", srcInitialBalance, dstInitialBalance)

	// test coin, address is recipient of ibc transfer on dst chain
	testCoin := WalletAmount{
		Address: user.DstChainAddress,
		Denom:   testDenom,
		Amount:  1000000,
	}

	// send ibc transfer from the user wallet using its fullnode
	// timeout is nil so that it will use the default timeout
	txHash, err := srcChain.SendIBCTransfer(ctx, channels[0].ChannelID, user.KeyName, testCoin, nil)
	if err != nil {
		return err
	}

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

	// query final balance of user wallet for src chain native denom on the src chain
	srcFinalBalance, err := srcChain.GetBalance(ctx, user.SrcChainAddress, testDenom)
	if err != nil {
		return err
	}

	// query final balance of user wallet for src chain native denom on the dst chain
	dstFinalBalance, err := dstChain.GetBalance(ctx, user.DstChainAddress, dstIbcDenom)
	if err != nil {
		return err
	}

	fmt.Printf("First balance check: Source: %d, Destination: %d\n", srcFinalBalance, dstFinalBalance)

	totalFees := srcChain.GetGasFeesInNativeDenom(srcTx.GasWanted)
	expectedDifference := testCoin.Amount + totalFees

	if srcFinalBalance != srcInitialBalance-expectedDifference {
		return fmt.Errorf("source balances do not match. expected: %d, actual: %d", srcInitialBalance-expectedDifference, srcFinalBalance)
	}

	if dstFinalBalance != dstInitialBalance+testCoin.Amount {
		return fmt.Errorf("destination balances do not match. expected: %d, actual: %d", dstInitialBalance+testCoin.Amount, dstFinalBalance)
	}

	// IBC is confirmed working on 2.1.0, now use bad contract to halt chain

	executablePath, err := os.Executable()
	if err != nil {
		return err
	}
	rootPath := filepath.Dir(executablePath)
	contractPath := path.Join(rootPath, "assets", "badcontract.wasm")

	contractAddress, err := srcChain.InstantiateContract(ctx, userAccountKeyName, WalletAmount{Amount: 100, Denom: srcChain.Config().Denom}, contractPath, "{\"count\":0}", false)
	if err != nil {
		return err
	}

	resets := []int{0, 15, 84, 0, 84, 42, 55, 42, 15, 84, 42}

	for _, resetCount := range resets {
		// run reset
		if err := srcChain.ExecuteContract(ctx, userAccountKeyName, contractAddress, fmt.Sprintf("{\"reset\":{\"count\": %d}}", resetCount)); err != nil {
			return err
		}
		// halt happens here on the first 42 reset
		latestHeight, err := srcChain.WaitForBlocks(5)
		if err != nil {
			fmt.Println("Chain is halted")
			break
		}

		// dump current contract state
		res, err := srcChain.DumpContractState(ctx, contractAddress, latestHeight)
		if err != nil {
			return err
		}
		contractData, err := base64.StdEncoding.DecodeString(res.Models[1].Value)
		if err != nil {
			return err
		}
		fmt.Printf("Contract data: %s\n", contractData)

		// run increment a bunch of times.
		// Actual mainnet halt included this, but this test shows they are not necessary to cause halt
		// for i := 0; i < 5; i++ {
		// 	if err := srcChain.ExecuteContract(ctx, userAccountKeyName, contractAddress, "{\"increment\":{}}"); err != nil {
		// 		return err
		// 	}
		// 	if _, err := srcChain.WaitForBlocks(1); err != nil {
		// 		return err
		// 	}
		// }
	}

	haltHeight, err := srcChain.Height()
	if err != nil {
		return err
	}

	junoChainAsCosmosChain := srcChain.(*CosmosChain)

	// stop juno chain (2/3 consensus and user node) and relayer
	for i := 3; i < len(junoChainAsCosmosChain.ChainNodes); i++ {
		node := junoChainAsCosmosChain.ChainNodes[i]
		if err := node.StopContainer(); err != nil {
			return nil
		}
		_ = node.Pool.Client.RemoveContainer(docker.RemoveContainerOptions{ID: node.Container.ID})
	}

	// relayer should be stopped by now, but just in case
	_ = relayer.StopRelayer(ctx)

	// export state from first validator
	newGenesisJson, err := srcChain.ExportState(ctx, haltHeight)
	if err != nil {
		return err
	}

	fmt.Printf("New genesis json: %s\n", newGenesisJson)

	newGenesisJson = strings.ReplaceAll(newGenesisJson, fmt.Sprintf("\"initial_height\":%d", 0), fmt.Sprintf("\"initial_height\":%d", haltHeight+2))

	juno3Chain, err := GetChain(testName, "juno", "v3.0.0", "juno-1", 10, 1)
	if err != nil {
		return err
	}

	// write modified genesis file to 2/3 vals and fullnode
	for i := 3; i < len(junoChainAsCosmosChain.ChainNodes); i++ {
		if err := junoChainAsCosmosChain.ChainNodes[i].UnsafeResetAll(ctx); err != nil {
			return err
		}
		if err := os.WriteFile(junoChainAsCosmosChain.ChainNodes[i].GenesisFilePath(), []byte(newGenesisJson), 0644); err != nil {
			return err
		}
		junoChainAsCosmosChain.ChainNodes[i].Chain = juno3Chain
		if err := junoChainAsCosmosChain.ChainNodes[i].UnsafeResetAll(ctx); err != nil {
			return err
		}
	}

	if err := junoChainAsCosmosChain.ChainNodes.LogGenesisHashes(); err != nil {
		return err
	}

	for i := 3; i < len(junoChainAsCosmosChain.ChainNodes); i++ {
		node := junoChainAsCosmosChain.ChainNodes[i]
		if err := node.CreateNodeContainer(); err != nil {
			return err
		}
		if err := node.StartContainer(ctx); err != nil {
			return nil
		}
	}

	time.Sleep(1 * time.Minute)

	if _, err = srcChain.WaitForBlocks(5); err != nil {
		return err
	}

	// check IBC again
	// note: this requires relayer version with hack to use old RPC for blocks before the halt, and new RPC for blocks after new genesis
	if err = relayer.UpdateClients(ctx, testPathName); err != nil {
		return err
	}

	if err = relayer.StartRelayer(ctx, testPathName); err != nil {
		return err
	}

	// wait for relayer to start up
	time.Sleep(60 * time.Second)

	// send ibc transfer from the user wallet using its fullnode
	// timeout is nil so that it will use the default timeout
	txHash, err = srcChain.SendIBCTransfer(ctx, channels[0].ChannelID, user.KeyName, testCoin, nil)
	if err != nil {
		return err
	}

	// wait for both chains to produce 10 blocks
	if err := WaitForBlocks(srcChain, dstChain, 10); err != nil {
		return err
	}

	// fetch ibc transfer tx
	srcTx2, err := srcChain.GetTransaction(ctx, txHash)
	if err != nil {
		return err
	}

	fmt.Printf("Transaction:\n%v\n", srcTx2)

	// query final balance of user wallet for src chain native denom on the src chain
	srcFinalBalance2, err := srcChain.GetBalance(ctx, user.SrcChainAddress, testDenom)
	if err != nil {
		return err
	}

	// query final balance of user wallet for src chain native denom on the dst chain
	dstFinalBalance2, err := dstChain.GetBalance(ctx, user.DstChainAddress, dstIbcDenom)
	if err != nil {
		return err
	}

	totalFees = srcChain.GetGasFeesInNativeDenom(srcTx2.GasWanted)
	expectedDifference = testCoin.Amount + totalFees

	if srcFinalBalance2 != srcFinalBalance-expectedDifference {
		return fmt.Errorf("source balances do not match. expected: %d, actual: %d", srcFinalBalance-expectedDifference, srcFinalBalance2)
	}

	if dstFinalBalance2 != dstFinalBalance+testCoin.Amount {
		return fmt.Errorf("destination balances do not match. expected: %d, actual: %d", dstFinalBalance+testCoin.Amount, dstFinalBalance2)
	}

	return nil

}
