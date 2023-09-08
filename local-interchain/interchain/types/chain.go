package types

import (
	"github.com/go-playground/validator"
	"github.com/strangelove-ventures/interchaintest/v7/chain/cosmos"
)

type Chain struct {
	// ibc chain config (optional)
	ChainType      string `json:"chain_type" validate:"min=1"`
	CoinType       int    `json:"coin_type" validate:"gt=0"`
	Binary         string `json:"binary" validate:"min=1"`
	Bech32Prefix   string `json:"bech32_prefix" validate:"min=1"`
	Denom          string `json:"denom" validate:"min=1"`
	TrustingPeriod string `json:"trusting_period"`
	Debugging      bool   `json:"debugging"`
	BlockTime      string `json:"block_time"`

	// Required
	Name    string `json:"name" validate:"min=1"`
	ChainID string `json:"chain_id" validate:"min=3"`

	DockerImage DockerImage `json:"docker_image" validate:"url"`

	GasPrices     string   `json:"gas_prices"`
	GasAdjustment float64  `json:"gas_adjustment"`
	NumberVals    int      `json:"number_vals" validate:"gte=1"`
	NumberNode    int      `json:"number_node"`
	IBCPaths      []string `json:"ibc_paths"`
	Genesis       Genesis  `json:"genesis"`
}

func (chain *Chain) Validate() error {
	validate := validator.New()
	return validate.Struct(chain)
}

func (chain *Chain) SetChainDefaults() {
	if chain.ChainType == "" {
		chain.ChainType = "cosmos"
	}

	if chain.CoinType == 0 {
		chain.CoinType = 118
	}

	if chain.DockerImage.UidGid == "" {
		chain.DockerImage.UidGid = "1025:1025"
	}

	if chain.NumberVals == 0 {
		chain.NumberVals = 1
	}

	if chain.TrustingPeriod == "" {
		chain.TrustingPeriod = "112h"
	}

	if chain.BlockTime == "" {
		chain.BlockTime = "2s"
	}

	if chain.IBCPaths == nil {
		chain.IBCPaths = []string{}
	}

	// Genesis
	if chain.Genesis.StartupCommands == nil {
		chain.Genesis.StartupCommands = []string{}
	}
	if chain.Genesis.Accounts == nil {
		chain.Genesis.Accounts = []GenesisAccount{}
	}
	if chain.Genesis.Modify == nil {
		chain.Genesis.Modify = []cosmos.GenesisKV{}
	}

	// TODO: Error here instead?
	if chain.Binary == "" {
		panic("'binary' is required in your config for " + chain.ChainID)
	}
	if chain.Denom == "" {
		panic("'denom' is required in your config for " + chain.ChainID)
	}
	if chain.Bech32Prefix == "" {
		panic("'bech32_prefix' is required in your config for " + chain.ChainID)
	}
}
