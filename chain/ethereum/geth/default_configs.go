package geth

import (
	"github.com/strangelove-ventures/interchaintest/v9/ibc"
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
				UIDGID:     "1025:1025",
			},
		},
		Bin: "geth",
		AdditionalStartArgs: []string{
			"--dev.period", "2", // 2 second block time
			"--verbosity", "4", // Level = debug
			"--networkid", "15",
			"--rpc.txfeecap", "50.0", // 50 eth
			"--rpc.gascap", "30000000", // 30M
			"--gpo.percentile", "150", // default 60
			"--gpo.ignoreprice", "1000000000", // 1gwei, default 2
			"--dev.gaslimit", "30000000", // 30M, default 11.5M
			"--rpc.enabledeprecatedpersonal", // required (in this version) for recover key and unlocking accounts
		},
	}
}

func DefaultBscChainConfig(
	name string,
) ibc.ChainConfig {
	return ibc.ChainConfig{
		Type:           "ethereum",
		Name:           name,
		ChainID:        "11337",
		Bech32Prefix:   "n/a",
		CoinType:       "60",
		Denom:          "wei",
		GasPrices:      "2000000000", // 2gwei, default 1M
		GasAdjustment:  0,
		TrustingPeriod: "0",
		NoHostMount:    false,
		Images: []ibc.DockerImage{
			{
				Repository: "ghcr.io/bnb-chain/bsc",
				Version:    "1.2.13", // same version as other sim tests
				// Version:    "1.4.13", // this version does not work in dev mode (1.3.x+)
				UIDGID: "1000:1000",
			},
		},
		Bin: "geth",
		AdditionalStartArgs: []string{
			"-mine",
			"--dev.period", "2", // 2 second block time
			"--verbosity", "4", // Level = debug
			"--networkid", "15",
			"--rpc.txfeecap", "50.0", // 50 eth
			"--rpc.gascap", "30000000", // 30M
			"--gpo.percentile", "150", // default 60
			"--gpo.ignoreprice", "1000000000", // 1gwei, default 2
			"--dev.gaslimit", "30000000", // 30M, default 11.5M
		},
	}
}
