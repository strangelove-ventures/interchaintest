package ibctest

import (
	"context"

	"github.com/cosmos/cosmos-sdk/client/tx"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/strangelove-ventures/ibctest/broadcast"
)

// BroadcastTx uses the provided Broadcaster to broadcast all the provided messages which will be signed
// by the User provided. The sdk.TxResponse and an error are returned.
func BroadcastTx(ctx context.Context, broadcaster broadcast.Broadcaster, broadcastingUser broadcast.User, msgs ...sdk.Msg) (sdk.TxResponse, error) {
	for _, msg := range msgs {
		if err := msg.ValidateBasic(); err != nil {
			return sdk.TxResponse{}, err
		}
	}

	f, err := broadcaster.GetFactory(ctx, broadcastingUser)
	if err != nil {
		return sdk.TxResponse{}, err
	}

	cc, err := broadcaster.GetClientContext(ctx, broadcastingUser)
	if err != nil {
		return sdk.TxResponse{}, err
	}

	if err := tx.BroadcastTx(cc, f, msgs...); err != nil {
		return sdk.TxResponse{}, err
	}

	txBytes, err := broadcaster.GetTxResponseBytes(ctx, broadcastingUser)
	if err != nil {
		return sdk.TxResponse{}, err
	}

	return broadcaster.UnmarshalTxResponseBytes(ctx, txBytes)
}
