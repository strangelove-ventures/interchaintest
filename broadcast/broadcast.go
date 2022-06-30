package broadcast

import (
	"context"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/tx"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

type ClientContextOpt func(clientContext client.Context) client.Context

func NewClientContext(opts ...ClientContextOpt) client.Context {
	c := client.Context{}
	for _, opt := range opts {
		c = opt(c)
	}
	return c
}

type FactoryOpt func(factory tx.Factory) tx.Factory

func NewTxFactory(opts ...FactoryOpt) tx.Factory {
	f := tx.Factory{}
	for _, opt := range opts {
		f = opt(f)
	}
	return f
}

type User interface {
	GetKeyName() string
	Bech32Address(bech32Prefix string) string
}

type Broadcaster interface {
	GetFactory(ctx context.Context, user User) (tx.Factory, error)
	GetClientContext(ctx context.Context, user User) (client.Context, error)
	GetTxResponseBytes(ctx context.Context, user User) ([]byte, error)
	UnmarshalTxResponseBytes(ctx context.Context, bytes []byte) (sdk.TxResponse, error)
}

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
