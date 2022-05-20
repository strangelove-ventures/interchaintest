package test

import (
	"context"
	"fmt"

	"github.com/strangelove-ventures/ibctest/ibc"
)

type ChainAcker interface {
	ChainHeighter
	AcknowledgementPacket(ctx context.Context, height uint64) (ibc.PacketAcknowledgment, error)
}

func PollForAcks(ctx context.Context, heightTimeout int, chain ChainAcker) (zero ibc.PacketAcknowledgment, _ error) {
	var (
		height  = &height{Chain: chain}
		lastErr error
	)
	for {
		if err := height.UpdateOnce(ctx); err != nil {
			return zero, err
		}
		if height.Delta() >= heightTimeout {
			return zero, fmt.Errorf("height timeout %d reached: %w", heightTimeout, lastErr)
		}
		ack, err := chain.AcknowledgementPacket(ctx, height.Current())
		if err != nil {
			lastErr = err
			continue
		}
		return ack, nil
	}
}
