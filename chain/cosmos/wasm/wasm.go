package wasm

import (
	simappparams "github.com/cosmos/cosmos-sdk/simapp/params"
	"github.com/strangelove-ventures/ibctest/v3/chain/cosmos"
	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
)

func WasmEncoding() *simappparams.EncodingConfig {
	cfg := cosmos.DefaultEncoding()

	wasmtypes.RegisterInterfaces(cfg.InterfaceRegistry)

	return &cfg
}