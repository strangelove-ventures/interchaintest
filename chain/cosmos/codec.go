package cosmos

import (
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/simapp"
	simappparams "github.com/cosmos/cosmos-sdk/simapp/params"
	"github.com/cosmos/cosmos-sdk/std"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authTx "github.com/cosmos/cosmos-sdk/x/auth/tx"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	transfertypes "github.com/cosmos/ibc-go/v3/modules/apps/transfer/types"
	ibctypes "github.com/cosmos/ibc-go/v3/modules/core/types"
)

func newTestEncoding() simappparams.EncodingConfig {
	// core modules
	cfg := simappparams.MakeTestEncodingConfig()
	std.RegisterLegacyAminoCodec(cfg.Amino)
	std.RegisterInterfaces(cfg.InterfaceRegistry)
	simapp.ModuleBasics.RegisterLegacyAminoCodec(cfg.Amino)
	simapp.ModuleBasics.RegisterInterfaces(cfg.InterfaceRegistry)

	// external modules
	banktypes.RegisterInterfaces(cfg.InterfaceRegistry)
	ibctypes.RegisterInterfaces(cfg.InterfaceRegistry)
	transfertypes.RegisterInterfaces(cfg.InterfaceRegistry)

	return cfg
}

var (
	defaultEncoding = newTestEncoding()
)

func decodeTX(txbz []byte) (sdk.Tx, error) {
	cdc := codec.NewProtoCodec(defaultEncoding.InterfaceRegistry)
	return authTx.DefaultTxDecoder(cdc)(txbz)
}

func encodeTxToJSON(tx sdk.Tx) ([]byte, error) {
	cdc := codec.NewProtoCodec(defaultEncoding.InterfaceRegistry)
	return authTx.DefaultJSONTxEncoder(cdc)(tx)
}
