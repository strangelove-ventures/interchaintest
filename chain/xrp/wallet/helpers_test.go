package wallet

import (
	"crypto/sha512"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSeedToXrpWallet(t *testing.T) {
	tests := []struct {
		name          string
		seed          string
		accountId     string
		keyType       CryptoAlgorithm
		masterSeedHex string
		publicKey     string
		publicKeyHex  string
		shouldError   bool
	}{
		{
			name:          "Valid SECP256K1 seed",
			seed:          "sswVV2EMPn8bcUqWnMKxQpVmZGgKT",
			accountId:     "r4qmPsHfdoqtNMPx9popoXG3nDtsCSzUZQ",
			keyType:       SECP256K1,
			masterSeedHex: "21A66FE3D048F8EE6071A84C6070D5DA",
			publicKey:     "aB4PwLt3AMgsvLSUWjYyun7hdGr6tcbnbAU8TKjHgHRxjXycAwS2",
			publicKeyHex:  "0237FEF6D393A2D209C879A344EFD39C20C01A8E2413298EBC6E6CCDECEEBAA7AD",
			shouldError:   false,
		},
		{
			name:          "root account SECP256K1 seed",
			seed:          "snoPBrXtMeMyMHUVTgbuqAfg1SUTb",
			accountId:     "rHb9CJAWyB4rj91VRWn96DkukG4bwdtyTh",
			keyType:       SECP256K1,
			masterSeedHex: "DEDCE9CE67B451D852FD4E846FCDE31C",
			publicKey:     "aBQG8RQAzjs1eTKFEAQXr2gS4utcDiEC9wmi7pfUPTi27VCahwgw",
			publicKeyHex:  "0330E7FC9D56BB25D6893BA3F317AE5BCF33B3291BD63DB32654A313222F7FD020",
			shouldError:   false,
		},
		{
			name:      "dart SECP256K1 seed",
			seed:      "sa9g98F1dxRtLbprVeAP5MonKgqPS",
			accountId: "rs3xN42EFLE23gUDG2Rw4rwxhR9MnjwZKQ", // classic address
			// Xaddress: "X72W51px1i7iPTf4EwKFY2Nygdh5tGGNkvBFfbiuXKPxEPY"
			// XtestNetAddress: "T7Ws3yBAjFp1Fx1yWyhbSZztwhbXPqvG5a9GRHaSf1fZnqk"
			keyType:       SECP256K1,
			masterSeedHex: "f7f9ff93d716eaced222a3c52a3b2a36",
			publicKey:     "ab4fw1tjaqpcd5eemppubbrggkax62of1nvtdbiwpxbsw7asudqn",
			publicKeyHex:  "027190BF2204E1F99A9346C0717508788A73A8A3B7E5A925C349969ED1BA7FF2A0",
			shouldError:   false,
		},
		{
			name:      "dart ED25519 seed",
			seed:      "sEdVkC96W1DQXBgcmNQFDcetKQqBvXw",
			accountId: "rELnd6Ae5ZYDhHkaqjSVg2vgtBnzjeDshm", // classic address
			// Xaddress: "XVGNvtm1P2N6A6oyQ3TWFsjyXS124KjGTNeki4i9E5DGVp1"
			// XtestNetAddress: "TVBmLzviEX8jPD22CAUH5sV1ztQ41uPJQQcDwhnCiMVzSCn"
			keyType:       ED25519,
			masterSeedHex: "f7f9ff93d716eaced222a3c52a3b2a36",
			publicKey:     "akgguljomjqdlzfw65hf4anmcy6osaz2c3xf7ztxttcdgqtekegh",
			publicKeyHex:  "EDFB7C70E528FE161ADDFDA8CB224BC19B9E6455916970F7992A356C3E77AC7EF8",
			shouldError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wallet, err := GenerateXrpWalletFromSeed(tt.name, tt.seed)
			if tt.shouldError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, strings.ToLower(tt.accountId), strings.ToLower(wallet.AccountID))
				require.Equal(t, tt.keyType, wallet.KeyType)
				require.Equal(t, strings.ToLower(tt.seed), strings.ToLower(wallet.MasterSeed))
				require.Equal(t, strings.ToLower(tt.masterSeedHex), strings.ToLower(wallet.MasterSeedHex))
				require.Equal(t, strings.ToLower(tt.publicKey), strings.ToLower(wallet.PublicKey))
				require.Equal(t, strings.ToLower(tt.publicKeyHex), strings.ToLower(wallet.PublicKeyHex))
				require.Equal(t, strings.ToLower(tt.publicKeyHex), hex.EncodeToString(wallet.Keys.GetFormattedPublicKey()))
			}
		})
	}
}

// // Helper function to create a test SECP256K1 public key
// func createTestSecp256k1PubKey(t *testing.T) *btcec.PublicKey {
// 	// This is the well-known Bitcoin generator point
// 	pubKeyBytes, err := hex.DecodeString("0279BE667EF9DCBBAC55A06295CE870B07029BFCDB2DCE28D959F2815B16F81798")
// 	if err != nil {
// 		t.Fatalf("Failed to decode public key hex: %v", err)
// 	}

// 	pubKey, err := btcec.ParsePubKey(pubKeyBytes, btcec.S256())
// 	if err != nil {
// 		t.Fatalf("Failed to parse public key: %v", err)
// 	}

// 	return pubKey
// }

func TestMasterSeedBase58ToMasterSeedHex(t *testing.T) {
	tests := []struct {
		name             string
		masterSeedBase58 string
		keyType          CryptoAlgorithm
		expected         string
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
			name:             "SECP256K1 root account master seed",
			masterSeedBase58: "snoPBrXtMeMyMHUVTgbuqAfg1SUTb",
			expected:         "DEDCE9CE67B451D852FD4E846FCDE31C",
			keyType:          SECP256K1,
		},
		{
			name:             "SECP256K1 random master seed",
			masterSeedBase58: "sswVV2EMPn8bcUqWnMKxQpVmZGgKT",
			expected:         "21A66FE3D048F8EE6071A84C6070D5DA",
			keyType:          SECP256K1,
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
		name       string
		passphrase string
		keyType    CryptoAlgorithm
		expected   string
	}{
		{
			name:       "SECP256K1 root account master key",
			passphrase: "masterpassphrase",
			expected:   "snoPBrXtMeMyMHUVTgbuqAfg1SUTb",
			keyType:    SECP256K1,
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
