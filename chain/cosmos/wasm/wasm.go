package wasm

import (
	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	simappparams "github.com/cosmos/cosmos-sdk/simapp/params"
	"github.com/strangelove-ventures/interchaintest/v4/chain/cosmos"
)

func WasmEncoding() *simappparams.EncodingConfig {
	cfg := cosmos.DefaultEncoding()

	wasmtypes.RegisterInterfaces(cfg.InterfaceRegistry)

	return &cfg
}
