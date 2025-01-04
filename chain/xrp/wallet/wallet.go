package wallet

import (
    //"bytes"
    //"crypto/rand"
    //"crypto/sha512"
    //"encoding/binary"
    //"encoding/hex"
    //"fmt"
    //"math/big"
    //"sort"

    //"crypto/ecdsa"
    //"crypto/elliptic"
    
    //"golang.org/x/crypto/ripemd160"
)

// Base58 alphabet used by XRP
// const alphabet = "rpshnaf39wBUDNEGHJKLM4PQRST7VWXYZ2bcdeCg65jkm8oFqi1tuvAxyz"

// func NewWallet(keyname string) *XrpWallet {
// 	return &XrpWallet{
// 		keyName: keyname,
// 	}
// }

func (w *XrpWallet) KeyName() string {
	return w.keyName
}

// Get formatted address, passing in a prefix.
func (w *XrpWallet) FormattedAddress() string {
	return w.AccountID
}

// Get mnemonic, only used for relayer wallets.
func (w *XrpWallet) Mnemonic() string {
	return ""
}

// Get Address with chain's prefix.
func (w *XrpWallet) Address() []byte {
	return []byte(w.AccountID)
}

// func GenerateAccount(keyName string, keyType string) (*XrpWallet, error) {
//     keyPair, err := GenerateKeyPair(keyType)
//     if err != nil {
//         return nil, err
//     }

//     // Generate seed
//     seed := make([]byte, 16)
//     if _, err := rand.Read(seed); err != nil {
//         return nil, fmt.Errorf("failed to generate random seed: %v", err)
//     }

//     publicKeyHex := KeyPairToPubKeyHexStr(keyPair)
    
//     account := &XrpWallet{
//         keyName:       keyName,
//         KeyType:       keyType,
//         PublicKeyHex:  publicKeyHex,
//         MasterSeedHex: hex.EncodeToString(seed),
//         keyPair:       keyPair,
//         AccountID:     KeyPairToAddress(keyPair),
//         //MasterKey:     masterKey,
//         //MasterSeed:    base58.Encode(seed),
//         //MasterSeedHex: seedHex,
//         //PublicKey:     base58.Encode(publicKey),
//         //PublicKeyHex:  publicKeyHex,
//         //Status:        "success",
//     }

//     return account, nil
// }


// // Generate new XRPL account
// func GenerateAccount(keyName string) (*XrpWallet, error) {
//     // Generate random seed
//     seed := make([]byte, 16)
//     if _, err := rand.Read(seed); err != nil {
//         return nil, fmt.Errorf("failed to generate random seed: %v", err)
//     }

//     // Generate ED25519 keypair
//     privateKey := ed25519.NewKeyFromSeed(seed)
//     publicKey := privateKey.Public().(ed25519.PublicKey)

//     // Convert to hex strings
//     seedHex := hex.EncodeToString(seed)
//     publicKeyHex := hex.EncodeToString(publicKey)
//     masterKey := "00" + hex.EncodeToString(privateKey)

//     // Generate account ID from public key
//     accountID := publicKeyToAccountID(publicKey)

//     account := &XrpWallet{
// 		keyName:       keyName,
//         AccountID:     accountID,
//         KeyType:       "ed25519",
//         MasterKey:     masterKey,
//         MasterSeed:    base58.Encode(seed),
//         MasterSeedHex: seedHex,
//         PublicKey:     base58.Encode(publicKey),
//         PublicKeyHex:  publicKeyHex,
//         Status:        "success",
//     }

//     return account, nil
// }

func GetRootAccountSeed() string {
	return "snoPBrXtMeMyMHUVTgbuqAfg1SUTb"
}

// func GetRootAccountSee() string {
// 	rootAccount := &XrpWallet{
// 		keyName: keyName,
// 		AccountID: "rHb9CJAWyB4rj91VRWn96DkukG4bwdtyTh",
// 		KeyType: "secp256k1",
// 		// MasterKey: "I IRE BOND BOW TRIO LAID SEAT GOAL HEN IBIS IBIS DARE",
// 		MasterSeed: "snoPBrXtMeMyMHUVTgbuqAfg1SUTb",
// 		MasterSeedHex: "DEDCE9CE67B451D852FD4E846FCDE31C",
// 		PublicKey: "aBQG8RQAzjs1eTKFEAQXr2gS4utcDiEC9wmi7pfUPTi27VCahwgw",
// 		PublicKeyHex: "0330E7FC9D56BB25D6893BA3F317AE5BCF33B3291BD63DB32654A313222F7FD020",
// 		// Status: "success",
// 	}

// 	return rootAccount
// }
// {
//     "result" : {
//        "account_id" : "r4qmPsHfdoqtNMPx9popoXG3nDtsCSzUZQ",
//        "key_type" : "secp256k1",
//        "master_key" : "SHOE LAWS GUY HOFF FULL LISA TRAG NAVY PLY OBEY WEST EAT",
//        "master_seed" : "sswVV2EMPn8bcUqWnMKxQpVmZGgKT",
//        "master_seed_hex" : "21A66FE3D048F8EE6071A84C6070D5DA",
//        "public_key" : "aB4PwLt3AMgsvLSUWjYyun7hdGr6tcbnbAU8TKjHgHRxjXycAwS2",
//        "public_key_hex" : "0237FEF6D393A2D209C879A344EFD39C20C01A8E2413298EBC6E6CCDECEEBAA7AD",
//        "status" : "success"
//     }
//  }