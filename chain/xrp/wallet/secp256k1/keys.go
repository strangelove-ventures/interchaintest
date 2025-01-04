package secp256k1

import (
	"fmt"
	"encoding/asn1"
	"math/big"
	"github.com/ethereum/go-ethereum/crypto"
)


func (k *Keys) GetCompressedMasterPublicKey() []byte {
	return k.compressedMasterPublicKey
}

func (k *Keys) Sign(message []byte) ([]byte, error) {
	signature, err := crypto.Sign(message, k.masterPrivateKey)
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
