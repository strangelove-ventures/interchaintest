package client

import (
	"encoding/json"
)

// Function to force ledger close.
func (x XrpClient) ForceLedgerClose() error {
	_, err := makeRPCCall(x.url, "ledger_accept", nil)
	if err != nil {
		return err
	}
	return nil
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
