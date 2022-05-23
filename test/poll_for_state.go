package test

import (
	"context"
	"errors"

	"github.com/strangelove-ventures/ibctest/ibc"
)

type ChainAcker interface {
	ChainHeighter
	Acknowledgements(ctx context.Context, height uint64) ([]ibc.PacketAcknowledgement, error)
}

func PollForAck(ctx context.Context, chain ChainAcker, startHeight, maxHeight uint64, packet ibc.Packet) (zero ibc.PacketAcknowledgement, _ error) {
	if maxHeight < startHeight {
		panic("maxHeight must be greater than or equal to startHeight")
	}
	var (
		cursor  = startHeight
		lastErr error
	)

	for cursor <= maxHeight {
		curHeight, err := chain.Height(ctx)
		if err != nil {
			return zero, err
		}
		if cursor > curHeight {
			continue
		}

		acks, err := chain.Acknowledgements(ctx, cursor)
		if err != nil {
			lastErr = err
			cursor++
			continue
		}
		for _, ack := range acks {
			if packet.Equal(ack.Packet) {
				return ack, nil
			}
		}
		cursor++
	}

	if err := lastErr; err != nil {
		return zero, err
	}
	return zero, errors.New("acknowledgement not found")
}
