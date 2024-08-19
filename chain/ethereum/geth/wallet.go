package geth

import (
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
)

var _ ibc.Wallet = &EthereumWallet{}

type EthereumWallet struct {
	address []byte
	keyName string
}

func NewWallet(keyname string, address []byte) ibc.Wallet {
	return &EthereumWallet{
		address: address,
		keyName: keyname,
	}
}

func (w *EthereumWallet) KeyName() string {
	return w.keyName
}

// Get formatted address, passing in a prefix
func (w *EthereumWallet) FormattedAddress() string {
	return hexutil.Encode(w.address)
}

// Get mnemonic, only used for relayer wallets
func (w *EthereumWallet) Mnemonic() string {
	return ""
}

// Get Address with chain's prefix
func (w *EthereumWallet) Address() []byte {
	return w.address
}
