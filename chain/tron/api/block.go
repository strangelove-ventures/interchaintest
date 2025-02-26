package api

type Block struct {
	BlockId string `json:"blockID"`
	Header  struct {
		RawData struct {
			Number    int64 `json:"number"`
			Timestamp int64 `json:"timestamp"`
		} `json:"raw_data"`
	} `json:"block_header"`
	Transactions []Transaction `json:"transactions"`
}
