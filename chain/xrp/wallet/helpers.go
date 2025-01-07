package wallet

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/strangelove-ventures/interchaintest/v8/chain/xrp/wallet/ed25519"
	"github.com/strangelove-ventures/interchaintest/v8/chain/xrp/wallet/secp256k1"
)

// Generate a master seed for a specific key type
// New account generation
func GenerateSeed(keyType CryptoAlgorithm) (string, error) {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		return "", fmt.Errorf("fail to generate seed: %v", err)
	}
	return EncodeSeed(b, keyType)
}

// Key derivation from seed
func GenerateXrpWalletFromSeed(keyName string, masterSeed string) (*XrpWallet, error) {
	masterSeedBytes, keyType, err := DecodeSeed(masterSeed)
	if err != nil {
		return nil, fmt.Errorf("seed decode error: %v", err)
	}

	var keys Keys
	switch keyType {
	case ED25519:
		keys, err = ed25519.DeriveKeysFromSeed(masterSeedBytes)
		if err != nil {
			return nil, fmt.Errorf("fail generate xrp wallet from ed25519 seed: %v", err)
		}
	case SECP256K1:
		keys, err = secp256k1.DeriveKeysFromSeed(masterSeedBytes)
		if err != nil {
			return nil, fmt.Errorf("fail generate xrp wallet from secp256k1 seed: %v", err)
		}

	default:
		return nil, fmt.Errorf("unsupported key type")
	}

	return &XrpWallet{
		keyName:       keyName,
		AccountID:     masterPubKeyToAccountId(keys.GetFormattedPublicKey()),
		KeyType:       keyType,
		MasterSeed:    masterSeed,
		MasterSeedHex: hex.EncodeToString(masterSeedBytes),
		PublicKey:     EncodePublicKey(keys.GetFormattedPublicKey()),
		PublicKeyHex:  hex.EncodeToString(keys.GetFormattedPublicKey()),
		Keys:          keys,
	}, nil
}
