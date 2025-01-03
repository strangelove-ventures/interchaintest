package wallet

import (
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
)

var _ ibc.Wallet = &XrpWallet{}

type XrpWallet struct {
	keyName    string
	AccountID     string `json:"account_id"`
    KeyType       string `json:"key_type"`
    MasterKey     string `json:"master_key"`
    MasterSeed    string `json:"master_seed"`
    MasterSeedHex string `json:"master_seed_hex"`
    PublicKey     string `json:"public_key"`
    PublicKeyHex  string `json:"public_key_hex"`
    Status        string `json:"status"`
    keyPair       *KeyPair
	//mu         sync.Mutex
	//txLock     sync.Mutex
}

// KeyPair represents either an ED25519 or SECP256K1 key pair
type KeyPair struct {
    PrivateKey interface{} // either ed25519.PrivateKey or *btcec.PrivateKey
    PublicKey  interface{} // either ed25519.PublicKey or *btcec.PublicKey
    KeyType    string
}