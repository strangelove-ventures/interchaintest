package cardano

import (
	"github.com/kocubinski/gardano/address"

	"github.com/strangelove-ventures/interchaintest/v8/ibc"
)

var _ ibc.Wallet = (*wallet)(nil)

type wallet struct {
	keyName  string
	address  address.Address
	mnemonic string
}

func (w *wallet) Address() []byte {
	return w.address
}

func (w *wallet) FormattedAddress() string {
	return w.address.String()
}

func (w *wallet) KeyName() string {
	return w.keyName
}

// Mnemonic implements ibc.Wallet.
func (w *wallet) Mnemonic() string {
	return w.mnemonic
}
