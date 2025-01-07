package types

// Transaction structures.
type Payment struct {
	TransactionType string `json:"TransactionType"`
	Account         string `json:"Account"`
	Destination     string `json:"Destination"`
	Amount          string `json:"Amount"`
	Sequence        int    `json:"Sequence"`
	Fee             string `json:"Fee"`
	SigningPubKey   string `json:"SigningPubKey"`
	NetworkID       uint32 `json:"NetworkID"`
	TxnSignature    string `json:"TxnSignature,omitempty"`
	Flags           uint32
	Memos           []Memo `json:"Memos,omitempty"`
}

// TransactionResponse represents the top-level response structure.
type TransactionResponse struct {
	Accepted                 bool   `json:"accepted"`
	AccountSequenceAvailable int    `json:"account_sequence_available"`
	AccountSequenceNext      int    `json:"account_sequence_next"`
	Applied                  bool   `json:"applied"`
	Broadcast                bool   `json:"broadcast"`
	EngineResult             string `json:"engine_result"`
	EngineResultCode         int    `json:"engine_result_code"`
	EngineResultMessage      string `json:"engine_result_message"`
	Error                    string `json:"error,omitempty"`
	ErrorCode                int    `json:"error_code,omitempty"`
	ErrorMessage             string `json:"error_message,omitempty"`
	Kept                     bool   `json:"kept"`
	OpenLedgerCost           string `json:"open_ledger_cost"`
	Queued                   bool   `json:"queued"`
	Request                  any    `json:"request,omitempty"`
	Status                   string `json:"status"`
	TxBlob                   string `json:"tx_blob"`
	TxJSON                   TxJSON `json:"tx_json"`
	ValidatedLedgerIndex     int    `json:"validated_ledger_index"`
}

// TxJSON represents the transaction details.
type TxJSON struct {
	Account         string `json:"Account"`
	Amount          string `json:"Amount"`
	Destination     string `json:"Destination"`
	Fee             string `json:"Fee"`
	Flags           int    `json:"Flags"`
	NetworkID       int    `json:"NetworkID"`
	Sequence        int    `json:"Sequence"`
	SigningPubKey   string `json:"SigningPubKey"`
	TransactionType string `json:"TransactionType"`
	TxnSignature    string `json:"TxnSignature"`
	Hash            string `json:"hash"`
}
