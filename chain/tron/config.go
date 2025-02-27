package tron

import "github.com/strangelove-ventures/interchaintest/v8/ibc"

func DefaultChainConfig(name string) ibc.ChainConfig {
	dockerImage := ibc.DockerImage{
		Repository: "starsquid/tron-daemon",
		Version:    "4.7.7",
		UIDGID:     "1025:1025",
	}

	return ibc.ChainConfig{
		Type:           "tron",
		Name:           name,
		ChainID:        "mocknet",
		Bech32Prefix:   "n/a",
		CoinType:       "195",
		Denom:          "trx",
		GasPrices:      "0.0001",
		GasAdjustment:  0,
		TrustingPeriod: "0",
		NoHostMount:    false,
		Images:         []ibc.DockerImage{dockerImage},
		Bin:            "echo",
		HostPortOverride: map[int]int{
			8090: 8090,
			8091: 8091,
		},
		Env: []string{
			"TRON_NODE_TYPE=mocknet-fullnode",
		},
	}
}
