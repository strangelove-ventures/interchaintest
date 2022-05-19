package ibc

import (
	"errors"

	"go.uber.org/multierr"
)

// Tx is a generalized IBC transaction.
type Tx struct {
	// The block height.
	Height uint64
	// The transaction hash.
	TxHash string
	// Amount of gas charged to the account.
	GasSpent int64

	Packet Packet
}

// Validate returns an error if the transaction is not well-formed.
func (tx Tx) Validate() error {
	var err error
	if tx.Height == 0 {
		err = multierr.Append(err, errors.New("tx height cannot be 0"))
	}
	if len(tx.TxHash) == 0 {
		err = multierr.Append(err, errors.New("tx hash cannot be empty"))
	}
	if tx.GasSpent == 0 {
		err = multierr.Append(err, errors.New("tx gas spent cannot be 0"))
	}
	return multierr.Append(err, tx.Packet.Validate())
}
