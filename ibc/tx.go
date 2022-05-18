package ibc

import "errors"

// Tx is a generalized IBC transaction.
type Tx struct {
	// The block height.
	Height uint64
	// The transaction hash.
	TxHash string
	// Amount of gas spent by transaction.
	GasSpent int64

	Packet Packet
}

// Validate returns an error if the transaction is not well-formed.
func (tx Tx) Validate() error {
	if tx.Height == 0 {
		return errors.New("tx height cannot be 0")
	}
	if len(tx.TxHash) == 0 {
		return errors.New("tx hash cannot be empty")
	}
	if tx.GasSpent == 0 {
		return errors.New("tx gas spent cannot be 0")
	}
	return tx.Packet.Validate()
}
