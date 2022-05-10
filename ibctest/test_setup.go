package ibctest

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/types"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"github.com/strangelove-ventures/ibc-test-framework/dockerutil"
	"github.com/strangelove-ventures/ibc-test-framework/ibc"
	"github.com/strangelove-ventures/ibc-test-framework/log"
	"golang.org/x/sync/errgroup"
)

const (
	srcAccountKeyName    = "src-chain"
	dstAccountKeyName    = "dst-chain"
	faucetAccountKeyName = "faucet"
	testPathName         = "test-path"
)

func SetupTestRun(t *testing.T) (context.Context, string, *dockertest.Pool, string, error) {
	ctx := context.Background()

	pool, err := dockertest.NewPool("")
	if err != nil {
		return ctx, "", nil, "", err
	}

	// schedule cleanup for afterwards
	t.Cleanup(dockerCleanup(t.Name(), pool))

	// run cleanup for this test first in case previous run was killed (e.g. Ctrl+C)
	dockerCleanup(t.Name(), pool)()

	home := t.TempDir()

	networkName := fmt.Sprintf("ibc-test-framework-%s", dockerutil.RandLowerCaseLetterString(8))
	network, err := CreateTestNetwork(pool, networkName, t.Name())
	if err != nil {
		return ctx, "", nil, "", err
	}

	return ctx, home, pool, network.ID, nil
}

// startup both chains and relayer
// creates wallets in the relayer for src and dst chain
// funds relayer src and dst wallets on respective chain in genesis
// creates a faucet account on the both chains (separate fullnode)
// funds faucet accounts in genesis
func StartChainsAndRelayerFromFactory(
	t *testing.T,
	ctx context.Context,
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
	if err := srcChain.Initialize(testName, home, pool, networkID); err != nil {
		return errResponse(fmt.Errorf("failed to initialize source chain: %w", err))
	}
	if err := dstChain.Initialize(testName, home, pool, networkID); err != nil {
		return errResponse(fmt.Errorf("failed to initialize dest chain: %w", err))
	}

	srcChainCfg := srcChain.Config()
	dstChainCfg := dstChain.Config()

	kr := keyring.NewInMemory()

	// NOTE: this is hardcoded to the cosmos coin type.
	// We will need to choose other coin types for non-cosmos IBC once that happens.
	const coinType = types.CoinType

	// Create accounts out of band, because the chain genesis needs to know where to send initial funds.
	srcInfo, srcMnemonic, err := kr.NewMnemonic(srcAccountKeyName, keyring.English, hd.CreateHDPath(coinType, 0, 0).String(), "", hd.Secp256k1)
	if err != nil {
		return errResponse(fmt.Errorf("failed to create source account: %w", err))
	}
	srcAccount := types.MustBech32ifyAddressBytes(srcChainCfg.Bech32Prefix, srcInfo.GetAddress().Bytes())

	dstInfo, dstMnemonic, err := kr.NewMnemonic(dstAccountKeyName, keyring.English, hd.CreateHDPath(coinType, 0, 0).String(), "", hd.Secp256k1)
	if err != nil {
		return errResponse(fmt.Errorf("failed to create dest account: %w", err))
	}
	dstAccount := types.MustBech32ifyAddressBytes(dstChainCfg.Bech32Prefix, dstInfo.GetAddress().Bytes())

	// Fund relayer account on src chain
	srcRelayerWalletAmount := ibc.WalletAmount{
		Address: srcAccount,
		Denom:   srcChainCfg.Denom,
		Amount:  10_000_000,
	}

	// Fund relayer account on dst chain
	dstRelayerWalletAmount := ibc.WalletAmount{
		Address: dstAccount,
		Denom:   dstChainCfg.Denom,
		Amount:  10_000_000,
	}

	// create faucets on both chains

	if err := srcChain.CreateKey(ctx, faucetAccountKeyName); err != nil {
		return errResponse(fmt.Errorf("failed to create faucet key on source chain: %w", err))
	}

	srcFaucetAccountAddressBytes, err := srcChain.GetAddress(ctx, faucetAccountKeyName)
	if err != nil {
		return errResponse(fmt.Errorf("failed to get source faucet account address: %w", err))
	}

	srcFaucetAccount, err := types.Bech32ifyAddressBytes(srcChainCfg.Bech32Prefix, srcFaucetAccountAddressBytes)
	if err != nil {
		return errResponse(err)
	}

	if err := dstChain.CreateKey(ctx, faucetAccountKeyName); err != nil {
		return errResponse(fmt.Errorf("failed to create faucet key on destination chain: %w", err))
	}

	dstFaucetAccountAddressBytes, err := dstChain.GetAddress(ctx, faucetAccountKeyName)
	if err != nil {
		return errResponse(fmt.Errorf("failed to get destination faucet account address: %w", err))
	}

	dstFaucetAccount, err := types.Bech32ifyAddressBytes(dstChainCfg.Bech32Prefix, dstFaucetAccountAddressBytes)
	if err != nil {
		return errResponse(err)
	}

	srcFaucetWalletAmount := ibc.WalletAmount{
		Address: srcFaucetAccount,
		Denom:   srcChainCfg.Denom,
		Amount:  10_000_000_000_000,
	}

	dstFaucetWalletAmount := ibc.WalletAmount{
		Address: dstFaucetAccount,
		Denom:   dstChainCfg.Denom,
		Amount:  10_000_000_000_000,
	}

	// start chains from genesis, wait until they are producing blocks
	chainsGenesisWaitGroup := errgroup.Group{}
	chainsGenesisWaitGroup.Go(func() error {
		if err := srcChain.Start(testName, ctx, srcRelayerWalletAmount, srcFaucetWalletAmount); err != nil {
			return fmt.Errorf("failed to start source chain: %w", err)
		}
		return nil
	})
	chainsGenesisWaitGroup.Go(func() error {
		if err := dstChain.Start(testName, ctx, dstRelayerWalletAmount, dstFaucetWalletAmount); err != nil {
			return fmt.Errorf("failed to start dest chain: %w", err)
		}
		return nil
	})

	if err := chainsGenesisWaitGroup.Wait(); err != nil {
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

	if err := relayerImpl.AddChainConfiguration(ctx,
		srcChainCfg, srcAccountKeyName,
		srcRPCAddr, srcGRPCAddr,
	); err != nil {
		return errResponse(fmt.Errorf("failed to configure relayer for source chain: %w", err))
	}

	if err := relayerImpl.AddChainConfiguration(ctx,
		dstChainCfg, dstAccountKeyName,
		dstRPCAddr, dstGRPCAddr,
	); err != nil {
		return errResponse(fmt.Errorf("failed to configure relayer for dest chain: %w", err))
	}

	if err := relayerImpl.RestoreKey(ctx, srcChain.Config().ChainID, srcAccountKeyName, srcMnemonic); err != nil {
		return errResponse(fmt.Errorf("failed to restore key to source chain: %w", err))
	}
	if err := relayerImpl.RestoreKey(ctx, dstChain.Config().ChainID, dstAccountKeyName, dstMnemonic); err != nil {
		return errResponse(fmt.Errorf("failed to restore key to dest chain: %w", err))
	}

	if err := relayerImpl.GeneratePath(ctx, srcChainCfg.ChainID, dstChainCfg.ChainID, testPathName); err != nil {
		return errResponse(fmt.Errorf("failed to generate path: %w", err))
	}

	if err := relayerImpl.LinkPath(ctx, testPathName); err != nil {
		return errResponse(fmt.Errorf("failed to create link in relayer: %w", err))
	}

	channels, err := relayerImpl.GetChannels(ctx, srcChainCfg.ChainID)
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

	if err := relayerImpl.StartRelayer(ctx, testPathName); err != nil {
		return errResponse(fmt.Errorf("failed to start relayer: %w", err))
	}
	t.Cleanup(func() {
		if err := relayerImpl.StopRelayer(ctx); err != nil {
			t.Logf("error stopping relayer: %v", err)
		}
	})

	// wait for relayer to start up
	time.Sleep(5 * time.Second)

	return relayerImpl, channels, nil
}

func WaitForBlocks(blocksToWait int64, chains ...ibc.Chain) error {
	chainsConsecutiveBlocksWaitGroup := errgroup.Group{}
	for _, chain := range chains {
		chain := chain
		chainsConsecutiveBlocksWaitGroup.Go(func() error {
			_, err := chain.WaitForBlocks(blocksToWait)
			return err
		})
	}
	return chainsConsecutiveBlocksWaitGroup.Wait()
}

func CreateTestNetwork(pool *dockertest.Pool, name string, testName string) (*docker.Network, error) {
	return pool.Client.CreateNetwork(docker.CreateNetworkOptions{
		Name:           name,
		Options:        map[string]interface{}{},
		Labels:         map[string]string{"ibc-test": testName},
		CheckDuplicate: true,
		Internal:       false,
		EnableIPv6:     false,
		Context:        context.Background(),
	})
}

// dockerCleanup will clean up Docker containers, networks, and the other various config files generated in testing
func dockerCleanup(testName string, pool *dockertest.Pool) func() {
	logger := log.New(os.Stderr, "console", "info") // dockerCleanup never called from cmd/ibctest main()
	return func() {
		showContainerLogs := os.Getenv("SHOW_CONTAINER_LOGS")
		cont, _ := pool.Client.ListContainers(docker.ListContainersOptions{All: true})
		for _, c := range cont {
			for k, v := range c.Labels {
				if k == "ibc-test" && v == testName {
					names := strings.Join(c.Names, ",")
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
						logger.
							With("containers", names).
							With("stdout", stdout).
							With("stderr", stderr).
							Info("docker cleanup")
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
