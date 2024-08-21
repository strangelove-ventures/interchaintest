package geth

import (
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
)

func DefaultEthereumGethChainConfig(
	name string,
) ibc.ChainConfig {
	return ibc.ChainConfig{
		Type:           "ethereum",
		Name:           name,
		ChainID:        "1337", // default geth chain-id
		Bech32Prefix:   "n/a",
		CoinType:       "60",
		Denom:          "wei",
		GasPrices:      "2000000000", // 2gwei, default 1M
		GasAdjustment:  0,
		TrustingPeriod: "0",
		NoHostMount:    false,
		Images: []ibc.DockerImage{
			{
				Repository: "ethereum/client-go",
				Version:    "v1.14.7",
				UidGid:     "1025:1025",
			},
		},
		Bin: "geth",
		AdditionalStartArgs: []string{
			"--dev.period", "2", // 2 second block time
			"--verbosity", "4", // Level = debug
			"--networkid", "15",
			"--rpc.txfeecap", "50.0", // 50 eth
			"--rpc.gascap", "30000000", //30M
			"--gpo.percentile", "150", // default 60
			"--gpo.ignoreprice", "1000000000", // 1gwei, default 2
			"--dev.gaslimit", "30000000", // 30M, default 11.5M
		},
	}
}
