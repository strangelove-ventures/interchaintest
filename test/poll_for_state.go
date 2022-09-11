package test

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/davecgh/go-spew/spew"
	"github.com/strangelove-ventures/ibctest/v5/ibc"
)

var ErrNotFound = errors.New("not found")

// ChainAcker is a chain that can get its acknowledgements at a specified height
type ChainAcker interface {
	ChainHeighter
	Acknowledgements(ctx context.Context, height uint64) ([]ibc.PacketAcknowledgement, error)
}

// PollForAck attempts to find an acknowledgement containing a packet equal to the packet argument.
// Polling starts at startHeight and continues until maxHeight. It is safe to call this function even if
// the chain has yet to produce blocks for the target min/max height range. Polling delays until heights exist
// on the chain. Returns an error if acknowledgement not found or problems getting height or acknowledgements.
func PollForAck(ctx context.Context, chain ChainAcker, startHeight, maxHeight uint64, packet ibc.Packet) (ibc.PacketAcknowledgement, error) {
	poller := blockPoller{CurrentHeight: chain.Height, Acker: chain}
	found, err := poller.doPoll(ctx, startHeight, maxHeight, packet)
	if err != nil {
		return ibc.PacketAcknowledgement{}, err
	}
	return found.(ibc.PacketAcknowledgement), nil
}

// ChainTimeouter is a chain that can get its timeouts at a specified height
type ChainTimeouter interface {
	ChainHeighter
	Timeouts(ctx context.Context, height uint64) ([]ibc.PacketTimeout, error)
}

// PollForTimeout attempts to find a timeout containing a packet equal to the packet argument.
// Otherwise, works identically to PollForAck.
func PollForTimeout(ctx context.Context, chain ChainTimeouter, startHeight, maxHeight uint64, packet ibc.Packet) (ibc.PacketTimeout, error) {
	poller := blockPoller{CurrentHeight: chain.Height, Timeouter: chain}
	found, err := poller.doPoll(ctx, startHeight, maxHeight, packet)
	if err != nil {
		return ibc.PacketTimeout{}, err
	}
	return found.(ibc.PacketTimeout), nil
}

type ChainProposalStatuser interface {
	ChainHeighter
	QueryProposal(ctx context.Context, proposalID string) (string, error)
}

type proposalSearch struct {
	id     string
	status string
}

// PollForTimeout attempts to find a timeout containing a packet equal to the packet argument.
// Otherwise, works identically to PollForAck.
func PollForProposalStatus(ctx context.Context, chain ChainProposalStatuser, startHeight, maxHeight uint64, proposalID string, status string) error {
	poller := blockPoller{CurrentHeight: chain.Height, ProposalStatuser: chain}
	_, err := poller.doPoll(ctx, startHeight, maxHeight, proposalSearch{
		id:     proposalID,
		status: status,
	})
	return err
}

type blockPoller struct {
	CurrentHeight    func(ctx context.Context) (uint64, error)
	Acker            ChainAcker
	Timeouter        ChainTimeouter
	ProposalStatuser ChainProposalStatuser
	pollErr          *pollError
}

func (p blockPoller) doPoll(ctx context.Context, startHeight, maxHeight uint64, pollData any) (any, error) {
	if maxHeight < startHeight {
		panic("maxHeight must be greater than or equal to startHeight")
	}
	switch {
	case p.Acker != nil, p.Timeouter != nil:
		p.pollErr = &pollError{targetPacket: pollData.(ibc.Packet)}
	}

	cursor := startHeight
	for cursor <= maxHeight {
		curHeight, err := p.CurrentHeight(ctx)
		if err != nil {
			return nil, err
		}
		if cursor > curHeight {
			continue
		}

		var (
			found   any
			findErr error
		)
		switch {
		case p.Acker != nil:
			found, findErr = p.findAck(ctx, cursor, pollData.(ibc.Packet))
		case p.Timeouter != nil:
			found, findErr = p.findTimeout(ctx, cursor, pollData.(ibc.Packet))
		case p.ProposalStatuser != nil:
			findErr = p.findProposalStatus(ctx, cursor, pollData.(proposalSearch))
		default:
			panic("poller misconfiguration")
		}

		if findErr != nil {
			p.pollErr.SetErr(findErr)
			cursor++
			continue
		}

		return found, nil
	}
	return nil, p.pollErr
}

func (p blockPoller) findAck(ctx context.Context, height uint64, packet ibc.Packet) (ibc.PacketAcknowledgement, error) {
	var zero ibc.PacketAcknowledgement
	acks, err := p.Acker.Acknowledgements(ctx, height)
	if err != nil {
		return zero, err
	}
	for _, ack := range acks {
		p.pollErr.PushSearched(ack)
		if ack.Packet.Equal(packet) {
			return ack, nil
		}
	}
	return zero, ErrNotFound
}

func (p blockPoller) findTimeout(ctx context.Context, height uint64, packet ibc.Packet) (ibc.PacketTimeout, error) {
	var zero ibc.PacketTimeout
	timeouts, err := p.Timeouter.Timeouts(ctx, height)
	if err != nil {
		return zero, err
	}
	for _, t := range timeouts {
		p.pollErr.PushSearched(t)
		if t.Packet.Equal(packet) {
			return t, nil
		}
	}
	return zero, ErrNotFound
}

func (p blockPoller) findProposalStatus(ctx context.Context, height uint64, proposal proposalSearch) error {
	status, err := p.ProposalStatuser.QueryProposal(ctx, proposal.id)
	if err != nil {
		return err
	}
	if status != proposal.status {
		return fmt.Errorf("status: (%s) is not equal to expected: (%s)", status, proposal.status)
	}
	return nil
}

type pollError struct {
	error
	targetPacket    ibc.Packet
	searchedPackets []string
}

func (pe *pollError) SetErr(err error) {
	pe.error = err
}

func (pe *pollError) PushSearched(packet any) {
	pe.searchedPackets = append(pe.searchedPackets, spew.Sdump(packet))
}

func (pe *pollError) Unwrap() error {
	return pe.error
}

// Format is expected to be used by testify/require which prints errors via %+v
func (pe *pollError) Format(s fmt.State, verb rune) {
	if verb != 'v' && !s.Flag('+') {
		fmt.Fprint(s, pe.error.Error())
		return
	}

	searched := strings.Join(pe.searchedPackets, "\n")
	if len(searched) == 0 {
		searched = "(none)"
	}
	target := spew.Sdump(pe.targetPacket)
	final := fmt.Sprintf("%s\n- target packet:\n%s\n- searched:\n%+v", pe.error, target, searched)
	fmt.Fprint(s, final)
}
