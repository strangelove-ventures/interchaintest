package hermes

import "github.com/strangelove-ventures/interchaintest/v7/ibc"

var _ ibc.Wallet = &Wallet{}

type WalletModel struct {
	Mnemonic string `json:"mnemonic"`
	Address  string `json:"address"`
}

type Wallet struct {
	mnemonic string
	address  string
	keyName  string
}

func NewWallet(keyname string, address string, mnemonic string) *Wallet {
	return &Wallet{
		mnemonic: mnemonic,
		address:  address,
		keyName:  keyname,
	}
}

func (w *Wallet) KeyName() string {
	return w.keyName
}

func (w *Wallet) FormattedAddress() string {
	return w.address
}

// Get mnemonic, only used for relayer wallets
func (w *Wallet) Mnemonic() string {
	return w.mnemonic
}

// Get Address
func (w *Wallet) Address() []byte {
	return []byte(w.address)
}
