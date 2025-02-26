package api

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/strangelove-ventures/interchaintest/v8/chain/tron/api/core"

	"google.golang.org/protobuf/proto"
)

type Transaction struct {
	TxId       string   `json:"txID"`
	Ret        []Ret    `json:"ret,omitempty"`
	RawData    RawData  `json:"raw_data"`
	Signature  []string `json:"signature"`
	RawDataHex string   `json:"raw_data_hex"`
	Visible    bool     `json:"visible,omitempty"`
}

type Ret struct {
	ContractRet string `json:"contractRet"`
}

type RawData struct {
	Data          string     `json:"data,omitempty"`
	Timestamp     int64      `json:"timestamp,omitempty"`
	Expiration    int64      `json:"expiration,omitempty"`
	RefBlockBytes string     `json:"ref_block_bytes,omitempty"`
	RefBlockHash  string     `json:"ref_block_hash,omitempty"`
	Contract      []Contract `json:"contract"`
	FeeLimit      uint64     `json:"fee_limit,omitempty"`
}

type Contract struct {
	Parameter Parameter `json:"parameter"`
	Type      string    `json:"type"`
}

type Parameter struct {
	TypeUrl string `json:"type_url"`
	Value   struct {
		Data            string `json:"data,omitempty"`
		Amount          int64  `json:"amount,omitempty"`
		OwnerAddress    string `json:"owner_address"`
		ToAddress       string `json:"to_address,omitempty"`
		ContractAddress string `json:"contract_address,omitempty"`
	} `json:"value"`
}

func (t *Transaction) Rehash() error {
	var raw core.TransactionRaw
	data, err := hex.DecodeString(t.RawDataHex)
	if err != nil {
		return fmt.Errorf("failed to decode raw_data_hex: %w", err)
	}

	err = proto.Unmarshal(data, &raw)
	if err != nil {
		return fmt.Errorf("failed to unmarshal raw_data_hex: %w", err)
	}

	raw.Timestamp = t.RawData.Timestamp
	raw.Expiration = t.RawData.Expiration

	raw.RefBlockBytes, err = hex.DecodeString(t.RawData.RefBlockBytes)
	if err != nil {
		return fmt.Errorf("failed to decode ref_block_bytes: %w", err)
	}

	raw.RefBlockHash, err = hex.DecodeString(t.RawData.RefBlockHash)
	if err != nil {
		return fmt.Errorf("failed to decode ref_block_hash: %w", err)
	}

	if t.RawData.Data != "" {
		memo, err := hex.DecodeString(t.RawData.Data)
		if err != nil {
			return fmt.Errorf("failed to decode raw_data_hex: %w", err)
		}

		raw.Data = memo
	}

	data, err = proto.Marshal(&raw)
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}

	hash := sha256.Sum256(data)

	t.TxId = hex.EncodeToString(hash[:])
	t.RawDataHex = hex.EncodeToString(data)

	return nil
}
