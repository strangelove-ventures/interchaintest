package types

import (
	"os"

	"github.com/strangelove-ventures/interchaintest/v8/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v8/testutil"
)

var (
	currPath, _ = os.Getwd()
)

func ChainCosmosHub() *Chain {
	bech32 := "cosmos"

	cosmosHub := NewChainBuilder("gaia", "localcosmos-1", "gaiad", "uatom").SetDebugging(true)

	cosmosHub.SetBlockTime("500ms")
	cosmosHub.SetDockerImage(DockerImage{
		Version: "v16.0.0",
	})

	cosmosHub.SetGenesis(Genesis{
		Modify: []cosmos.GenesisKV{
			cosmos.NewGenesisKV("app_state.gov.params.voting_period", "15s"),
			cosmos.NewGenesisKV("app_state.gov.params.max_deposit_period", "15s"),
			cosmos.NewGenesisKV("app_state.gov.params.min_deposit.0.denom", cosmosHub.Denom),
		},
		Accounts: append(
			[]GenesisAccount{NewGenesisAccount("acc0", bech32, "25000000000%DENOM%", cosmosHub.CoinType, "decorate bright ozone fork gallery riot bus exhaust worth way bone indoor calm squirrel merry zero scheme cotton until shop any excess stage laundry")},
			GenerateRandomAccounts(5, bech32, cosmosHub.CoinType)...,
		),
	})

	return cosmosHub
}

func ChainEthereum() *Chain {
	eth := NewChainBuilder("ethereum", "31337", "anvil", "wei").SetDebugging(true)

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
