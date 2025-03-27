package client

import (
	"encoding/json"
)

type RPCRequest struct {
	Method string `json:"method"`
	Params []any  `json:"params"`
	ID     int    `json:"id"`
}

type RPCResponse struct {
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Message string `json:"message"`
		Code    int    `json:"code"`
	} `json:"error,omitempty"`
	ID int `json:"id"`
}

// Reaponse from tx api call.
type TxResponse struct {
	Account         string        `json:"Account"`
	Amount          string        `json:"Amount"`
	DeliverMax      string        `json:"DeliverMax"`
	Destination     string        `json:"Destination"`
	Fee             string        `json:"Fee"`
	Memos           []MemoWrapper `json:"Memos"`
	NetworkID       int           `json:"NetworkID"`
	Sequence        int           `json:"Sequence"`
	SigningPubKey   string        `json:"SigningPubKey"`
	TransactionType string        `json:"TransactionType"`
	TxnSignature    string        `json:"TxnSignature"`
	Ctid            string        `json:"ctid"`
	Date            int64         `json:"date"`
	Hash            string        `json:"hash"`
	InLedger        int           `json:"inLedger"`
	LedgerIndex     int           `json:"ledger_index"`
	Meta            Meta          `json:"meta"`
	Status          string        `json:"status"`
	Validated       bool          `json:"validated"`
}

type MemoWrapper struct {
	Memo Memo `json:"Memo"`
}

// Memo represents a single memo attached to a transaction.
type Memo struct {
	MemoType   string `json:"MemoType,omitempty"`   // must be hex-encoded
	MemoData   string `json:"MemoData,omitempty"`   // must be hex-encoded
	MemoFormat string `json:"MemoFormat,omitempty"` // must be hex-encoded
}

type Meta struct {
	AffectedNodes     []AffectedNode `json:"AffectedNodes"`
	TransactionIndex  int            `json:"TransactionIndex"`
	TransactionResult string         `json:"TransactionResult"`
	DeliveredAmount   string         `json:"delivered_amount"`
}

type AffectedNode struct {
	ModifiedNode ModifiedNode `json:"ModifiedNode"`
}

type ModifiedNode struct {
	FinalFields       AccountFields `json:"FinalFields"`
	LedgerEntryType   string        `json:"LedgerEntryType"`
	LedgerIndex       string        `json:"LedgerIndex"`
	PreviousFields    AccountFields `json:"PreviousFields"`
	PreviousTxnID     string        `json:"PreviousTxnID"`
	PreviousTxnLgrSeq int           `json:"PreviousTxnLgrSeq"`
}

type AccountFields struct {
	Account    string `json:"Account"`
	Balance    string `json:"Balance"`
	Flags      int    `json:"Flags"`
	OwnerCount int    `json:"OwnerCount"`
	Sequence   int    `json:"Sequence"`
}
