package rly

import (
	"github.com/strangelove-ventures/ibctest/v6/ibc"
	"github.com/cosmos/cosmos-sdk/types"
)

var _ ibc.Wallet = &RlyWallet{}

type RlyWallet struct {
	Mnemonic string `json:"mnemonic"`
	Address  string `json:"address"`
	keyName  string
}

func NewWallet(keyname string, address string, mnemonic string) *RlyWallet {
	return &RlyWallet{
		Mnemonic: mnemonic,
		Address: address,
		keyName: keyname,
	}
}

func (w *RlyWallet) GetKeyName() string {
	return w.keyName
}

func (w *RlyWallet) GetFormattedAddress(prefix string) string {
	return types.MustBech32ifyAddressBytes(prefix, []byte(w.Address))
}

// Get mnemonic, only used for relayer wallets
func (w *RlyWallet) GetMnemonic() string {
	return w.Mnemonic
}

// Get Address
func (w *RlyWallet) GetAddress() string {
	return w.Address
}