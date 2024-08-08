package utxo

import (
	"fmt"

	"github.com/strangelove-ventures/interchaintest/v8/ibc"
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
		GasPrices:      "0",
		GasAdjustment:  0,
		TrustingPeriod: "0",
		NoHostMount:    false,
		Images: []ibc.DockerImage{
			{
				Repository: "bitcoin/bitcoin",
				Version:    "26.2",
				UidGid:     "1025:1025",
			},
		},
		Bin: "bitcoind,bitcoin-cli",
		AdditionalStartArgs: []string{
			fmt.Sprintf("-rpcuser=%s", rpcUser),
			fmt.Sprintf("-rpcpassword=%s", rpcPassword),
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
		GasPrices:      "0",
		GasAdjustment:  0,
		TrustingPeriod: "0",
		NoHostMount:    false,
		Images: []ibc.DockerImage{
			{
				Repository: "zquestz/bitcoin-cash-node",
				Version:    "27.1.0",
				UidGid:     "1025:1025",
			},
		},
		Bin: "bitcoind,bitcoin-cli",
		AdditionalStartArgs: []string{
			fmt.Sprintf("-rpcuser=%s", rpcUser),
			fmt.Sprintf("-rpcpassword=%s", rpcPassword),
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
		GasPrices:      "0",
		GasAdjustment:  0,
		TrustingPeriod: "0",
		NoHostMount:    false,
		Images: []ibc.DockerImage{
			{
				Repository: "uphold/litecoin-core",
				Version:    "0.21",
				UidGid:     "1025:1025",
			},
		},
		Bin: "litecoind,litecoin-cli",
		AdditionalStartArgs: []string{
			fmt.Sprintf("-rpcuser=%s", rpcUser),
			fmt.Sprintf("-rpcpassword=%s", rpcPassword),
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
		GasPrices:      "0",
		GasAdjustment:  0,
		TrustingPeriod: "0",
		NoHostMount:    false,
		Images: []ibc.DockerImage{
			{
				Repository: "registry.gitlab.com/thorchain/devops/node-launcher",
				Version:    "dogecoin-daemon-1.14.7",
				//Repository: "coinmetrics/dogecoin",
				//Version:    "1.14.7",
				UidGid:     "1000:1000",
			},
		},
		Bin: "dogecoind,dogecoin-cli",
		AdditionalStartArgs: []string{
			fmt.Sprintf("-rpcuser=%s", rpcUser),
			fmt.Sprintf("-rpcpassword=%s", rpcPassword),
		},
	}
}