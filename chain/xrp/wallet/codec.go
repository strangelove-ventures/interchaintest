package wallet

import (
	"bytes"
	"crypto/sha256"
	"fmt"

	"golang.org/x/crypto/ripemd160" //nolint:gosec,staticcheck

	"github.com/strangelove-ventures/interchaintest/v8/chain/xrp/client/base58"
)

// Key derivation constants.
var (
	accountPrefix          = []byte{0x00}
	accountPublicKeyPrefix = []byte{0x23}
	SeedPrefixEd25519      = []byte{0x01, 0xe1, 0x4b} //nolint:stylecheck
)

// Seed type constants.
const (
	SeedPrefixSecp256k1 = 0x21
	Secp256k1SeedLength = 21
	Ed25519SeedLength   = 23
)

// checksum: first four bytes of sha256^2.
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
		prefix = SeedPrefixEd25519
	case SECP256K1:
		prefix = []byte{SeedPrefixSecp256k1}
	default:
		return "", fmt.Errorf("unknown seed keytype: %s", keyType)
	}

	return Encode(seed, prefix), nil
}

// DecodeSeed extracts the seed payload and determines the intended algorithm.
func DecodeSeed(encodedSeed string) (payload []byte, keyType CryptoAlgorithm, err error) {
	decoded := base58.Decode(encodedSeed)
	switch len(decoded) {
	case Ed25519SeedLength:
		keyType = ED25519
		if !bytes.Equal(decoded[:3], SeedPrefixEd25519) {
			return nil, keyType, fmt.Errorf("invalid ed25519 seed prefix: %x", decoded[:3])
		}
		payload = decoded[3:19]
	case Secp256k1SeedLength:
		keyType = SECP256K1
		if decoded[0] != SeedPrefixSecp256k1 {
			return nil, keyType, fmt.Errorf("invalid secp256k1 seed prefix: %x", decoded[0])
		}
		payload = decoded[1:17]
	default:
		return nil, keyType, fmt.Errorf("invalid seed length: %d", len(decoded))
	}
	// TODO: check checksum.

	return payload, keyType, nil
}

func masterPubKeyToAccountID(compressedMasterPubKey []byte) string {
	// Generate SHA-256 hash of public key.
	sha256Hash := sha256.Sum256(compressedMasterPubKey)

	// Generate RIPEMD160 hash.
	ripemd160Hash := ripemd160.New() //nolint:gosec
	ripemd160Hash.Write(sha256Hash[:])
	accountID := ripemd160Hash.Sum(nil)

	// Add version prefix (0x00).
	versionedAccountID := append(accountPrefix, accountID...)

	// Generate checksum (first 4 bytes of double SHA256).
	firstHash := sha256.Sum256(versionedAccountID)
	secondHash := sha256.Sum256(firstHash[:])
	checksum := secondHash[:4]

	// Combine everything.
	finalAccountID := append(versionedAccountID, checksum...)
	return base58.Encode(finalAccountID)
}
