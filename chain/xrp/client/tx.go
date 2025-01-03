package client

import (
    "crypto/ed25519"
	"crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "fmt"

	//"github.com/btcsuite/btcd/btcec/v2"
)

// Get account sequence number
func (x XrpClient) GetAccountSequence(account string) (int, error) {
    params := []any{
        map[string]string{
            "account": account,
            "strict":  "true",
        },
    }
    
    response, err := makeRPCCall(x.url, "account_info", params)
    if err != nil {
        return 0, err
    }

    var result struct {
        Account_data struct {
            Sequence int `json:"Sequence"`
        } `json:"account_data"`
    }
    
    if err := json.Unmarshal(response.Result, &result); err != nil {
        return 0, err
    }

    return result.Account_data.Sequence, nil
}

func signPayment(payment *Payment, keyPair *KeyPair) (string, error) {
    // // In a real implementation, you'd need to serialize the transaction fields
    // // in the exact order specified by XRPL
    // txBytes, err := json.Marshal(payment)
    // if err != nil {
    //     return "", fmt.Errorf("failed to marshal payment: %v", err)
    // }

	txBytes := SerializePayment(payment)

    // Hash the transaction data
    hasher := sha256.New()
    hasher.Write(txBytes)
    messageHash := hasher.Sum(nil)

    var signature []byte

    switch keyPair.KeyType {
    case "ed25519":
        privateKey := keyPair.PrivateKey.(ed25519.PrivateKey)
        signature = ed25519.Sign(privateKey, messageHash)

    // case "secp256k1":
    //     privateKey := keyPair.PrivateKey.(*btcec.PrivateKey)
    //     sig, err := privateKey.Sign(messageHash)
    //     if err != nil {
    //         return "", fmt.Errorf("failed to sign with secp256k1: %v", err)
    //     }
    //     signature = sig.Serialize()

    default:
        return "", fmt.Errorf("unsupported key type: %s", keyPair.KeyType)
    }

    return hex.EncodeToString(signature), nil
}


func (x XrpClient) SignAndSubmitPayment(keyPair *KeyPair, payment *Payment) error {
    // Set the public key in the payment
    payment.SigningPubKey = getPublicKeyHex(keyPair)

    // Sign the transaction
    signature, err := signPayment(payment, keyPair)
    if err != nil {
        return fmt.Errorf("failed to sign payment: %v", err)
    }
    payment.TxnSignature = signature

    // Submit the signed transaction
    params := []any{
        map[string]interface{}{
            "tx_blob": payment,
        },
    }

    response, err := makeRPCCall(x.url, "submit", params)
    if err != nil {
        return err
    }

    fmt.Printf("Transaction submitted: %s\n", response.Result)
    return nil
}

// // Sign and submit transaction
// func (x XrpClient) SignAndSubmitPayment(privateKeyHex string, payment *Payment) error {
//     // Decode private key
//     privateKeyBytes, err := hex.DecodeString(privateKeyHex)
//     if err != nil {
//         return fmt.Errorf("failed to decode private key: %v", err)
//     }

//     // Create ed25519 private key
//     privateKey := ed25519.NewKeyFromSeed(privateKeyBytes[:32])

//     // Serialize transaction for signing (this is a simplified version)
//     txBytes, err := json.Marshal(payment)
//     if err != nil {
//         return fmt.Errorf("failed to marshal payment: %v", err)
//     }

//     // Sign the transaction
//     signature := ed25519.Sign(privateKey, txBytes)
//     payment.TxnSignature = hex.EncodeToString(signature)

//     // Submit the signed transaction
//     params := []any{
//         map[string]interface{}{
//             "tx_blob": payment,
//         },
//     }

//     response, err := makeRPCCall(x.url, "submit", params)
//     if err != nil {
//         return err
//     }

//     fmt.Printf("Transaction submitted: %s\n", response.Result)
//     return nil
// }

// Signature verification
func VerifySignature(message, signature []byte, publicKey interface{}, keyType string) bool {
    switch keyType {
    case "ed25519":
        pubKey := publicKey.(ed25519.PublicKey)
        return ed25519.Verify(pubKey, message, signature)
        
    // case "secp256k1":
    //     pubKey := publicKey.(*btcec.PublicKey)
    //     sig, err := btcec.ParseSignature(signature)
    //     if err != nil {
    //         return false
    //     }
    //     hash := sha256.Sum256(message)
    //     return sig.Verify(hash[:], pubKey)
    }
    
    return false
}