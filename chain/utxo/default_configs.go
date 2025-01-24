package utxo

import (
	"fmt"

	"github.com/strangelove-ventures/interchaintest/v9/ibc"
)

func DefaultBitcoinChainConfig(
	name string,
	rpcUser string,
	rpcPassword string,
) ibc.ChainConfig {
	return ibc.ChainConfig{
		Type:           "utxo",
		Name:           name,
		ChainID:        name,
		Bech32Prefix:   "n/a",
		CoinType:       "0",
		Denom:          "sat",
		GasPrices:      "0.00001", // min fee / kb
		GasAdjustment:  4,         // min fee multiplier
		TrustingPeriod: "0",
		NoHostMount:    false,
		Images: []ibc.DockerImage{
			{
				Repository: "bitcoin/bitcoin",
				Version:    "26.2",
				UIDGID:     "1025:1025",
			},
		},
		Bin: "bitcoind,bitcoin-cli",
		AdditionalStartArgs: []string{
			fmt.Sprintf("-rpcuser=%s", rpcUser),
			fmt.Sprintf("-rpcpassword=%s", rpcPassword),
			"-fallbackfee=0.0005",
			"-mintxfee=0.00001",
		},
	}
}

func DefaultBitcoinCashChainConfig(
	name string,
	rpcUser string,
	rpcPassword string,
) ibc.ChainConfig {
	return ibc.ChainConfig{
		Type:           "utxo",
		Name:           name,
		ChainID:        name,
		Bech32Prefix:   "n/a",
		CoinType:       "145",
		Denom:          "sat",
		GasPrices:      "0.00001", // min fee / kb
		GasAdjustment:  4,         // min fee multiplier
		TrustingPeriod: "0",
		NoHostMount:    false,
		Images: []ibc.DockerImage{
			{
				Repository: "zquestz/bitcoin-cash-node",
				Version:    "27.1.0",
				UIDGID:     "1025:1025",
			},
		},
		Bin: "bitcoind,bitcoin-cli",
		AdditionalStartArgs: []string{
			fmt.Sprintf("-rpcuser=%s", rpcUser),
			fmt.Sprintf("-rpcpassword=%s", rpcPassword),
			"-fallbackfee=0.0005",
			"-mintxfee=0.00001",
		},
	}
}

func DefaultLitecoinChainConfig(
	name string,
	rpcUser string,
	rpcPassword string,
) ibc.ChainConfig {
	return ibc.ChainConfig{
		Type:           "utxo",
		Name:           name,
		ChainID:        name,
		Bech32Prefix:   "n/a",
		CoinType:       "2",
		Denom:          "sat",
		GasPrices:      "0.0001", // min fee / kb
		GasAdjustment:  4,        // min fee multiplier
		TrustingPeriod: "0",
		NoHostMount:    false,
		Images: []ibc.DockerImage{
			{
				Repository: "uphold/litecoin-core",
				Version:    "0.21",
				UIDGID:     "1025:1025",
			},
		},
		Bin: "litecoind,litecoin-cli",
		AdditionalStartArgs: []string{
			fmt.Sprintf("-rpcuser=%s", rpcUser),
			fmt.Sprintf("-rpcpassword=%s", rpcPassword),
			"-fallbackfee=0.005",
			"-mintxfee=0.0001",
		},
	}
}

func DefaultDogecoinChainConfig(
	name string,
	rpcUser string,
	rpcPassword string,
) ibc.ChainConfig {
	return ibc.ChainConfig{
		Type:           "utxo",
		Name:           name,
		ChainID:        name,
		Bech32Prefix:   "n/a",
		CoinType:       "3",
		Denom:          "sat",
		GasPrices:      "0.01", // min fee / kb
		GasAdjustment:  4,      // min fee multiplier
		TrustingPeriod: "0",
		NoHostMount:    false,
		Images: []ibc.DockerImage{
			{
				Repository: "registry.gitlab.com/thorchain/devops/node-launcher",
				Version:    "dogecoin-daemon-1.14.7",
				// Repository: "coinmetrics/dogecoin",
				// Version:    "1.14.7",
				UIDGID: "1000:1000",
			},
		},
		Bin: "dogecoind,dogecoin-cli",
		AdditionalStartArgs: []string{
			fmt.Sprintf("-rpcuser=%s", rpcUser),
			fmt.Sprintf("-rpcpassword=%s", rpcPassword),
			"-fallbackfee=0.5",
			"-mintxfee=0.01",
		},
	}
}
