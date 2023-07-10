package hyperspace

import (
	"github.com/strangelove-ventures/interchaintest/v7/ibc"
)

var _ ibc.Wallet = &HyperspaceWallet{}

type WalletModel struct {
	Mnemonic string `json:"mnemonic"`
	Address  string `json:"address"`
}

type HyperspaceWallet struct {
	mnemonic string
	address  string
	keyName  string
}

func NewWallet(keyname string, address string, mnemonic string) *HyperspaceWallet {
	return &HyperspaceWallet{
		mnemonic: mnemonic,
		address:  address,
		keyName:  keyname,
	}
}

func (w *HyperspaceWallet) KeyName() string {
	return w.keyName
}

func (w *HyperspaceWallet) FormattedAddress() string {
	return w.address
}

// Get mnemonic, only used for relayer wallets
func (w *HyperspaceWallet) Mnemonic() string {
	return w.mnemonic
}

// Get Address
func (w *HyperspaceWallet) Address() []byte {
	return []byte(w.address)
}
