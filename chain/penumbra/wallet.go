package penumbra

import (
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
)

var _ ibc.Wallet = &PenumbraWallet{}

// PenumbraWallet represents a wallet for the Penumbra application.
type PenumbraWallet struct {
	mnemonic string
	address  []byte
	keyName  string
	chainCfg ibc.ChainConfig
}

// NewWallet creates a new instance of PenumbraWallet with the provided parameters.
func NewWallet(keyname string, address []byte, mnemonic string, chainCfg ibc.ChainConfig) *PenumbraWallet {
	return &PenumbraWallet{
		mnemonic: mnemonic,
		address:  address,
		keyName:  keyname,
		chainCfg: chainCfg,
	}
}

// KeyName returns the key name associated with a PenumbraWallet instance.
func (w *PenumbraWallet) KeyName() string {
	return w.keyName
}

// FormattedAddress returns the formatted address associated with a PenumbraWallet instance.
func (w *PenumbraWallet) FormattedAddress() string {
	return string(w.address)
}

// Mnemonic returns the mnemonic associated with a PenumbraWallet instance.
func (w *PenumbraWallet) Mnemonic() string {
	return w.mnemonic
}

// Address returns the slice of bytes representing this PenumbraWallet instance's address.
func (w *PenumbraWallet) Address() []byte {
	return w.address
}

// FormattedAddressWithPrefix returns the formatted address string with a given prefix.
// The prefix is a string that will be appended to the beginning of the address.
// It takes the address stored in the PenumbraWallet instance and converts it to a string.
func (w *PenumbraWallet) FormattedAddressWithPrefix(prefix string) string {
	return string(w.address)
}
