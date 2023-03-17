package interchaintest

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/docker/docker/client"
	"github.com/strangelove-ventures/interchaintest/v7/ibc"
	"github.com/strangelove-ventures/interchaintest/v7/internal/dockerutil"
	"github.com/strangelove-ventures/interchaintest/v7/internal/version"
	"github.com/strangelove-ventures/interchaintest/v7/testreporter"
)

const (
	testPathName = "test-path"

	FaucetAccountKeyName = "faucet"
)

// KeepDockerVolumesOnFailure sets whether volumes associated with a particular test
// are retained or deleted following a test failure.
//
// The value is false by default, but can be initialized to true by setting the
// environment variable IBCTEST_SKIP_FAILURE_CLEANUP to a non-empty value.
// Alternatively, importers of the interchaintest package may call KeepDockerVolumesOnFailure(true).
func KeepDockerVolumesOnFailure(b bool) {
	dockerutil.KeepVolumesOnFailure = b
}

// DockerSetup returns a new Docker Client and the ID of a configured network, associated with t.
//
// If any part of the setup fails, t.Fatal is called.
func DockerSetup(t *testing.T) (*client.Client, string) {
	t.Helper()
	return dockerutil.DockerSetup(t)
}

// startup both chains
// creates wallets in the relayer for src and dst chain
// funds relayer src and dst wallets on respective chain in genesis
// creates a faucet account on the both chains (separate fullnode)
// funds faucet accounts in genesis
func StartChainPair(
	t *testing.T,
	ctx context.Context,
	rep *testreporter.Reporter,
	cli *client.Client,
	networkID string,
	srcChain, dstChain ibc.Chain,
	f RelayerFactory,
	preRelayerStartFuncs []func([]ibc.ChannelOutput),
) (ibc.Relayer, error) {
	relayerImpl := f.Build(t, cli, networkID)

	ic := NewInterchain().
		AddChain(srcChain).
		AddChain(dstChain).
		AddRelayer(relayerImpl, "r").
		AddLink(InterchainLink{
			Chain1:  srcChain,
			Chain2:  dstChain,
			Relayer: relayerImpl,
			Path:    testPathName,
		})

	blockSqlite := DefaultBlockDatabaseFilepath()
	t.Logf("View block history using sqlite console at %s", blockSqlite)

	eRep := rep.RelayerExecReporter(t)
	if err := ic.Build(ctx, eRep, InterchainBuildOptions{
		TestName:          t.Name(),
		Client:            cli,
		NetworkID:         networkID,
		GitSha:            version.GitSha,
		BlockDatabaseFile: blockSqlite,
	}); err != nil {
		return nil, err
	}
	t.Cleanup(func() {
		_ = ic.Close()
	})

	return relayerImpl, nil
}

// StopStartRelayerWithPreStartFuncs will stop the relayer if it is currently running,
// then execute the preRelayerStartFuncs and wait for all to complete before starting
// the relayer.
func StopStartRelayerWithPreStartFuncs(
	t *testing.T,
	ctx context.Context,
	srcChainID string,
	relayerImpl ibc.Relayer,
	eRep *testreporter.RelayerExecReporter,
	preRelayerStartFuncs []func([]ibc.ChannelOutput),
	pathNames ...string,
) ([]ibc.ChannelOutput, error) {
	if err := relayerImpl.StopRelayer(ctx, eRep); err != nil {
		t.Logf("error stopping relayer: %v", err)
	}
	channels, err := relayerImpl.GetChannels(ctx, eRep, srcChainID)
	if err != nil {
		return nil, fmt.Errorf("failed to get channels: %w", err)
	}
	if len(channels) == 0 {
		return nil, fmt.Errorf("channel count invalid. expected: > 0, actual: %d", len(channels))
	}

	wg := sync.WaitGroup{}
	for _, preRelayerStart := range preRelayerStartFuncs {
		if preRelayerStart == nil {
			continue
		}
		preRelayerStart := preRelayerStart
		wg.Add(1)
		go func() {
			preRelayerStart(channels)
			wg.Done()
		}()
	}
	wg.Wait()

	if len(pathNames) == 0 {
		if err := relayerImpl.StartRelayer(ctx, eRep, testPathName); err != nil {
			return nil, fmt.Errorf("failed to start relayer: %w", err)
		}
	} else {
		if err := relayerImpl.StartRelayer(ctx, eRep, pathNames...); err != nil {
			return nil, fmt.Errorf("failed to start relayer: %w", err)
		}
	}

	// TODO: cleanup since this will stack multiple StopRelayer calls for
	// multiple calls to this func, requires StopRelayer to be idempotent.
	t.Cleanup(func() {
		if err := relayerImpl.StopRelayer(ctx, eRep); err != nil {
			t.Logf("error stopping relayer: %v", err)
		}
	})

	// wait for relayer(s) to start up
	time.Sleep(5 * time.Second)

	return channels, nil
}
