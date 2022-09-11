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

const (
	ProposalVoteYes        = "yes"
	ProposalVoteNo         = "no"
	ProposalVoteNoWithVeto = "noWithVeto"
	ProposalVoteAbstain    = "abstain"

	ProposalStatusUnspecified   = "PROPOSAL_STATUS_UNSPECIFIED"
	ProposalStatusPassed        = "PROPOSAL_STATUS_PASSED"
	ProposalStatusFailed        = "PROPOSAL_STATUS_FAILED"
	ProposalStatusRejected      = "PROPOSAL_STATUS_REJECTED"
	ProposalStatusVotingPeriod  = "PROPOSAL_STATUS_VOTING_PERIOD"
	ProposalStatusDepositPeriod = "PROPOSAL_STATUS_DEPOSIT_PERIOD"
)

// TxProposal contains chain proposal transaction details.
type TxProposal struct {
	// The block height.
	Height uint64
	// The transaction hash.
	TxHash string
	// Amount of gas charged to the account.
	GasSpent int64

	// Amount deposited for proposal.
	DepositAmount string
	// ID of proposal.
	ProposalID string
	// Type of proposal.
	ProposalType string
}
