package ethereum

import (
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
)

var _ ibc.Wallet = &EthereumWallet{}

type EthereumWallet struct {
	address  string
	privKey  string
	keyName  string
}

func NewWallet(keyname string, address string, privKey string) ibc.Wallet {
	return &EthereumWallet{
		address:  address,
		privKey:  privKey,
		keyName:  keyname,
	}
}

func (w *EthereumWallet) KeyName() string {
	return w.keyName
}

// Get formatted address, passing in a prefix
func (w *EthereumWallet) FormattedAddress() string {
	return w.address
}

// Get mnemonic, only used for relayer wallets
func (w *EthereumWallet) Mnemonic() string {
	return ""
}

// Get Address with chain's prefix
func (w *EthereumWallet) Address() []byte {
	return hexutil.MustDecode(w.address)
}

type GenesisWallets struct {
	total uint32
	used uint32
}

func NewGenesisWallet() GenesisWallets {
	return GenesisWallets{
		total: 10, // Make this configurable, but this is default
		used: 0,
	}
}

func (w *GenesisWallets) GetUnusedWallet(keyname string) ibc.Wallet {
	return NewWallet(keyname, "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266", "0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80")
}