package ibc

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/strangelove-ventures/interchaintest/v6"
	"github.com/strangelove-ventures/interchaintest/v6/conformance"
	"github.com/strangelove-ventures/interchaintest/v6/ibc"
	"github.com/strangelove-ventures/interchaintest/v6/relayer"
	"github.com/strangelove-ventures/interchaintest/v6/relayer/rly"
	"github.com/strangelove-ventures/interchaintest/v6/testreporter"
	"github.com/strangelove-ventures/interchaintest/v6/testutil"
	"go.uber.org/zap/zaptest"
)

func TestSeiStrideConformance(t *testing.T) {
	ctx := context.Background()

	log := zaptest.NewLogger(t)

	seiConfigFileOverrides := make(map[string]any)
	seiAppTomlOverrides := make(testutil.Toml)

	seiBlockTime := 100 * time.Millisecond

	seiBlockT := seiBlockTime.String()
	seiAppTomlOverrides["timeout-commit"] = seiBlockT
	seiAppTomlOverrides["timeout-propose"] = seiBlockT

	seiConfigFileOverrides[filepath.Join("config", "app.toml")] = seiAppTomlOverrides

	cf := interchaintest.NewBuiltinChainFactory(log, []*interchaintest.ChainSpec{
		{Name: "stride", Version: "v6.0.0"},
		{Name: "sei", Version: "2.0.39beta-internal-2", ChainConfig: ibc.ChainConfig{
			ConfigFileOverrides: seiConfigFileOverrides,
		}},
	})

	rf := interchaintest.NewBuiltinRelayerFactory(
		ibc.CosmosRly,
		log,
		relayer.CustomDockerImage("ghcr.io/cosmos/relayer", "andrew-consolidate_cosmos_tx_broadcast", rly.RlyDefaultUidGid),
	)

	conformance.Test(
		t,
		ctx,
		[]interchaintest.ChainFactory{cf},
		[]interchaintest.RelayerFactory{rf},
		testreporter.NewNopReporter(),
	)
}
