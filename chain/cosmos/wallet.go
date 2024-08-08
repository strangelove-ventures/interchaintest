package cosmos

import (
	"github.com/cosmos/cosmos-sdk/types"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
)

var _ ibc.Wallet = &CosmosWallet{}
var _ User = &CosmosWallet{}

type CosmosWallet struct {
	mnemonic string
	address  []byte
	keyName  string
	chainCfg ibc.ChainConfig
}

// cosmosWalletExported is used for accounts created on startup.
// Useful to get account defaults such as the faucet.
type cosmosWalletExported struct {
	Name     string `json:"name"`
	Address  string `json:"address"`
	PubKey   string `json:"pubkey"`
	Mnemonic string `json:"mnemonic"`
}

func NewWallet(keyname string, address []byte, mnemonic string, chainCfg ibc.ChainConfig) ibc.Wallet {
	return &CosmosWallet{
		mnemonic: mnemonic,
		address:  address,
		keyName:  keyname,
		chainCfg: chainCfg,
	}
}

func (w *CosmosWallet) KeyName() string {
	return w.keyName
}

// Get formatted address, passing in a prefix
func (w *CosmosWallet) FormattedAddress() string {
	return types.MustBech32ifyAddressBytes(w.chainCfg.Bech32Prefix, w.address)
}

// Get mnemonic, only used for relayer wallets
func (w *CosmosWallet) Mnemonic() string {
	return w.mnemonic
}

// Get Address with chain's prefix
func (w *CosmosWallet) Address() []byte {
	return w.address
}

func (w *CosmosWallet) FormattedAddressWithPrefix(prefix string) string {
	return types.MustBech32ifyAddressBytes(prefix, w.address)
}
