package client

import (
	"encoding/hex"
	"encoding/json"
	"fmt"

	"go.uber.org/zap"

	"github.com/strangelove-ventures/interchaintest/v8/chain/xrp/client/types"
	xrpwallet "github.com/strangelove-ventures/interchaintest/v8/chain/xrp/wallet"
)

// Get account sequence number.
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
		AccountData struct {
			Sequence int `json:"Sequence"`
		} `json:"account_data"`
	}

	if err := json.Unmarshal(response.Result, &result); err != nil {
		return 0, err
	}

	return result.AccountData.Sequence, nil
}

func signPayment(wallet *xrpwallet.XrpWallet, payment *types.Payment) (string, error) {
	// // In a real implementation, you'd need to serialize the transaction fields
	// // in the exact order specified by XRPL
	// txBytes, err := json.Marshal(payment)
	// if err != nil {
	//     return "", fmt.Errorf("failed to marshal payment: %v", err)
	// }

	txBytes, err := SerializePayment(payment, false)
	if err != nil {
		return "", err
	}

	signature, err := wallet.Keys.Sign(txBytes)
	if err != nil {
		return "", fmt.Errorf("error sign payment: %v", err)
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

func (x XrpClient) SignAndSubmitPayment(wallet *xrpwallet.XrpWallet, payment *types.Payment) (string, error) {
	// Set the public key in the payment.
	payment.SigningPubKey = wallet.PublicKeyHex

	// Sign the transaction.
	signature, err := signPayment(wallet, payment)
	if err != nil {
		return "", fmt.Errorf("failed to sign payment: %v", err)
	}
	payment.TxnSignature = signature

	serializedTx, err := SerializePayment(payment, true)
	if err != nil {
		return "", err
	}
	txBlob := hex.EncodeToString(serializedTx)

	// Submit the signed transaction.
	params := []any{
		map[string]interface{}{
			"tx_blob": txBlob,
		},
	}

	// _, err = x.GetFee(payment)
	// if err != nil {
	// 	return "", fmt.Errorf("get fee error: %v", err)
	// }

	response, err := makeRPCCall(x.url, "submit", params)
	if err != nil {
		return "", err
	}

	if response.Error != nil {
		return "", fmt.Errorf("error submitting payment tx, code id: %d, message: %s", response.Error.Code, response.Error.Message)
	}

	var results types.TransactionResponse
	err = json.Unmarshal(response.Result, &results)
	if err != nil {
		return "", fmt.Errorf("fail unmarshal submit response: %v", err)
	}

	if results.Error != "" || results.ErrorCode != 0 {
		return "", fmt.Errorf("server side error submitting payment, error: %s, error code: %d, error message: %s, status: %s", results.Error, results.ErrorCode, results.ErrorMessage, results.Status)
	}

	x.log.Info("payment tx submitted", zap.String("results", string(response.Result)))
	return results.TxJSON.Hash, nil
}

// Signature verification.
func VerifySignature(wallet *xrpwallet.XrpWallet, payment *types.Payment, signature []byte) (bool, error) {
	txBytes, err := SerializePayment(payment, false)
	if err != nil {
		return false, err
	}
	return wallet.Keys.Verify(txBytes, signature)
}
