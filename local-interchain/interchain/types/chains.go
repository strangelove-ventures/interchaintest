package types

import (
	"github.com/strangelove-ventures/interchaintest/v8/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"github.com/strangelove-ventures/interchaintest/v8/testutil"
)

func ChainCosmosHub(chainID string) *Chain {
	chain := NewChainBuilder("gaia", chainID, "gaiad", "uatom", "cosmos").SetDebugging(true)
	chain.SetBech32Prefix("cosmos")
	chain.SetBlockTime("500ms")
	chain.SetDockerImage(ibc.DockerImage{
		Version: "v16.0.0",
	})
	chain.SetGenesis(defaultSDKv47Genesis(chain))
	return chain
}

func ChainEthereum() *Chain {
	eth := NewChainBuilder("ethereum", "31337", "anvil", "wei", "0x").SetDebugging(true)
	eth.SetChainType("ethereum").
		SetCoinType(60).
		SetBech32Prefix("0x").
		SetGasPrices("0").
		SetTrustingPeriod("0").
		SetGasAdjustment(0).
		SetDockerImage(ibc.DockerImage{
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
	chain := NewChainBuilder("juno", chainID, "junod", "ujuno", "juno").SetDebugging(true)
	chain.SetBlockTime("500ms")
	chain.SetDockerImage(ibc.DockerImage{
		Version: "v21.0.0",
	})
	chain.SetGenesis(defaultSDKv47Genesis(chain))
	return chain
}

func ChainStargaze() *Chain {
	chain := NewChainBuilder("stargaze", "localstars-1", "starsd", "ustars", "stars").SetDebugging(true)
	chain.SetBlockTime("500ms")
	chain.SetDockerImage(ibc.DockerImage{
		Version: "v13.0.0",
	})
	chain.SetGenesis(defaultSDKv47Genesis(chain))
	return chain
}

func ChainOsmosis() *Chain {
	chain := NewChainBuilder("osmosis", "localosmo-1", "osmosisd", "uosmo", "osmo").SetDebugging(true)
	chain.SetBlockTime("500ms")
	chain.SetDockerImage(ibc.DockerImage{
		Version: "v25.0.0",
	})
	chain.SetGenesis(defaultSDKv47Genesis(chain))
	return chain
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
