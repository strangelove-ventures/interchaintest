package ed25519

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha512"
	"fmt"
)

const (
	// ED25519 prefix - value is 237
	ED25519Prefix = 0xED
)

func DeriveKeysFromSeed(masterSeedBytes []byte) (k *Keys, err error) {
	h := sha512.Sum512(masterSeedBytes)
	rawPriv := h[:32]
	pubKey, privKey, err := ed25519.GenerateKey(bytes.NewBuffer(rawPriv))
	if err != nil {
		return nil, fmt.Errorf("error derive keys from seed: %v", err)
	}
	pubKey = append([]byte{ED25519Prefix}, pubKey...)
	privKey = privKey[:32]

	return &Keys{
		publicKey: pubKey,
		privateKey: privKey,
	}, nil
	
}