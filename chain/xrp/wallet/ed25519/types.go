package ed25519

import (
	"crypto/ed25519"
)

type Keys struct {
	publicKey ed25519.PublicKey
	privateKey ed25519.PrivateKey
}
