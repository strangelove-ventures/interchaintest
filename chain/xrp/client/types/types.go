package types

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

// Memo represents a single memo attached to a transaction.
type Memo struct {
	MemoType   string `json:"MemoType,omitempty"`   // must be hex-encoded
	MemoData   string `json:"MemoData,omitempty"`   // must be hex-encoded
	MemoFormat string `json:"MemoFormat,omitempty"` // must be hex-encoded
}
