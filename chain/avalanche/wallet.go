package avalanche

import (
	"github.com/ethereum/go-ethereum/common"

	"github.com/strangelove-ventures/interchaintest/v8/ibc"
)

var _ ibc.Wallet = &AvalancheWallet{}

type AvalancheWallet struct {
	mnemonic string
	address  common.Address
	keyName  string
	chainCfg ibc.ChainConfig
}

func NewWallet(keyname string, address common.Address, mnemonic string, chainCfg ibc.ChainConfig) *AvalancheWallet {
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
	return w.address.Bytes()
}

func (w *AvalancheWallet) FormattedAddress() string {
	return w.address.String()
}
