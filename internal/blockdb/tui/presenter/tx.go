package presenter

import (
	"bytes"
	"encoding/json"
	"strconv"
	"sync"

	"github.com/strangelove-ventures/ibctest/internal/blockdb"
)

var bufPool = sync.Pool{New: func() interface{} { return new(bytes.Buffer) }}

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
