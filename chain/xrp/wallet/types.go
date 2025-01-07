package wallet

import (
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
)

var _ ibc.Wallet = &XrpWallet{}

type XrpWallet struct {
	keyName       string
	AccountID     string          `json:"account_id"`
	MasterSeed    string          `json:"master_seed"`
	MasterSeedHex string          `json:"master_seed_hex"`
	PublicKey     string          `json:"public_key"`
	PublicKeyHex  string          `json:"public_key_hex"`
	KeyType       CryptoAlgorithm `json:"key_type"`
	Keys          Keys
}

type Keys interface {
	GetFormattedPublicKey() []byte
	Sign(message []byte) ([]byte, error)
	Verify(message, signature []byte) (bool, error)
}

// Algorithm represents supported cryptographic algorithms
type CryptoAlgorithm int

const (
	// SECP256K1 represents the secp256k1 elliptic curve algorithm
	SECP256K1 CryptoAlgorithm = iota
	// ED25519 represents the Ed25519 elliptic curve algorithm
	ED25519
)

// String returns the string representation of the CryptoAlgorithm
func (a CryptoAlgorithm) String() string {
	switch a {
	case SECP256K1:
		return "secp256k1"
	case ED25519:
		return "ed25519"
	default:
		return "unknown"
	}
}
