package penumbra

import (
	"github.com/cosmos/cosmos-sdk/types"
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

// Get Address formatted with chain's prefix
func (w *Wallet) FormattedAddress() string {
	return types.MustBech32ifyAddressBytes(w.chainCfg.Bech32Prefix, w.address)
}

// Get mnemonic, only used for relayer wallets
func (w *Wallet) Mnemonic() string {
	return w.mnemonic
}

// Get Address
func (w *Wallet) Address() []byte {
	return w.address
}

func (w *Wallet) FormattedAddressWithPrefix(prefix string) string {
	return types.MustBech32ifyAddressBytes(prefix, w.address)
}
