package api

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/btcsuite/btcutil/base58"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type TronApi struct {
	logger  zerolog.Logger
	http    *http.Client
	url     string
	timeout time.Duration
}

func NewTronApi(url string, timeout time.Duration) *TronApi {
	return &TronApi{
		logger:  log.Logger.With().Str("module", "tron_api").Logger(),
		url:     url,
		timeout: timeout,
		http:    &http.Client{Timeout: timeout},
	}
}

// public
// ----------------------------------------------------------------------------

func (api *TronApi) GetLatestBlock() (Block, error) {
	data, err := api.post("getnowblock", nil)
	if err != nil {
		return Block{}, err
	}

	var block Block

	err = json.Unmarshal(data, &block)
	if err != nil {
		return block, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return block, nil
}

func (api *TronApi) GetBlock(height int64) (Block, error) {
	params := map[string]any{
		"num": height,
	}

	data, err := api.post("getblockbynum", params)
	if err != nil {
		return Block{}, err
	}

	var block Block

	err = json.Unmarshal(data, &block)
	if err != nil {
		return block, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return block, nil
}

func (api *TronApi) GetTransactionInfo(
	hash string,
) (TransactionInfo, error) {
	params := map[string]any{
		"value": hash,
	}

	data, err := api.post("gettransactioninfobyid", params)
	if err != nil {
		return TransactionInfo{}, err
	}

	var info TransactionInfo

	err = json.Unmarshal(data, &info)
	if err != nil {
		return TransactionInfo{}, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return info, nil
}

func (api *TronApi) GetBalance(
	address string,
) (uint64, error) {
	account, err := api.GetAccount(address)
	if err != nil {
		return 0, err
	}

	return uint64(account.Balance), nil
}

func (api *TronApi) GetAccount(
	address string,
) (Account, error) {
	data, err := api.post("getaccount", map[string]any{
		"address": address,
		"visible": true,
	})
	if err != nil {
		return Account{}, err
	}

	var account Account

	err = json.Unmarshal(data, &account)
	if err != nil {
		api.logger.Err(err).Msg("failed to unmarshal response")
		return Account{}, err
	}

	return account, nil
}

func (api *TronApi) CreateTransaction(
	from, to string,
	amount uint64,
	memo string,
) (Transaction, error) {
	data, err := api.post("createtransaction", map[string]any{
		"owner_address": from,
		"to_address":    to,
		"amount":        amount,
		"extra_data":    memo,
		"visible":       true,
	})
	if err != nil {
		return Transaction{}, err
	}

	var tx Transaction

	err = json.Unmarshal(data, &tx)
	if err != nil {
		return tx, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return tx, nil
}

func (api *TronApi) TriggerSmartContract(
	from string,
	contract string,
	selector string,
	input string,
	feeLimit uint64,
) (Transaction, error) {
	data, err := api.post("triggersmartcontract", map[string]any{
		"owner_address":     from,
		"contract_address":  contract,
		"function_selector": selector,
		"parameter":         input,
		"fee_limit":         feeLimit,
		"call_value":        0,
		"visible":           true,
	})
	if err != nil {
		return Transaction{}, err
	}

	var response struct {
		Result struct {
			Result bool `json:"result"`
		} `json:"result"`
		Transaction Transaction `json:"transaction"`
	}

	err = json.Unmarshal(data, &response)
	if err != nil {
		return Transaction{}, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return response.Transaction, nil
}

func (api *TronApi) BroadcastTransaction(tx []byte) (BroadcastResponse, error) {
	var params map[string]any

	err := json.Unmarshal(tx, &params)
	if err != nil {
		return BroadcastResponse{}, fmt.Errorf("failed to unmarshal tx: %w", err)
	}

	data, err := api.post("broadcasttransaction", params)
	if err != nil {
		return BroadcastResponse{}, err
	}

	var response BroadcastResponse

	err = json.Unmarshal(data, &response)
	if err != nil {
		return response, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return response, nil
}

func (api *TronApi) GetChainParameters() (ChainParameters, error) {
	data, err := api.get("getchainparameters")
	if err != nil {
		return ChainParameters{}, err
	}

	var response ChainParametersResponse

	err = json.Unmarshal(data, &response)
	if err != nil {
		api.logger.Err(err).Msg("failed to unmarshal response")
		return ChainParameters{}, err
	}

	updates := 0
	params := ChainParameters{}
	for _, param := range response.Parameters {
		switch param.Key {
		case "getMemoFee":
			params.MemoFee = param.Value
			updates++
		case "getEnergyFee":
			params.EnergyFee = param.Value
			updates++
		case "getTransactionFee":
			updates++
			params.BandwidthFee = param.Value
		}
	}

	if updates != 3 {
		return params, fmt.Errorf("failed to get necessary parameters")
	}

	return params, nil
}

func (api *TronApi) EstimateEnergy(
	from, contract, selector, input string,
) (int64, error) {
	data, err := api.post("estimateenergy", map[string]any{
		"owner_address":     from,
		"contract_address":  contract,
		"function_selector": selector,
		"parameter":         input,
		"visible":           true,
	})
	if err != nil {
		return 0, err
	}

	var response EstimateEnergyResponse
	err = json.Unmarshal(data, &response)
	if err != nil {
		api.logger.Err(err).Msg("failed to unmarshal response")
		return 0, err
	}

	return response.Energy, nil
}

// private
// ----------------------------------------------------------------------------

func (api *TronApi) getContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), api.timeout)
}

func (api *TronApi) get(
	method string,
) ([]byte, error) {
	data, err := api.do("GET", method, nil)
	if err != nil {
		api.logger.Err(err)
		return nil, fmt.Errorf("failed to call %s: %w", method, err)
	}

	return data, nil
}

func (api *TronApi) post(
	method string,
	params map[string]any,
) ([]byte, error) {
	data, err := api.do("POST", method, params)
	if err != nil {
		api.logger.Err(err)
		return nil, fmt.Errorf("failed to call %s: %w", method, err)
	}

	return data, nil
}

func (api *TronApi) do(
	httpMethod string,
	apiMethod string,
	params map[string]any,
) ([]byte, error) {
	ctx, cancel := api.getContext()
	defer cancel()

	body, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal params: %w", err)
	}

	url_, err := url.JoinPath(api.url, "wallet", apiMethod)
	if err != nil {
		return nil, fmt.Errorf("failed to create url")
	}
	req, err := http.NewRequestWithContext(
		ctx, httpMethod, url_, bytes.NewBuffer(body),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := api.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to perform request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("response status: %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return data, nil
}

// util
// ----------------------------------------------------------------------------

func ConvertAddress(address string) (string, error) {
	switch len(address) {
	case 34:
		// base58
		data := base58.Decode(address)

		return hex.EncodeToString(data[:len(data)-4]), nil
	case 42:
		if strings.HasPrefix(address, "0x") {
			address = strings.Replace(address, "0x", "41", 1)
		}

		data, err := hex.DecodeString(address)
		if err != nil {
			break
		}

		hash := func(d []byte) []byte {
			h := sha256.New()
			h.Write(d)
			return h.Sum(nil)
		}

		checksum := hash(hash(data))

		data = append(data, checksum[:4]...)

		return base58.Encode(data), nil
	}

	return "", fmt.Errorf("address not valid")
}
