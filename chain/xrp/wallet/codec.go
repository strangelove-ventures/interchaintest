package wallet

import (
	"bytes"
	"crypto/sha256"
	"fmt"

	"golang.org/x/crypto/ripemd160"

	"github.com/strangelove-ventures/interchaintest/v8/chain/xrp/client/base58"
)

// Key derivation constants
var (
	accountPrefix          = []byte{0x00}
	accountPublicKeyPrefix = []byte{0x23}
	SEED_PREFIX_ED25519    = []byte{0x01, 0xe1, 0x4b}
)

// Seed type constants
const (
	SEED_PREFIX_SECP256K1 = 0x21
	SECP256K1_SEED_LENGTH = 21
	ED25519_SEED_LENGTH   = 23
)

// checksum: first four bytes of sha256^2
func checksum(input []byte) (cksum [4]byte) {
	h := sha256.Sum256(input)
	h2 := sha256.Sum256(h[:])
	copy(cksum[:], h2[:4])
	return cksum
}

func Encode(b []byte, prefix []byte) string {
	buf := make([]byte, 0, len(b)+len(prefix))
	buf = append(buf, prefix...)
	buf = append(buf, b...)
	cs := checksum(buf)
	buf = append(buf, cs[:]...)
	return base58.Encode(buf)
}

func EncodePublicKey(pk []byte) string {
	return Encode(pk, accountPublicKeyPrefix)
}

func EncodeSeed(seed []byte, keyType CryptoAlgorithm) (string, error) {
	var prefix []byte
	switch keyType {
	case ED25519:
		prefix = SEED_PREFIX_ED25519
	case SECP256K1:
		prefix = []byte{SEED_PREFIX_SECP256K1}
	default:
		return "", fmt.Errorf("unknown seed keytype: %s", keyType)
	}

	return Encode(seed, prefix), nil
}

// DecodeSeed extracts the seed payload and determines the intended algorithm
func DecodeSeed(encodedSeed string) (payload []byte, keyType CryptoAlgorithm, err error) {
	decoded := base58.Decode(encodedSeed)
	switch len(decoded) {
	case ED25519_SEED_LENGTH:
		keyType = ED25519
		if !bytes.Equal(decoded[:3], SEED_PREFIX_ED25519) {
			return nil, keyType, fmt.Errorf("invalid ed25519 seed prefix: %x", decoded[:3])
		}
		payload = decoded[3:19]
	case SECP256K1_SEED_LENGTH:
		keyType = SECP256K1
		if decoded[0] != SEED_PREFIX_SECP256K1 {
			return nil, keyType, fmt.Errorf("invalid secp256k1 seed prefix: %x", decoded[0])
		}
		payload = decoded[1:17]
	default:
		return nil, keyType, fmt.Errorf("invalid seed length: %d", len(decoded))
	}
	// TODO: check checksum

	return payload, keyType, nil
}

func masterPubKeyToAccountId(compressedMasterPubKey []byte) string {
	// Generate SHA-256 hash of public key
	sha256Hash := sha256.Sum256(compressedMasterPubKey)

	// Generate RIPEMD160 hash
	ripemd160Hash := ripemd160.New()
	ripemd160Hash.Write(sha256Hash[:])
	accountId := ripemd160Hash.Sum(nil)

	// Add version prefix (0x00)
	versionedAccountId := append(accountPrefix, accountId...)

	// Generate checksum (first 4 bytes of double SHA256)
	firstHash := sha256.Sum256(versionedAccountId)
	secondHash := sha256.Sum256(firstHash[:])
	checksum := secondHash[:4]

	// Combine everything
	finalAccountId := append(versionedAccountId, checksum...)
	return base58.Encode(finalAccountId)
}
