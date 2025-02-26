package tron

import (
	"crypto/ecdsa"
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/strangelove-ventures/interchaintest/v8/chain/tron/api"
	"github.com/tyler-smith/go-bip32"
	"github.com/tyler-smith/go-bip39"
)

type Wallet struct {
	key      *ecdsa.PrivateKey
	name     string
	mnemonic string
}

func NewWallet() (Wallet, error) {
	entropy, err := bip39.NewEntropy(256)
	if err != nil {
		return Wallet{}, err
	}

	mnemonic, err := bip39.NewMnemonic(entropy)
	if err != nil {
		return Wallet{}, err
	}

	return NewWalletFromMnemonic(mnemonic)
}

func NewWalletFromKey(hexkey string) (Wallet, error) {
	key, err := crypto.HexToECDSA(hexkey)
	if err != nil {
		return Wallet{}, err
	}

	return Wallet{
		key: key,
	}, err
}

func NewWalletFromMnemonic(mnemonic string) (Wallet, error) {
	seed, err := bip39.NewSeedWithErrorChecking(mnemonic, "")
	if err != nil {
		return Wallet{}, err
	}

	master, err := bip32.NewMasterKey(seed)
	if err != nil {
		return Wallet{}, err
	}

	path, err := accounts.ParseDerivationPath("m/44'/195'/0'/0/0")
	if err != nil {
		return Wallet{}, err
	}

	key := master
	for _, n := range path {
		key, err = key.NewChildKey(n)
		if err != nil {
			return Wallet{}, err
		}
	}

	priv, err := crypto.ToECDSA(key.Key)
	if err != nil {
		return Wallet{}, err
	}

	return Wallet{
		key:      priv,
		mnemonic: mnemonic,
	}, nil
}

func (w Wallet) KeyName() string {
	return w.name
}

func (w Wallet) FormattedAddress() string {
	address, _ := api.ConvertAddress(
		crypto.PubkeyToAddress(w.key.PublicKey).String(),
	)
	return address
}

func (w Wallet) Mnemonic() string {
	return w.mnemonic
}

func (w Wallet) Address() []byte {
	pub := w.key.Public().(*ecdsa.PublicKey)
	return crypto.FromECDSAPub(pub)
}
