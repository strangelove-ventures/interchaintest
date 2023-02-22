package polkadot

import (
	"github.com/strangelove-ventures/interchaintest/v7/ibc"
)

var _ ibc.Wallet = &Wallet{}

type Wallet struct {
	mnemonic string
	address  []byte
	keyName  string
	chainCfg ibc.ChainConfig
}

func NewWallet(keyname string, address []byte, mnemonic string, chainCfg ibc.ChainConfig) *Wallet {
	return &Wallet{
		mnemonic: mnemonic,
		address:  address,
		keyName:  keyname,
		chainCfg: chainCfg,
	}
}

func (w *Wallet) KeyName() string {
	return w.keyName
}

func (w *Wallet) FormattedAddress() string {
	return string(w.address)
}

// Get mnemonic, only used for relayer wallets
func (w *Wallet) Mnemonic() string {
	return w.mnemonic
}

// Get Address
// TODO Change to SS58
func (w *Wallet) Address() []byte {
	return w.address
}
