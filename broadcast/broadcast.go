package broadcast

import (
	"context"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/tx"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

type ClientContextOpt func(clientContext client.Context) client.Context

type FactoryOpt func(factory tx.Factory) tx.Factory

type User interface {
	GetKeyName() string
	Bech32Address(bech32Prefix string) string
}

type Broadcaster interface {
	ConfigureFactoryOptions(opts ...FactoryOpt)
	ConfigureClientContextOptions(opts ...ClientContextOpt)
	GetFactory(ctx context.Context, user User) (tx.Factory, error)
	GetClientContext(ctx context.Context, user User) (client.Context, error)
	GetTxResponseBytes(ctx context.Context, user User) ([]byte, error)
	UnmarshalTxResponseBytes(ctx context.Context, bytes []byte) (sdk.TxResponse, error)
}

// Tx uses the provided Broadcaster to broadcast all the provided messages which will be signed
// by the User provided. The sdk.TxResponse and an error are returned.
func Tx(ctx context.Context, broadcaster Broadcaster, broadcastingUser User, msgs ...sdk.Msg) (sdk.TxResponse, error) {
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
