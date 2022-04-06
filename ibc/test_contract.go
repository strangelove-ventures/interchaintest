package ibc

import (
	"encoding/base64"
	"fmt"
	"os"
	"path"
	"path/filepath"

	"github.com/cosmos/cosmos-sdk/types"
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
