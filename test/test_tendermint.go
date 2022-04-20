package ibc

import (
	"context"
	"fmt"

	"github.com/CosmWasm/wasmvm/types"
	"github.com/btcsuite/btcd/rpcclient"
	"github.com/ory/dockertest"
	"github.com/ory/dockertest/docker"
)

var chainConfigs = []ChainConfig{
	//NewCosmosHeighlinerChainConfig("huahua", "gaiad", "cosmos", "uatom", "0.01uatom", 1.3, "504h"),
	NewCosmosHeighlinerChainConfig("juno", "junod", "juno", "ujuno", "0.0025ujuno", 1.3, "672h"),
}

// ChainNode represents a node in the test network that is being created
type ChainNode struct {
	Home         string
	Index        int
	Chain        Chain
	GenesisCoins string
	Validator    bool
	NetworkID    string
	Pool         *dockertest.Pool
	Client       rpcclient.Client
	Container    *docker.Container
	testName     string
}

// ChainNodes is a collection of ChainNode
type ChainNodes []*ChainNode

// AddGenesisAccount adds a genesis account for each key <- potential source of disagreement
func (tn *ChainNode) AddGenesisAccount(ctx context.Context, address string, genesisAmount []types.Coin) error {
	amount := ""
	for i, coin := range genesisAmount {
		if i != 0 {
			amount += ","
		}
		amount += fmt.Sprintf("%d%s", coin.Amount.Int64(), coin.Denom)
	}
	command := []string{tn.Chain.Config().Bin, "add-genesis-account", address, amount,
		"--home", tn.NodeHome(),
	}
	return handleNodeJobError(tn.NodeJob(ctx, command))
}

// Create ValidatorSet with 9 ChainNodes that has 3 with genesis-1.json and 6 with genesis-2.json
// GitHub Copilot nonsense:
// func (tn *ChainNode) CreateValidatorSet(ctx context.Context, validatorSet []*ChainNode) error {
// 	for _, v := range validatorSet {
// 		if err := v.AddGenesisAccount(ctx, v.Chain.Config().Bech32Prefix+"valoper1", []types.Coin{types.NewCoin(v.Chain.Config().Denom, types.NewInt(1000000000))}); err != nil {
// 			return err
// 		}
// 	}
// 	return nil
// }


// ????

firstGenesisJson = strings.ReplaceAll(firstGenesisJson, fmt.Sprintf("\"initial_height\":%d", 0), fmt.Sprintf("\"initial_height\":%d", haltHeight+2))
secondGenesisJson = strings.ReplaceAll(secondGenesisJson, fmt.Sprintf("\"initial_height\":%d", 0), fmt.Sprintf("\"initial_height\":%d", haltHeight+2))

// PROFIT1!11