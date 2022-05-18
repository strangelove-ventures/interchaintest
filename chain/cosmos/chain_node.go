package cosmos

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/simapp"
	"github.com/cosmos/cosmos-sdk/types"
	authTx "github.com/cosmos/cosmos-sdk/x/auth/tx"
	bankTypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	transfertypes "github.com/cosmos/ibc-go/v3/modules/apps/transfer/types"
	ibctypes "github.com/cosmos/ibc-go/v3/modules/core/types"
	"github.com/davecgh/go-spew/spew"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"github.com/strangelove-ventures/ibctest/dockerutil"
	"github.com/strangelove-ventures/ibctest/ibc"
	"github.com/strangelove-ventures/ibctest/test"
	tmconfig "github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/p2p"
	rpcclient "github.com/tendermint/tendermint/rpc/client"
	rpchttp "github.com/tendermint/tendermint/rpc/client/http"
	libclient "github.com/tendermint/tendermint/rpc/jsonrpc/client"
	"go.uber.org/zap"
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
	log  *zap.Logger
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
func (node *ChainNode) NewClient(addr string) error {
	httpClient, err := libclient.DefaultHTTPClient(addr)
	if err != nil {
		return err
	}

	httpClient.Timeout = 10 * time.Second
	rpcClient, err := rpchttp.NewWithClient(addr, "/websocket", httpClient)
	if err != nil {
		return err
	}

	node.Client = rpcClient
	return nil
}

// CliContext creates a new Cosmos SDK client context
func (node *ChainNode) CliContext() client.Context {
	encoding := simapp.MakeTestEncodingConfig()
	bankTypes.RegisterInterfaces(encoding.InterfaceRegistry)
	ibctypes.RegisterInterfaces(encoding.InterfaceRegistry)
	transfertypes.RegisterInterfaces(encoding.InterfaceRegistry)
	return client.Context{
		Client:            node.Client,
		ChainID:           node.Chain.Config().ChainID,
		InterfaceRegistry: encoding.InterfaceRegistry,
		Input:             os.Stdin,
		Output:            os.Stdout,
		OutputFormat:      "json",
		LegacyAmino:       encoding.Amino,
		TxConfig:          encoding.TxConfig,
	}
}

// Name of the test node container
func (node *ChainNode) Name() string {
	return fmt.Sprintf("node-%d-%s-%s", node.Index, node.Chain.Config().ChainID, dockerutil.SanitizeContainerName(node.TestName))
}

// hostname of the test node container
func (node *ChainNode) HostName() string {
	return dockerutil.CondenseHostName(node.Name())
}

// Dir is the directory where the test node files are stored
func (node *ChainNode) Dir() string {
	return filepath.Join(node.Home, node.Name())
}

// MkDir creates the directory for the testnode
func (node *ChainNode) MkDir() {
	if err := os.MkdirAll(node.Dir(), 0755); err != nil {
		panic(err)
	}
}

// GentxPath returns the path to the gentx for a node
func (node *ChainNode) GentxPath() (string, error) {
	id, err := node.NodeID()
	return filepath.Join(node.Dir(), "config", "gentx", fmt.Sprintf("gentx-%s.json", id)), err
}

func (node *ChainNode) GenesisFilePath() string {
	return filepath.Join(node.Dir(), "config", "genesis.json")
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

func (node *ChainNode) PrivValKeyFilePath() string {
	return filepath.Join(node.Dir(), "config", "priv_validator_key.json")
}

func (node *ChainNode) TMConfigPath() string {
	return filepath.Join(node.Dir(), "config", "config.toml")
}

// Bind returns the home folder bind point for running the node
func (node *ChainNode) Bind() []string {
	return []string{fmt.Sprintf("%s:%s", node.Dir(), node.NodeHome())}
}

func (node *ChainNode) NodeHome() string {
	return filepath.Join("/tmp", node.Chain.Config().Name)
}

// Keybase returns the keyring for a given node
func (node *ChainNode) Keybase() keyring.Keyring {
	kr, err := keyring.New("", keyring.BackendTest, node.Dir(), os.Stdin)
	if err != nil {
		panic(err)
	}
	return kr
}

// SetValidatorConfigAndPeers modifies the config for a validator node to start a chain
func (node *ChainNode) SetValidatorConfigAndPeers(peers string) {
	// Pull default config
	cfg := tmconfig.DefaultConfig()

	// change config to include everything needed
	applyConfigChanges(cfg, peers)

	// overwrite with the new config
	tmconfig.WriteConfigFile(node.TMConfigPath(), cfg)
}

func (node *ChainNode) Height(ctx context.Context) (uint64, error) {
	res, err := node.Client.Status(ctx)
	if err != nil {
		return 0, fmt.Errorf("tendermint rpc client status: %w", err)
	}
	height := res.SyncInfo.LatestBlockHeight
	node.maybeLogBlock(height)
	return uint64(height), nil
}

func (node *ChainNode) maybeLogBlock(height int64) {
	if !node.logger().Core().Enabled(zap.DebugLevel) {
		return
	}
	ctx := context.Background()
	blockRes, err := node.Client.Block(ctx, &height)
	if err != nil {
		node.logger().Info("Failed to get block", zap.Error(err))
		return
	}
	txs := blockRes.Block.Txs
	if len(txs) == 0 {
		return
	}
	buf := new(bytes.Buffer)
	buf.WriteString("BLOCK INFO\n")
	fmt.Fprintf(buf, "BLOCK HEIGHT: %d\n", height)
	fmt.Fprintf(buf, "TOTAL TXs: %d\n", len(blockRes.Block.Txs))

	for i, tx := range blockRes.Block.Txs {
		fmt.Fprintf(buf, "TX #%d\n", i)
		txResp, err := authTx.QueryTx(node.CliContext(), hex.EncodeToString(tx.Hash()))
		if err != nil {
			fmt.Fprintf(buf, "(Failed to query tx: %v)", err)
			continue
		}
		fmt.Fprintf(buf, "TX TYPE: %s\n", txResp.Tx.TypeUrl)

		// Purposefully zero out fields to make spew's output less verbose
		txResp.Data = "[redacted]"
		txResp.RawLog = "[redacted]"
		txResp.Events = nil // already present in TxResponse.Logs

		spew.Fprint(buf, txResp)
	}

	node.logger().Debug(buf.String())
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
func (node *ChainNode) InitHomeFolder(ctx context.Context) error {
	command := []string{node.Chain.Config().Bin, "init", CondenseMoniker(node.Name()),
		"--chain-id", node.Chain.Config().ChainID,
		"--home", node.NodeHome(),
	}
	return dockerutil.HandleNodeJobError(node.NodeJob(ctx, command))
}

// CreateKey creates a key in the keyring backend test for the given node
func (node *ChainNode) CreateKey(ctx context.Context, name string) error {
	command := []string{node.Chain.Config().Bin, "keys", "add", name,
		"--keyring-backend", keyring.BackendTest,
		"--output", "json",
		"--home", node.NodeHome(),
	}
	node.lock.Lock()
	defer node.lock.Unlock()
	return dockerutil.HandleNodeJobError(node.NodeJob(ctx, command))
}

// AddGenesisAccount adds a genesis account for each key
func (node *ChainNode) AddGenesisAccount(ctx context.Context, address string, genesisAmount []types.Coin) error {
	amount := ""
	for i, coin := range genesisAmount {
		if i != 0 {
			amount += ","
		}
		amount += fmt.Sprintf("%d%s", coin.Amount.Int64(), coin.Denom)
	}
	command := []string{node.Chain.Config().Bin, "add-genesis-account", address, amount,
		"--home", node.NodeHome(),
	}
	node.lock.Lock()
	defer node.lock.Unlock()
	return dockerutil.HandleNodeJobError(node.NodeJob(ctx, command))
}

// Gentx generates the gentx for a given node
func (node *ChainNode) Gentx(ctx context.Context, name string, genesisSelfDelegation types.Coin) error {
	command := []string{node.Chain.Config().Bin, "gentx", valKey, fmt.Sprintf("%d%s", genesisSelfDelegation.Amount.Int64(), genesisSelfDelegation.Denom),
		"--keyring-backend", keyring.BackendTest,
		"--home", node.NodeHome(),
		"--chain-id", node.Chain.Config().ChainID,
	}
	node.lock.Lock()
	defer node.lock.Unlock()
	return dockerutil.HandleNodeJobError(node.NodeJob(ctx, command))
}

// CollectGentxs runs collect gentxs on the node's home folders
func (node *ChainNode) CollectGentxs(ctx context.Context) error {
	command := []string{node.Chain.Config().Bin, "collect-gentxs",
		"--home", node.NodeHome(),
	}
	node.lock.Lock()
	defer node.lock.Unlock()
	return dockerutil.HandleNodeJobError(node.NodeJob(ctx, command))
}

type IBCTransferTx struct {
	TxHash string `json:"txhash"`
}

func (node *ChainNode) SendIBCTransfer(ctx context.Context, channelID string, keyName string, amount ibc.WalletAmount, timeout *ibc.IBCTimeout) (string, error) {
	command := []string{node.Chain.Config().Bin, "tx", "ibc-transfer", "transfer", "transfer", channelID,
		amount.Address, fmt.Sprintf("%d%s", amount.Amount, amount.Denom),
		"--keyring-backend", keyring.BackendTest,
		"--gas-prices", node.Chain.Config().GasPrices,
		"--gas-adjustment", fmt.Sprint(node.Chain.Config().GasAdjustment),
		"--node", fmt.Sprintf("tcp://%s:26657", node.HostName()),
		"--from", keyName,
		"--output", "json",
		"-y",
		"--home", node.NodeHome(),
		"--chain-id", node.Chain.Config().ChainID,
	}
	if timeout != nil {
		if timeout.NanoSeconds > 0 {
			command = append(command, "--packet-timeout-timestamp", fmt.Sprint(timeout.NanoSeconds))
		} else if timeout.Height > 0 {
			command = append(command, "--packet-timeout-height", fmt.Sprintf("0-%d", timeout.Height))
		}
	}
	node.lock.Lock()
	defer node.lock.Unlock()
	exitCode, stdout, stderr, err := node.NodeJob(ctx, command)
	if err != nil {
		return "", dockerutil.HandleNodeJobError(exitCode, stdout, stderr, err)
	}
	err = test.WaitForBlocks(ctx, 2, node)
	if err != nil {
		return "", fmt.Errorf("wait for blocks: %w", err)
	}
	output := IBCTransferTx{}
	err = json.Unmarshal([]byte(stdout), &output)
	return output.TxHash, err
}

func (node *ChainNode) SendFunds(ctx context.Context, keyName string, amount ibc.WalletAmount) error {
	command := []string{node.Chain.Config().Bin, "tx", "bank", "send", keyName,
		amount.Address, fmt.Sprintf("%d%s", amount.Amount, amount.Denom),
		"--keyring-backend", keyring.BackendTest,
		"--node", fmt.Sprintf("tcp://%s:26657", node.HostName()),
		"--output", "json",
		"-y",
		"--home", node.NodeHome(),
		"--chain-id", node.Chain.Config().ChainID,
	}
	return node.NodeJobThenWaitForBlocksLocked(ctx, command)
}

func (node *ChainNode) NodeJobThenWaitForBlocksLocked(ctx context.Context, command []string) error {
	node.lock.Lock()
	defer node.lock.Unlock()
	exitCode, stdout, stderr, err := node.NodeJob(ctx, command)
	if err != nil {
		return dockerutil.HandleNodeJobError(exitCode, stdout, stderr, err)
	}
	return test.WaitForBlocks(ctx, 2, node)
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

func (node *ChainNode) InstantiateContract(ctx context.Context, keyName string, amount ibc.WalletAmount, fileName, initMessage string, needsNoAdminFlag bool) (string, error) {
	_, file := filepath.Split(fileName)
	newFilePath := filepath.Join(node.Dir(), file)
	newFilePathContainer := filepath.Join(node.NodeHome(), file)
	if _, err := dockerutil.CopyFile(fileName, newFilePath); err != nil {
		return "", err
	}

	command := []string{node.Chain.Config().Bin, "tx", "wasm", "store", newFilePathContainer,
		"--from", keyName,
		"--gas-prices", node.Chain.Config().GasPrices,
		"--gas-adjustment", fmt.Sprint(node.Chain.Config().GasAdjustment),
		"--keyring-backend", keyring.BackendTest,
		"--node", fmt.Sprintf("tcp://%s:26657", node.HostName()),
		"--output", "json",
		"-y",
		"--home", node.NodeHome(),
		"--chain-id", node.Chain.Config().ChainID,
	}
	node.lock.Lock()
	defer node.lock.Unlock()
	exitCode, stdout, stderr, err := node.NodeJob(ctx, command)
	if err != nil {
		return "", dockerutil.HandleNodeJobError(exitCode, stdout, stderr, err)
	}

	err = test.WaitForBlocks(ctx, 5, node.Chain)
	if err != nil {
		return "", fmt.Errorf("wait for blocks: %w", err)
	}

	command = []string{node.Chain.Config().Bin,
		"query", "wasm", "list-code", "--reverse",
		"--node", fmt.Sprintf("tcp://%s:26657", node.HostName()),
		"--output", "json",
		"--home", node.NodeHome(),
		"--chain-id", node.Chain.Config().ChainID,
	}

	exitCode, stdout, stderr, err = node.NodeJob(ctx, command)
	if err != nil {
		return "", dockerutil.HandleNodeJobError(exitCode, stdout, stderr, err)
	}

	res := CodeInfosResponse{}
	if err := json.Unmarshal([]byte(stdout), &res); err != nil {
		return "", err
	}

	codeID := res.CodeInfos[0].CodeID

	command = []string{node.Chain.Config().Bin,
		"tx", "wasm", "instantiate", codeID, initMessage,
		"--gas-prices", node.Chain.Config().GasPrices,
		"--gas-adjustment", fmt.Sprint(node.Chain.Config().GasAdjustment),
		"--label", "satoshi-test",
		"--from", keyName,
		"--keyring-backend", keyring.BackendTest,
		"--node", fmt.Sprintf("tcp://%s:26657", node.HostName()),
		"--output", "json",
		"-y",
		"--home", node.NodeHome(),
		"--chain-id", node.Chain.Config().ChainID,
	}

	if needsNoAdminFlag {
		command = append(command, "--no-admin")
	}

	exitCode, stdout, stderr, err = node.NodeJob(ctx, command)
	if err != nil {
		return "", dockerutil.HandleNodeJobError(exitCode, stdout, stderr, err)
	}

	err = test.WaitForBlocks(ctx, 5, node.Chain)
	if err != nil {
		return "", fmt.Errorf("wait for blocks: %w", err)
	}

	command = []string{node.Chain.Config().Bin,
		"query", "wasm", "list-contract-by-code", codeID,
		"--node", fmt.Sprintf("tcp://%s:26657", node.HostName()),
		"--output", "json",
		"--home", node.NodeHome(),
		"--chain-id", node.Chain.Config().ChainID,
	}

	exitCode, stdout, stderr, err = node.NodeJob(ctx, command)
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

func (node *ChainNode) ExecuteContract(ctx context.Context, keyName string, contractAddress string, message string) error {
	command := []string{node.Chain.Config().Bin,
		"tx", "wasm", "execute", contractAddress, message,
		"--from", keyName,
		"--gas-prices", node.Chain.Config().GasPrices,
		"--gas-adjustment", fmt.Sprint(node.Chain.Config().GasAdjustment),
		"--keyring-backend", keyring.BackendTest,
		"--node", fmt.Sprintf("tcp://%s:26657", node.HostName()),
		"--output", "json",
		"-y",
		"--home", node.NodeHome(),
		"--chain-id", node.Chain.Config().ChainID,
	}
	return node.NodeJobThenWaitForBlocksLocked(ctx, command)
}

func (node *ChainNode) DumpContractState(ctx context.Context, contractAddress string, height int64) (*ibc.DumpContractStateResponse, error) {
	command := []string{node.Chain.Config().Bin,
		"query", "wasm", "contract-state", "all", contractAddress,
		"--height", fmt.Sprint(height),
		"--node", fmt.Sprintf("tcp://%s:26657", node.HostName()),
		"--output", "json",
		"--home", node.NodeHome(),
		"--chain-id", node.Chain.Config().ChainID,
	}
	exitCode, stdout, stderr, err := node.NodeJob(ctx, command)
	if err != nil {
		return nil, dockerutil.HandleNodeJobError(exitCode, stdout, stderr, err)
	}

	res := &ibc.DumpContractStateResponse{}
	if err := json.Unmarshal([]byte(stdout), res); err != nil {
		return nil, err
	}
	return res, nil
}

func (node *ChainNode) ExportState(ctx context.Context, height int64) (string, error) {
	command := []string{node.Chain.Config().Bin,
		"export",
		"--height", fmt.Sprint(height),
		"--home", node.NodeHome(),
	}
	exitCode, stdout, stderr, err := node.NodeJob(ctx, command)
	if err != nil {
		return "", dockerutil.HandleNodeJobError(exitCode, stdout, stderr, err)
	}
	// output comes to stderr for some reason
	return stderr, nil
}

func (node *ChainNode) UnsafeResetAll(ctx context.Context) error {
	command := []string{node.Chain.Config().Bin,
		"unsafe-reset-all",
		"--home", node.NodeHome(),
	}

	return dockerutil.HandleNodeJobError(node.NodeJob(ctx, command))
}

func (node *ChainNode) CreatePool(ctx context.Context, keyName string, contractAddress string, swapFee float64, exitFee float64, assets []ibc.WalletAmount) error {
	// TODO generate --pool-file
	poolFilePath := "TODO"
	command := []string{node.Chain.Config().Bin,
		"tx", "gamm", "create-pool",
		"--pool-file", poolFilePath,
		"--gas-prices", node.Chain.Config().GasPrices,
		"--gas-adjustment", fmt.Sprint(node.Chain.Config().GasAdjustment),
		"--from", keyName,
		"--keyring-backend", keyring.BackendTest,
		"--node", fmt.Sprintf("tcp://%s:26657", node.HostName()),
		"--output", "json",
		"-y",
		"--home", node.NodeHome(),
		"--chain-id", node.Chain.Config().ChainID,
	}
	return node.NodeJobThenWaitForBlocksLocked(ctx, command)
}

func (node *ChainNode) CreateNodeContainer() error {
	chainCfg := node.Chain.Config()
	cmd := []string{chainCfg.Bin, "start", "--home", node.NodeHome(), "--x-crisis-skip-assert-invariants"}
	if chainCfg.NoHostMount {
		cmd = []string{"sh", "-c", fmt.Sprintf("cp -r %s %s_nomnt && %s start --home %s_nomnt --x-crisis-skip-assert-invariants", node.NodeHome(), node.NodeHome(), chainCfg.Bin, node.NodeHome())}
	}
	node.logger().
		Info("Running command",
			zap.String("command", strings.Join(cmd, " ")),
			zap.String("container", node.Name()),
		)

	cont, err := node.Pool.Client.CreateContainer(docker.CreateContainerOptions{
		Name: node.Name(),
		Config: &docker.Config{
			User:         dockerutil.GetDockerUserString(),
			Cmd:          cmd,
			Hostname:     node.HostName(),
			ExposedPorts: sentryPorts,
			DNS:          []string{},
			Image:        fmt.Sprintf("%s:%s", node.Image.Repository, node.Image.Version),
			Labels:       map[string]string{"ibc-test": node.TestName},
			Entrypoint:   []string{},
		},
		HostConfig: &docker.HostConfig{
			Binds:           node.Bind(),
			PublishAllPorts: true,
			AutoRemove:      false,
		},
		NetworkingConfig: &docker.NetworkingConfig{
			EndpointsConfig: map[string]*docker.EndpointConfig{
				node.NetworkID: {},
			},
		},
		Context: nil,
	})
	if err != nil {
		return err
	}
	node.Container = cont
	return nil
}

func (node *ChainNode) StopContainer() error {
	return node.Pool.Client.StopContainer(node.Container.ID, 30)
}

func (node *ChainNode) StartContainer(ctx context.Context) error {
	if err := node.Pool.Client.StartContainer(node.Container.ID, nil); err != nil {
		return err
	}

	c, err := node.Pool.Client.InspectContainer(node.Container.ID)
	if err != nil {
		return err
	}
	node.Container = c

	port := dockerutil.GetHostPort(c, rpcPort)
	node.logger().Info("Rpc", zap.String("container", node.Name()), zap.String("port", port))

	err = node.NewClient(fmt.Sprintf("tcp://%s", port))
	if err != nil {
		return err
	}

	time.Sleep(5 * time.Second)
	return retry.Do(func() error {
		stat, err := node.Client.Status(ctx)
		if err != nil {
			return err
		}
		// TODO: reenable this check, having trouble with it for some reason
		if stat != nil && stat.SyncInfo.CatchingUp {
			return fmt.Errorf("still catching up: height(%d) catching-up(%t)",
				stat.SyncInfo.LatestBlockHeight, stat.SyncInfo.CatchingUp)
		}
		return nil
	}, retry.Context(ctx), retry.Attempts(100), retry.Delay(5*time.Second), retry.DelayType(retry.FixedDelay))
}

// InitValidatorFiles creates the node files and signs a genesis transaction
func (node *ChainNode) InitValidatorFiles(
	ctx context.Context,
	chainType *ibc.ChainConfig,
	genesisAmounts []types.Coin,
	genesisSelfDelegation types.Coin,
) error {
	if err := node.InitHomeFolder(ctx); err != nil {
		return err
	}
	if err := node.CreateKey(ctx, valKey); err != nil {
		return err
	}
	key, err := node.GetKey(valKey)
	if err != nil {
		return err
	}
	bech32, err := types.Bech32ifyAddressBytes(chainType.Bech32Prefix, key.GetAddress().Bytes())
	if err != nil {
		return err
	}
	if err := node.AddGenesisAccount(ctx, bech32, genesisAmounts); err != nil {
		return err
	}
	return node.Gentx(ctx, valKey, genesisSelfDelegation)
}

func (node *ChainNode) InitFullNodeFiles(ctx context.Context) error {
	return node.InitHomeFolder(ctx)
}

// NodeID returns the node of a given node
func (node *ChainNode) NodeID() (string, error) {
	nodeKey, err := p2p.LoadNodeKey(filepath.Join(node.Dir(), "config", "node_key.json"))
	if err != nil {
		return "", err
	}
	return string(nodeKey.ID()), nil
}

// GetKey gets a key, waiting until it is available
func (node *ChainNode) GetKey(name string) (info keyring.Info, err error) {
	return info, retry.Do(func() (err error) {
		info, err = node.Keybase().Key(name)
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
		nodes.logger().Info("Peering",
			zap.String("host_name", hostName),
			zap.String("peer", ps),
			zap.String("container", n.Name()),
		)
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
		nodes.logger().Info("Genesis", zap.String("hash", fmt.Sprintf("%X", sha256.Sum256(gen))))
	}
	return nil
}

func (nodes ChainNodes) logger() *zap.Logger {
	if len(nodes) == 0 {
		return zap.NewNop()
	}
	return nodes[0].logger()
}

// NodeJob run a container for a specific job and block until the container exits
// NOTE: on job containers generate random name
func (node *ChainNode) NodeJob(ctx context.Context, cmd []string) (int, string, string, error) {
	counter, _, _, _ := runtime.Caller(1)
	caller := runtime.FuncForPC(counter).Name()
	funcName := strings.Split(caller, ".")
	container := fmt.Sprintf("%s-%s-%s", node.Name(), funcName[len(funcName)-1], dockerutil.RandLowerCaseLetterString(3))
	node.logger().
		Info("Running command",
			zap.String("command", strings.Join(cmd, " ")),
			zap.String("container", container),
		)
	cont, err := node.Pool.Client.CreateContainer(docker.CreateContainerOptions{
		Name: container,
		Config: &docker.Config{
			User: dockerutil.GetDockerUserString(),
			// random hostname is fine here since this is just for setup
			Hostname:     dockerutil.CondenseHostName(container),
			ExposedPorts: sentryPorts,
			DNS:          []string{},
			Image:        fmt.Sprintf("%s:%s", node.Image.Repository, node.Image.Version),
			Cmd:          cmd,
			Labels:       map[string]string{"ibc-test": node.TestName},
			Entrypoint:   []string{},
		},
		HostConfig: &docker.HostConfig{
			Binds:           node.Bind(),
			PublishAllPorts: true,
			AutoRemove:      false,
		},
		NetworkingConfig: &docker.NetworkingConfig{
			EndpointsConfig: map[string]*docker.EndpointConfig{
				node.NetworkID: {},
			},
		},
		Context: nil,
	})
	if err != nil {
		return 1, "", "", err
	}
	if err := node.Pool.Client.StartContainer(cont.ID, nil); err != nil {
		return 1, "", "", err
	}

	exitCode, err := node.Pool.Client.WaitContainerWithContext(cont.ID, ctx)
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	_ = node.Pool.Client.Logs(docker.LogsOptions{Context: ctx, Container: cont.ID, OutputStream: stdout, ErrorStream: stderr, Stdout: true, Stderr: true, Tail: "50", Follow: false, Timestamps: false})
	_ = node.Pool.Client.RemoveContainer(docker.RemoveContainerOptions{ID: cont.ID})
	node.logger().
		Debug(
			fmt.Sprintf("stdout:\n%s\nstderr:\n%s", stdout.String(), stderr.String()),
			zap.String("container", container),
		)
	return exitCode, stdout.String(), stderr.String(), err
}

func (node *ChainNode) logger() *zap.Logger {
	return node.log.With(
		zap.String("chain_id", node.Chain.Config().ChainID),
		zap.String("test", node.TestName),
	)
}
