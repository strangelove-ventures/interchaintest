package types

import (
	"fmt"
	"os"

	"github.com/strangelove-ventures/interchaintest/v8/chain/cosmos"
)

func CosmosHubChain() {
	bech32 := "cosmos"

	cosmosHub := NewChainBuilder("gaia", "localcosmos-1", "gaiad", "uatom")
	cosmosHub.WithDockerImage(DockerImage{
		Version: "v10.0.1",
	})

	cosmosHub.WithBlockTime("500ms")
	cosmosHub.WithDebugging(true)

	cosmosHub.WithGenesis(Genesis{
		Modify: []cosmos.GenesisKV{
			cosmos.NewGenesisKV("app_state.gov.voting_params.voting_period", "15s"),
			cosmos.NewGenesisKV("app_state.gov.deposit_params.max_deposit_period", "15s"),
			cosmos.NewGenesisKV("app_state.gov.deposit_params.min_deposit.0.denom", cosmosHub.Denom),
		},
		Accounts: append(
			[]GenesisAccount{NewGenesisAccount("acc0", bech32, "25000000000%DENOM%", cosmosHub.CoinType, "decorate bright ozone fork gallery riot bus exhaust worth way bone indoor calm squirrel merry zero scheme cotton until shop any excess stage laundry")},
			GenerateRandomAccounts(10, bech32, cosmosHub.CoinType)...,
		),
	})

	currPath, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	if err := cosmosHub.SaveJSON(fmt.Sprintf("%s/chains/cosmoshubgenerated.json", currPath)); err != nil {
		panic(err)
	}
}
