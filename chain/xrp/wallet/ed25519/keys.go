package ed25519

import (
	"crypto/ed25519"
)

func (k *Keys) GetFormattedPublicKey() []byte {
	return k.publicKey
}

func (k *Keys) Sign(message []byte) ([]byte, error) {
	// TODO: do I need NewKeyFromSeed?
	rawPriv := ed25519.NewKeyFromSeed(k.privateKey)
	return ed25519.Sign(rawPriv, message), nil
}

func (k *Keys) Verify(message, signature []byte) (bool, error) {
	return ed25519.Verify(k.publicKey[1:], message, signature), nil
}
