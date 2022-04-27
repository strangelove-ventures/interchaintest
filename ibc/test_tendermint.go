package ibc

import (
	"fmt"
	"io/ioutil"
	"strings"
	"time"

	"github.com/ory/dockertest/docker"
)

// USAGE: `./ibc-test-framework test -s juno:v2.3.0 --src-chain-id juno-1 --src-vals 9 TwoThrirdMajorityGoodValidatorsWorks`
//
// Tendermint requires a 2/3+ majority.
// This test has 9 validators: 7 with the original genesis file and 2 with an invalid genesis file.
// Which means this test should not halt.
func (ibc IBCTestCase) TwoThrirdMajorityGoodValidatorsWorks(testName string, chainFactory ChainFactory, relayerImplementation RelayerImplementation) error {
	ctx, home, pool, network, cleanup, err := SetupTestRun(testName)
	if err != nil {
		return err
	}
	defer cleanup()

	srcChain, _, err := chainFactory.Pair(testName)
	if err != nil {
		return err
	}

	if err := srcChain.Initialize(testName, home, pool, network); err != nil {
		return err
	}

	if err := srcChain.Start(testName, ctx, []WalletAmount{}); err != nil {
		return err
	}

	haltHeight, err := srcChain.WaitForBlocks(10)
	if err != nil {
		fmt.Println("Err: Chain is halted")
		return err
	}

	fmt.Println("haltHeight: ", haltHeight)

	// Stop 1/3 of the validators:
	junoChain := srcChain.(*CosmosChain)

	// HACk: Currently the last node in junoChain.ChainNodes[] is a full node.
	// We will stop that node, but we wont change its genesis file.
	// We need to clean this up to be less brittle.

	// NOTE: so that the evil containers can startup without error,
	// we are going to create a chain halt by stopping one of the good validators so that
	// they don't have a 2/3+ majority to vote on new blocks while we are editing their genesis files.
	for i := 6; i < len(junoChain.ChainNodes); i++ {
		node := junoChain.ChainNodes[i]
		if err := node.StopContainer(); err != nil {
			return err
		}
		_ = node.Pool.Client.RemoveContainer(docker.RemoveContainerOptions{ID: node.Container.ID})
	}

	newGenesisJson, err := srcChain.ExportState(ctx, haltHeight)
	if err != nil {
		return err
	}

	fmt.Printf("New genesis json: %s\n", newGenesisJson)
	newGenesisJson = strings.ReplaceAll(newGenesisJson, fmt.Sprintf("\"initial_height\":%d", 0), fmt.Sprintf("\"initial_height\":%d", haltHeight+2))

	// Write modified genesis file to 1/3 - 1 of the vals (ignoring the last full node)
	for i := 7; i < len(junoChain.ChainNodes)-1; i++ {
		if err := junoChain.ChainNodes[i].UnsafeResetAll(ctx); err != nil {
			return err
		}
		if err := ioutil.WriteFile(junoChain.ChainNodes[i].GenesisFilePath(), []byte(newGenesisJson), 0644); err != nil {
			return err
		}
		if err := junoChain.ChainNodes[i].UnsafeResetAll(ctx); err != nil {
			return err
		}
	}

	if err := junoChain.ChainNodes.LogGenesisHashes(); err != nil {
		return err
	}

	// Start the evil vals + fullnode
	for i := 7; i < len(junoChain.ChainNodes); i++ {
		node := junoChain.ChainNodes[i]
		if err := node.CreateNodeContainer(); err != nil {
			return err
		}
		if err := node.StartContainer(ctx); err != nil {
			return err
		}
	}

	// Now that the evil nodes are up, start the stopped good node to restart the chain:
	node := junoChain.ChainNodes[6]
	if err := node.CreateNodeContainer(); err != nil {
		return err
	}
	if err := node.StartContainer(ctx); err != nil {
		return err
	}

	time.Sleep(1 * time.Minute)

	finalHeight, err := srcChain.WaitForBlocks(5)
	if err != nil {
		// NOTE: This shouldnt happen in this case
		fmt.Println("!!!Unexpected Chain Halt!!!")
		return err
	}

	fmt.Println("Final height: ", finalHeight)

	return nil
}

// USAGE: `./ibc-test-framework test -s juno:v2.3.0 --src-chain-id juno-1 --src-vals 9 OneThirdEvilValidatorsHalts`
func (ibc IBCTestCase) OneThirdEvilValidatorsHalts(testName string, chainFactory ChainFactory, relayerImplementation RelayerImplementation) error {
	ctx, home, pool, network, cleanup, err := SetupTestRun(testName)
	if err != nil {
		return err
	}
	defer cleanup()

	srcChain, _, err := chainFactory.Pair(testName)
	if err != nil {
		return err
	}

	if err := srcChain.Initialize(testName, home, pool, network); err != nil {
		return err
	}

	if err := srcChain.Start(testName, ctx, []WalletAmount{}); err != nil {
		return err
	}

	haltHeight, err := srcChain.WaitForBlocks(10)
	if err != nil {
		fmt.Println("Err: Chain is halted")
		return err
	}

	fmt.Println("haltHeight: ", haltHeight)

	// Stop 1/3 of the validators
	junoChain := srcChain.(*CosmosChain)

	// NOTE: Currently the last node in junoChain.ChainNodes[] is a full node
	for i := 6; i < len(junoChain.ChainNodes); i++ {
		node := junoChain.ChainNodes[i]
		if err := node.StopContainer(); err != nil {
			return err
		}
		_ = node.Pool.Client.RemoveContainer(docker.RemoveContainerOptions{ID: node.Container.ID})
	}

	newGenesisJson, err := srcChain.ExportState(ctx, haltHeight)
	if err != nil {
		return err
	}

	fmt.Printf("New genesis json: %s\n", newGenesisJson)
	newGenesisJson = strings.ReplaceAll(newGenesisJson, fmt.Sprintf("\"initial_height\":%d", 0), fmt.Sprintf("\"initial_height\":%d", haltHeight+2))

	// Write modified genesis file to 1/3 of the vals (ignoring the last node (full node) for now)
	for i := 6; i < len(junoChain.ChainNodes)-1; i++ {
		if err := junoChain.ChainNodes[i].UnsafeResetAll(ctx); err != nil {
			return err
		}
		if err := ioutil.WriteFile(junoChain.ChainNodes[i].GenesisFilePath(), []byte(newGenesisJson), 0644); err != nil {
			return err
		}
		if err := junoChain.ChainNodes[i].UnsafeResetAll(ctx); err != nil {
			return err
		}
	}

	if err := junoChain.ChainNodes.LogGenesisHashes(); err != nil {
		return err
	}

	// Start 1/3 of the vals + fullnode
	for i := 6; i < len(junoChain.ChainNodes); i++ {
		node := junoChain.ChainNodes[i]
		if err := node.CreateNodeContainer(); err != nil {
			return err
		}
		if err := node.StartContainer(ctx); err != nil {
			return err
		}
	}

	time.Sleep(1 * time.Minute)

	finalHeight, err := srcChain.WaitForBlocks(5)
	if err != nil {
		// TODO: Just make this test not pass if this time out error isnt returned
		fmt.Println("!!!Expected Chain Halt!!!")
		return err
	}

	fmt.Println("Halt height: ", haltHeight)
	fmt.Println("Final height: ", finalHeight)

	return nil
}
