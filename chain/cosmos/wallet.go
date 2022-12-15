package cosmos

import (
	"github.com/strangelove-ventures/ibctest/v6/ibc"
	"github.com/cosmos/cosmos-sdk/types"
)

var _ ibc.Wallet = &CosmosWallet{}

type CosmosWallet struct {
	mnemonic string
	address  []byte
	keyName  string
	chainCfg ibc.ChainConfig
}

func NewWallet(keyname string, address []byte, mnemonic string, chainCfg ibc.ChainConfig) ibc.Wallet {
	return &CosmosWallet{
		mnemonic: mnemonic,
		address: address,
		keyName: keyname,
		chainCfg: chainCfg,
	}
}

func (w *CosmosWallet) GetKeyName() string {
	return w.keyName
}

// Get formatted address, passing in a prefix
func (w *CosmosWallet) GetFormattedAddress(prefix string) string {
	return types.MustBech32ifyAddressBytes(prefix, w.address)
}

// Get mnemonic, only used for relayer wallets
func (w *CosmosWallet) GetMnemonic() string {
	return w.mnemonic
}

// Get Address with chain's prefix
func (w *CosmosWallet) GetAddress() string {
	return types.MustBech32ifyAddressBytes(w.chainCfg.Bech32Prefix, w.address)
}