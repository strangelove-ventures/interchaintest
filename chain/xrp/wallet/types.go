package wallet

import (
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
)

var _ ibc.Wallet = &XrpWallet{}

type XrpWallet struct {
	keyName    string
	AccountID     string `json:"account_id"`
    KeyType       string `json:"key_type"`
    MasterSeed    string `json:"master_seed"`
    MasterSeedHex string `json:"master_seed_hex"`
    PublicKey     string `json:"public_key"`
    PublicKeyHex  string `json:"public_key_hex"`
	Keys          Keys
	//mu         sync.Mutex
	//txLock     sync.Mutex
}

type Keys interface {
	GetCompressedMasterPublicKey() []byte
	Sign(message []byte) ([]byte, error)
}