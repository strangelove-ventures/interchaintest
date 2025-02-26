package tron

import "github.com/strangelove-ventures/interchaintest/v8/ibc"

func DefaultChainConfig(name string) ibc.ChainConfig {
	return ibc.ChainConfig{
		Type:           "tron",
		Name:           name,
		ChainID:        "732465",
		Bech32Prefix:   "n/a",
		CoinType:       "195",
		Denom:          "trx",
		GasPrices:      "0.0001",
		GasAdjustment:  0,
		TrustingPeriod: "0",
		NoHostMount:    false,
		Images: []ibc.DockerImage{
			{
				Repository: "starsquid/tron-daemon",
				Version:    "latest",
				UIDGID:     "1025:1025",
			},
		},
		Bin:              "echo",
		HostPortOverride: map[int]int{},
	}
}
