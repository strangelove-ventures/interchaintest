package secp256k1

import (
	"crypto/ecdsa"
	"crypto/sha512"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/crypto"
)

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
	// for sequence := uint32(0); sequence < math.MaxUint32; sequence++ {
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

func DeriveKeysFromSeed(masterSeedBytes []byte) (k *Keys, err error) {
	// secp256k1 curve order (N)
	curveOrder, ok := new(big.Int).SetString("FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEBAAEDCE6AF48A03BBFD25E8CD0364141", 16)
	if !ok {
		return nil, fmt.Errorf("curve order not okay")
	}

	rootKeyPair, err := generateSecp256k1KeyPair(masterSeedBytes, curveOrder)
	if err != nil {
		return nil, fmt.Errorf("failed to generate root key pair: %v", err)
	}

	rootPublicKey := crypto.CompressPubkey(rootKeyPair.PublicKey)

	if rootPublicKey == nil {
		return nil, fmt.Errorf("failed to generate public key")
	}

	intermediateKeyPair, err := generateSecp256k1KeyPair(append(rootPublicKey, byte(0x00), byte(0x00), byte(0x00), byte(0x00)), curveOrder)
	if err != nil {
		return nil, fmt.Errorf("failed to generate intermediate key pair: %v", err)
	}

	masterPrivateKey := addPrivateKeys(rootKeyPair.PrivateKey, intermediateKeyPair.PrivateKey, curveOrder)

	// Convert master private key to ECDSA format
	masterPrivateKeyECDSA, err := crypto.ToECDSA(masterPrivateKey.Bytes())
	if err != nil {
		return nil, fmt.Errorf("failed to create master ECDSA key: %v", err)
	}

	// Get master public key from private key
	masterPublicKey := &masterPrivateKeyECDSA.PublicKey

	// Verify by adding public keys - should match master public key
	verificationPubKey, err := addPublicKeys(rootKeyPair.PublicKey, intermediateKeyPair.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to verify master key: %v", err)
	}

	// Verify public keys match
	if verificationPubKey.X.Cmp(masterPublicKey.X) != 0 || verificationPubKey.Y.Cmp(masterPublicKey.Y) != 0 {
		return nil, fmt.Errorf("key verification failed")
	}

	// Compress the master public key
	compressedMasterPubKey := crypto.CompressPubkey(masterPublicKey)

	return &Keys{
		masterPublicKey:           masterPublicKey,
		masterPrivateKey:          masterPrivateKeyECDSA,
		compressedMasterPublicKey: compressedMasterPubKey,
	}, nil
}
