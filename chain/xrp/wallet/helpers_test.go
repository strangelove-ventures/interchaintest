package wallet

import (
	//"crypto/ed25519"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"

	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcd/btcutil/base58"
	"github.com/strangelove-ventures/interchaintest/v8/chain/xrp/address-codec"
	"github.com/stretchr/testify/require"
)

func TestKeyPairToAddress(t *testing.T) {
    
    // Derive ED25519 keypair
    // edKeyPair, err := DeriveKeypair(seed, "ed25519")
    // if err != nil {
    //     //fmt.Printf("Error deriving ED25519 keypair: %v\n", err)
    //     t.Errorf("Error deriving ED25519 keypair: %v\n", err)
    // }
    secpKeyPair, err := DeriveKeypair("sswVV2EMPn8bcUqWnMKxQpVmZGgKT")
    if err != nil {
        //fmt.Printf("Error deriving ED25519 keypair: %v\n", err)
        t.Errorf("Error deriving ED25519 keypair: %v\n", err)
    }
	tests := []struct {
		name     string
		keyPair  *KeyPair
		expected string
		wantPanic bool
	}{
		// {
		// 	name: "Valid ED25519 KeyPair",
		// 	keyPair: &KeyPair{
		// 		KeyType: "ed25519",
		// 		PublicKey: ed25519.PublicKey([]byte{
		// 			0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
		// 			0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10,
		// 			0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18,
		// 			0x19, 0x1a, 0x1b, 0x1c, 0x1d, 0x1e, 0x1f, 0x20,
		// 		}),
		// 	},
		// 	expected:  "rHb9CJAWyB4rj91VRWn96DkukG4bwdtyTh", // Replace with actual expected value
		// 	wantPanic: false,
		// },
		{
			name: "Valid SECP256K1 KeyPair",
			// keyPair: &KeyPair{
			// 	KeyType:   "secp256k1",
			// 	PublicKey: createTestSecp256k1PubKey(t),
			// },
			// expected:  "rHb9CJAWyB4rj91VRWn96DkukG4bwdtyTh", // Replace with actual expected value
			keyPair: secpKeyPair,
			expected: "r4qmPsHfdoqtNMPx9popoXG3nDtsCSzUZQ",
			wantPanic: false,
		},
		{
			name: "Invalid KeyType",
			keyPair: &KeyPair{
				KeyType:   "invalid",
				PublicKey: nil,
			},
			wantPanic: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantPanic {
				defer func() {
					if r := recover(); r == nil {
						t.Error("Expected panic but got none")
					}
				}()
			}

			got := KeyPairToAddress(tt.keyPair)
			if !tt.wantPanic && got != tt.expected {
				t.Errorf("KeyPairToAddress() = %v, want %v", got, tt.expected)
			}
		})
	}
}
func TestSeedToAccountId(t *testing.T) {    
	tests := []struct {
		name     string
		seed string
		expected string
		wantPanic bool
	}{
		
		{
			name: "Valid SECP256K1 seed",
			// keyPair: &KeyPair{
			// 	KeyType:   "secp256k1",
			// 	PublicKey: createTestSecp256k1PubKey(t),
			// },
			// expected:  "rHb9CJAWyB4rj91VRWn96DkukG4bwdtyTh", // Replace with actual expected value
			seed: "sswVV2EMPn8bcUqWnMKxQpVmZGgKT",
			expected: "r4qmPsHfdoqtNMPx9popoXG3nDtsCSzUZQ",
			wantPanic: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantPanic {
				defer func() {
					if r := recover(); r == nil {
						t.Error("Expected panic but got none")
					}
				}()
			}

			wallet, err := SeedToXrpWallet(tt.seed)
			fmt.Println("public key:", wallet.PublicKeyHex)
			require.NoError(t, err)
			if !tt.wantPanic && wallet.AccountID != tt.expected {
				t.Errorf("KeyPairToAddress() = %v, want %v", wallet.AccountID, tt.expected)
			}
		})
	}
}

func TestKeyPairToPubKeyHexStr(t *testing.T) {
	secpKeyPair, err := DeriveKeypair("sswVV2EMPn8bcUqWnMKxQpVmZGgKT")
    if err != nil {
        //fmt.Printf("Error deriving ED25519 keypair: %v\n", err)
        t.Errorf("Error deriving ED25519 keypair: %v\n", err)
    }
	tests := []struct {
		name     string
		keyPair  *KeyPair
		expected string
	}{
		// {
		// 	name: "ED25519 Public Key",
		// 	keyPair: &KeyPair{
		// 		KeyType: "ed25519",
		// 		PublicKey: ed25519.PublicKey([]byte{
		// 			0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
		// 			0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10,
		// 			0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18,
		// 			0x19, 0x1a, 0x1b, 0x1c, 0x1d, 0x1e, 0x1f, 0x20,
		// 		}),
		// 	},
		// 	expected: "ED0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20",
		// },
		{
			name: "SECP256K1 Public Key",
			// keyPair: &KeyPair{
			// 	KeyType:   "secp256k1",
			// 	PublicKey: createTestSecp256k1PubKey(t),
			// },
			// expected: "0279BE667EF9DCBBAC55A06295CE870B07029BFCDB2DCE28D959F2815B16F81798", // Example compressed public key
			keyPair: secpKeyPair,
			expected: "0237FEF6D393A2D209C879A344EFD39C20C01A8E2413298EBC6E6CCDECEEBAA7AD",
		},
		{
			name: "Invalid KeyType",
			keyPair: &KeyPair{
				KeyType:   "invalid",
				PublicKey: nil,
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := KeyPairToPubKeyHexStr(tt.keyPair)
			if strings.ToLower(got) != strings.ToLower(tt.expected) {
				t.Errorf("KeyPairToPubKeyHexStr() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// Helper function to create a test SECP256K1 public key
func createTestSecp256k1PubKey(t *testing.T) *btcec.PublicKey {
	// This is the well-known Bitcoin generator point
	pubKeyBytes, err := hex.DecodeString("0279BE667EF9DCBBAC55A06295CE870B07029BFCDB2DCE28D959F2815B16F81798")
	if err != nil {
		t.Fatalf("Failed to decode public key hex: %v", err)
	}
	
	pubKey, err := btcec.ParsePubKey(pubKeyBytes, btcec.S256())
	if err != nil {
		t.Fatalf("Failed to parse public key: %v", err)
	}
	
	return pubKey
}

// Test PublicKey (base58) to PublicKeyHex decoding
func TestPubKeyBase58ToPubKeyHex(t *testing.T) {
	tests := []struct {
		name     string
		pubkeyBase58 string
		expected string
		xrpDecoding bool
	}{
		// {
		// 	name: "ED25519 Public Key",
		// 	keyPair: &KeyPair{
		// 		KeyType: "ed25519",
		// 		PublicKey: ed25519.PublicKey([]byte{
		// 			0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
		// 			0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10,
		// 			0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18,
		// 			0x19, 0x1a, 0x1b, 0x1c, 0x1d, 0x1e, 0x1f, 0x20,
		// 		}),
		// 	},
		// 	expected: "ED0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20",
		// },
		{
			name: "SECP256K1 random public key",
			pubkeyBase58: "aBQG8RQAzjs1eTKFEAQXr2gS4utcDiEC9wmi7pfUPTi27VCahwgw",
			expected: "0330E7FC9D56BB25D6893BA3F317AE5BCF33B3291BD63DB32654A313222F7FD020",
			xrpDecoding: true,
		},
		{
			name: "SECP256K1 root account public key",
			pubkeyBase58: "aB4PwLt3AMgsvLSUWjYyun7hdGr6tcbnbAU8TKjHgHRxjXycAwS2",
			expected: "0237FEF6D393A2D209C879A344EFD39C20C01A8E2413298EBC6E6CCDECEEBAA7AD",
			xrpDecoding: true,
		},
		{
			name: "SECP256K1 random public key",
			pubkeyBase58: "aBQG8RQAzjs1eTKFEAQXr2gS4utcDiEC9wmi7pfUPTi27VCahwgw",
			expected: "4b1e07506a53e0aa8d3c44a4294ffb1b164417553eac4769458862201044c95ad5",
			xrpDecoding: false,
		},
		{
			name: "SECP256K1 root account public key",
			pubkeyBase58: "aB4PwLt3AMgsvLSUWjYyun7hdGr6tcbnbAU8TKjHgHRxjXycAwS2",
			expected: "40e5d6845dd92dbad78eed52affdc4dac55dd88188fb264354672023f68280c5b0",
			xrpDecoding: false,
		},
		// {
		// 	name: "Invalid KeyType",
		// 	keyPair: &KeyPair{
		// 		KeyType:   "invalid",
		// 		PublicKey: nil,
		// 	},
		// 	expected: "",
		// },
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var pubKeyBytes []byte
			if tt.xrpDecoding {
				pubKeyBytes = addresscodec.DecodeBase58(tt.pubkeyBase58)
			} else {
				pubKeyBytes = base58.Decode(tt.pubkeyBase58)

			}
			got := hex.EncodeToString(pubKeyBytes[1:len(pubKeyBytes)-4])
			if !strings.EqualFold(got, tt.expected) {
				t.Errorf("KeyPairToPubKeyHexStr() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestMasterSeedBase58ToMasterSeedHex(t *testing.T) {
	tests := []struct {
		name     string
		masterSeedBase58 string
		keyType string
		expected string
	}{
		// {
		// 	name: "ED25519 Public Key",
		// 	keyPair: &KeyPair{
		// 		KeyType: "ed25519",
		// 		PublicKey: ed25519.PublicKey([]byte{
		// 			0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
		// 			0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10,
		// 			0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18,
		// 			0x19, 0x1a, 0x1b, 0x1c, 0x1d, 0x1e, 0x1f, 0x20,
		// 		}),
		// 	},
		// 	expected: "ED0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20",
		// },
		{
			name: "SECP256K1 root account master seed",
			masterSeedBase58: "snoPBrXtMeMyMHUVTgbuqAfg1SUTb",
			expected: "DEDCE9CE67B451D852FD4E846FCDE31C",
			keyType: "secp256k1",
		},
		{
			name: "SECP256K1 random master seed",
			masterSeedBase58: "sswVV2EMPn8bcUqWnMKxQpVmZGgKT",
			expected: "21A66FE3D048F8EE6071A84C6070D5DA",
			keyType: "secp256k1",
		},
		// {
		// 	name: "Invalid KeyType",
		// 	keyPair: &KeyPair{
		// 		KeyType:   "invalid",
		// 		PublicKey: nil,
		// 	},
		// 	expected: "",
		// },
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			masterSeedBytes, keyType, err := DecodeSeed(tt.masterSeedBase58)
			require.NoError(t, err)
			require.Equal(t, keyType, tt.keyType)
			got := hex.EncodeToString(masterSeedBytes)
			require.Equal(t, strings.ToLower(tt.expected), strings.ToLower(got))

			// also test the reverse/encoding
			masterSeedBytes, err = hex.DecodeString(tt.expected)
			require.NoError(t, err)
			encodedSeed, err := EncodeSeed(masterSeedBytes, tt.keyType)
			require.NoError(t, err)
			require.Equal(t, tt.masterSeedBase58, encodedSeed)

		})
	}
}

func TestPassphraseToMasterSeed(t *testing.T) {
	tests := []struct {
		name     string
		passphrase string
		keyType string
		expected string
	}{
		{
			name: "SECP256K1 root account master key",
			passphrase: "masterpassphrase",
			expected: "snoPBrXtMeMyMHUVTgbuqAfg1SUTb",
			keyType: "secp256k1",
		},
		
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash := sha512.Sum512([]byte(tt.passphrase))
			masterSeed, err := EncodeSeed(hash[:16], tt.keyType)
			require.NoError(t, err)
			require.Equal(t, tt.expected, masterSeed)
		})
	}
}