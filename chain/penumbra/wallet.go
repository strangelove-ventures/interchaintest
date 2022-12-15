package penumbra

import (
	"github.com/strangelove-ventures/ibctest/v6/ibc"
	"github.com/cosmos/cosmos-sdk/types"
)

var _ ibc.Wallet = &PenumbraWallet{}

type PenumbraWallet struct {
	mnemonic string
	address  []byte
	keyName  string
	chainCfg ibc.ChainConfig
}

func NewWallet(keyname string, address []byte, mnemonic string, chainCfg ibc.ChainConfig) *PenumbraWallet {
	return &PenumbraWallet{
		mnemonic: mnemonic,
		address: address,
		keyName: keyname,
		chainCfg: chainCfg,
	}
}

func (w *PenumbraWallet) GetKeyName() string {
	return w.keyName
}

// Get Address using a custom prefix
func (w *PenumbraWallet) GetFormattedAddress(prefix string) string {
	return types.MustBech32ifyAddressBytes(prefix, w.address)
}

// Get mnemonic, only used for relayer wallets
func (w *PenumbraWallet) GetMnemonic() string {
	return w.mnemonic
}

// Get Address using chain config prefix
func (w *PenumbraWallet) GetAddress() string {
	return types.MustBech32ifyAddressBytes(w.chainCfg.Bech32Prefix, w.address)
}