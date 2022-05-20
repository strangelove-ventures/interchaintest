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

func PollForAcks(ctx context.Context, heightTimeout int, chain ChainAcker, cb func(ibc.PacketAcknowledgment) bool) error {
	var (
		height  = &height{Chain: chain}
		lastErr error
	)
	for {
		if err := height.UpdateOnce(ctx); err != nil {
			return err
		}
		if height.Delta() >= heightTimeout {
			if lastErr != nil {
				return fmt.Errorf("height timeout %d reached: %w", heightTimeout, lastErr)
			} else {
				return fmt.Errorf("height timeout %d reached", heightTimeout)
			}
		}
		ack, err := chain.AcknowledgementPacket(ctx, height.Current())
		if err != nil {
			lastErr = err
			continue
		}
		if ok := cb(ack); ok {
			return nil
		}
	}
}
