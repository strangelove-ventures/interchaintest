package test

import (
	"context"
	"errors"
	"fmt"

	"github.com/strangelove-ventures/ibctest/ibc"
)

type ChainAcker interface {
	ChainHeighter
	Acknowledgement(ctx context.Context, height uint64) (ibc.PacketAcknowledgement, error)
}

// TODO: pass in sequence and match on sequence, src port chan, dst port chan, then return the ack?
func PollForAck(ctx context.Context, startHeight, maxHeight uint64, chain ChainAcker, cb func(ibc.PacketAcknowledgement) bool) error {
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

		fmt.Println("searching block", heightIdx)
		ack, err := chain.Acknowledgement(ctx, heightIdx)
		if err != nil {
			// TODO: capture error
			fmt.Println("ERR:", err)
			heightIdx++
			continue
		}
		if ok := cb(ack); ok {
			return nil
		}
		heightIdx++
	}
	return errors.New("TODO: NOT FOUND")
	//var (
	//	height  = &height{Chain: chain}
	//	lastErr error
	//)
	//for {
	//	if err := height.UpdateOnce(ctx); err != nil {
	//		return err
	//	}
	//	if height.Delta() >= heightTimeout {
	//		if lastErr != nil {
	//			return fmt.Errorf("height timeout %d reached: %w", heightTimeout, lastErr)
	//		} else {
	//			return fmt.Errorf("height timeout %d reached", heightTimeout)
	//		}
	//	}
	//	cur := height.Current()
	//	ack, err := chain.Acknowledgement(ctx, cur)
	//	if err != nil {
	//		lastErr = err
	//		continue
	//	}
	//	if ok := cb(ack); ok {
	//		return nil
	//	}
	//}
}
