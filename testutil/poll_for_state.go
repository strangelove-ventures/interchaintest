package testutil

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/davecgh/go-spew/spew"
	"github.com/strangelove-ventures/interchaintest/v7/ibc"
)

var ErrNotFound = errors.New("not found")

type BlockPoller[T any] struct {
	CurrentHeight func(ctx context.Context) (uint64, error)
	PollFunc      func(ctx context.Context, height uint64) (T, error)
}

func (p BlockPoller[T]) DoPoll(ctx context.Context, startHeight, maxHeight uint64) (T, error) {
	if maxHeight < startHeight {
		panic("maxHeight must be greater than or equal to startHeight")
	}

	var (
		pollErr error
		zero    T
	)

	cursor := startHeight
	for cursor <= maxHeight {
		curHeight, err := p.CurrentHeight(ctx)
		if err != nil {
			return zero, err
		}
		if cursor > curHeight {
			continue
		}

		found, findErr := p.PollFunc(ctx, cursor)

		if findErr != nil {
			pollErr = findErr
			cursor++
			continue
		}

		return found, nil
	}
	return zero, pollErr
}

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
	var zero ibc.PacketAcknowledgement
	pollError := &packetPollError{targetPacket: packet}
	poll := func(ctx context.Context, height uint64) (ibc.PacketAcknowledgement, error) {
		acks, err := chain.Acknowledgements(ctx, height)
		if err != nil {
			return zero, err
		}
		for _, ack := range acks {
			pollError.PushSearched(ack)
			if ack.Packet.Equal(packet) {
				return ack, nil
			}
		}
		return zero, ErrNotFound
	}

	poller := BlockPoller[ibc.PacketAcknowledgement]{CurrentHeight: chain.Height, PollFunc: poll}
	found, err := poller.DoPoll(ctx, startHeight, maxHeight)
	if err != nil {
		pollError.SetErr(err)
		return zero, pollError
	}
	return found, nil
}

// ChainTimeouter is a chain that can get its timeouts at a specified height
type ChainTimeouter interface {
	ChainHeighter
	Timeouts(ctx context.Context, height uint64) ([]ibc.PacketTimeout, error)
}

// PollForTimeout attempts to find a timeout containing a packet equal to the packet argument.
// Otherwise, works identically to PollForAck.
func PollForTimeout(ctx context.Context, chain ChainTimeouter, startHeight, maxHeight uint64, packet ibc.Packet) (ibc.PacketTimeout, error) {
	pollError := &packetPollError{targetPacket: packet}
	var zero ibc.PacketTimeout
	poll := func(ctx context.Context, height uint64) (ibc.PacketTimeout, error) {
		timeouts, err := chain.Timeouts(ctx, height)
		if err != nil {
			return zero, err
		}
		for _, t := range timeouts {
			pollError.PushSearched(t)
			if t.Packet.Equal(packet) {
				return t, nil
			}
		}
		return zero, ErrNotFound
	}

	poller := BlockPoller[ibc.PacketTimeout]{CurrentHeight: chain.Height, PollFunc: poll}
	found, err := poller.DoPoll(ctx, startHeight, maxHeight)
	if err != nil {
		pollError.SetErr(err)
		return zero, pollError
	}
	return found, nil
}

type packetPollError struct {
	error
	targetPacket    ibc.Packet
	searchedPackets []string
}

func (pe *packetPollError) SetErr(err error) {
	pe.error = err
}

func (pe *packetPollError) PushSearched(packet any) {
	pe.searchedPackets = append(pe.searchedPackets, spew.Sdump(packet))
}

func (pe *packetPollError) Unwrap() error {
	return pe.error
}

// Format is expected to be used by testify/require which prints errors via %+v
func (pe *packetPollError) Format(s fmt.State, verb rune) {
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
