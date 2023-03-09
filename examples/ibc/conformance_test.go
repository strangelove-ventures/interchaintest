package ibc

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/strangelove-ventures/interchaintest/v7"
	"github.com/strangelove-ventures/interchaintest/v7/conformance"
	"github.com/strangelove-ventures/interchaintest/v7/ibc"
	"github.com/strangelove-ventures/interchaintest/v7/relayer"
	"github.com/strangelove-ventures/interchaintest/v7/relayer/rly"
	"github.com/strangelove-ventures/interchaintest/v7/testreporter"
	"github.com/strangelove-ventures/interchaintest/v7/testutil"
	"go.uber.org/zap/zaptest"
)

func TestSeiStrideConformance(t *testing.T) {
	ctx := context.Background()

	log := zaptest.NewLogger(t)

	seiConfigFileOverrides := make(map[string]any)
	seiConfigTomlOverrides := make(testutil.Toml)

	seiConfigTomlOverrides["mode"] = "validator"

	seiBlockTime := 100 * time.Millisecond

	consensus := make(testutil.Toml)

	seiBlockT := seiBlockTime.String()
	consensus["timeout-commit"] = seiBlockT
	consensus["timeout-propose"] = seiBlockT
	seiConfigTomlOverrides["consensus"] = consensus

	seiConfigFileOverrides[filepath.Join("config", "config.toml")] = seiConfigTomlOverrides

	nf := 0

	cf := interchaintest.NewBuiltinChainFactory(log, []*interchaintest.ChainSpec{
		{Name: "stride", Version: "v6.0.0"},
		{Name: "sei", Version: "2.0.39beta-internal-2", NumFullNodes: &nf, ChainConfig: ibc.ChainConfig{
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
