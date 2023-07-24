package wasm

import (
	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	// simappparams "github.com/cosmos/cosmos-sdk/simapp/params"
	"github.com/strangelove-ventures/interchaintest/v7/chain/cosmos"

	"github.com/cosmos/cosmos-sdk/types/module/testutil"
)

func WasmEncoding() *testutil.TestEncodingConfig {
	cfg := cosmos.DefaultEncoding()

	wasmtypes.RegisterInterfaces(cfg.InterfaceRegistry)

	return &cfg
}
