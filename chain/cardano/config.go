package cardano

import "github.com/strangelove-ventures/interchaintest/v8/ibc"

func DefaultConfig(
	name string,
) ibc.ChainConfig {
	return ibc.ChainConfig{
		Type:           "ada",
		Name:           name,
		ChainID:        "1234",
		Bech32Prefix:   "addr_test",
		CoinType:       "144",
		Denom:          "lovelace",
		GasPrices:      "180000", // flat fee
		GasAdjustment:  0,        // N/A
		TrustingPeriod: "0",
		NoHostMount:    false,
		Images: []ibc.DockerImage{
			{
				Repository: "kocubinski/cardano-devnet",
				Version:    "0.1.7",
			},
		},
		//Bin: "rippled,/opt/ripple/bin/validator-keys",
		HostPortOverride: map[int]int{
			7007: 7007,
			3001: 3001,
		},
	}
}
