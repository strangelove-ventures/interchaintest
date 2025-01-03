package wallet

import (
	//"bytes"
	"crypto/ecdsa"
	"crypto/ed25519"
	//"math"

	//"crypto/rand"
	"crypto/sha256"
	"crypto/sha512"

	//"encoding/binary"
	"encoding/hex"
	"fmt"
	"math/big"

	//"sort"

	//"crypto/ecdsa"
	//"crypto/elliptic"

	"github.com/btcsuite/btcd/btcec/v2"
	//"github.com/btcsuite/btcd/btcutil/base58"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/strangelove-ventures/interchaintest/v8/chain/xrp/address-codec"
	"golang.org/x/crypto/ripemd160"
	//"github.com/decred/dcrd/dcrec/secp256k1/v2"
)

// Key derivation constants
var (
	familySeed    = []byte("secp256k1")
	ed25519Prefix = []byte{0xED, 0x00, 0x00, 0x00}
	accountPrefix = []byte{0x00}

)

// Seed type constants
const (
	SEED_PREFIX_ED25519   = 0x01
	SEED_PREFIX_SECP256K1 = 0x21
)

// checksum: first four bytes of sha256^2
func checksum(input []byte) (cksum [4]byte) {
	h := sha256.Sum256(input)
	h2 := sha256.Sum256(h[:])
	copy(cksum[:], h2[:4])
	return cksum
}

func EncodeSeed(seed []byte, keyType string) (string, error) {

	buf := make([]byte, 0, len(seed)+1)

	switch keyType {
	case "ed25519":
		buf = append(buf, SEED_PREFIX_ED25519)
	case "secp256k1":
		buf = append(buf, SEED_PREFIX_SECP256K1)
	default:
		return "", fmt.Errorf("unknown seed keytype: %s", keyType)
	}

	buf = append(buf, seed...)
	cs := checksum(buf)
	buf = append(buf, cs[:]...)

	return addresscodec.EncodeBase58(buf), nil
}

// DecodeSeed extracts the seed payload and determines the intended algorithm
func DecodeSeed(encodedSeed string) ([]byte, string, error) {
	decoded := addresscodec.DecodeBase58(encodedSeed)
	if len(decoded) != 21 { // 1 byte prefix + 16 bytes payload + 4 bytes checksum
		return nil, "", fmt.Errorf("invalid seed length: %d", len(decoded))
	}

	// First byte is the prefix indicating the key type
	prefix := decoded[0]
	payload := decoded[1:17] // 16 bytes of actual seed data

	var keyType string
	switch prefix {
	case SEED_PREFIX_ED25519:
		keyType = "ed25519"
		fmt.Println("ed25519")
	case SEED_PREFIX_SECP256K1:
		keyType = "secp256k1"
		fmt.Println("secp256k1")
	default:
		return nil, "", fmt.Errorf("unknown seed prefix: %x", prefix)
	}

	// TODO: check checksum

	return payload, keyType, nil
}

// Account generation from keys
func KeyPairToAddress(keyPair *KeyPair) string {
	var publicKey []byte

	switch keyPair.KeyType {
	case "ed25519":
		publicKey = keyPair.PublicKey.(ed25519.PublicKey)
	case "secp256k1":
		publicKey = keyPair.PublicKey.(*btcec.PublicKey).SerializeCompressed()
	default:
		panic("key type not supported")
	}
	var payload []byte

	if keyPair.KeyType == "ed25519" {
		// For ED25519, prepend the special prefix
		payload = append(ed25519Prefix, publicKey...)
	} else {
		// For SECP256K1, use the public key as is
		payload = publicKey
	}

	// SHA256
	sha := sha256.New()
	sha.Write(payload)
	hash := sha.Sum(nil)

	// RIPEMD160
	ripemd := ripemd160.New()
	ripemd.Write(hash)
	hash = ripemd.Sum(nil)

	// Add account prefix
	accountData := append(accountPrefix, hash...)

	// Double SHA256 for checksum
	sha.Reset()
	sha.Write(accountData)
	hash = sha.Sum(nil)
	sha.Reset()
	sha.Write(hash)
	hash = sha.Sum(nil)

	// Append first 4 bytes as checksum
	accountData = append(accountData, hash[:4]...)

	// Encode to base58
	return "r" + addresscodec.EncodeBase58(accountData)
}

// Sign message using either ED25519 or SECP256K1
func Sign(keyPair *KeyPair, message []byte) ([]byte, error) {
	switch keyPair.KeyType {
	case "ed25519":
		privateKey := keyPair.PrivateKey.(ed25519.PrivateKey)
		signature := ed25519.Sign(privateKey, message)
		return signature, nil

	// case "secp256k1":
	//     privateKey := keyPair.PrivateKey.(*btcec.PrivateKey)
	//     signature, err := privateKey.Sign(message)
	//     if err != nil {
	//         return nil, fmt.Errorf("failed to sign with secp256k1: %v", err)
	//     }
	//     return signature.Serialize(), nil

	default:
		return nil, fmt.Errorf("unsupported key type: %s", keyPair.KeyType)
	}
}

func KeyPairToPubKeyHexStr(keyPair *KeyPair) string {
	switch keyPair.KeyType {
	case "ed25519":
		pubKey := keyPair.PublicKey.(ed25519.PublicKey)
		return "ED" + hex.EncodeToString(pubKey)
	case "secp256k1":
		pubKey := keyPair.PublicKey.(*btcec.PublicKey)
		return hex.EncodeToString(pubKey.SerializeCompressed())
	default:
		return ""
	}
}

// Generate a master seed for a specific key type
// New account generation
func GenerateSeed(keyType string) (string, error) {
	return "", nil
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

// Key derivation from seed
func DeriveKeypair(seed string) (*KeyPair, error) {
	// seedBytes := base58.Decode(seed)
	// if len(seedBytes) != 22 {
	//     return nil, fmt.Errorf("invalid seed length, expected: 16, got: %d", len(seedBytes))
	// }

	seedBytes, keyType, err := DecodeSeed(seed)
	if err != nil {
		return nil, fmt.Errorf("seed decode error: %v", err)
	}

	switch keyType {
	case "ed25519":
		// ED25519 key derivation
		hasher := sha512.New()
		hasher.Write(seedBytes)
		hash := hasher.Sum(nil)
		privateKey := ed25519.NewKeyFromSeed(hash[:32])
		publicKey := privateKey.Public().(ed25519.PublicKey)

		return &KeyPair{
			PrivateKey: privateKey,
			PublicKey:  publicKey,
			KeyType:    "ed25519",
		}, nil

		// case "secp256k1":
		//     // SECP256K1 key derivation
		//     hasher := sha512.New()
		//     hasher.Write(append(seedBytes, byte(0x00)))
		//     //hasher.Write(append(seedBytes, familySeed...))
		//     hasher.Write(seedBytes)
		//     hash := hasher.Sum(nil)

		//     privateKey, _ := btcec.PrivKeyFromBytes(hash[:32])
		//     return &KeyPair{
		//         PrivateKey: privateKey,
		//         PublicKey:  privateKey.PubKey(),
		//         KeyType:    "secp256k1",
		//     }, nil
	case "secp256k1":
		// SECP256K1 key derivation
		hasher := sha512.New()
		hasher.Write(append(seedBytes, byte(0x00)))
		//hasher.Write(append(seedBytes, familySeed...))
		hasher.Write(seedBytes)
		hash := hasher.Sum(nil)

		privateKey, _ := btcec.PrivKeyFromBytes(hash[:32])
		return &KeyPair{
			PrivateKey: privateKey,
			PublicKey:  privateKey.PubKey(),
			KeyType:    "secp256k1",
		}, nil
	}

	return nil, fmt.Errorf("unsupported key type")
}

func generateSecp256k1KeyPairWithCurveOrder(seed []byte, sequence uint32, curveOrder *big.Int) (*KeyPair, error) {
	// Append sequence number to seed
	seedWithSequence := append(seed, byte(sequence>>24), byte(sequence>>16), byte(sequence>>8), byte(sequence))
	
	seedHash := sha512.Sum512(seedWithSequence)
	privateKeyBytes := seedHash[:32]

	// Convert private key bytes to big.Int
	privateKeyInt := new(big.Int).SetBytes(privateKeyBytes)

	// Ensure private key is within valid range (1 to N-1)
	if privateKeyInt.Cmp(curveOrder) >= 0 || privateKeyInt.Cmp(big.NewInt(0)) == 0 {
		return nil, fmt.Errorf("invalid private key")
	}

	// Convert to ECDSA private key
	privateKey, err := crypto.ToECDSA(privateKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to create ECDSA key: %v", err)
	}

	return &KeyPair{
		PrivateKey: privateKeyInt,
		PublicKey:  &privateKey.PublicKey,
	}, nil
}

func generateSecp256k1KeyPair(seed []byte, curveOrder *big.Int) (keypair *KeyPair, err error) {
	//for sequence := uint32(0); sequence < math.MaxUint32; sequence++ {
	for sequence := uint32(0); sequence < uint32(100); sequence++ {
		keypair, err = generateSecp256k1KeyPairWithCurveOrder(seed, sequence, curveOrder)
		if err == nil {
			return keypair, nil
		}
	}

	return nil, fmt.Errorf("fail generate private key, %v", err)
}

// addPrivateKeys adds private keys modulo the curve order
func addPrivateKeys(key1, key2 *big.Int, curveOrder *big.Int) *big.Int {
	sum := new(big.Int).Add(key1, key2)
	return new(big.Int).Mod(sum, curveOrder)
}

// addPublicKeys adds two public keys on the secp256k1 curve
func addPublicKeys(key1, key2 *ecdsa.PublicKey) (*ecdsa.PublicKey, error) {
	curve := crypto.S256()
	
	// Add the points
	x, y := curve.Add(key1.X, key1.Y, key2.X, key2.Y)
	
	return &ecdsa.PublicKey{
		Curve: curve,
		X:     x,
		Y:     y,
	}, nil
}

// Key derivation from seed
func SeedToXrpWallet(masterSeed string) (*XrpWallet, error) {
	masterSeedBytes, keyType, err := DecodeSeed(masterSeed)
	if err != nil {
		return nil, fmt.Errorf("seed decode error: %v", err)
	}

	switch keyType {
	case "secp256k1":
		// secp256k1 curve order (N)
		curveOrder, ok := new(big.Int).SetString("FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEBAAEDCE6AF48A03BBFD25E8CD0364141", 16)
		if !ok {
			return nil, fmt.Errorf("curve order not okay")
		}

		rootKeyPair, err := generateSecp256k1KeyPair(masterSeedBytes, curveOrder)
		if err != nil {
			return nil, fmt.Errorf("failed to generate root key pair: %v", err)
		}

		rootPublicKey := crypto.CompressPubkey(rootKeyPair.PublicKey.(*ecdsa.PublicKey))

		if rootPublicKey == nil {
			return nil, fmt.Errorf("failed to generate public key")
		}

		intermediateKeyPair, err := generateSecp256k1KeyPair(append(rootPublicKey, byte(0x00), byte(0x00), byte(0x00), byte(0x00)), curveOrder)
		if err != nil {
			return nil, fmt.Errorf("failed to generate intermediate key pair: %v", err)
		}

		masterPrivateKey := addPrivateKeys(rootKeyPair.PrivateKey.(*big.Int), intermediateKeyPair.PrivateKey.(*big.Int), curveOrder)

		// Convert master private key to ECDSA format
		masterPrivateKeyECDSA, err := crypto.ToECDSA(masterPrivateKey.Bytes())
		if err != nil {
			return nil, fmt.Errorf("failed to create master ECDSA key: %v", err)
		}

		// Get master public key from private key
		masterPublicKey := &masterPrivateKeyECDSA.PublicKey
		
		// Verify by adding public keys - should match master public key
		verificationPubKey, err := addPublicKeys(rootKeyPair.PublicKey.(*ecdsa.PublicKey), intermediateKeyPair.PublicKey.(*ecdsa.PublicKey))
		if err != nil {
			return nil, fmt.Errorf("failed to verify master key: %v", err)
		}

		// Verify public keys match
		if verificationPubKey.X.Cmp(masterPublicKey.X) != 0 || verificationPubKey.Y.Cmp(masterPublicKey.Y) != 0 {
			return nil, fmt.Errorf("key verification failed")
		}

		// Compress the master public key
		compressedMasterPubKey := crypto.CompressPubkey(masterPublicKey)

		// Generate SHA-256 hash of public key
		sha256Hash := sha256.Sum256(compressedMasterPubKey)

		// Generate RIPEMD160 hash
		ripemd160Hash := ripemd160.New()
		ripemd160Hash.Write(sha256Hash[:])
		accountId := ripemd160Hash.Sum(nil)

		// Add version prefix (0x00)
		versionedAccountId := append([]byte{0x00}, accountId...)

		// Generate checksum (first 4 bytes of double SHA256)
		firstHash := sha256.Sum256(versionedAccountId)
		secondHash := sha256.Sum256(firstHash[:])
		checksum := secondHash[:4]

		// Combine everything
		finalAccountId := append(versionedAccountId, checksum...)

		// masterSeedHex, err := EncodeSeed(masterSeedBytes, keyType)
		// if err != nil {
		// 	return nil, fmt.Errorf("fail seed to wallet, %v", err)
		// }

		return &XrpWallet{
			AccountID: addresscodec.EncodeBase58(finalAccountId),
			KeyType: keyType,
			MasterSeed: masterSeed,
			MasterSeedHex: hex.EncodeToString(masterSeedBytes),
			PublicKey: "", //TODO: encode public key
			PublicKeyHex: hex.EncodeToString(compressedMasterPubKey),
		}, nil
	}

	return nil, fmt.Errorf("unsupported key type")
}
