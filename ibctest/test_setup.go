package ibctest

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/cosmos/cosmos-sdk/types"
	"github.com/ory/dockertest"
	"github.com/ory/dockertest/docker"
	"github.com/strangelove-ventures/ibc-test-framework/ibc"
	"github.com/strangelove-ventures/ibc-test-framework/utils"
	"golang.org/x/sync/errgroup"
)

const (
	srcAccountKeyName  = "src-chain"
	dstAccountKeyName  = "dst-chain"
	userAccountKeyName = "user"
	testPathName       = "test-path"
)

// all methods on this struct have the same signature and are method names that will be called by the CLI:
//     func (ibc IBCTestCase) TestCaseName(testName string, cf ChainFactory, relayerImplementation RelayerImplementation) error
type IBCTestCase struct{}

// uses reflection to get test case
func GetTestCase(testCase string) (func(testName string, cf ChainFactory, relayerImplementation ibc.RelayerImplementation) error, error) {
	v := reflect.ValueOf(IBCTestCase{})
	m := v.MethodByName(testCase)

	if m.Kind() != reflect.Func {
		return nil, fmt.Errorf("invalid test case: %s", testCase)
	}

	testCaseFunc := func(testName string, cf ChainFactory, relayerImplementation ibc.RelayerImplementation) error {
		args := []reflect.Value{reflect.ValueOf(testName), reflect.ValueOf(cf), reflect.ValueOf(relayerImplementation)}
		result := m.Call(args)
		if len(result) != 1 || !result[0].CanInterface() {
			return errors.New("error reflecting error return var")
		}

		err, _ := result[0].Interface().(error)
		return err
	}

	return testCaseFunc, nil
}

func SetupTestRun(t *testing.T) (context.Context, string, *dockertest.Pool, string, error) {
	ctx := context.Background()

	pool, err := dockertest.NewPool("")
	if err != nil {
		return ctx, "", nil, "", err
	}
	t.Cleanup(dockerCleanup(t.Name(), pool))

	home := t.TempDir()

	networkName := fmt.Sprintf("ibc-test-framework-%s", utils.RandLowerCaseLetterString(8))
	network, err := CreateTestNetwork(pool, networkName, t.Name())
	if err != nil {
		return ctx, "", nil, "", err
	}

	return ctx, home, pool, network.ID, nil
}

type User struct {
	SrcChainAddress string
	DstChainAddress string
	KeyName         string
}

func StartChainsAndRelayer(
	t *testing.T,
	ctx context.Context,
	pool *dockertest.Pool,
	networkID string,
	home string,
	srcChain, dstChain ibc.Chain,
	relayerImplementation ibc.RelayerImplementation,
	preRelayerStart func([]ibc.ChannelOutput, User, User) error,
) (ibc.Relayer, []ibc.ChannelOutput, *User, *User, error) {
	return StartChainsAndRelayerFromFactory(
		t,
		ctx,
		pool,
		networkID,
		home,
		srcChain,
		dstChain,
		builtinRelayerFactory{impl: relayerImplementation},
		preRelayerStart,
	)
}

// startup both chains and relayer
// creates wallets in the relayer for src and dst chain
// funds relayer src and dst wallets on respective chain in genesis
// creates a user account on the src chain (separate fullnode)
// funds user account on src chain in genesis
func StartChainsAndRelayerFromFactory(
	t *testing.T,
	ctx context.Context,
	pool *dockertest.Pool,
	networkID string,
	home string,
	srcChain, dstChain ibc.Chain,
	f RelayerFactory,
	preRelayerStart func([]ibc.ChannelOutput, User, User) error,
) (ibc.Relayer, []ibc.ChannelOutput, *User, *User, error) {
	relayerImpl := f.Build(t, pool, networkID, home, srcChain, dstChain)

	errResponse := func(err error) (ibc.Relayer, []ibc.ChannelOutput, *User, *User, error) {
		return nil, []ibc.ChannelOutput{}, nil, nil, err
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

	if err := relayerImpl.AddChainConfiguration(ctx, srcChainCfg, srcAccountKeyName,
		srcChain.GetRPCAddress(), srcChain.GetGRPCAddress()); err != nil {
		return errResponse(fmt.Errorf("failed to configure relayer for source chain: %w", err))
	}

	if err := relayerImpl.AddChainConfiguration(ctx, dstChainCfg, dstAccountKeyName,
		dstChain.GetRPCAddress(), dstChain.GetGRPCAddress()); err != nil {
		return errResponse(fmt.Errorf("failed to configure relayer for dest chain: %w", err))
	}

	srcRelayerWallet, err := relayerImpl.AddKey(ctx, srcChain.Config().ChainID, srcAccountKeyName)
	if err != nil {
		return errResponse(fmt.Errorf("failed to add key to source chain: %w", err))
	}
	dstRelayerWallet, err := relayerImpl.AddKey(ctx, dstChain.Config().ChainID, dstAccountKeyName)
	if err != nil {
		return errResponse(fmt.Errorf("failed to add key to dest chain: %w", err))
	}

	srcAccount := srcRelayerWallet.Address
	dstAccount := dstRelayerWallet.Address

	if err := relayerImpl.GeneratePath(ctx, srcChainCfg.ChainID, dstChainCfg.ChainID, testPathName); err != nil {
		return errResponse(fmt.Errorf("failed to generate path: %w", err))
	}

	// Fund relayer account on src chain
	srcRelayerWalletAmount := ibc.WalletAmount{
		Address: srcAccount,
		Denom:   srcChainCfg.Denom,
		Amount:  10000000,
	}

	// Fund relayer account on dst chain
	dstRelayerWalletAmount := ibc.WalletAmount{
		Address: dstAccount,
		Denom:   dstChainCfg.Denom,
		Amount:  10000000,
	}

	// Generate key to be used for "user" that will execute IBC transaction
	if err := srcChain.CreateKey(ctx, userAccountKeyName); err != nil {
		return errResponse(fmt.Errorf("failed to create key on source chain: %w", err))
	}

	srcUserAccountAddressBytes, err := srcChain.GetAddress(userAccountKeyName)
	if err != nil {
		return errResponse(fmt.Errorf("failed to get source user account address: %w", err))
	}

	srcUserAccountSrc, err := types.Bech32ifyAddressBytes(srcChainCfg.Bech32Prefix, srcUserAccountAddressBytes)
	if err != nil {
		return errResponse(err)
	}

	srcUserAccountDst, err := types.Bech32ifyAddressBytes(dstChainCfg.Bech32Prefix, srcUserAccountAddressBytes)
	if err != nil {
		return errResponse(err)
	}

	if err := dstChain.CreateKey(ctx, userAccountKeyName); err != nil {
		return errResponse(fmt.Errorf("failed to create key on dest chain: %w", err))
	}

	dstUserAccountAddressBytes, err := dstChain.GetAddress(userAccountKeyName)
	if err != nil {
		return errResponse(fmt.Errorf("failed to get dest user account address: %w", err))
	}

	dstUserAccountSrc, err := types.Bech32ifyAddressBytes(srcChainCfg.Bech32Prefix, dstUserAccountAddressBytes)
	if err != nil {
		return errResponse(err)
	}

	dstUserAccountDst, err := types.Bech32ifyAddressBytes(dstChainCfg.Bech32Prefix, dstUserAccountAddressBytes)
	if err != nil {
		return errResponse(err)
	}

	srcUser := User{
		KeyName:         userAccountKeyName,
		SrcChainAddress: srcUserAccountSrc,
		DstChainAddress: srcUserAccountDst,
	}

	dstUser := User{
		KeyName:         userAccountKeyName,
		SrcChainAddress: dstUserAccountSrc,
		DstChainAddress: dstUserAccountDst,
	}

	// Fund user account on src chain in order to relay from src to dst
	srcUserWalletAmount := ibc.WalletAmount{
		Address: srcUserAccountSrc,
		Denom:   srcChainCfg.Denom,
		Amount:  10000000000,
	}

	// Fund user account on dst chain in order to relay from dst to src
	dstUserWalletAmount := ibc.WalletAmount{
		Address: dstUserAccountDst,
		Denom:   dstChainCfg.Denom,
		Amount:  10000000000,
	}

	// start chains from genesis, wait until they are producing blocks
	chainsGenesisWaitGroup := errgroup.Group{}
	chainsGenesisWaitGroup.Go(func() error {
		if err := srcChain.Start(testName, ctx, []ibc.WalletAmount{srcRelayerWalletAmount, srcUserWalletAmount}); err != nil {
			return fmt.Errorf("failed to start source chain: %w", err)
		}
		return nil
	})
	chainsGenesisWaitGroup.Go(func() error {
		if err := dstChain.Start(testName, ctx, []ibc.WalletAmount{dstRelayerWalletAmount, dstUserWalletAmount}); err != nil {
			return fmt.Errorf("failed to start dest chain: %w", err)
		}
		return nil
	})

	if err := chainsGenesisWaitGroup.Wait(); err != nil {
		return errResponse(err)
	}

	if err := relayerImpl.LinkPath(ctx, testPathName); err != nil {
		return errResponse(err)
	}

	channels, err := relayerImpl.GetChannels(ctx, srcChainCfg.ChainID)
	if err != nil {
		return errResponse(err)
	}
	if len(channels) != 1 {
		return errResponse(fmt.Errorf("channel count invalid. expected: 1, actual: %d", len(channels)))
	}

	if preRelayerStart != nil {
		if err := preRelayerStart(channels, srcUser, dstUser); err != nil {
			return errResponse(err)
		}
	}

	if err := relayerImpl.StartRelayer(ctx, testPathName); err != nil {
		return errResponse(err)
	}
	t.Cleanup(func() {
		if err := relayerImpl.StopRelayer(ctx); err != nil {
			t.Logf("error stopping relayer: %v", err)
		}
	})

	// wait for relayer to start up
	time.Sleep(5 * time.Second)

	return relayerImpl, channels, &srcUser, &dstUser, nil
}

func WaitForBlocks(srcChain, dstChain ibc.Chain, blocksToWait int64) error {
	chainsConsecutiveBlocksWaitGroup := errgroup.Group{}
	chainsConsecutiveBlocksWaitGroup.Go(func() (err error) {
		_, err = srcChain.WaitForBlocks(blocksToWait)
		return
	})
	chainsConsecutiveBlocksWaitGroup.Go(func() (err error) {
		_, err = dstChain.WaitForBlocks(blocksToWait)
		return
	})
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
	return func() {
		showContainerLogs := os.Getenv("SHOW_CONTAINER_LOGS")
		cont, _ := pool.Client.ListContainers(docker.ListContainersOptions{All: true})
		ctx := context.Background()
		for _, c := range cont {
			for k, v := range c.Labels {
				if k == "ibc-test" && v == testName {
					_ = pool.Client.StopContainer(c.ID, 10)
					_, _ = pool.Client.WaitContainerWithContext(c.ID, ctx)
					stdout := new(bytes.Buffer)
					stderr := new(bytes.Buffer)
					_ = pool.Client.Logs(docker.LogsOptions{Context: ctx, Container: c.ID, OutputStream: stdout, ErrorStream: stderr, Stdout: true, Stderr: true, Tail: "50", Follow: false, Timestamps: false})
					names := strings.Join(c.Names, ",")
					if showContainerLogs != "" {
						fmt.Printf("{%s} - stdout:\n%s\n{%s} - stderr:\n%s\n", names, stdout, names, stderr)
					}
					_ = pool.Client.RemoveContainer(docker.RemoveContainerOptions{ID: c.ID})
				}
			}
		}
		nets, _ := pool.Client.ListNetworks()
		for _, n := range nets {
			for k, v := range n.Labels {
				if k == "ibc-test" && v == testName {
					_ = pool.Client.RemoveNetwork(n.ID)
				}
			}
		}
	}
}
