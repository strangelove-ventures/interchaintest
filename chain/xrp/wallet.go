package xrp

import (
	"sync"
	
	"github.com/strangelove-ventures/interchaintest/v8/ibc"

	"github.com/Peersyst/xrpl-go/xrpl/wallet"
)

var _ ibc.Wallet = &WalletWrapper{}

type WalletWrapper struct {
	keyName       string
	Wallet        *wallet.Wallet
	txLock        sync.Mutex
}

func (w *WalletWrapper) KeyName() string {
	return w.keyName
}

// Get formatted address, passing in a prefix.
func (w *WalletWrapper) FormattedAddress() string {
	return w.Wallet.ClassicAddress.String()
}

// Get mnemonic, only used for relayer wallets.
func (w *WalletWrapper) Mnemonic() string {
	return ""
}

// Get Address with chain's prefix.
func (w *WalletWrapper) Address() []byte {
	return []byte(w.Wallet.ClassicAddress)
}
