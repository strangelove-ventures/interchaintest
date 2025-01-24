package types

import (
	"github.com/strangelove-ventures/interchaintest/v9/ibc"
	"github.com/strangelove-ventures/interchaintest/v9/testutil"
)

func ChainCosmosHub(chainID string) *Chain {
	chain := NewChainBuilder("gaia", chainID, "gaiad", "uatom", "cosmos").SetDebugging(true)
	chain.SetBech32Prefix("cosmos")
	chain.SetBlockTime("500ms")
	chain.SetDockerImage(ibc.DockerImage{
		Version: "v16.0.0",
	})
	chain.SetDefaultSDKv47Genesis(5)
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
	chain.SetDefaultSDKv47Genesis(5)
	chain.SetStartupCommands("%BIN% keys add example-key-after --keyring-backend test --home %HOME%")
	return chain
}

func ChainStargaze() *Chain {
	chain := NewChainBuilder("stargaze", "localstars-1", "starsd", "ustars", "stars").SetDebugging(true)
	chain.SetBlockTime("500ms")
	chain.SetDockerImage(ibc.DockerImage{
		Version: "v13.0.0",
	})
	chain.SetDefaultSDKv47Genesis(5)
	return chain
}

func ChainOsmosis() *Chain {
	chain := NewChainBuilder("osmosis", "localosmo-1", "osmosisd", "uosmo", "osmo").SetDebugging(true)
	chain.SetBlockTime("500ms")
	chain.SetDockerImage(ibc.DockerImage{
		Version: "v25.0.0",
	})
	chain.SetDefaultSDKv47Genesis(5)
	return chain
}
