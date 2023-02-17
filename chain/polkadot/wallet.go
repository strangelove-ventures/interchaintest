package polkadot

import (
	"github.com/strangelove-ventures/interchaintest/v7/ibc"
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
		address:  address,
		keyName:  keyname,
		chainCfg: chainCfg,
	}
}

func (w *PolkadotWallet) KeyName() string {
	return w.keyName
}

func (w *PolkadotWallet) FormattedAddress() string {
	return string(w.address)
}

// Get mnemonic, only used for relayer wallets
func (w *PolkadotWallet) Mnemonic() string {
	return w.mnemonic
}

// Get Address
// TODO Change to SS58
func (w *PolkadotWallet) Address() []byte {
	return w.address
}
