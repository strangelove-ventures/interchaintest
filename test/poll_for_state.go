package test

import (
	"context"
	"fmt"

	"github.com/strangelove-ventures/ibctest/ibc"
)

type ChainAcker interface {
	ChainHeighter
	Acknowledgement(ctx context.Context, height uint64) (ibc.PacketAcknowledgement, error)
}

func PollForAcks(ctx context.Context, heightTimeout int, chain ChainAcker, cb func(ibc.PacketAcknowledgement) bool) error {
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
		ack, err := chain.Acknowledgement(ctx, height.Current())
		if err != nil {
			lastErr = err
			continue
		}
		if ok := cb(ack); ok {
			return nil
		}
	}
}
