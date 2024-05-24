package types

import (
	"fmt"

	"github.com/strangelove-ventures/interchaintest/v8/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v8/testutil"
)

func ChainCosmosHub() *Chain {
	cosmosHub := NewChainBuilder("gaia", "localcosmos-1", "gaiad", "uatom", "cosmos").SetDebugging(true)
	cosmosHub.SetBech32Prefix("cosmos")
	cosmosHub.SetBlockTime("500ms")
	cosmosHub.SetDockerImage(DockerImage{
		Version: "v16.0.0",
	})
	cosmosHub.SetGenesis(defaultSDKv47Genesis(cosmosHub))

	return cosmosHub
}

func ChainEthereum() *Chain {
	eth := NewChainBuilder("ethereum", "31337", "anvil", "wei", "0x").SetDebugging(true)
	eth.SetChainType("ethereum").
		SetCoinType(60).
		SetBech32Prefix("0x").
		SetGasPrices("0").
		SetTrustingPeriod("0").
		SetGasAdjustment(0).
		SetDockerImage(DockerImage{
			Repository: "ghcr.io/foundry-rs/foundry",
			Version:    "latest",
		}).
		SetHostPortOverride(map[string]string{
			"8545": "8545",
		}).
		SetGenesis(Genesis{}).
		SetConfigFileOverrides([]ConfigFileOverrides{
			{
				Paths: testutil.Toml{
					"--load-state": "../../chains/state/avs-and-eigenlayer-deployed-anvil-state.json",
				},
			},
		})

	return eth
}

func ChainJuno(chainID string) *Chain {
	juno := NewChainBuilder("juno", chainID, "junod", "ujuno", "juno").SetDebugging(true)
	juno.SetBlockTime("500ms")
	juno.SetDockerImage(DockerImage{
		Version: "v21.0.0",
	})
	juno.SetGenesis(defaultSDKv47Genesis(juno))
	return juno
}

func ChainStargaze() *Chain {
	stars := NewChainBuilder("stargaze", "localstars-1", "starsd", "ustars", "stars").SetDebugging(true)
	stars.SetBlockTime("500ms")
	stars.SetDockerImage(DockerImage{
		Version: "v13.0.0",
	})
	stars.SetGenesis(defaultSDKv47Genesis(stars))
	return stars
}

func ChainOsmosis() *Chain {
	stars := NewChainBuilder("stargaze", "localstars-1", "starsd", "ustars", "stars").SetDebugging(true)
	stars.SetBlockTime("500ms")
	stars.SetDockerImage(DockerImage{
		Version: "v13.0.0",
	})
	stars.SetGenesis(defaultSDKv47Genesis(stars))
	return stars
}

func ChainsIBC(chainA, chainB *Chain) (ChainsConfig, error) {
	if chainA.ChainID == chainB.ChainID {
		return ChainsConfig{}, fmt.Errorf("chainA and chainB cannot have the same ChainID for ChainsIBC")
	}

	matchingPath := false
	for _, pathA := range chainA.IBCPaths {
		for _, pathB := range chainB.IBCPaths {
			if pathA == pathB {
				matchingPath = true
				break
			}
		}
	}

	if !matchingPath {
		ibcPath := fmt.Sprintf("%s_%s", chainA.ChainID, chainB.ChainID)
		chainA.IBCPaths = []string{ibcPath}
		chainB.IBCPaths = []string{ibcPath}
	}

	return ChainsConfig{
		Chains: []Chain{
			*chainA,
			*chainB,
		},
	}, nil
}

func defaultSDKv47Genesis(chain *Chain) Genesis {
	return Genesis{
		Modify: []cosmos.GenesisKV{
			cosmos.NewGenesisKV("app_state.gov.params.voting_period", "15s"),
			cosmos.NewGenesisKV("app_state.gov.params.max_deposit_period", "15s"),
			cosmos.NewGenesisKV("app_state.gov.params.min_deposit.0.denom", chain.Denom),
		},
		Accounts: append(
			[]GenesisAccount{NewGenesisAccount(
				"acc0", chain.Bech32Prefix, "25000000000%DENOM%", chain.CoinType,
				"decorate bright ozone fork gallery riot bus exhaust worth way bone indoor calm squirrel merry zero scheme cotton until shop any excess stage laundry",
			)},
			GenerateRandomAccounts(5, chain.Bech32Prefix, chain.CoinType)...,
		),
	}
}
