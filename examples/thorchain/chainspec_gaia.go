package thorchain_test

import (
	"github.com/strangelove-ventures/interchaintest/v8"
	"github.com/strangelove-ventures/interchaintest/v8/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v8/chain/thorchain/common"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
)

func GaiaChainSpec() *interchaintest.ChainSpec {
	name := common.GAIAChain.String() // Must use this name for tests
	version := "v18.1.0"
	numVals := 1
	numFn := 0
	denom := "uatom"
	gasPrices := "0.01uatom"
	genesisKVMods := []cosmos.GenesisKV{
		cosmos.NewGenesisKV("app_state.feemarket.params.enabled", false),
		cosmos.NewGenesisKV("app_state.feemarket.params.min_base_gas_price", "0.001000000000000000"),
		cosmos.NewGenesisKV("app_state.feemarket.state.base_gas_price", "0.001000000000000000"),
	}

	defaultChainConfig := ibc.ChainConfig{
		Denom:          denom,
		GasPrices:      gasPrices,
		ChainID:        "localgaia",
		ModifyGenesis:  cosmos.ModifyGenesis(genesisKVMods),
	}

	return &interchaintest.ChainSpec{
		Name:          "gaia",
		ChainName:     name,
		Version:       version,
		ChainConfig:   defaultChainConfig,
		NumValidators: &numVals,
		NumFullNodes:  &numFn,
	}
}
