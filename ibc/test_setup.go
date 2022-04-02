package ibc

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"io/ioutil"
	"math/big"
	"net"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cosmos/cosmos-sdk/types"
	"github.com/ory/dockertest"
	"github.com/ory/dockertest/docker"
	"github.com/stretchr/testify/require"
)

type RelayerImplementation int64

const (
	CosmosRly RelayerImplementation = iota
	Hermes
)

const (
	srcAccountKeyName  = "src-chain"
	dstAccountKeyName  = "dst-chain"
	userAccountKeyName = "user"
	testPathName       = "test-path"
)

// RandLowerCaseLetterString returns a lowercase letter string of given length
func RandLowerCaseLetterString(length int) string {
	chars := []rune("abcdefghijklmnopqrstuvwxyz")
	var b strings.Builder
	for i := 0; i < length; i++ {
		i, _ := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		b.WriteRune(chars[i.Int64()])
	}
	return b.String()
}

func SetupTestRun(t *testing.T) (context.Context, string, *dockertest.Pool, *docker.Network) {
	home, err := ioutil.TempDir("", "")
	require.NoError(t, err)

	pool, err := dockertest.NewPool("")
	require.NoError(t, err)

	network, err := CreateTestNetwork(pool, fmt.Sprintf("ibc-test-framework-%s", RandLowerCaseLetterString(8)), t)
	require.NoError(t, err)

	t.Cleanup(Cleanup(t, pool, home))

	return context.Background(), home, pool, network
}

type User struct {
	SrcChainAddress string
	DstChainAddress string
	KeyName         string
}

// startup both chains and relayer
// creates wallets in the relayer for src and dst chain
// funds relayer src and dst wallets on respective chain in genesis
// creates a user account on the src chain (separate fullnode)
// funds user account on src chain in genesis
func StartChainsAndRelayer(
	t *testing.T,
	ctx context.Context,
	pool *dockertest.Pool,
	networkID string,
	home string,
	srcChain Chain,
	dstChain Chain,
	relayerImplementation RelayerImplementation,
	preRelayerStart func(channels []ChannelOutput, user User),
) ([]ChannelOutput, User) {
	var relayerImpl Relayer
	switch relayerImplementation {
	case CosmosRly:
		relayerImpl = NewCosmosRelayerFromChains(
			t,
			srcChain,
			dstChain,
			pool,
			networkID,
			home,
		)
	case Hermes:
		// not yet supported
	}

	srcChainCfg := srcChain.Config()
	dstChainCfg := dstChain.Config()

	err := relayerImpl.AddChainConfiguration(ctx, srcChainCfg, srcAccountKeyName,
		srcChain.GetRPCAddress(), srcChain.GetGRPCAddress())
	require.NoError(t, err)

	err = relayerImpl.AddChainConfiguration(ctx, dstChainCfg, dstAccountKeyName,
		dstChain.GetRPCAddress(), dstChain.GetGRPCAddress())
	require.NoError(t, err)

	srcRelayerWallet, err := relayerImpl.AddKey(ctx, srcChain.Config().ChainID, srcAccountKeyName)
	require.NoError(t, err)
	dstRelayerWallet, err := relayerImpl.AddKey(ctx, dstChain.Config().ChainID, dstAccountKeyName)
	require.NoError(t, err)

	srcAccount := srcRelayerWallet.Address
	dstAccount := dstRelayerWallet.Address

	err = relayerImpl.GeneratePath(ctx, srcChainCfg.ChainID, dstChainCfg.ChainID, testPathName)
	require.NoError(t, err)

	// Fund relayer account on src chain
	srcWallet := WalletAmount{
		Address: srcAccount,
		Denom:   srcChainCfg.Denom,
		Amount:  10000000,
	}

	// Fund relayer account on dst chain
	dstWallet := WalletAmount{
		Address: dstAccount,
		Denom:   dstChainCfg.Denom,
		Amount:  10000000,
	}

	// Generate key to be used for "user" that will execute IBC transaction
	err = srcChain.CreateKey(ctx, userAccountKeyName)
	require.NoError(t, err)
	userAccountAddressBytes, err := srcChain.GetAddress(userAccountKeyName)
	require.NoError(t, err)

	userAccountSrc, err := types.Bech32ifyAddressBytes(srcChainCfg.Bech32Prefix, userAccountAddressBytes)
	require.NoError(t, err)

	userAccountDst, err := types.Bech32ifyAddressBytes(dstChainCfg.Bech32Prefix, userAccountAddressBytes)
	require.NoError(t, err)

	user := User{
		KeyName:         userAccountKeyName,
		SrcChainAddress: userAccountSrc,
		DstChainAddress: userAccountDst,
	}

	// Fund user account on src chain in order to relay from src to dst
	userWalletSrc := WalletAmount{
		Address: userAccountSrc,
		Denom:   srcChainCfg.Denom,
		Amount:  100000000,
	}

	// start chains from genesis, wait until they are producing blocks
	chainsGenesisWaitGroup := sync.WaitGroup{}
	chainsGenesisWaitGroup.Add(2)
	go func() {
		srcChain.Start(t, ctx, []WalletAmount{srcWallet, userWalletSrc})
		chainsGenesisWaitGroup.Done()
	}()
	go func() {
		dstChain.Start(t, ctx, []WalletAmount{dstWallet})
		chainsGenesisWaitGroup.Done()
	}()
	chainsGenesisWaitGroup.Wait()

	require.NoError(t, relayerImpl.LinkPath(ctx, testPathName))

	channels, err := relayerImpl.GetChannels(ctx, srcChainCfg.ChainID)
	require.NoError(t, err)
	require.Equal(t, len(channels), 1)

	if preRelayerStart != nil {
		preRelayerStart(channels, user)
	}

	require.NoError(t, relayerImpl.StartRelayer(ctx, testPathName))

	t.Cleanup(func() { _ = relayerImpl.StopRelayer(ctx) })

	// wait for relayer to start up
	time.Sleep(5 * time.Second)

	return channels, user
}

func WaitForBlocks(srcChain Chain, dstChain Chain, blocksToWait int64) {
	chainsConsecutiveBlocksWaitGroup := sync.WaitGroup{}
	chainsConsecutiveBlocksWaitGroup.Add(2)
	go func() {
		srcChain.WaitForBlocks(blocksToWait)
		chainsConsecutiveBlocksWaitGroup.Done()
	}()
	go func() {
		dstChain.WaitForBlocks(blocksToWait)
		chainsConsecutiveBlocksWaitGroup.Done()
	}()
	chainsConsecutiveBlocksWaitGroup.Wait()
}

// GetHostPort returns a resource's published port with an address.
func GetHostPort(cont *docker.Container, portID string) string {
	if cont == nil || cont.NetworkSettings == nil {
		return ""
	}

	m, ok := cont.NetworkSettings.Ports[docker.Port(portID)]
	if !ok || len(m) == 0 {
		return ""
	}

	ip := m[0].HostIP
	if ip == "0.0.0.0" {
		ip = "localhost"
	}
	return net.JoinHostPort(ip, m[0].HostPort)
}

func CreateTestNetwork(pool *dockertest.Pool, name string, t *testing.T) (*docker.Network, error) {
	return pool.Client.CreateNetwork(docker.CreateNetworkOptions{
		Name:           name,
		Options:        map[string]interface{}{},
		Labels:         map[string]string{"ibc-test": t.Name()},
		CheckDuplicate: true,
		Internal:       false,
		EnableIPv6:     false,
		Context:        context.Background(),
	})
}

// Cleanup will clean up Docker containers, networks, and the other various config files generated in testing
func Cleanup(t *testing.T, pool *dockertest.Pool, testDir string) func() {
	return func() {
		testName := t.Name()
		// testFailed := t.Failed()
		cont, _ := pool.Client.ListContainers(docker.ListContainersOptions{All: true})
		ctx := context.Background()
		for _, c := range cont {
			for k, v := range c.Labels {
				if k == "ibc-test" && v == testName {
					_ = pool.Client.StopContainer(c.ID, 10)
					_, _ = pool.Client.WaitContainerWithContext(c.ID, ctx)
					// if err != nil || testFailed {
					stdout := new(bytes.Buffer)
					stderr := new(bytes.Buffer)
					_ = pool.Client.Logs(docker.LogsOptions{Context: ctx, Container: c.ID, OutputStream: stdout, ErrorStream: stderr, Stdout: true, Stderr: true, Tail: "50", Follow: false, Timestamps: false})
					names := strings.Join(c.Names, ",")
					fmt.Printf("{%s} - stdout:\n%s\n{%s} - stderr:\n%s\n", names, stdout, names, stderr)
					// }
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
		_ = os.RemoveAll(testDir)
	}
}
