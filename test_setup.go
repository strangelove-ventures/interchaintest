package ibctest

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/docker/docker/client"
	"github.com/ory/dockertest/v3"
	"github.com/strangelove-ventures/ibctest/ibc"
	"github.com/strangelove-ventures/ibctest/internal/dockerutil"
	"github.com/strangelove-ventures/ibctest/internal/version"
	"github.com/strangelove-ventures/ibctest/testreporter"
)

const (
	testPathName = "test-path"

	FaucetAccountKeyName = "faucet"
)

// DockerSetup sets up a new dockertest.Pool (which is a client connection
// to a Docker engine) and configures a network associated with t.
//
// If any part of the setup fails, t.Fatal is called.
func DockerSetup(t *testing.T) (*client.Client, *dockertest.Pool, string) {
	t.Helper()
	return dockerutil.DockerSetup(t)
}

// startup both chains and relayer
// creates wallets in the relayer for src and dst chain
// funds relayer src and dst wallets on respective chain in genesis
// creates a faucet account on the both chains (separate fullnode)
// funds faucet accounts in genesis
func StartChainPairAndRelayer(
	t *testing.T,
	ctx context.Context,
	rep *testreporter.Reporter,
	client *client.Client,
	pool *dockertest.Pool,
	networkID string,
	home string,
	srcChain, dstChain ibc.Chain,
	f RelayerFactory,
	preRelayerStartFuncs []func([]ibc.ChannelOutput),
) (ibc.Relayer, []ibc.ChannelOutput, error) {
	relayerImpl := f.Build(t, client, networkID, home)

	errResponse := func(err error) (ibc.Relayer, []ibc.ChannelOutput, error) {
		return nil, []ibc.ChannelOutput{}, err
	}

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
		HomeDir:           home,
		Pool:              pool, // TODO: switch to client
		NetworkID:         networkID,
		GitSha:            version.GitSha,
		BlockDatabaseFile: blockSqlite,
		CreateChannelOpts: ibc.DefaultChannelOpts(),
	}); err != nil {
		return errResponse(err)
	}
	t.Cleanup(func() {
		_ = ic.Close()
	})

	channels, err := relayerImpl.GetChannels(ctx, eRep, srcChain.Config().ChainID)
	if err != nil {
		return errResponse(fmt.Errorf("failed to get channels: %w", err))
	}
	if len(channels) != 1 {
		return errResponse(fmt.Errorf("channel count invalid. expected: 1, actual: %d", len(channels)))
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

	if err := relayerImpl.StartRelayer(ctx, eRep, testPathName); err != nil {
		return errResponse(fmt.Errorf("failed to start relayer: %w", err))
	}
	t.Cleanup(func() {
		if err := relayerImpl.StopRelayer(ctx, eRep); err != nil {
			t.Logf("error stopping relayer: %v", err)
		}
	})

	// wait for relayer to start up
	time.Sleep(5 * time.Second)

	return relayerImpl, channels, nil
}
