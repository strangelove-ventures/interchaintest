package client

import (
	"encoding/json"
	"fmt"
)

// Function to force ledger close.
func (x XrpClient) ForceLedgerClose() error {
	_, err := makeRPCCall(x.url, "ledger_accept", nil)
	if err != nil {
		return err
	}
	return nil
}

// TODO: fix this if needed.
func (x XrpClient) GetFee(txBlob any) (int, error) {
	params := []any{
		map[string]any{
			"tx_blob": txBlob,
			"id":      1,
		},
	}
	if txBlob == "" {
		params = nil
	}

	resp, err := makeRPCCall(x.url, "fee", params)
	if err != nil {
		return 0, err
	}

	if resp.Error != nil {
		return 0, fmt.Errorf("get server info error, code id: %d, message: %s", resp.Error.Code, resp.Error.Message)
	}

	// fmt.Println("Fee:", string(resp.Result))

	return 10, nil
}

func (x XrpClient) GetTx(txHash string) (*TxResponse, error) {
	params := []any{
		map[string]string{
			"transaction": txHash,
		},
	}
	response, err := makeRPCCall(x.url, "tx", params)
	if err != nil {
		return nil, err
	}

	var txResponse TxResponse
	if err := json.Unmarshal(response.Result, &txResponse); err != nil {
		return nil, err
	}

	return &txResponse, nil
}
