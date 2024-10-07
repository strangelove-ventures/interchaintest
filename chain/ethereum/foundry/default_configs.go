package foundry

import (
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
)

func DefaultEthereumAnvilChainConfig(
	name string,
) ibc.ChainConfig {
	return ibc.ChainConfig{
		Type:           "ethereum",
		Name:           name,
		ChainID:        "31337", // default anvil chain-id
		Bech32Prefix:   "n/a",
		CoinType:       "60",
		Denom:          "wei",
		GasPrices:      "20000000000", // 20 gwei
		GasAdjustment:  0,
		TrustingPeriod: "0",
		NoHostMount:    false,
		Images: []ibc.DockerImage{
			{
				Repository: "ghcr.io/foundry-rs/foundry",
				Version:    "latest",
				UIDGID:     "1000:1000",
			},
		},
		Bin: "anvil",
		AdditionalStartArgs: []string{
			"--block-time", "2", // 2 second block times
			"--accounts", "10", // We current only use the first account for the faucet, but tests may expect the default
			"--balance", "10000000", // Genesis accounts loaded with 10mil ether, change as needed
			"--block-base-fee-per-gas", "0",
		},
	}
}
