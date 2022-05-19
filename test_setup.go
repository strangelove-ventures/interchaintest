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
	"github.com/strangelove-ventures/ibctest/dockerutil"
	"github.com/strangelove-ventures/ibctest/ibc"
	"github.com/strangelove-ventures/ibctest/testreporter"
)

const (
	srcAccountKeyName    = "src-chain"
	dstAccountKeyName    = "dst-chain"
	faucetAccountKeyName = "faucet"
	testPathName         = "test-path"
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

	testName := t.Name()
	cs := chainSet{srcChain, dstChain}
	if err := cs.Initialize(testName, home, pool, networkID); err != nil {
		return errResponse(err)
	}

	srcChainCfg := srcChain.Config()
	dstChainCfg := dstChain.Config()

	// Create addresses out of band, because the chain genesis needs to know where to send initial funds.
	addresses, mnemonics, err := cs.CreateKeys()
	if err != nil {
		return errResponse(err)
	}

	// Fund relayer account on src chain
	srcRelayerWalletAmount := ibc.WalletAmount{
		Address: addresses[0],
		Denom:   srcChainCfg.Denom,
		Amount:  10_000_000,
	}

	// Fund relayer account on dst chain
	dstRelayerWalletAmount := ibc.WalletAmount{
		Address: addresses[1],
		Denom:   dstChainCfg.Denom,
		Amount:  10_000_000,
	}

	// create faucets on both chains
	faucetAccounts, err := cs.CreateCommonAccount(ctx, faucetAccountKeyName)
	if err != nil {
		return errResponse(err)
	}

	srcFaucetWalletAmount := ibc.WalletAmount{
		Address: faucetAccounts[0],
		Denom:   srcChainCfg.Denom,
		Amount:  10_000_000_000_000,
	}

	dstFaucetWalletAmount := ibc.WalletAmount{
		Address: faucetAccounts[1],
		Denom:   dstChainCfg.Denom,
		Amount:  10_000_000_000_000,
	}

	// start chains from genesis, wait until they are producing blocks
	if err := cs.Start(ctx, testName, [][]ibc.WalletAmount{
		{srcRelayerWalletAmount, srcFaucetWalletAmount},
		{dstRelayerWalletAmount, dstFaucetWalletAmount},
	}); err != nil {
		return errResponse(err)
	}

	// Now that the chains are running, we can start the relayer.
	// (We couldn't do this earlier,
	// because a non-docker relayer would not have had an address for the nodes.)
	srcRPCAddr, srcGRPCAddr := srcChain.GetRPCAddress(), srcChain.GetGRPCAddress()
	dstRPCAddr, dstGRPCAddr := dstChain.GetRPCAddress(), dstChain.GetGRPCAddress()
	if !f.UseDockerNetwork() {
		srcRPCAddr, srcGRPCAddr = srcChain.GetHostRPCAddress(), srcChain.GetHostGRPCAddress()
		dstRPCAddr, dstGRPCAddr = dstChain.GetHostRPCAddress(), dstChain.GetHostGRPCAddress()
	}

	eRep := rep.RelayerExecReporter(t)

	if err := relayerImpl.AddChainConfiguration(ctx,
		eRep,
		srcChainCfg, srcAccountKeyName,
		srcRPCAddr, srcGRPCAddr,
	); err != nil {
		return errResponse(fmt.Errorf("failed to configure relayer for source chain: %w", err))
	}

	if err := relayerImpl.AddChainConfiguration(ctx,
		eRep,
		dstChainCfg, dstAccountKeyName,
		dstRPCAddr, dstGRPCAddr,
	); err != nil {
		return errResponse(fmt.Errorf("failed to configure relayer for dest chain: %w", err))
	}

	if err := relayerImpl.RestoreKey(ctx, eRep, srcChain.Config().ChainID, srcAccountKeyName, mnemonics[0]); err != nil {
		return errResponse(fmt.Errorf("failed to restore key to source chain: %w", err))
	}
	if err := relayerImpl.RestoreKey(ctx, eRep, dstChain.Config().ChainID, dstAccountKeyName, mnemonics[1]); err != nil {
		return errResponse(fmt.Errorf("failed to restore key to dest chain: %w", err))
	}

	if err := relayerImpl.GeneratePath(ctx, eRep, srcChainCfg.ChainID, dstChainCfg.ChainID, testPathName); err != nil {
		return errResponse(fmt.Errorf("failed to generate path: %w", err))
	}

	if err := relayerImpl.LinkPath(ctx, eRep, testPathName); err != nil {
		return errResponse(fmt.Errorf("failed to create link in relayer: %w", err))
	}

	channels, err := relayerImpl.GetChannels(ctx, eRep, srcChainCfg.ChainID)
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
