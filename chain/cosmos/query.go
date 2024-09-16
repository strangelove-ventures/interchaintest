package cosmos

import (
	"context"
	"fmt"

	"cosmossdk.io/x/tx/decode"
	tmtypes "github.com/cometbft/cometbft/rpc/core/types"
	baseapptestutil "github.com/cosmos/cosmos-sdk/baseapp/testutil"
	codectestutil "github.com/cosmos/cosmos-sdk/codec/testutil"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"go.uber.org/zap"
)

type blockClient interface {
	Block(ctx context.Context, height *int64) (*tmtypes.ResultBlock, error)
}

// RangeBlockMessages iterates through all a block's transactions and each transaction's messages yielding to f.
// Return true from f to stop iteration.
func RangeBlockMessages(ctx context.Context, interfaceRegistry codectypes.InterfaceRegistry, client blockClient, height int64, done func(sdk.Msg) bool) error {
	h := int64(height)
	block, err := client.Block(ctx, &h)
	if err != nil {
		return fmt.Errorf("tendermint rpc get block: %w", err)
	}
	for _, txbz := range block.Block.Txs {
		// TODO: move this to the root
		cdc := codectestutil.CodecOptions{}.NewCodec()
		baseapptestutil.RegisterInterfaces(cdc.InterfaceRegistry())
		signingCtx := cdc.InterfaceRegistry().SigningContext()
		ac := signingCtx.AddressCodec()
		// txCfg := authTx.NewTxConfig(cdc, signingCtx.AddressCodec(), signingCtx.ValidatorAddressCodec(), authTx.DefaultSignModes)

		dec, err := decode.NewDecoder(decode.Options{
			SigningContext: signingCtx,
			ProtoCodec:     cdc,
		})
		if err != nil {
			zap.L().Error("failed to create decoder", zap.Error(err))
			continue
		}

		tx, err := decodeTX(ac, interfaceRegistry, dec, txbz)
		if err != nil {
			return fmt.Errorf("decode tendermint tx: %w", err)
		}
		for _, m := range tx.GetMsgs() {
			if ok := done(m); ok {
				return nil
			}
		}
	}
	return nil
}
