package ibctest

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
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
func DockerSetup(t *testing.T) (*dockertest.Pool, string) {
	t.Helper()

	pool, err := dockertest.NewPool("")
	if err != nil {
		t.Fatalf("failed to create dockertest pool: %v", err)
	}

	// Clean up docker resources at end of test.
	t.Cleanup(dockerCleanup(t.Name(), pool))

	// Also eagerly clean up any leftover resources from a previous test run,
	// e.g. if the test was interrupted.
	dockerCleanup(t.Name(), pool)()

	network, err := pool.Client.CreateNetwork(docker.CreateNetworkOptions{
		Name:           fmt.Sprintf("ibctest-%s", dockerutil.RandLowerCaseLetterString(8)),
		Options:        map[string]interface{}{},
		Labels:         map[string]string{"ibc-test": t.Name()},
		CheckDuplicate: true,
		Internal:       false,
		EnableIPv6:     false,
		Context:        context.Background(),
	})
	if err != nil {
		t.Fatalf("failed to create docker network: %v", err)

	}

	return pool, network.ID
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
	pool *dockertest.Pool,
	networkID string,
	home string,
	srcChain, dstChain ibc.Chain,
	f RelayerFactory,
	preRelayerStartFuncs []func([]ibc.ChannelOutput),
) (ibc.Relayer, []ibc.ChannelOutput, error) {
	relayerImpl := f.Build(t, pool, networkID, home)

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
		Pool:              pool,
		NetworkID:         networkID,
		GitSha:            version.GitSha,
		BlockDatabaseFile: blockSqlite,
	}); err != nil {
		return errResponse(err)
	}

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

// dockerCleanup will clean up Docker containers, networks, and the other various config files generated in testing
func dockerCleanup(testName string, pool *dockertest.Pool) func() {
	return func() {
		showContainerLogs := os.Getenv("SHOW_CONTAINER_LOGS")
		cont, _ := pool.Client.ListContainers(docker.ListContainersOptions{All: true})
		for _, c := range cont {
			for k, v := range c.Labels {
				if k == "ibc-test" && v == testName {
					_ = pool.Client.StopContainer(c.ID, 10)
					ctxWait, cancelWait := context.WithTimeout(context.Background(), time.Duration(time.Second*5))
					defer cancelWait()
					_, _ = pool.Client.WaitContainerWithContext(c.ID, ctxWait)
					if showContainerLogs != "" {
						stdout := new(bytes.Buffer)
						stderr := new(bytes.Buffer)
						ctxLogs, cancelLogs := context.WithTimeout(context.Background(), time.Duration(time.Second*5))
						defer cancelLogs()
						_ = pool.Client.Logs(docker.LogsOptions{Context: ctxLogs, Container: c.ID, OutputStream: stdout, ErrorStream: stderr, Stdout: true, Stderr: true, Tail: "50", Follow: false, Timestamps: false})
					}
					_ = pool.Client.RemoveContainer(docker.RemoveContainerOptions{ID: c.ID, Force: true})
					break
				}
			}
		}
		nets, _ := pool.Client.ListNetworks()
		for _, n := range nets {
			for k, v := range n.Labels {
				if k == "ibc-test" && v == testName {
					_ = pool.Client.RemoveNetwork(n.ID)
					break
				}
			}
		}
	}
}
