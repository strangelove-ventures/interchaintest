package client

import (
	"encoding/json"
	"fmt"

	"github.com/strangelove-ventures/interchaintest/v8/chain/xrp/client/types"
)

func (x XrpClient) GetServerInfo() (*types.ServerInfoResponse, error) {
	resp, err := makeRPCCall(x.url, "server_info", nil)
	if err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("get server info error, code id: %d, message: %s", resp.Error.Code, resp.Error.Message)
	}
	var serverInfo types.ServerInfoResponse
	if err := json.Unmarshal(resp.Result, &serverInfo); err != nil {
		return nil, fmt.Errorf("get server info error, unmarshal: %v", err)
	}

	return &serverInfo, nil
}

func (x XrpClient) GetAccountInfo(account string, strict bool) (*types.AccountInfoResponse, error) {
	strictStr := "false"
	if strict {
		strictStr = "true"
	}
	accountParams := []any{
		map[string]string{
			"account": account,
			"strict":  strictStr,
		},
	}

	resp, err := makeRPCCall(x.url, "account_info", accountParams)
	if err != nil {
		return nil, err
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("get server info error, code id: %d, message: %s", resp.Error.Code, resp.Error.Message)
	}

	var accountInfo types.AccountInfoResponse
	if err := json.Unmarshal(resp.Result, &accountInfo); err != nil {
		return nil, fmt.Errorf("get account info error, unmarshal: %v", err)
	}

	return &accountInfo, nil
}

// Function to force ledger close.
func (x XrpClient) ForceLedgerClose() error {
	_, err := makeRPCCall(x.url, "ledger_accept", nil)
	if err != nil {
		return err
	}
	return nil
}

// Function to get current ledger index.
func (x XrpClient) GetCurrentLedger() (int64, error) {
	response, err := makeRPCCall(x.url, "ledger_current", nil)
	if err != nil {
		return 0, err
	}

	var result struct {
		LedgerCurrent int64 `json:"ledger_current_index"`
	}

	if err := json.Unmarshal(response.Result, &result); err != nil {
		return 0, err
	}

	return result.LedgerCurrent, nil
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

func (x XrpClient) GetTx(txHash string) (*types.TxResponse, error) {
	params := []any{
		map[string]string{
			"transaction": txHash,
		},
	}
	response, err := makeRPCCall(x.url, "tx", params)
	if err != nil {
		return nil, err
	}

	var txResponse types.TxResponse
	if err := json.Unmarshal(response.Result, &txResponse); err != nil {
		return nil, err
	}

	return &txResponse, nil
}
