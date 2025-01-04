package wallet

import (
	"crypto/sha256"
	"fmt"

	"golang.org/x/crypto/ripemd160"

	"github.com/strangelove-ventures/interchaintest/v8/chain/xrp/address-codec"
)

// Key derivation constants
var (
	familySeed    = []byte("secp256k1")
	ed25519Prefix = []byte{0xED, 0x00, 0x00, 0x00}
	accountPrefix = []byte{0x00}
	accountPublicKeyPrefix = []byte{0x23}
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

func Encode(b []byte, prefix []byte) string {
	buf := make([]byte, 0, len(b)+len(prefix)) 
	buf = append(buf, prefix...)
	buf = append(buf, b...)
	cs := checksum(buf)
	buf = append(buf, cs[:]...)	
	return addresscodec.EncodeBase58(buf)
}

func EncodePublicKey(pk []byte) string {
	return Encode(pk, accountPublicKeyPrefix)
}

func EncodeSeed(seed []byte, keyType string) (string, error) {
	var prefix []byte
	switch keyType {
	case "ed25519":
		prefix = []byte{SEED_PREFIX_ED25519}
	case "secp256k1":
		prefix = []byte{SEED_PREFIX_SECP256K1}
	default:
		return "", fmt.Errorf("unknown seed keytype: %s", keyType)
	}

	return Encode(seed, prefix), nil
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

func masterPubKeyToAccountId(compressedMasterPubKey []byte) string {
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
	return addresscodec.EncodeBase58(finalAccountId)
}
