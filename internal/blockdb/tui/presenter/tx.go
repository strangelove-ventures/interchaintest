package presenter

import (
	"bytes"
	"encoding/json"
	"strconv"
	"sync"

	"github.com/strangelove-ventures/interchaintest/v5/internal/blockdb"
)

var bufPool = sync.Pool{New: func() any { return new(bytes.Buffer) }}

type Tx struct {
	Result blockdb.TxResult
}

func (tx Tx) Height() string { return strconv.FormatInt(tx.Result.Height, 10) }

// Data attempts to pretty print JSON. If not valid JSON, returns tx data as-is which may not be human-readable.
func (tx Tx) Data() string {
	buf := bufPool.Get().(*bytes.Buffer)
	defer bufPool.Put(buf)
	defer buf.Reset()

	if err := json.Indent(buf, tx.Result.Tx, "", "  "); err != nil {
		return string(tx.Result.Tx)
	}
	return buf.String()
}

type Txs []blockdb.TxResult

// ToJSON always renders valid JSON given the blockdb.TxResult.
// If the tx data is not valid JSON, the tx data is represented as a base64 encoded string.
func (txs Txs) ToJSON() []byte {
	type jsonObj struct {
		Height int64
		Tx     json.RawMessage
	}
	type jsonBytes struct {
		Height int64
		Tx     []byte
	}
	objs := make([]any, len(txs))
	for i, tx := range txs {
		if !json.Valid(tx.Tx) {
			objs[i] = jsonBytes{
				Height: tx.Height,
				Tx:     tx.Tx,
			}
			continue
		}
		objs[i] = jsonObj{
			Height: tx.Height,
			Tx:     tx.Tx,
		}
	}
	b, err := json.Marshal(objs)
	if err != nil {
		// json.Valid check above should prevent an error here.
		panic(err)
	}
	return b
}
