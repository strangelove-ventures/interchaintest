package utxo

import (
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
)

var _ ibc.Wallet = &UtxoWallet{}

type UtxoWallet struct {
	address string
	keyName string
}

func NewWallet(keyname string, address string) ibc.Wallet {
	return &UtxoWallet{
		address: address,
		keyName: keyname,
	}
}

func (w *UtxoWallet) KeyName() string {
	return w.keyName
}

// Get formatted address, passing in a prefix
func (w *UtxoWallet) FormattedAddress() string {
	return w.address
}

// Get mnemonic, only used for relayer wallets
func (w *UtxoWallet) Mnemonic() string {
	return ""
}

// Get Address with chain's prefix
func (w *UtxoWallet) Address() []byte {
	return []byte(w.address)
}