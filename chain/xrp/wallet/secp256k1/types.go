package secp256k1

import (
	"crypto/ecdsa"
	"math/big"
)

type Keys struct {
	masterPublicKey           *ecdsa.PublicKey
	masterPrivateKey          *ecdsa.PrivateKey
	compressedMasterPublicKey []byte
}

type KeyPair struct {
	PrivateKey *big.Int
	PublicKey  *ecdsa.PublicKey
}

// ECDSASignature represents the R and S components of a signature
type ECDSASignature struct {
	R, S *big.Int
}
