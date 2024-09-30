package utxo

import (
	"fmt"
	"sync"

	"github.com/strangelove-ventures/interchaintest/v8/ibc"
)

var _ ibc.Wallet = &UtxoWallet{}

type UtxoWallet struct {
	address string
	keyName string
}

func NewWallet(keyname string, address string) ibc.Wallet {
	return &UtxoWallet{
		address: address,
		keyName: keyname,
	}
}

func (w *UtxoWallet) KeyName() string {
	return w.keyName
}

// Get formatted address, passing in a prefix.
func (w *UtxoWallet) FormattedAddress() string {
	return w.address
}

// Get mnemonic, only used for relayer wallets.
func (w *UtxoWallet) Mnemonic() string {
	return ""
}

// Get Address with chain's prefix.
func (w *UtxoWallet) Address() []byte {
	return []byte(w.address)
}

type NodeWallet struct {
	keyName   string
	address   string
	mu        sync.Mutex
	txLock    sync.Mutex
	loadCount int
	ready     bool
}

func (c *UtxoChain) getWalletForNewAddress(keyName string) (*NodeWallet, error) {
	wallet, found := c.KeyNameToWalletMap[keyName]
	if c.WalletVersion >= noDefaultKeyWalletVersion {
		if !found {
			return nil, fmt.Errorf("wallet keyname (%s) not found, has it been created?", keyName)
		}
		if wallet.address != "" {
			return nil, fmt.Errorf("wallet keyname (%s) already has an address", keyName)
		}
	}

	if c.WalletVersion < noDefaultKeyWalletVersion {
		if found {
			return nil, fmt.Errorf("wallet keyname (%s) already has an address", keyName)
		} else {
			wallet = &NodeWallet{
				keyName: keyName,
			}
			c.KeyNameToWalletMap[keyName] = wallet
		}
	}

	return wallet, nil
}

func (c *UtxoChain) getWalletForSetAccount(keyName string, addr string) (*NodeWallet, error) {
	wallet, found := c.KeyNameToWalletMap[keyName]
	if !found {
		return nil, fmt.Errorf("wallet keyname (%s) not found, get new address not called", keyName)
	}
	if wallet.address != addr {
		return nil, fmt.Errorf("wallet keyname (%s) is associated with address (%s), not (%s)", keyName, wallet.address, addr)
	}
	return wallet, nil
}

func (c *UtxoChain) getWalletForUse(keyName string) (*NodeWallet, error) {
	wallet, err := c.getWallet(keyName)
	if err != nil {
		return nil, err
	}
	// Verifies wallet has expected state on node
	// For chain without wallet support, GetNewAddress() and SetAccount() must be called.
	// For chains with wallet support, CreateWallet() and GetNewAddress() must be called.
	if !wallet.ready {
		return nil, fmt.Errorf("wallet keyname (%s) is not ready for use, check creation steps", keyName)
	}
	return wallet, nil
}

func (c *UtxoChain) getWallet(keyName string) (*NodeWallet, error) {
	wallet, found := c.KeyNameToWalletMap[keyName]
	if !found {
		return nil, fmt.Errorf("wallet keyname (%s) not found", keyName)
	}
	return wallet, nil
}
