package polkadot

import (
	"github.com/strangelove-ventures/ibctest/v6/ibc"
	"github.com/cosmos/cosmos-sdk/types"
)

var _ ibc.Wallet = &PolkadotWallet{}

type PolkadotWallet struct {
	mnemonic string
	address  []byte
	keyName  string
	chainCfg ibc.ChainConfig
}

func NewWallet(keyname string, address []byte, mnemonic string, chainCfg ibc.ChainConfig) *PolkadotWallet {
	return &PolkadotWallet{
		mnemonic: mnemonic,
		address: address,
		keyName: keyname,
		chainCfg: chainCfg,
	}
}

func (w *PolkadotWallet) GetKeyName() string {
	return w.keyName
}

// TODO Change to SS58
func (w *PolkadotWallet) GetFormattedAddress(prefix string) string {
	return types.MustBech32ifyAddressBytes(prefix, w.address)
}

// Get mnemonic, only used for relayer wallets
func (w *PolkadotWallet) GetMnemonic() string {
	return w.mnemonic
}

// Get Address
// TODO Change to SS58
func (w *PolkadotWallet) GetAddress() string {
	return types.MustBech32ifyAddressBytes(w.chainCfg.Bech32Prefix, w.address)
}