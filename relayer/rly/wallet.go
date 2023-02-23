package rly

import (
	"github.com/strangelove-ventures/interchaintest/v7/ibc"
)

var _ ibc.Wallet = &RlyWallet{}

type WalletModel struct {
	Mnemonic string `json:"mnemonic"`
	Address  string `json:"address"`
}

type RlyWallet struct {
	mnemonic string
	address  string
	keyName  string
}

func NewWallet(keyname string, address string, mnemonic string) *RlyWallet {
	return &RlyWallet{
		mnemonic: mnemonic,
		address:  address,
		keyName:  keyname,
	}
}

func (w *RlyWallet) KeyName() string {
	return w.keyName
}

func (w *RlyWallet) FormattedAddress() string {
	return w.address
}

// Get mnemonic, only used for relayer wallets
func (w *RlyWallet) Mnemonic() string {
	return w.mnemonic
}

// Get Address
func (w *RlyWallet) Address() []byte {
	return []byte(w.address)
}
