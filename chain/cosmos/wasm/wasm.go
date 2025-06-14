package wasm

import (
	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"

	"github.com/cosmos/cosmos-sdk/types/module/testutil"

	// simappparams "github.com/cosmos/cosmos-sdk/simapp/params".
	"github.com/cosmos/interchaintest/v10/chain/cosmos"
)

func WasmEncoding() *testutil.TestEncodingConfig {
	cfg := cosmos.DefaultEncoding()

	wasmtypes.RegisterInterfaces(cfg.InterfaceRegistry)

	return &cfg
}
