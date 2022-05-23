package test

import (
	"context"
	"fmt"

	"github.com/strangelove-ventures/ibctest/ibc"
)

type ChainAcker interface {
	ChainHeighter
	Acknowledgements(ctx context.Context, height uint64) ([]ibc.PacketAcknowledgement, error)
}

func PollForAck(ctx context.Context, chain ChainAcker, startHeight, maxHeight uint64, packet ibc.Packet) (zero ibc.PacketAcknowledgement, _ error) {
	heightIdx := startHeight
	for heightIdx <= maxHeight {
		curHeight, err := chain.Height(ctx)
		if err != nil {
			// TODO
			panic(err)
		}
		if heightIdx > curHeight {
			continue
		}

		fmt.Println("searching block", heightIdx) // TODO: delete me
		acks, err := chain.Acknowledgements(ctx, heightIdx)
		if err != nil {
			// TODO: capture error
			fmt.Println("ERR:", err)
			heightIdx++
			continue
		}
		for _, ack := range acks {
			if packet.Equal(ack.Packet) {
				return ack, nil
			}
		}
		heightIdx++
	}
	return zero, fmt.Errorf("packet %d not found", packet.Sequence)
}
