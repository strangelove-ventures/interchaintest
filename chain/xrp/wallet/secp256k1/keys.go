package secp256k1

import (
	"crypto/sha512"
	"encoding/asn1"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/crypto"
)

func (k *Keys) GetFormattedPublicKey() []byte {
	return k.compressedMasterPublicKey
}

func (k *Keys) Sign(message []byte) ([]byte, error) {
	messageHashFull := sha512.Sum512(message)
	messageHash := messageHashFull[:32]
	signature, err := crypto.Sign(messageHash, k.masterPrivateKey)
	if err != nil {
		return nil, err
	}
	// Extract R and S from the signature
	// The signature is in the format R || S || V where V is the recovery ID
	r := new(big.Int).SetBytes(signature[:32])
	s := new(big.Int).SetBytes(signature[32:64])

	// Create an ECDSASignature struct for ASN.1 DER encoding
	sig := ECDSASignature{
		R: r,
		S: s,
	}

	// Encode the signature in DER format
	derSignature, err := asn1.Marshal(sig)
	if err != nil {
		return nil, fmt.Errorf("failed to DER encode signature: %v", err)
	}

	return derSignature, nil
}

func (k *Keys) Verify(message, signature []byte) (bool, error) {
	// Hash the transaction data
	messageHashFull := sha512.Sum512(message)
	messageHash := messageHashFull[:32]

	// Parse the DER signature
	var sig ECDSASignature
	_, err := asn1.Unmarshal(signature, &sig)
	if err != nil {
		return false, fmt.Errorf("failed to parse DER signature: %v", err)
	}

	// Decode the public key from hex
	publicKey, err := crypto.DecompressPubkey(k.GetFormattedPublicKey())
	if err != nil {
		return false, fmt.Errorf("failed to parse public key: %v", err)
	}

	// Verify the signature
	return crypto.VerifySignature(
		crypto.CompressPubkey(publicKey),
		messageHash,
		append(sig.R.Bytes(), sig.S.Bytes()...),
	), nil
}
