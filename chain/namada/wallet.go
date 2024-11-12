package namada

import (
	"fmt"

	"github.com/strangelove-ventures/interchaintest/v8/ibc"
)

var _ ibc.Wallet = &NamadaWallet{}

// NamadaWallet represents a wallet for the Namada application.
type NamadaWallet struct {
	mnemonic string
	address  []byte
	keyName  string
	chainCfg ibc.ChainConfig
}

// NewWallet creates a new instance of NamadaWallet with the provided parameters.
func NewWallet(keyname string, address []byte, mnemonic string, chainCfg ibc.ChainConfig) *NamadaWallet {
	return &NamadaWallet{
		mnemonic: mnemonic,
		address:  address,
		keyName:  keyname,
		chainCfg: chainCfg,
	}
}

// KeyName returns the key name associated with a NamadaWallet instance.
func (w *NamadaWallet) KeyName() string {
	return w.keyName
}

// FormattedAddress returns the formatted address associated with a NamadaWallet instance.
// If the account is shielded, it returns the payment address.
func (w *NamadaWallet) FormattedAddress() string {
	return string(w.address)
}

// Mnemonic returns the mnemonic associated with a NamadaWallet instance.
func (w *NamadaWallet) Mnemonic() string {
	return w.mnemonic
}

// Address returns the slice of bytes representing this NamadaWallet instance's address.
func (w *NamadaWallet) Address() []byte {
	return w.address
}

// FormattedAddressWithPrefix returns the formatted address string with a given prefix.
// The prefix is a string that will be appended to the beginning of the address.
// It takes the address stored in the NamadaWallet instance and converts it to a string.
func (w *NamadaWallet) FormattedAddressWithPrefix(prefix string) string {
	return string(w.address)
}

func (w *NamadaWallet) PaymentAddressKeyName() string {
	return fmt.Sprintf("%s-payment", w.keyName)
}
