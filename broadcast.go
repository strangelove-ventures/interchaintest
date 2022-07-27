package ibctest

import (
	"context"
	"fmt"
)

// BroadcastTx is deprecated.
//
// Deprecated: Use chain/cosmos.BroadcastTx for broadcasting to chains built with Cosmos SDK.
func BroadcastTx(ctx context.Context, broadcaster any, broadcastingUser any, msgs ...any) (any, error) {
	panic(fmt.Errorf("BroadcastTx is deprecated; use chain/cosmos.BroadcastTx"))
}
