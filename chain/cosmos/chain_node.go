package cosmos

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/strangelove-ventures/ibc-test-framework/log"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/simapp"
	"github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	transfertypes "github.com/cosmos/ibc-go/v3/modules/apps/transfer/types"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"github.com/strangelove-ventures/ibc-test-framework/dockerutil"
	"github.com/strangelove-ventures/ibc-test-framework/ibc"
	tmconfig "github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/p2p"
	rpcclient "github.com/tendermint/tendermint/rpc/client"
	rpchttp "github.com/tendermint/tendermint/rpc/client/http"
	libclient "github.com/tendermint/tendermint/rpc/jsonrpc/client"
	"golang.org/x/sync/errgroup"
)

// ChainNode represents a node in the test network that is being created
type ChainNode struct {
	Home         string
	Index        int
	Chain        ibc.Chain
	GenesisCoins string
	Validator    bool
	NetworkID    string
	Pool         *dockertest.Pool
	Client       rpcclient.Client
	Container    *docker.Container
	TestName     string
	Image        ibc.ChainDockerImage

	lock sync.Mutex
	log  log.Logger
}

// ChainNodes is a collection of ChainNode
type ChainNodes []*ChainNode

type ContainerPort struct {
	Name      string
	Container *docker.Container
	Port      docker.Port
}

type Hosts []ContainerPort

const (
	valKey      = "validator"
	blockTime   = 2 // seconds
	p2pPort     = "26656/tcp"
	rpcPort     = "26657/tcp"
	grpcPort    = "9090/tcp"
	apiPort     = "1317/tcp"
	privValPort = "1234/tcp"
)

var (
	sentryPorts = map[docker.Port]struct{}{
		docker.Port(p2pPort):     {},
		docker.Port(rpcPort):     {},
		docker.Port(grpcPort):    {},
		docker.Port(apiPort):     {},
		docker.Port(privValPort): {},
	}
)

// NewClient creates and assigns a new Tendermint RPC client to the ChainNode
func (tn *ChainNode) NewClient(addr string) error {
	httpClient, err := libclient.DefaultHTTPClient(addr)
	if err != nil {
		return err
	}

	httpClient.Timeout = 10 * time.Second
	rpcClient, err := rpchttp.NewWithClient(addr, "/websocket", httpClient)
	if err != nil {
		return err
	}

	tn.Client = rpcClient
	return nil
}

// CliContext creates a new Cosmos SDK client context
func (tn *ChainNode) CliContext() client.Context {
	encoding := simapp.MakeTestEncodingConfig()
	transfertypes.RegisterInterfaces(encoding.InterfaceRegistry)
	return client.Context{
		Client:            tn.Client,
		ChainID:           tn.Chain.Config().ChainID,
		InterfaceRegistry: encoding.InterfaceRegistry,
		Input:             os.Stdin,
		Output:            os.Stdout,
		OutputFormat:      "json",
		LegacyAmino:       encoding.Amino,
		TxConfig:          encoding.TxConfig,
	}
}

// Name of the test node container
func (tn *ChainNode) Name() string {
	return fmt.Sprintf("node-%d-%s-%s", tn.Index, tn.Chain.Config().ChainID, dockerutil.SanitizeContainerName(tn.TestName))
}

// hostname of the test node container
func (tn *ChainNode) HostName() string {
	return dockerutil.CondenseHostName(tn.Name())
}

// Dir is the directory where the test node files are stored
func (tn *ChainNode) Dir() string {
	return filepath.Join(tn.Home, tn.Name())
}

// MkDir creates the directory for the testnode
func (tn *ChainNode) MkDir() {
	if err := os.MkdirAll(tn.Dir(), 0755); err != nil {
		panic(err)
	}
}

// GentxPath returns the path to the gentx for a node
func (tn *ChainNode) GentxPath() (string, error) {
	id, err := tn.NodeID()
	return filepath.Join(tn.Dir(), "config", "gentx", fmt.Sprintf("gentx-%s.json", id)), err
}

func (tn *ChainNode) GenesisFilePath() string {
	return filepath.Join(tn.Dir(), "config", "genesis.json")
}

type PrivValidatorKey struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

type PrivValidatorKeyFile struct {
	Address string           `json:"address"`
	PubKey  PrivValidatorKey `json:"pub_key"`
	PrivKey PrivValidatorKey `json:"priv_key"`
}

func (tn *ChainNode) PrivValKeyFilePath() string {
	return filepath.Join(tn.Dir(), "config", "priv_validator_key.json")
}

func (tn *ChainNode) TMConfigPath() string {
	return filepath.Join(tn.Dir(), "config", "config.toml")
}

// Bind returns the home folder bind point for running the node
func (tn *ChainNode) Bind() []string {
	return []string{fmt.Sprintf("%s:%s", tn.Dir(), tn.NodeHome())}
}

func (tn *ChainNode) NodeHome() string {
	return filepath.Join("/tmp", tn.Chain.Config().Name)
}

// Keybase returns the keyring for a given node
func (tn *ChainNode) Keybase() keyring.Keyring {
	kr, err := keyring.New("", keyring.BackendTest, tn.Dir(), os.Stdin)
	if err != nil {
		panic(err)
	}
	return kr
}

// SetValidatorConfigAndPeers modifies the config for a validator node to start a chain
func (tn *ChainNode) SetValidatorConfigAndPeers(peers string) {
	// Pull default config
	cfg := tmconfig.DefaultConfig()

	// change config to include everything needed
	applyConfigChanges(cfg, peers)

	// overwrite with the new config
	tmconfig.WriteConfigFile(tn.TMConfigPath(), cfg)
}

// Wait until we have signed n blocks in a row
func (tn *ChainNode) WaitForBlocks(blocks int64) (int64, error) {
	stat, err := tn.Client.Status(context.Background())
	if err != nil {
		return -1, err
	}

	startingBlock := stat.SyncInfo.LatestBlockHeight
	mostRecentBlock := startingBlock
	tn.logger().
		WithField("initialHeight", startingBlock).
		Info("wait for blocks")
	// timeout after ~1 minute plus block time
	timeoutSeconds := blocks*int64(blockTime) + int64(60)
	for i := int64(0); i < timeoutSeconds; i++ {
		time.Sleep(1 * time.Second)

		stat, err := tn.Client.Status(context.Background())
		if err != nil {
			return mostRecentBlock, err
		}

		mostRecentBlock = stat.SyncInfo.LatestBlockHeight

		deltaBlocks := mostRecentBlock - startingBlock

		if deltaBlocks >= blocks {
			tn.logger().
				WithField("initialHeight", startingBlock).
				Infof("Time (sec) waiting for %d blocks: %d", blocks, i+1)
			return mostRecentBlock, nil // done waiting for consecutive signed blocks
		}
	}
	return mostRecentBlock, errors.New("timed out waiting for blocks")
}

func (tn *ChainNode) Height() (int64, error) {
	stat, err := tn.Client.Status(context.Background())
	if err != nil {
		return -1, err
	}
	return stat.SyncInfo.LatestBlockHeight, nil
}

func applyConfigChanges(cfg *tmconfig.Config, peers string) {
	// turn down blocktimes to make the chain faster
	cfg.Consensus.TimeoutCommit = time.Duration(blockTime) * time.Second
	cfg.Consensus.TimeoutPropose = time.Duration(blockTime) * time.Second

	// Open up rpc address
	cfg.RPC.ListenAddress = "tcp://0.0.0.0:26657"

	// Allow for some p2p weirdness
	cfg.P2P.AllowDuplicateIP = true
	cfg.P2P.AddrBookStrict = false

	// Set log level to info
	cfg.BaseConfig.LogLevel = "info"

	// set persistent peer nodes
	cfg.P2P.PersistentPeers = peers
}

// CondenseMoniker fits a moniker into the cosmos character limit for monikers.
// If the moniker already fits, it is returned unmodified.
// Otherwise, the middle is truncated, and a hash is appended to the end
// in case the only unique data was in the middle.
func CondenseMoniker(m string) string {
	if len(m) <= stakingtypes.MaxMonikerLength {
		return m
	}

	// Get the hash suffix, a 32-bit uint formatted in base36.
	// fnv32 was chosen because a 32-bit number ought to be sufficient
	// as a distinguishing suffix, and it will be short enough so that
	// less of the middle will be truncated to fit in the character limit.
	// It's also non-cryptographic, not that this function will ever be a bottleneck in tests.
	h := fnv.New32()
	h.Write([]byte(m))
	suffix := "-" + strconv.FormatUint(uint64(h.Sum32()), 36)

	wantLen := stakingtypes.MaxMonikerLength - len(suffix)

	// Half of the want length, minus 2 to account for half of the ... we add in the middle.
	keepLen := (wantLen / 2) - 2

	return m[:keepLen] + "..." + m[len(m)-keepLen:] + suffix
}

// InitHomeFolder initializes a home folder for the given node
func (tn *ChainNode) InitHomeFolder(ctx context.Context) error {
	command := []string{tn.Chain.Config().Bin, "init", CondenseMoniker(tn.Name()),
		"--chain-id", tn.Chain.Config().ChainID,
		"--home", tn.NodeHome(),
	}
	return dockerutil.HandleNodeJobError(tn.NodeJob(ctx, command))
}

// CreateKey creates a key in the keyring backend test for the given node
func (tn *ChainNode) CreateKey(ctx context.Context, name string) error {
	command := []string{tn.Chain.Config().Bin, "keys", "add", name,
		"--keyring-backend", keyring.BackendTest,
		"--output", "json",
		"--home", tn.NodeHome(),
	}
	tn.lock.Lock()
	defer tn.lock.Unlock()
	return dockerutil.HandleNodeJobError(tn.NodeJob(ctx, command))
}

// AddGenesisAccount adds a genesis account for each key
func (tn *ChainNode) AddGenesisAccount(ctx context.Context, address string, genesisAmount []types.Coin) error {
	amount := ""
	for i, coin := range genesisAmount {
		if i != 0 {
			amount += ","
		}
		amount += fmt.Sprintf("%d%s", coin.Amount.Int64(), coin.Denom)
	}
	command := []string{tn.Chain.Config().Bin, "add-genesis-account", address, amount,
		"--home", tn.NodeHome(),
	}
	tn.lock.Lock()
	defer tn.lock.Unlock()
	return dockerutil.HandleNodeJobError(tn.NodeJob(ctx, command))
}

// Gentx generates the gentx for a given node
func (tn *ChainNode) Gentx(ctx context.Context, name string, genesisSelfDelegation types.Coin) error {
	command := []string{tn.Chain.Config().Bin, "gentx", valKey, fmt.Sprintf("%d%s", genesisSelfDelegation.Amount.Int64(), genesisSelfDelegation.Denom),
		"--keyring-backend", keyring.BackendTest,
		"--home", tn.NodeHome(),
		"--chain-id", tn.Chain.Config().ChainID,
	}
	tn.lock.Lock()
	defer tn.lock.Unlock()
	return dockerutil.HandleNodeJobError(tn.NodeJob(ctx, command))
}

// CollectGentxs runs collect gentxs on the node's home folders
func (tn *ChainNode) CollectGentxs(ctx context.Context) error {
	command := []string{tn.Chain.Config().Bin, "collect-gentxs",
		"--home", tn.NodeHome(),
	}
	tn.lock.Lock()
	defer tn.lock.Unlock()
	return dockerutil.HandleNodeJobError(tn.NodeJob(ctx, command))
}

type IBCTransferTx struct {
	TxHash string `json:"txhash"`
}

func (tn *ChainNode) SendIBCTransfer(ctx context.Context, channelID string, keyName string, amount ibc.WalletAmount, timeout *ibc.IBCTimeout) (string, error) {
	command := []string{tn.Chain.Config().Bin, "tx", "ibc-transfer", "transfer", "transfer", channelID,
		amount.Address, fmt.Sprintf("%d%s", amount.Amount, amount.Denom),
		"--keyring-backend", keyring.BackendTest,
		"--gas-prices", tn.Chain.Config().GasPrices,
		"--gas-adjustment", fmt.Sprint(tn.Chain.Config().GasAdjustment),
		"--node", fmt.Sprintf("tcp://%s:26657", tn.HostName()),
		"--from", keyName,
		"--output", "json",
		"-y",
		"--home", tn.NodeHome(),
		"--chain-id", tn.Chain.Config().ChainID,
	}
	if timeout != nil {
		if timeout.NanoSeconds > 0 {
			command = append(command, "--packet-timeout-timestamp", fmt.Sprint(timeout.NanoSeconds))
		} else if timeout.Height > 0 {
			command = append(command, "--packet-timeout-height", fmt.Sprintf("0-%d", timeout.Height))
		}
	}
	tn.lock.Lock()
	defer tn.lock.Unlock()
	exitCode, stdout, stderr, err := tn.NodeJob(ctx, command)
	if err != nil {
		return "", dockerutil.HandleNodeJobError(exitCode, stdout, stderr, err)
	}
	if _, err := tn.WaitForBlocks(2); err != nil {
		return "", err
	}
	output := IBCTransferTx{}
	err = json.Unmarshal([]byte(stdout), &output)
	if err != nil {
		return "", err
	}
	return output.TxHash, nil
}

func (tn *ChainNode) SendFunds(ctx context.Context, keyName string, amount ibc.WalletAmount) error {
	command := []string{tn.Chain.Config().Bin, "tx", "bank", "send", keyName,
		amount.Address, fmt.Sprintf("%d%s", amount.Amount, amount.Denom),
		"--keyring-backend", keyring.BackendTest,
		"--node", fmt.Sprintf("tcp://%s:26657", tn.HostName()),
		"--output", "json",
		"-y",
		"--home", tn.NodeHome(),
		"--chain-id", tn.Chain.Config().ChainID,
	}
	return tn.NodeJobThenWaitForBlocksLocked(ctx, command)
}

func (tn *ChainNode) NodeJobThenWaitForBlocksLocked(ctx context.Context, command []string) error {
	tn.lock.Lock()
	defer tn.lock.Unlock()
	exitCode, stdout, stderr, err := tn.NodeJob(ctx, command)
	if err != nil {
		return dockerutil.HandleNodeJobError(exitCode, stdout, stderr, err)
	}
	_, err = tn.WaitForBlocks(2)
	return err
}

type InstantiateContractAttribute struct {
	Value string `json:"value"`
}

type InstantiateContractEvent struct {
	Attributes []InstantiateContractAttribute `json:"attributes"`
}

type InstantiateContractLog struct {
	Events []InstantiateContractEvent `json:"event"`
}

type InstantiateContractResponse struct {
	Logs []InstantiateContractLog `json:"log"`
}

type QueryContractResponse struct {
	Contracts []string `json:"contracts"`
}

type CodeInfo struct {
	CodeID string `json:"code_id"`
}
type CodeInfosResponse struct {
	CodeInfos []CodeInfo `json:"code_infos"`
}

func (tn *ChainNode) InstantiateContract(ctx context.Context, keyName string, amount ibc.WalletAmount, fileName, initMessage string, needsNoAdminFlag bool) (string, error) {
	_, file := filepath.Split(fileName)
	newFilePath := filepath.Join(tn.Dir(), file)
	newFilePathContainer := filepath.Join(tn.NodeHome(), file)
	if _, err := dockerutil.CopyFile(fileName, newFilePath); err != nil {
		return "", err
	}

	command := []string{tn.Chain.Config().Bin, "tx", "wasm", "store", newFilePathContainer,
		"--from", keyName,
		"--gas-prices", tn.Chain.Config().GasPrices,
		"--gas-adjustment", fmt.Sprint(tn.Chain.Config().GasAdjustment),
		"--keyring-backend", keyring.BackendTest,
		"--node", fmt.Sprintf("tcp://%s:26657", tn.HostName()),
		"--output", "json",
		"-y",
		"--home", tn.NodeHome(),
		"--chain-id", tn.Chain.Config().ChainID,
	}
	tn.lock.Lock()
	defer tn.lock.Unlock()
	exitCode, stdout, stderr, err := tn.NodeJob(ctx, command)
	if err != nil {
		return "", dockerutil.HandleNodeJobError(exitCode, stdout, stderr, err)
	}

	if _, err := tn.Chain.WaitForBlocks(5); err != nil {
		return "", err
	}

	command = []string{tn.Chain.Config().Bin,
		"query", "wasm", "list-code", "--reverse",
		"--node", fmt.Sprintf("tcp://%s:26657", tn.HostName()),
		"--output", "json",
		"--home", tn.NodeHome(),
		"--chain-id", tn.Chain.Config().ChainID,
	}

	exitCode, stdout, stderr, err = tn.NodeJob(ctx, command)
	if err != nil {
		return "", dockerutil.HandleNodeJobError(exitCode, stdout, stderr, err)
	}

	res := CodeInfosResponse{}
	if err := json.Unmarshal([]byte(stdout), &res); err != nil {
		return "", err
	}

	codeID := res.CodeInfos[0].CodeID

	command = []string{tn.Chain.Config().Bin,
		"tx", "wasm", "instantiate", codeID, initMessage,
		"--gas-prices", tn.Chain.Config().GasPrices,
		"--gas-adjustment", fmt.Sprint(tn.Chain.Config().GasAdjustment),
		"--label", "satoshi-test",
		"--from", keyName,
		"--keyring-backend", keyring.BackendTest,
		"--node", fmt.Sprintf("tcp://%s:26657", tn.HostName()),
		"--output", "json",
		"-y",
		"--home", tn.NodeHome(),
		"--chain-id", tn.Chain.Config().ChainID,
	}

	if needsNoAdminFlag {
		command = append(command, "--no-admin")
	}

	exitCode, stdout, stderr, err = tn.NodeJob(ctx, command)
	if err != nil {
		return "", dockerutil.HandleNodeJobError(exitCode, stdout, stderr, err)
	}

	if _, err := tn.Chain.WaitForBlocks(5); err != nil {
		return "", err
	}

	command = []string{tn.Chain.Config().Bin,
		"query", "wasm", "list-contract-by-code", codeID,
		"--node", fmt.Sprintf("tcp://%s:26657", tn.HostName()),
		"--output", "json",
		"--home", tn.NodeHome(),
		"--chain-id", tn.Chain.Config().ChainID,
	}

	exitCode, stdout, stderr, err = tn.NodeJob(ctx, command)
	if err != nil {
		return "", dockerutil.HandleNodeJobError(exitCode, stdout, stderr, err)
	}

	contactsRes := QueryContractResponse{}
	if err := json.Unmarshal([]byte(stdout), &contactsRes); err != nil {
		return "", err
	}

	contractAddress := contactsRes.Contracts[len(contactsRes.Contracts)-1]
	return contractAddress, nil
}

func (tn *ChainNode) ExecuteContract(ctx context.Context, keyName string, contractAddress string, message string) error {
	command := []string{tn.Chain.Config().Bin,
		"tx", "wasm", "execute", contractAddress, message,
		"--from", keyName,
		"--gas-prices", tn.Chain.Config().GasPrices,
		"--gas-adjustment", fmt.Sprint(tn.Chain.Config().GasAdjustment),
		"--keyring-backend", keyring.BackendTest,
		"--node", fmt.Sprintf("tcp://%s:26657", tn.HostName()),
		"--output", "json",
		"-y",
		"--home", tn.NodeHome(),
		"--chain-id", tn.Chain.Config().ChainID,
	}
	return tn.NodeJobThenWaitForBlocksLocked(ctx, command)
}

func (tn *ChainNode) DumpContractState(ctx context.Context, contractAddress string, height int64) (*ibc.DumpContractStateResponse, error) {
	command := []string{tn.Chain.Config().Bin,
		"query", "wasm", "contract-state", "all", contractAddress,
		"--height", fmt.Sprint(height),
		"--node", fmt.Sprintf("tcp://%s:26657", tn.HostName()),
		"--output", "json",
		"--home", tn.NodeHome(),
		"--chain-id", tn.Chain.Config().ChainID,
	}
	exitCode, stdout, stderr, err := tn.NodeJob(ctx, command)
	if err != nil {
		return nil, dockerutil.HandleNodeJobError(exitCode, stdout, stderr, err)
	}

	res := &ibc.DumpContractStateResponse{}
	if err := json.Unmarshal([]byte(stdout), res); err != nil {
		return nil, err
	}
	return res, nil
}

func (tn *ChainNode) ExportState(ctx context.Context, height int64) (string, error) {
	command := []string{tn.Chain.Config().Bin,
		"export",
		"--height", fmt.Sprint(height),
		"--home", tn.NodeHome(),
	}
	exitCode, stdout, stderr, err := tn.NodeJob(ctx, command)
	if err != nil {
		return "", dockerutil.HandleNodeJobError(exitCode, stdout, stderr, err)
	}
	// output comes to stderr for some reason
	return stderr, nil
}

func (tn *ChainNode) UnsafeResetAll(ctx context.Context) error {
	command := []string{tn.Chain.Config().Bin,
		"unsafe-reset-all",
		"--home", tn.NodeHome(),
	}

	return dockerutil.HandleNodeJobError(tn.NodeJob(ctx, command))
}

func (tn *ChainNode) CreatePool(ctx context.Context, keyName string, contractAddress string, swapFee float64, exitFee float64, assets []ibc.WalletAmount) error {
	// TODO generate --pool-file
	poolFilePath := "TODO"
	command := []string{tn.Chain.Config().Bin,
		"tx", "gamm", "create-pool",
		"--pool-file", poolFilePath,
		"--gas-prices", tn.Chain.Config().GasPrices,
		"--gas-adjustment", fmt.Sprint(tn.Chain.Config().GasAdjustment),
		"--from", keyName,
		"--keyring-backend", keyring.BackendTest,
		"--node", fmt.Sprintf("tcp://%s:26657", tn.HostName()),
		"--output", "json",
		"-y",
		"--home", tn.NodeHome(),
		"--chain-id", tn.Chain.Config().ChainID,
	}
	return tn.NodeJobThenWaitForBlocksLocked(ctx, command)
}

func (tn *ChainNode) CreateNodeContainer() error {
	chainCfg := tn.Chain.Config()
	cmd := []string{chainCfg.Bin, "start", "--home", tn.NodeHome(), "--x-crisis-skip-assert-invariants"}
	tn.logger().
		WithField("container", tn.Name()).
		WithField("command", strings.Join(cmd, " ")).
		Info()

	cont, err := tn.Pool.Client.CreateContainer(docker.CreateContainerOptions{
		Name: tn.Name(),
		Config: &docker.Config{
			User:         dockerutil.GetDockerUserString(),
			Cmd:          cmd,
			Hostname:     tn.HostName(),
			ExposedPorts: sentryPorts,
			DNS:          []string{},
			Image:        fmt.Sprintf("%s:%s", tn.Image.Repository, tn.Image.Version),
			Labels:       map[string]string{"ibc-test": tn.TestName},
		},
		HostConfig: &docker.HostConfig{
			Binds:           tn.Bind(),
			PublishAllPorts: true,
			AutoRemove:      false,
		},
		NetworkingConfig: &docker.NetworkingConfig{
			EndpointsConfig: map[string]*docker.EndpointConfig{
				tn.NetworkID: {},
			},
		},
		Context: nil,
	})
	if err != nil {
		return err
	}
	tn.Container = cont
	return nil
}

func (tn *ChainNode) StopContainer() error {
	return tn.Pool.Client.StopContainer(tn.Container.ID, 30)
}

func (tn *ChainNode) StartContainer(ctx context.Context) error {
	if err := tn.Pool.Client.StartContainer(tn.Container.ID, nil); err != nil {
		return err
	}

	c, err := tn.Pool.Client.InspectContainer(tn.Container.ID)
	if err != nil {
		return err
	}
	tn.Container = c

	port := dockerutil.GetHostPort(c, rpcPort)
	tn.logger().WithField("container", tn.Name()).Infof("RPC => %s", port)

	err = tn.NewClient(fmt.Sprintf("tcp://%s", port))
	if err != nil {
		return err
	}

	time.Sleep(5 * time.Second)
	return retry.Do(func() error {
		stat, err := tn.Client.Status(ctx)
		if err != nil {
			// tn.t.Log(err)
			return err
		}
		// TODO: reenable this check, having trouble with it for some reason
		if stat != nil && stat.SyncInfo.CatchingUp {
			return fmt.Errorf("still catching up: height(%d) catching-up(%t)",
				stat.SyncInfo.LatestBlockHeight, stat.SyncInfo.CatchingUp)
		}
		return nil
	}, retry.Context(ctx), retry.DelayType(retry.BackOffDelay))
}

// InitValidatorFiles creates the node files and signs a genesis transaction
func (tn *ChainNode) InitValidatorFiles(
	ctx context.Context,
	chainType *ibc.ChainConfig,
	genesisAmounts []types.Coin,
	genesisSelfDelegation types.Coin,
) error {
	if err := tn.InitHomeFolder(ctx); err != nil {
		return err
	}
	if err := tn.CreateKey(ctx, valKey); err != nil {
		return err
	}
	key, err := tn.GetKey(valKey)
	if err != nil {
		return err
	}
	bech32, err := types.Bech32ifyAddressBytes(chainType.Bech32Prefix, key.GetAddress().Bytes())
	if err != nil {
		return err
	}
	if err := tn.AddGenesisAccount(ctx, bech32, genesisAmounts); err != nil {
		return err
	}
	return tn.Gentx(ctx, valKey, genesisSelfDelegation)
}

func (tn *ChainNode) InitFullNodeFiles(ctx context.Context) error {
	return tn.InitHomeFolder(ctx)
}

// NodeID returns the node of a given node
func (tn *ChainNode) NodeID() (string, error) {
	nodeKey, err := p2p.LoadNodeKey(filepath.Join(tn.Dir(), "config", "node_key.json"))
	if err != nil {
		return "", err
	}
	return string(nodeKey.ID()), nil
}

// GetKey gets a key, waiting until it is available
func (tn *ChainNode) GetKey(name string) (info keyring.Info, err error) {
	return info, retry.Do(func() (err error) {
		info, err = tn.Keybase().Key(name)
		return err
	})
}

// PeerString returns the string for connecting the nodes passed in
func (nodes ChainNodes) PeerString() string {
	addrs := make([]string, len(nodes))
	for i, n := range nodes {
		id, err := n.NodeID()
		if err != nil {
			// TODO: would this be better to panic?
			// When would NodeId return an error?
			break
		}
		hostName := n.HostName()
		ps := fmt.Sprintf("%s@%s:26656", id, hostName)
		nodes.logger().WithField("container", n.Name()).Infof("(%s) peering (%s)", hostName, ps)
		addrs[i] = ps
	}
	return strings.Join(addrs, ",")
}

// LogGenesisHashes logs the genesis hashes for the various nodes
func (nodes ChainNodes) LogGenesisHashes() error {
	for _, n := range nodes {
		gen, err := os.ReadFile(filepath.Join(n.Dir(), "config", "genesis.json"))
		if err != nil {
			return err
		}
		nodes.logger().WithField("container", n.Name()).Infof("genesis hash %x", sha256.Sum256(gen))
	}
	return nil
}

func (nodes ChainNodes) WaitForHeight(height int64) error {
	var eg errgroup.Group
	nodes.logger().Infof("Waiting For Nodes To Reach Block Height %d...", height)
	for _, n := range nodes {
		n := n
		eg.Go(func() error {
			return retry.Do(func() error {
				stat, err := n.Client.Status(context.Background())
				if err != nil {
					return err
				}

				if stat.SyncInfo.CatchingUp || stat.SyncInfo.LatestBlockHeight < height {
					return fmt.Errorf("node still under block %d: %d", height, stat.SyncInfo.LatestBlockHeight)
				}
				nodes.logger().WithField("container", n.Name()).Infof("reached block %d", height)
				return nil
				// TODO: setup backup delay here
			}, retry.DelayType(retry.BackOffDelay), retry.Attempts(15))
		})
	}
	return eg.Wait()
}

func (nodes ChainNodes) logger() log.Logger {
	if len(nodes) == 0 {
		return log.Nop()
	}
	return nodes[0].logger()
}

// NodeJob run a container for a specific job and block until the container exits
// NOTE: on job containers generate random name
func (tn *ChainNode) NodeJob(ctx context.Context, cmd []string) (int, string, string, error) {
	counter, _, _, _ := runtime.Caller(1)
	caller := runtime.FuncForPC(counter).Name()
	funcName := strings.Split(caller, ".")
	container := fmt.Sprintf("%s-%s-%s", tn.Name(), funcName[len(funcName)-1], dockerutil.RandLowerCaseLetterString(3))
	tn.logger().
		WithField("container", container).
		WithField("command", strings.Join(cmd, " ")).
		Info()
	cont, err := tn.Pool.Client.CreateContainer(docker.CreateContainerOptions{
		Name: container,
		Config: &docker.Config{
			User: dockerutil.GetDockerUserString(),
			// random hostname is fine here since this is just for setup
			Hostname:     dockerutil.CondenseHostName(container),
			ExposedPorts: sentryPorts,
			DNS:          []string{},
			Image:        fmt.Sprintf("%s:%s", tn.Image.Repository, tn.Image.Version),
			Cmd:          cmd,
			Labels:       map[string]string{"ibc-test": tn.TestName},
		},
		HostConfig: &docker.HostConfig{
			Binds:           tn.Bind(),
			PublishAllPorts: true,
			AutoRemove:      false,
		},
		NetworkingConfig: &docker.NetworkingConfig{
			EndpointsConfig: map[string]*docker.EndpointConfig{
				tn.NetworkID: {},
			},
		},
		Context: nil,
	})
	if err != nil {
		return 1, "", "", err
	}
	if err := tn.Pool.Client.StartContainer(cont.ID, nil); err != nil {
		return 1, "", "", err
	}

	exitCode, err := tn.Pool.Client.WaitContainerWithContext(cont.ID, ctx)
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	_ = tn.Pool.Client.Logs(docker.LogsOptions{Context: ctx, Container: cont.ID, OutputStream: stdout, ErrorStream: stderr, Stdout: true, Stderr: true, Tail: "50", Follow: false, Timestamps: false})
	_ = tn.Pool.Client.RemoveContainer(docker.RemoveContainerOptions{ID: cont.ID})
	tn.logger().
		WithField("container", container).
		WithField("stdout", stdout.String()).
		WithField("stderr", stderr.String()).
		Info()
	return exitCode, stdout.String(), stderr.String(), err
}

func (tn *ChainNode) logger() log.Logger {
	return tn.log.
		WithField("chainID", tn.Chain.Config().ChainID).
		WithField("test", tn.TestName)
}
