package cosmos

import (
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/simapp"
	simappparams "github.com/cosmos/cosmos-sdk/simapp/params"
	"github.com/cosmos/cosmos-sdk/std"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authTx "github.com/cosmos/cosmos-sdk/x/auth/tx"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	transfertypes "github.com/cosmos/ibc-go/v3/modules/apps/transfer/types"
	ibctypes "github.com/cosmos/ibc-go/v3/modules/core/types"
	ccvprovidertypes "github.com/cosmos/interchain-security/x/ccv/provider/types"
)

func DefaultEncoding() simappparams.EncodingConfig {
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
	ccvprovidertypes.RegisterInterfaces(cfg.InterfaceRegistry)

	return cfg
}

func decodeTX(interfaceRegistry codectypes.InterfaceRegistry, txbz []byte) (sdk.Tx, error) {
	cdc := codec.NewProtoCodec(interfaceRegistry)
	return authTx.DefaultTxDecoder(cdc)(txbz)
}

func encodeTxToJSON(interfaceRegistry codectypes.InterfaceRegistry, tx sdk.Tx) ([]byte, error) {
	cdc := codec.NewProtoCodec(interfaceRegistry)
	return authTx.DefaultJSONTxEncoder(cdc)(tx)
}
