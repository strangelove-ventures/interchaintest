package utxo


type ListReceivedByAddress []ReceivedByAddress

type ReceivedByAddress struct {
	Address string `json:"address"`
	Amount  float64 `json:"amount"`
}

type TransactionReceipt struct {
	TxHash string `json:"transactionHash"`
}

type ListUtxo []Utxo

type Utxo struct {
	TxId string `json:"txid,omitempty"`
	Vout int `json:"vout,omitempty"`
	Address string `json:"address,omitempty"`
	Label string `json:"label,omitempty"`
	ScriptPubKey string `json:"scriptPubKey,omitempty"`
	Amount float64 `json:"amount,omitempty"`
	Confirmations int `json:"confirmations,omitempty"`
	Spendable bool `json:"spendable,omitempty"`
	Solvable bool `json:"solvable,omitempty"`
	Desc string `json:"desc,omitempty"`
	Safe bool `json:"safe,omitempty"`
}

type SendInputs []SendInput

type SendInput struct {
	TxId string `json:"txid"` // hex
	Vout int `json:"vout"`
}

type SendOutputs []SendOutput

type SendOutput struct {
	Amount float64 `json:"replaceWithAddress,omitempty"`
	Data string `json:"data,omitempty"` // hex
}

type SignRawTxError struct {
	Txid string `json:"txid"`
	Vout int `json:"vout"`
	Witness []string `json:"witness"`
	ScriptSig string `json:"scriptSig"`
	Sequence int `json:"sequence"`
	Error string `json:"error"`
}

type SignRawTxOutput struct {
	Hex string `json:"hex"`
	Complete bool `json:"complete"`
	Errors []SignRawTxError `json:"errors"`
}

type WalletInfo struct {
	WalletVersion int `json:"walletversion"`
}
