package avalanche

import (
	"github.com/strangelove-ventures/interchaintest/v7/ibc"
)

var _ ibc.Wallet = &AvalancheWallet{}

type AvalancheWallet struct {
	mnemonic string
	address  []byte
	keyName  string
	chainCfg ibc.ChainConfig
}

func NewWallet(keyname string, address []byte, mnemonic string, chainCfg ibc.ChainConfig) *AvalancheWallet {
	return &AvalancheWallet{
		mnemonic: mnemonic,
		address:  address,
		keyName:  keyname,
		chainCfg: chainCfg,
	}
}

func (w *AvalancheWallet) KeyName() string {
	return w.keyName
}

func (w *AvalancheWallet) Mnemonic() string {
	return w.mnemonic
}

func (w *AvalancheWallet) Address() []byte {
	return w.address
}

func (w *AvalancheWallet) FormattedAddress() string {
	// ToDo: show formatted address
	panic("ToDo: implement me")
}
