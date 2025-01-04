package client

import (
	"crypto/sha512"
	"encoding/asn1"
	"encoding/hex"
	"encoding/json"
	"fmt"

	//"github.com/btcsuite/btcd/btcec/v2"
	xrpwallet "github.com/strangelove-ventures/interchaintest/v8/chain/xrp/wallet"
	"github.com/strangelove-ventures/interchaintest/v8/chain/xrp/wallet/secp256k1"

	"github.com/ethereum/go-ethereum/crypto"
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

func signPayment(wallet *xrpwallet.XrpWallet, payment *Payment) (string, error) {
    // // In a real implementation, you'd need to serialize the transaction fields
    // // in the exact order specified by XRPL
    // txBytes, err := json.Marshal(payment)
    // if err != nil {
    //     return "", fmt.Errorf("failed to marshal payment: %v", err)
    // }

	txBytes := SerializePayment2(payment, false)

    // Hash the transaction data
    // hasher := sha256.New()
    // hasher.Write(txBytes)
    // messageHash := hasher.Sum(nil)
	messageHashFull := sha512.Sum512(txBytes)
	messageHash := messageHashFull[:32]

    var signature []byte
	var err error

    switch wallet.KeyType {
    // case "ed25519":
    //     privateKey := keyPair.PrivateKey.(ed25519.PrivateKey)
    //     signature = ed25519.Sign(privateKey, messageHash)

    case "secp256k1":
        // privateKey := wallet.Keys
        // sig, err := privateKey.Sign(messageHash)
        // if err != nil {
        //     return "", fmt.Errorf("failed to sign with secp256k1: %v", err)
        // }
        // signature = sig.Serialize()
		signature, err = wallet.Keys.Sign(messageHash)
		if err != nil {
			return "", fmt.Errorf("error sign payment: %v", err)
		}

    default:
        return "", fmt.Errorf("unsupported key type: %s", wallet.KeyType)
    }

	verified, err := VerifySignature(wallet, payment, signature)
	if err != nil {
		return "", fmt.Errorf("error verifying signature: %v", err)
	}

	if !verified {
		return "", fmt.Errorf("signature verification failed")
	}

    return hex.EncodeToString(signature), nil
}


func (x XrpClient) SignAndSubmitPayment(wallet *xrpwallet.XrpWallet, payment *Payment) error {
    // Set the public key in the payment
    payment.SigningPubKey = wallet.PublicKeyHex

    // Sign the transaction
    signature, err := signPayment(wallet, payment)
    if err != nil {
        return fmt.Errorf("failed to sign payment: %v", err)
    }
    payment.TxnSignature = signature

	serializedTx := SerializePayment2(payment, true)
	//serializedTx2 := SerializePayment2(payment, true)
	txBlob := hex.EncodeToString(serializedTx)
	//txBlob2 := hex.EncodeToString(serializedTx2)

	fmt.Println("txBlob2:", txBlob)
	//fmt.Println("txBlob2:", txBlob2)

    // Submit the signed transaction
    params := []any{
        map[string]interface{}{
            "tx_blob": txBlob,
        },
    }

	_, err = x.GetFee(payment)
	if err != nil {
		return fmt.Errorf("get fee error: %v", err)
	}

    response, err := makeRPCCall(x.url, "submit", params)
    if err != nil {
        return err
    }

	if response.Error != nil {
		return fmt.Errorf("error submitting payment tx, code id: %d, message: %s", response.Error.Code, response.Error.Message)
	}

	var results SubmitResponse
	err = json.Unmarshal(response.Result, &results)
	if err != nil {
		return fmt.Errorf("fail unmarshal submit response: %v", err)
	}

	if results.Error != "" || results.ErrorCode != 0 {
		return fmt.Errorf("server side error submitting payment, error: %s, error code: %d, error message: %s, status: %s", results.Error, results.ErrorCode, results.ErrorMessage, results.Status)
	}

    fmt.Printf("Transaction submitted: %s\n", response.Result)
    return nil
}

// Signature verification
func VerifySignature(wallet *xrpwallet.XrpWallet, payment *Payment, signature []byte) (bool, error) {
    
	txBytes := SerializePayment2(payment, false)

    // Hash the transaction data
	messageHashFull := sha512.Sum512(txBytes)
	messageHash := messageHashFull[:32]

	switch wallet.KeyType {
    // case "ed25519":
    //     pubKey := publicKey.(ed25519.PublicKey)
    //     return ed25519.Verify(pubKey, message, signature)

    case "secp256k1":
		// Parse the DER signature
		var sig secp256k1.ECDSASignature
		_, err := asn1.Unmarshal(signature, &sig)
		if err != nil {
			return false, fmt.Errorf("failed to parse DER signature: %v", err)
		}
	
		// Decode the public key from hex
		publicKey, err := crypto.DecompressPubkey(wallet.Keys.GetCompressedMasterPublicKey())
		if err != nil {
			return false, fmt.Errorf("failed to parse public key: %v", err)
		}
	
		// Verify the signature
		return crypto.VerifySignature(
			crypto.CompressPubkey(publicKey),
			messageHash,
			append(sig.R.Bytes(), sig.S.Bytes()...),
		), nil
		// uncompressedPubKey, err := crypto.Ecrecover(messageHash, signature)
		// if err != nil {
		// 	return false, fmt.Errorf("failed to recover public key: %v", err)
		// }

		// // Parse the uncompressed public key
		// recoveredPubKey, err := crypto.UnmarshalPubkey(uncompressedPubKey)
		// if err != nil {
		// 	return false, fmt.Errorf("failed to unmarshal recovered public key: %v", err)
		// }

		// // Compress the recovered public key
		// recoveredCompressedPubKey = crypto.CompressPubkey(recoveredPubKey)

	default:
		return false, fmt.Errorf("verify signature, unsupported key type")
        
    }
}