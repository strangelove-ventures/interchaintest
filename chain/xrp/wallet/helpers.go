package wallet

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/strangelove-ventures/interchaintest/v8/chain/xrp/wallet/secp256k1"
)

// // Sign message using either ED25519 or SECP256K1
// func Sign(keyPair *KeyPair, message []byte) ([]byte, error) {
// 	switch keyPair.KeyType {
// 	case "ed25519":
// 		privateKey := keyPair.PrivateKey.(ed25519.PrivateKey)
// 		signature := ed25519.Sign(privateKey, message)
// 		return signature, nil

// 	// case "secp256k1":
// 	//     privateKey := keyPair.PrivateKey.(*btcec.PrivateKey)
// 	//     signature, err := privateKey.Sign(message)
// 	//     if err != nil {
// 	//         return nil, fmt.Errorf("failed to sign with secp256k1: %v", err)
// 	//     }
// 	//     return signature.Serialize(), nil

// 	default:
// 		return nil, fmt.Errorf("unsupported key type: %s", keyPair.KeyType)
// 	}
// }

// func KeyPairToPubKeyHexStr(keyPair *KeyPair) string {
// 	switch keyPair.KeyType {
// 	case "ed25519":
// 		pubKey := keyPair.PublicKey.(ed25519.PublicKey)
// 		return "ED" + hex.EncodeToString(pubKey)
// 	case "secp256k1":
// 		pubKey := keyPair.PublicKey.(*ecdsa.PublicKey)
// 		return hex.EncodeToString(pubKey.SerializeCompressed())
// 	default:
// 		return ""
// 	}
// }

// Generate a master seed for a specific key type
// New account generation
func GenerateSeed(keyType string) (string, error) {
	b := make([]byte, 16)
    _, err := rand.Read(b)
	if err != nil {
		return "", fmt.Errorf("fail to generate seed: %v", err)
	}
	return EncodeSeed(b, keyType)
}

// func GenerateKeyPair(keyType string) (*KeyPair, error) {
// 	switch keyType {
// 	case "ed25519":
// 		pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
// 		if err != nil {
// 			return nil, fmt.Errorf("failed to generate ed25519 key: %v", err)
// 		}
// 		return &KeyPair{
// 			PrivateKey: privKey,
// 			PublicKey:  pubKey,
// 			KeyType:    "ed25519",
// 		}, nil

// 	case "secp256k1":
// 		privKey, err := btcec.NewPrivateKey()
// 		if err != nil {
// 			return nil, fmt.Errorf("failed to generate secp256k1 key: %v", err)
// 		}
// 		return &KeyPair{
// 			PrivateKey: privKey,
// 			PublicKey:  privKey.PubKey(),
// 			KeyType:    "secp256k1",
// 		}, nil

// 	default:
// 		return nil, fmt.Errorf("unsupported key type: %s", keyType)
// 	}
// }

// // Key derivation from seed
// func DeriveKeypair(seed string) (*KeyPair, error) {
// 	seedBytes, keyType, err := DecodeSeed(seed)
// 	if err != nil {
// 		return nil, fmt.Errorf("seed decode error: %v", err)
// 	}

// 	switch keyType {
// 	case "ed25519":
// 		// ED25519 key derivation
// 		hasher := sha512.New()
// 		hasher.Write(seedBytes)
// 		hash := hasher.Sum(nil)
// 		privateKey := ed25519.NewKeyFromSeed(hash[:32])
// 		publicKey := privateKey.Public().(ed25519.PublicKey)

// 		return &KeyPair{
// 			PrivateKey: privateKey,
// 			PublicKey:  publicKey,
// 			KeyType:    "ed25519",
// 		}, nil
// 	}

// 	return nil, fmt.Errorf("unsupported key type")
// }



// Key derivation from seed
func GenerateXrpWalletFromSeed(keyName string, masterSeed string) (*XrpWallet, error) {
	masterSeedBytes, keyType, err := DecodeSeed(masterSeed)
	if err != nil {
		return nil, fmt.Errorf("seed decode error: %v", err)
	}

	var keys Keys
	switch keyType {
	case "secp256k1":
		keys, err = secp256k1.DeriveKeysFromSeed(masterSeedBytes)
		if err != nil {
			return nil, fmt.Errorf("fail generate xrp wallet from seed: %v", err)
		}
	
	default:
		return nil, fmt.Errorf("unsupported key type")	
	}

	return &XrpWallet{
		keyName: keyName,
		AccountID: masterPubKeyToAccountId(keys.GetCompressedMasterPublicKey()),
		KeyType: keyType,
		MasterSeed: masterSeed,
		MasterSeedHex: hex.EncodeToString(masterSeedBytes),
		PublicKey: EncodePublicKey(keys.GetCompressedMasterPublicKey()),
		PublicKeyHex: hex.EncodeToString(keys.GetCompressedMasterPublicKey()),
		Keys: keys,
	}, nil
}