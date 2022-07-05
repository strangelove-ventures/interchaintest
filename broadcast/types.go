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

// Broadcaster implementations can broadcast messages as the provided user.
type Broadcaster interface {
	ConfigureFactoryOptions(opts ...FactoryOpt)
	ConfigureClientContextOptions(opts ...ClientContextOpt)
	GetFactory(ctx context.Context, user User) (tx.Factory, error)
	GetClientContext(ctx context.Context, user User) (client.Context, error)
	GetTxResponseBytes(ctx context.Context, user User) ([]byte, error)
	UnmarshalTxResponseBytes(ctx context.Context, bytes []byte) (sdk.TxResponse, error)
}
