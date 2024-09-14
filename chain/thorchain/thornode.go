package thorchain

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"math/rand"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/avast/retry-go/v4"
	tmjson "github.com/cometbft/cometbft/libs/json"
	"github.com/cometbft/cometbft/p2p"
	rpcclient "github.com/cometbft/cometbft/rpc/client"
	rpchttp "github.com/cometbft/cometbft/rpc/client/http"
	libclient "github.com/cometbft/cometbft/rpc/jsonrpc/client"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authTx "github.com/cosmos/cosmos-sdk/x/auth/tx"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	volumetypes "github.com/docker/docker/api/types/volume"
	dockerclient "github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/icza/dyno"
	"go.uber.org/zap"
	"golang.org/x/mod/semver"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/strangelove-ventures/interchaintest/v8/dockerutil"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"github.com/strangelove-ventures/interchaintest/v8/testutil"
)

type ChainNode struct {
	VolumeName   string
	Index        int
	Chain        ibc.Chain
	Validator    bool
	NetworkID    string
	DockerClient *dockerclient.Client
	Client       rpcclient.Client
	GrpcConn     *grpc.ClientConn
	TestName     string
	Image        ibc.DockerImage
	preStartNode func(*ChainNode)

	// Env
	ValidatorMnemonic string // SIGNER_SEED_PHRASE
	NodeAccount       *NodeAccount
	KeyringCreated    bool

	// Additional processes that need to be run on a per-validator basis.
	Sidecars SidecarProcesses

	lock sync.Mutex
	log  *zap.Logger

	containerLifecycle *dockerutil.ContainerLifecycle

	// Ports set during StartContainer.
	hostRPCPort   string
	hostAPIPort   string
	hostGRPCPort  string
	hostP2PPort   string
	cometHostname string
}

func NewChainNode(log *zap.Logger, validator bool, chain *Thorchain, dockerClient *dockerclient.Client, networkID string, testName string, image ibc.DockerImage, index int) *ChainNode {
	tn := &ChainNode{
		log: log.With(zap.Bool("validator", validator), zap.Int("i", index)),

		Validator: validator,

		Chain:        chain,
		DockerClient: dockerClient,
		NetworkID:    networkID,
		TestName:     testName,
		Image:        image,
		Index:        index,
	}

	tn.containerLifecycle = dockerutil.NewContainerLifecycle(log, dockerClient, tn.Name())

	return tn
}

// WithPreStartNode sets the preStartNode function for the ChainNode
func (tn *ChainNode) WithPreStartNode(preStartNode func(*ChainNode)) *ChainNode {
	tn.preStartNode = preStartNode
	return tn
}

// ChainNodes is a collection of ChainNode
type ChainNodes []*ChainNode

const (
	valKey      = "thorchain"
	blockTime   = 2 // seconds
	p2pPort     = "26656/tcp"
	rpcPort     = "26657/tcp"
	grpcPort    = "9090/tcp"
	apiPort     = "1317/tcp"
	privValPort = "1234/tcp"

	cometMockRawPort = "22331"
)

var (
	sentryPorts = nat.PortMap{
		nat.Port(p2pPort):     {},
		nat.Port(rpcPort):     {},
		nat.Port(grpcPort):    {},
		nat.Port(apiPort):     {},
		nat.Port(privValPort): {},
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

	grpcConn, err := grpc.NewClient(
		tn.hostGRPCPort, grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return fmt.Errorf("grpc dial: %w", err)
	}
	tn.GrpcConn = grpcConn

	return nil
}

func (tn *ChainNode) NewSidecarProcess(
	ctx context.Context,
	preStart bool,
	processName string,
	cli *dockerclient.Client,
	networkID string,
	image ibc.DockerImage,
	homeDir string,
	ports []string,
	startCmd []string,
	env []string,
) error {
	s := NewSidecar(tn.log, true, preStart, tn.Chain, cli, networkID, processName, tn.TestName, image, homeDir, tn.Index, ports, startCmd, env)

	v, err := cli.VolumeCreate(ctx, volumetypes.CreateOptions{
		Labels: map[string]string{
			dockerutil.CleanupLabel:   tn.TestName,
			dockerutil.NodeOwnerLabel: s.Name(),
		},
	})
	if err != nil {
		return fmt.Errorf("creating volume for sidecar process: %w", err)
	}
	s.VolumeName = v.Name

	if err := dockerutil.SetVolumeOwner(ctx, dockerutil.VolumeOwnerOptions{
		Log: tn.log,

		Client: cli,

		VolumeName: v.Name,
		ImageRef:   image.Ref(),
		TestName:   tn.TestName,
		UidGid:     image.UidGid,
	}); err != nil {
		return fmt.Errorf("set volume owner: %w", err)
	}

	tn.Sidecars = append(tn.Sidecars, s)

	return nil
}

// CliContext creates a new Cosmos SDK client context
func (tn *ChainNode) CliContext() client.Context {
	cfg := tn.Chain.Config()
	return client.Context{
		Client:            tn.Client,
		GRPCClient:        tn.GrpcConn,
		ChainID:           cfg.ChainID,
		InterfaceRegistry: cfg.EncodingConfig.InterfaceRegistry,
		Input:             os.Stdin,
		Output:            os.Stdout,
		OutputFormat:      "json",
		LegacyAmino:       cfg.EncodingConfig.Amino,
		TxConfig:          cfg.EncodingConfig.TxConfig,
	}
}

// Name of the test node container
func (tn *ChainNode) Name() string {
	return fmt.Sprintf("%s-%s-%d-%s", tn.Chain.Config().ChainID, tn.NodeType(), tn.Index, dockerutil.SanitizeContainerName(tn.TestName))
}

func (tn *ChainNode) NodeType() string {
	nodeType := "fn"
	if tn.Validator {
		nodeType = "val"
	}
	return nodeType
}

func (tn *ChainNode) ContainerID() string {
	return tn.containerLifecycle.ContainerID()
}

// hostname of the test node container
func (tn *ChainNode) HostName() string {
	return dockerutil.CondenseHostName(tn.Name())
}

// hostname of the comet mock container
func (tn *ChainNode) HostnameCometMock() string {
	return tn.cometHostname
}

func (tn *ChainNode) GenesisFileContent(ctx context.Context) ([]byte, error) {
	gen, err := tn.ReadFile(ctx, "config/genesis.json")
	if err != nil {
		return nil, fmt.Errorf("getting genesis.json content: %w", err)
	}

	return gen, nil
}

func (tn *ChainNode) OverwriteGenesisFile(ctx context.Context, content []byte) error {
	err := tn.WriteFile(ctx, content, "config/genesis.json")
	if err != nil {
		return fmt.Errorf("overwriting genesis.json: %w", err)
	}

	return nil
}

func (tn *ChainNode) PrivValFileContent(ctx context.Context) ([]byte, error) {
	gen, err := tn.ReadFile(ctx, "config/priv_validator_key.json")
	if err != nil {
		return nil, fmt.Errorf("getting priv_validator_key.json content: %w", err)
	}

	return gen, nil
}

func (tn *ChainNode) OverwritePrivValFile(ctx context.Context, content []byte) error {
	fw := dockerutil.NewFileWriter(tn.logger(), tn.DockerClient, tn.TestName)
	if err := fw.WriteFile(ctx, tn.VolumeName, "config/priv_validator_key.json", content); err != nil {
		return fmt.Errorf("overwriting priv_validator_key.json: %w", err)
	}

	return nil
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

// Bind returns the home folder bind point for running the node
func (tn *ChainNode) Bind() []string {
	return []string{fmt.Sprintf("%s:%s", tn.VolumeName, tn.HomeDir())}
}

func (tn *ChainNode) HomeDir() string {
	return "/home/heighliner/.thornode"
}

// SetTestConfig modifies the config to reasonable values for use within interchaintest.
func (tn *ChainNode) SetTestConfig(ctx context.Context) error {
	c := make(testutil.Toml)

	// Set Log Level to info
	c["log_level"] = "info"

	p2p := make(testutil.Toml)

	// Allow p2p strangeness
	p2p["allow_duplicate_ip"] = true
	p2p["addr_book_strict"] = false

	c["p2p"] = p2p

	consensus := make(testutil.Toml)

	blockT := (time.Duration(blockTime) * time.Second).String()
	consensus["timeout_commit"] = blockT
	consensus["timeout_propose"] = blockT

	c["consensus"] = consensus

	rpc := make(testutil.Toml)

	// Enable public RPC
	rpc["laddr"] = "tcp://0.0.0.0:26657"
	if tn.Chain.Config().UsesCometMock() {
		rpc["laddr"] = fmt.Sprintf("tcp://%s:%s", tn.HostnameCometMock(), cometMockRawPort)
	}

	rpc["allowed_origins"] = []string{"*"}

	c["rpc"] = rpc

	if err := testutil.ModifyTomlConfigFile(
		ctx,
		tn.logger(),
		tn.DockerClient,
		tn.TestName,
		tn.VolumeName,
		"config/config.toml",
		c,
	); err != nil {
		return err
	}

	a := make(testutil.Toml)
	a["minimum-gas-prices"] = tn.Chain.Config().GasPrices

	grpc := make(testutil.Toml)

	// Enable public GRPC
	grpc["address"] = "0.0.0.0:9090"

	a["grpc"] = grpc

	api := make(testutil.Toml)

	// Enable public REST API
	api["enable"] = true
	api["swagger"] = false
	api["address"] = "tcp://0.0.0.0:1317"
	api["enabled-unsafe-cors"] = true

	a["api"] = api

	return testutil.ModifyTomlConfigFile(
		ctx,
		tn.logger(),
		tn.DockerClient,
		tn.TestName,
		tn.VolumeName,
		"config/app.toml",
		a,
	)
}

// SetPeers modifies the config persistent_peers for a node
func (tn *ChainNode) SetPeers(ctx context.Context, peers string) error {
	c := make(testutil.Toml)
	p2p := make(testutil.Toml)

	// Set peers
	p2p["persistent_peers"] = peers
	c["p2p"] = p2p

	return testutil.ModifyTomlConfigFile(
		ctx,
		tn.logger(),
		tn.DockerClient,
		tn.TestName,
		tn.VolumeName,
		"config/config.toml",
		c,
	)
}

func (tn *ChainNode) Height(ctx context.Context) (int64, error) {
	time.Sleep(time.Millisecond * 200) // TODO: slow down WaitForBlocks instead of here
	res, err := tn.Client.Status(ctx)
	if err != nil {
		return 0, fmt.Errorf("tendermint rpc client status: %w", err)
	}
	height := res.SyncInfo.LatestBlockHeight
	return height, nil
}

// TxCommand is a helper to retrieve a full command for broadcasting a tx
// with the chain node binary.
func (tn *ChainNode) TxCommand(keyName string, command ...string) []string {
	command = append([]string{"tx"}, command...)
	var gasPriceFound, gasAdjustmentFound, feesFound = false, false, false
	for i := 0; i < len(command); i++ {
		if command[i] == "--gas-prices" {
			gasPriceFound = true
		}
		if command[i] == "--gas-adjustment" {
			gasAdjustmentFound = true
		}
		if command[i] == "--fees" {
			feesFound = true
		}
	}
	if !gasPriceFound && !feesFound {
		command = append(command, "--gas-prices", tn.Chain.Config().GasPrices)
	}
	if !gasAdjustmentFound {
		command = append(command, "--gas-adjustment", strconv.FormatFloat(tn.Chain.Config().GasAdjustment, 'f', -1, 64))
	}
	return tn.NodeCommand(append(command,
		"--from", keyName,
		"--output", "json",
		"-y",
		"--chain-id", tn.Chain.Config().ChainID,
	)...)
}

// ExecTx executes a transaction, waits for 2 blocks if successful, then returns the tx hash.
func (tn *ChainNode) ExecTx(ctx context.Context, keyName string, command ...string) (string, error) {
	tn.lock.Lock()
	defer tn.lock.Unlock()

	keyringCommand := []string{
		"sh",
		"-c",
		fmt.Sprintf(`echo %s | %s --keyring-backend %s`, "password", strings.Join(tn.TxCommand(keyName, command...), " "), keyring.BackendFile), // TODO: get password from env
	}

	stdout, _, err := tn.Exec(ctx, keyringCommand, tn.Chain.Config().Env)
	if err != nil {
		return "", err
	}
	output := CosmosTx{}
	err = json.Unmarshal([]byte(stdout), &output)
	if err != nil {
		return "", err
	}
	if output.Code != 0 {
		return output.TxHash, fmt.Errorf("transaction failed with code %d: %s", output.Code, output.RawLog)
	}
	if err := testutil.WaitForBlocks(ctx, 2, tn); err != nil {
		return "", err
	}
	// The transaction can at first appear to succeed, but then fail when it's actually included in a block.
	stdout, _, err = tn.ExecQuery(ctx, "tx", output.TxHash)
	if err != nil {
		return "", err
	}
	output = CosmosTx{}
	err = json.Unmarshal([]byte(stdout), &output)
	if err != nil {
		return "", err
	}
	if output.Code != 0 {
		return output.TxHash, fmt.Errorf("transaction failed with code %d: %s", output.Code, output.RawLog)
	}
	return output.TxHash, nil
}

// TxHashToResponse returns the sdk transaction response struct for a given transaction hash.
func (tn *ChainNode) TxHashToResponse(ctx context.Context, txHash string) (*sdk.TxResponse, error) {
	stdout, stderr, err := tn.ExecQuery(ctx, "tx", txHash)
	if err != nil {
		fmt.Println("TxHashToResponse err: ", err.Error()+" "+string(stderr))
	}

	i := &sdk.TxResponse{}

	// ignore the error since some types do not unmarshal (ex: height of int64 vs string)
	_ = json.Unmarshal(stdout, &i)
	return i, nil
}

// NodeCommand is a helper to retrieve a full command for a chain node binary.
// when interactions with the RPC endpoint are necessary.
// For example, if chain node binary is `gaiad`, and desired command is `gaiad keys show key1`,
// pass ("keys", "show", "key1") for command to return the full command.
// Will include additional flags for node URL, home directory, and chain ID.
func (tn *ChainNode) NodeCommand(command ...string) []string {
	command = tn.BinCommand(command...)

	endpoint := fmt.Sprintf("tcp://%s:26657", tn.HostName())

	if tn.Chain.Config().UsesCometMock() {
		endpoint = fmt.Sprintf("tcp://%s:%s", tn.HostnameCometMock(), cometMockRawPort)
	}

	return append(command,
		"--node", endpoint,
	)
}

// BinCommand is a helper to retrieve a full command for a chain node binary.
// For example, if chain node binary is `gaiad`, and desired command is `gaiad keys show key1`,
// pass ("keys", "show", "key1") for command to return the full command.
// Will include additional flags for home directory and chain ID.
func (tn *ChainNode) BinCommand(command ...string) []string {
	command = append([]string{tn.Chain.Config().Bin}, command...)
	return append(command,
		"--home", tn.HomeDir(),
	)
}

// ExecBin is a helper to execute a command for a chain node binary.
// For example, if chain node binary is `gaiad`, and desired command is `gaiad keys show key1`,
// pass ("keys", "show", "key1") for command to execute the command against the node.
// Will include additional flags for home directory and chain ID.
func (tn *ChainNode) ExecBin(ctx context.Context, backendfile bool, command ...string) ([]byte, []byte, error) {
	if !backendfile {
		return tn.Exec(ctx, tn.BinCommand(command...), tn.Chain.Config().Env)
	}

	keyringCommand := []string{
		"sh",
		"-c",
		fmt.Sprintf(`cat <<EOF | %s --keyring-backend %s
password
password
EOF
`, strings.Join(tn.BinCommand(command...), " "), keyring.BackendFile), // TODO: get password from env
	}
	return tn.Exec(ctx, keyringCommand, tn.Chain.Config().Env)
}

// QueryCommand is a helper to retrieve the full query command. For example,
// if chain node binary is gaiad, and desired command is `gaiad query gov params`,
// pass ("gov", "params") for command to return the full command with all necessary
// flags to query the specific node.
func (tn *ChainNode) QueryCommand(command ...string) []string {
	command = append([]string{"query"}, command...)
	return tn.NodeCommand(append(command,
		"--output", "json",
	)...)
}

// ExecQuery is a helper to execute a query command. For example,
// if chain node binary is gaiad, and desired command is `gaiad query gov params`,
// pass ("gov", "params") for command to execute the query against the node.
// Returns response in json format.
func (tn *ChainNode) ExecQuery(ctx context.Context, command ...string) ([]byte, []byte, error) {
	return tn.Exec(ctx, tn.QueryCommand(command...), tn.Chain.Config().Env)
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
	tn.lock.Lock()
	defer tn.lock.Unlock()

	_, _, err := tn.ExecBin(ctx, false,
		"init", CondenseMoniker(tn.Name()),
		"--chain-id", tn.Chain.Config().ChainID,
	)
	return err
}

// WriteFile accepts file contents in a byte slice and writes the contents to
// the docker filesystem. relPath describes the location of the file in the
// docker volume relative to the home directory
func (tn *ChainNode) WriteFile(ctx context.Context, content []byte, relPath string) error {
	fw := dockerutil.NewFileWriter(tn.logger(), tn.DockerClient, tn.TestName)
	return fw.WriteFile(ctx, tn.VolumeName, relPath, content)
}

// CopyFile adds a file from the host filesystem to the docker filesystem
// relPath describes the location of the file in the docker volume relative to
// the home directory
func (tn *ChainNode) CopyFile(ctx context.Context, srcPath, dstPath string) error {
	content, err := os.ReadFile(srcPath)
	if err != nil {
		return err
	}
	return tn.WriteFile(ctx, content, dstPath)
}

// ReadFile reads the contents of a single file at the specified path in the docker filesystem.
// relPath describes the location of the file in the docker volume relative to the home directory.
func (tn *ChainNode) ReadFile(ctx context.Context, relPath string) ([]byte, error) {
	fr := dockerutil.NewFileRetriever(tn.logger(), tn.DockerClient, tn.TestName)
	gen, err := fr.SingleFileContent(ctx, tn.VolumeName, relPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file at %s: %w", relPath, err)
	}
	return gen, nil
}

// CreateKey creates a key in the keyring backend test for the given node
func (tn *ChainNode) CreateKey(ctx context.Context, name string) error {
	tn.lock.Lock()
	defer tn.lock.Unlock()

	command := []string{
		"sh",
		"-c",
		fmt.Sprintf(`cat <<EOF | %s keys add %s --keyring-backend %s --coin-type %s --home %s --output json
password
password
EOF
`, tn.Chain.Config().Bin, name, keyring.BackendFile, tn.Chain.Config().CoinType, tn.HomeDir()),
	}

	stdout, _, err := tn.Exec(ctx, command, tn.Chain.Config().Env)
	if err != nil {
		return err
	}

	if tn.Validator && tn.ValidatorMnemonic == "" {
		var createKeyOutput map[string]string
		if err := json.Unmarshal(stdout, &createKeyOutput); err != nil {
			return err
		}

		tn.ValidatorMnemonic = createKeyOutput["mnemonic"]
	}

	return nil
}

// RecoverKey restores a key from a given mnemonic.
func (tn *ChainNode) RecoverKey(ctx context.Context, keyName, mnemonic string) error {
	command := []string{
		"sh",
		"-c",
		fmt.Sprintf(`cat <<EOF | %s keys add %s --recover --keyring-backend %s --coin-type %s --home %s --output json
%s
password
password
EOF
`, tn.Chain.Config().Bin, keyName, keyring.BackendFile, tn.Chain.Config().CoinType, tn.HomeDir(), mnemonic),
	}

	tn.lock.Lock()
	defer tn.lock.Unlock()

	_, _, err := tn.Exec(ctx, command, tn.Chain.Config().Env)
	return err
}

func (tn *ChainNode) IsAboveSDK47(ctx context.Context) bool {
	// In SDK v47, a new genesis core command was added. This spec has many state breaking features
	// so we use this to switch between new and legacy SDK logic.
	// https://github.com/cosmos/cosmos-sdk/pull/14149
	return tn.HasCommand(ctx, "genesis")
}

// ICSVersion returns the version of interchain-security the binary was built with.
// If it doesn't depend on interchain-security, it returns an empty string.
func (tn *ChainNode) ICSVersion(ctx context.Context) string {
	if strings.HasPrefix(tn.Chain.Config().Bin, "interchain-security") {
		// This isn't super pretty, but it's the best we can do for an interchain-security binary.
		// It doesn't depend on itself, and the version command doesn't actually output a version.
		// Ideally if you have a binary called something like "v3.3.0-my-fix" you can use it as a version, since the v3.3.0 part is in it.
		return semver.Canonical(tn.Image.Version)
	}
	info := tn.GetBuildInformation(ctx)
	for _, dep := range info.BuildDeps {
		if strings.HasPrefix(dep.Parent, "github.com/cosmos/interchain-security") {
			return semver.Canonical(dep.Version)
		}
	}
	return ""
}

// AddGenesisAccount adds a genesis account for each key
func (tn *ChainNode) AddGenesisAccount(ctx context.Context, address string, genesisAmount []sdk.Coin) error {
	amount := ""
	for i, coin := range genesisAmount {
		if i != 0 {
			amount += ","
		}
		amount += fmt.Sprintf("%s%s", coin.Amount.String(), coin.Denom)
	}

	tn.lock.Lock()
	defer tn.lock.Unlock()

	// Adding a genesis account should complete instantly,
	// so use a 1-minute timeout to more quickly detect if Docker has locked up.
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	var command []string
	if tn.IsAboveSDK47(ctx) {
		command = append(command, "genesis")
	}

	command = append(command, "add-genesis-account", address, amount)

	if tn.Chain.Config().UsingChainIDFlagCLI {
		command = append(command, "--chain-id", tn.Chain.Config().ChainID)
	}

	_, _, err := tn.ExecBin(ctx, false, command...)

	return err
}

func (tn *ChainNode) Version(ctx context.Context) (string, error) {
	command := []string{tn.Chain.Config().Bin, "query", "thorchain", "version", "--output", "json",
		"--home", tn.HomeDir(),
	}

	stdout, stderr, err := tn.Exec(ctx, command, tn.Chain.Config().Env)
	if err != nil {
		return "", fmt.Errorf("failed to query version (stderr=%q): %w", stderr, err)
	}

	var version VersionOutput
	if err := json.Unmarshal(stdout, &version); err != nil {
		return "", fmt.Errorf("failed to unmarshal version: %w", err)
	}

	return version.Version, nil
}

func (tn *ChainNode) GetValidatorConsPubKey(ctx context.Context) (string, error) {
	command := []string{tn.Chain.Config().Bin, "tendermint", "show-validator",
		"--home", tn.HomeDir(),
	}

	stdout, stderr, err := tn.Exec(ctx, command, tn.Chain.Config().Env)
	if err != nil {
		return "", fmt.Errorf("failed to show validator (stderr=%q): %w", stderr, err)
	}

	command2 := []string{
		"sh",
		"-c",
		fmt.Sprintf(`echo %q | %s pubkey --bech cons`,
			stdout, tn.Chain.Config().Bin),
	}

	stdout, stderr, err = tn.Exec(ctx, command2, tn.Chain.Config().Env)
	if err != nil {
		return "", fmt.Errorf("failed to show validator pubkey (stderr=%q): %w", stderr, err)
	}

	return string(bytes.TrimSuffix(stdout, []byte("\n"))), nil
}

func (tn *ChainNode) GetNodePubKey(ctx context.Context) (string, error) {
	command := []string{
		"keys", "show", valKey, "-p",
	}

	stdout, stderr, err := tn.ExecBin(ctx, true, command...)
	if err != nil {
		return "", fmt.Errorf("failed to show node pub key json (stderr=%q): %w", stderr, err)
	}

	command2 := []string{
		"sh",
		"-c",
		fmt.Sprintf(`echo %q | %s pubkey`,
			stdout, tn.Chain.Config().Bin),
	}

	stdout, stderr, err = tn.Exec(ctx, command2, tn.Chain.Config().Env)
	if err != nil {
		return "", fmt.Errorf("failed to show node pubkey (stderr=%q): %w", stderr, err)
	}

	return string(bytes.TrimSuffix(stdout, []byte("\n"))), nil
}

func (tn *ChainNode) GenerateEd25519(ctx context.Context) (string, error) {
	command := []string{
		"sh",
		"-c",
		fmt.Sprintf(`cat <<EOF | %s ed25519 --home %s
password
%s
EOF
`,
			tn.Chain.Config().Bin, tn.HomeDir(), tn.ValidatorMnemonic,
		),
	}

	stdout, stderr, err := tn.Exec(ctx, command, tn.Chain.Config().Env)
	if err != nil {
		return "", fmt.Errorf("failed to generate ed25519 (stdout=%q,stderr=%q): %w", stdout, stderr, err)
	}

	return string(bytes.TrimSuffix(stdout, []byte("\n"))), nil
}

func (tn *ChainNode) GetNodeAccount(ctx context.Context) error {
	if tn.NodeAccount != nil {
		return nil // Already populated
	}

	bech32NodeAddr, err := tn.AccountKeyBech32(ctx, valKey)
	if err != nil {
		return err
	}

	validator, err := tn.GetValidatorConsPubKey(ctx)
	if err != nil {
		return err
	}

	version, err := tn.Version(ctx)
	if err != nil {
		return err
	}

	nodePubKey, err := tn.GetNodePubKey(ctx)
	if err != nil {
		return err
	}

	tn.log.Info("Generating Ed25519 key for node account", zap.String("mnemonic", tn.ValidatorMnemonic))

	nodePubKeyEd25519, err := tn.GenerateEd25519(ctx)
	if err != nil {
		return fmt.Errorf("val: %t, index: %d, mnemonic: %s: %w", tn.Validator, tn.Index, tn.ValidatorMnemonic, err)
	}

	tn.NodeAccount = &NodeAccount{
		NodeAddress:         bech32NodeAddr,
		Version:             version,
		IpAddress:           "192.168.0.10", // TODO: may need to populate real ip after chain start
		Status:              "Active",
		Bond:                "100000000", // 1 rune
		ActiveBlockHeight:   "0",
		BondAddress:         bech32NodeAddr,
		SignerMembership:    []string{},
		ValidatorConsPubKey: validator,
		PubKeySet: NodeAccountPubKeySet{
			Secp256k1: nodePubKey,
			Ed25519:   nodePubKeyEd25519,
		},
	}

	return nil
}

func (tn *ChainNode) AddNodeAccount(ctx context.Context, nodeAccount NodeAccount) error {
	genbz, err := tn.GenesisFileContent(ctx)
	if err != nil {
		return err
	}

	newNodeAccount := NewGenesisKV("app_state.thorchain.node_accounts", nodeAccount)

	g := make(map[string]interface{})
	if err := json.Unmarshal(genbz, &g); err != nil {
		return fmt.Errorf("failed to unmarshal genesis file: %w", err)
	}

	splitPath := strings.Split(newNodeAccount.Key, ".")

	path := make([]interface{}, len(splitPath))
	for i, component := range splitPath {
		if v, err := strconv.Atoi(component); err == nil {
			path[i] = v
		} else {
			path[i] = component
		}
	}

	if err := dyno.Append(g, newNodeAccount.Value, path...); err != nil {
		newNodeAccount.Value = []NodeAccount{nodeAccount}
		if err := dyno.Set(g, newNodeAccount.Value, path...); err != nil {
			return fmt.Errorf("failed to set key '%s' as '%+v' in genesis json: %w", newNodeAccount.Key, newNodeAccount.Value, err)
		}
	}

	genbz, err = json.Marshal(g)
	if err != nil {
		return fmt.Errorf("failed to marshal genesis bytes to json: %w", err)
	}

	if err := tn.OverwriteGenesisFile(ctx, genbz); err != nil {
		return err
	}

	return nil
}

type BaseAccount struct {
	AccountNumber string `json:"account_number"`
	Address       string `json:"address"`
	PubKey        []byte `json:"pub_key"`
	Sequence      string `json:"sequence"`
}

type ModuleAccount struct {
	Type        string      `json:"@type"`
	BaseAccount BaseAccount `json:"base_account"`
	Name        string      `json:"name"`
	Permissions []string    `json:"permissions"`
}

type State struct {
	Accounts []ModuleAccount `json:"accounts"`
}

type CoinBalance struct {
	Denom  string `json:"denom"`
	Amount string `json:"amount"`
}

type Balance struct {
	Address string        `json:"address"`
	Coins   []CoinBalance `json:"coins"`
}

func (tn *ChainNode) AddBondModule(ctx context.Context) error {
	genbz, err := tn.GenesisFileContent(ctx)
	if err != nil {
		return err
	}

	bondModule := ModuleAccount{
		Type: "/cosmos.auth.v1beta1.ModuleAccount",
		BaseAccount: BaseAccount{
			AccountNumber: "0",
			Address:       "tthor17gw75axcnr8747pkanye45pnrwk7p9c3uhzgff",
			PubKey:        nil,
			Sequence:      "0",
		},
		Name:        "bond",
		Permissions: []string{},
	}

	newModuleAccount := NewGenesisKV("app_state.state", State{Accounts: []ModuleAccount{bondModule}})

	g := make(map[string]interface{})
	if err := json.Unmarshal(genbz, &g); err != nil {
		return fmt.Errorf("failed to unmarshal genesis file: %w", err)
	}

	splitPath := strings.Split(newModuleAccount.Key, ".")

	path := make([]interface{}, len(splitPath))
	for i, component := range splitPath {
		if v, err := strconv.Atoi(component); err == nil {
			path[i] = v
		} else {
			path[i] = component
		}
	}

	if err := dyno.Set(g, newModuleAccount.Value, path...); err != nil {
		return fmt.Errorf("failed to set key '%s' as '%+v' in genesis json: %w", newModuleAccount.Key, newModuleAccount.Value, err)
	}

	bondBalance := Balance{
		Address: "tthor17gw75axcnr8747pkanye45pnrwk7p9c3uhzgff",
		Coins: []CoinBalance{
			{
				Denom:  "rune",
				Amount: fmt.Sprintf("%d0000000000000", tn.Chain.(*Thorchain).NumValidators),
			},
		},
	}
	newBondBalance := NewGenesisKV("app_state.bank.balances", bondBalance)

	splitPath = strings.Split(newBondBalance.Key, ".")

	path = make([]interface{}, len(splitPath))
	for i, component := range splitPath {
		if v, err := strconv.Atoi(component); err == nil {
			path[i] = v
		} else {
			path[i] = component
		}
	}

	if err := dyno.Append(g, newBondBalance.Value, path...); err != nil {
		newBondBalance.Value = []Balance{bondBalance}
		if err := dyno.Set(g, newBondBalance.Value, path...); err != nil {
			return fmt.Errorf("failed to set key '%s' as '%+v' in genesis json: %w", newBondBalance.Key, newBondBalance.Value, err)
		}
	}

	genbz, err = json.Marshal(g)
	if err != nil {
		return fmt.Errorf("failed to marshal genesis bytes to json: %w", err)
	}

	if err := tn.OverwriteGenesisFile(ctx, genbz); err != nil {
		return err
	}

	return nil
}

type CosmosTx struct {
	TxHash string `json:"txhash"`
	Code   int    `json:"code"`
	RawLog string `json:"raw_log"`
}

func (tn *ChainNode) GetTransaction(clientCtx client.Context, txHash string) (*sdk.TxResponse, error) {
	// Retry because sometimes the tx is not committed to state yet.
	var txResp *sdk.TxResponse
	err := retry.Do(func() error {
		var err error
		txResp, err = authTx.QueryTx(clientCtx, txHash)
		return err
	},
		// retry for total of 3 seconds
		retry.Attempts(15),
		retry.Delay(200*time.Millisecond),
		retry.DelayType(retry.FixedDelay),
		retry.LastErrorOnly(true),
	)
	return txResp, err
}

// HasCommand checks if a command in the chain binary is available.
func (tn *ChainNode) HasCommand(ctx context.Context, command ...string) bool {
	_, _, err := tn.ExecBin(ctx, false, command...)
	if err == nil {
		return true
	}

	if strings.Contains(string(err.Error()), "Error: unknown command") {
		return false
	}

	// cmd just needed more arguments, but it is a valid command (ex: appd tx bank send)
	if strings.Contains(string(err.Error()), "Error: accepts") {
		return true
	}

	return false
}

// GetBuildInformation returns the build information and dependencies for the chain binary.
func (tn *ChainNode) GetBuildInformation(ctx context.Context) *BinaryBuildInformation {
	stdout, _, err := tn.ExecBin(ctx, false, "version", "--long", "--output", "json")
	if err != nil {
		return nil
	}

	type tempBuildDeps struct {
		Name             string   `json:"name"`
		ServerName       string   `json:"server_name"`
		Version          string   `json:"version"`
		Commit           string   `json:"commit"`
		BuildTags        string   `json:"build_tags"`
		Go               string   `json:"go"`
		BuildDeps        []string `json:"build_deps"`
		CosmosSdkVersion string   `json:"cosmos_sdk_version"`
	}

	var deps tempBuildDeps
	if err := json.Unmarshal([]byte(stdout), &deps); err != nil {
		return nil
	}

	getRepoAndVersion := func(dep string) (string, string) {
		split := strings.Split(dep, "@")
		return split[0], split[1]
	}

	var buildDeps []BuildDependency
	for _, dep := range deps.BuildDeps {
		var bd BuildDependency

		if strings.Contains(dep, "=>") {
			// Ex: "github.com/aaa/bbb@v1.2.1 => github.com/ccc/bbb@v1.2.0"
			split := strings.Split(dep, " => ")
			main, replacement := split[0], split[1]

			parent, parentVersion := getRepoAndVersion(main)
			r, rV := getRepoAndVersion(replacement)

			bd = BuildDependency{
				Parent:             parent,
				Version:            parentVersion,
				IsReplacement:      true,
				Replacement:        r,
				ReplacementVersion: rV,
			}

		} else {
			// Ex: "github.com/aaa/bbb@v0.0.0-20191008050251-8e49817e8af4"
			parent, version := getRepoAndVersion(dep)

			bd = BuildDependency{
				Parent:             parent,
				Version:            version,
				IsReplacement:      false,
				Replacement:        "",
				ReplacementVersion: "",
			}
		}

		buildDeps = append(buildDeps, bd)
	}

	return &BinaryBuildInformation{
		BuildDeps:        buildDeps,
		Name:             deps.Name,
		ServerName:       deps.ServerName,
		Version:          deps.Version,
		Commit:           deps.Commit,
		BuildTags:        deps.BuildTags,
		Go:               deps.Go,
		CosmosSdkVersion: deps.CosmosSdkVersion,
	}
}

// QueryParam returns the state and details of a subspace param.
func (tn *ChainNode) QueryParam(ctx context.Context, subspace, key string) (*ParamChange, error) {
	stdout, _, err := tn.ExecQuery(ctx, "params", "subspace", subspace, key)
	if err != nil {
		return nil, err
	}
	var param ParamChange
	err = json.Unmarshal(stdout, &param)
	if err != nil {
		return nil, err
	}
	return &param, nil
}

// QueryBankMetadata returns the bank metadata of a token denomination.
func (tn *ChainNode) QueryBankMetadata(ctx context.Context, denom string) (*BankMetaData, error) {
	stdout, _, err := tn.ExecQuery(ctx, "bank", "denom-metadata", "--denom", denom)
	if err != nil {
		return nil, err
	}
	var meta BankMetaData
	err = json.Unmarshal(stdout, &meta)
	if err != nil {
		return nil, err
	}
	return &meta, nil
}

func (tn *ChainNode) ExportState(ctx context.Context, height int64) (string, error) {
	tn.lock.Lock()
	defer tn.lock.Unlock()

	var (
		doc              = "state_export.json"
		docPath          = path.Join(tn.HomeDir(), doc)
		isNewerThanSdk47 = tn.IsAboveSDK47(ctx)
		command          = []string{"export", "--height", fmt.Sprint(height), "--home", tn.HomeDir()}
	)

	if isNewerThanSdk47 {
		command = append(command, "--output-document", docPath)
	}

	stdout, stderr, err := tn.ExecBin(ctx, false, command...)
	if err != nil {
		return "", err
	}

	if isNewerThanSdk47 {
		content, err := tn.ReadFile(ctx, doc)
		if err != nil {
			return "", err
		}
		return string(content), nil
	}

	// output comes to stderr on older versions
	return string(stdout) + string(stderr), nil
}

func (tn *ChainNode) UnsafeResetAll(ctx context.Context) error {
	tn.lock.Lock()
	defer tn.lock.Unlock()

	command := []string{tn.Chain.Config().Bin}
	if tn.IsAboveSDK47(ctx) {
		command = append(command, "comet")
	}

	command = append(command, "unsafe-reset-all", "--home", tn.HomeDir())

	_, _, err := tn.Exec(ctx, command, tn.Chain.Config().Env)
	return err
}

func (tn *ChainNode) CreateNodeContainer(ctx context.Context) error {
	chainCfg := tn.Chain.Config()

	var cmd []string
	if chainCfg.NoHostMount {
		startCmd := fmt.Sprintf("cp -r %s %s_nomnt && %s start --home %s_nomnt", tn.HomeDir(), tn.HomeDir(), chainCfg.Bin, tn.HomeDir())
		if len(chainCfg.AdditionalStartArgs) > 0 {
			startCmd = fmt.Sprintf("%s %s", startCmd, chainCfg.AdditionalStartArgs)
		}
		cmd = []string{"sh", "-c", startCmd}
	} else {
		cmd = []string{chainCfg.Bin, "start", "--home", tn.HomeDir()}
		if len(chainCfg.AdditionalStartArgs) > 0 {
			cmd = append(cmd, chainCfg.AdditionalStartArgs...)
		}
	}

	if chainCfg.UsesCometMock() {
		abciAppAddr := fmt.Sprintf("tcp://%s:26658", tn.HostName())
		connectionMode := "grpc"

		cmd = append(cmd, "--with-tendermint=false", fmt.Sprintf("--transport=%s", connectionMode), fmt.Sprintf("--address=%s", abciAppAddr))

		blockTime := chainCfg.CometMock.BlockTimeMs
		if blockTime <= 0 {
			blockTime = 100
		}
		blockTimeFlag := fmt.Sprintf("--block-time=%d", blockTime)

		defaultListenAddr := fmt.Sprintf("tcp://0.0.0.0:%s", cometMockRawPort)
		genesisFile := path.Join(tn.HomeDir(), "config", "genesis.json")

		containerName := fmt.Sprintf("cometmock-%s-%d", tn.Name(), rand.Intn(50_000))
		tn.Sidecars = append(tn.Sidecars, &SidecarProcess{
			ProcessName:      containerName,
			validatorProcess: true,
			Image:            chainCfg.CometMock.Image,
			preStart:         true,
			startCmd:         []string{"cometmock", blockTimeFlag, abciAppAddr, genesisFile, defaultListenAddr, tn.HomeDir(), connectionMode},
			ports: nat.PortMap{
				nat.Port(cometMockRawPort): {},
			},
			Chain:              tn.Chain,
			TestName:           tn.TestName,
			VolumeName:         tn.VolumeName,
			DockerClient:       tn.DockerClient,
			NetworkID:          tn.NetworkID,
			Index:              tn.Index,
			homeDir:            tn.HomeDir(),
			log:                tn.log,
			env:                chainCfg.Env,
			containerLifecycle: dockerutil.NewContainerLifecycle(tn.log, tn.DockerClient, containerName),
		})
	}

	usingPorts := nat.PortMap{}
	for k, v := range sentryPorts {
		usingPorts[k] = v
	}
	for _, port := range chainCfg.ExposeAdditionalPorts {
		usingPorts[nat.Port(port)] = []nat.PortBinding{}
	}

	// to prevent port binding conflicts, host port overrides are only exposed on the first validator node.
	if tn.Validator && tn.Index == 0 && chainCfg.HostPortOverride != nil {
		for intP, extP := range chainCfg.HostPortOverride {
			usingPorts[nat.Port(fmt.Sprintf("%d/tcp", intP))] = []nat.PortBinding{
				{
					HostPort: fmt.Sprintf("%d", extP),
				},
			}
		}

		fmt.Printf("Port Overrides: %v. Using: %v\n", chainCfg.HostPortOverride, usingPorts)
	}

	env := chainCfg.Env
	if tn.Validator {
		env = append(env, fmt.Sprintf("NODES=%d", tn.Chain.(*Thorchain).NumValidators))
		env = append(env, fmt.Sprintf("SIGNER_SEED_PHRASE=%s", tn.ValidatorMnemonic))
	}

	return tn.containerLifecycle.CreateContainer(ctx, tn.TestName, tn.NetworkID, tn.Image, usingPorts, tn.Bind(), nil, tn.HostName(), cmd, env, []string{})
}

func (tn *ChainNode) StartContainer(ctx context.Context) error {
	rpcOverrideAddr := ""

	for _, s := range tn.Sidecars {
		err := s.containerLifecycle.Running(ctx)

		if s.preStart && err != nil {
			if err := s.CreateContainer(ctx, tn.Bind()); err != nil {
				return err
			}

			if err := s.StartContainer(ctx); err != nil {
				return err
			}

			if s.Image.Repository == tn.Chain.Config().CometMock.Image.Repository {
				hostPorts, err := s.containerLifecycle.GetHostPorts(ctx, cometMockRawPort+"/tcp")
				if err != nil {
					return err
				}

				rpcOverrideAddr = hostPorts[0]
				tn.cometHostname = s.HostName()

				tn.log.Info(
					"Using comet mock as RPC override",
					zap.String("RPC host port override", rpcOverrideAddr),
					zap.String("comet mock hostname", tn.cometHostname),
				)
			}
		}
	}

	if tn.preStartNode != nil {
		tn.preStartNode(tn)
	}

	if err := tn.containerLifecycle.StartContainer(ctx); err != nil {
		return err
	}

	// Set the host ports once since they will not change after the container has started.
	hostPorts, err := tn.containerLifecycle.GetHostPorts(ctx, rpcPort, grpcPort, apiPort, p2pPort)
	if err != nil {
		return err
	}
	tn.hostRPCPort, tn.hostGRPCPort, tn.hostAPIPort, tn.hostP2PPort = hostPorts[0], hostPorts[1], hostPorts[2], hostPorts[3]

	// Override the default RPC behavior if Comet Mock is being used.
	if tn.cometHostname != "" {
		tn.hostRPCPort = rpcOverrideAddr
	}

	err = tn.NewClient("tcp://" + tn.hostRPCPort)
	if err != nil {
		return err
	}

	time.Sleep(5 * time.Second)
	return retry.Do(func() error {
		stat, err := tn.Client.Status(ctx)
		if err != nil {
			return err
		}
		// TODO: re-enable this check, having trouble with it for some reason
		if stat != nil && stat.SyncInfo.CatchingUp {
			return fmt.Errorf("still catching up: height(%d) catching-up(%t)",
				stat.SyncInfo.LatestBlockHeight, stat.SyncInfo.CatchingUp)
		}
		return nil
	}, retry.Context(ctx), retry.Attempts(40), retry.Delay(3*time.Second), retry.DelayType(retry.FixedDelay))
}

func (tn *ChainNode) PauseContainer(ctx context.Context) error {
	for _, s := range tn.Sidecars {
		if err := s.PauseContainer(ctx); err != nil {
			return err
		}
	}
	return tn.containerLifecycle.PauseContainer(ctx)
}

func (tn *ChainNode) UnpauseContainer(ctx context.Context) error {
	for _, s := range tn.Sidecars {
		if err := s.UnpauseContainer(ctx); err != nil {
			return err
		}
	}
	return tn.containerLifecycle.UnpauseContainer(ctx)
}

func (tn *ChainNode) StopContainer(ctx context.Context) error {
	for _, s := range tn.Sidecars {
		if err := s.StopContainer(ctx); err != nil {
			return err
		}
	}
	return tn.containerLifecycle.StopContainer(ctx)
}

func (tn *ChainNode) RemoveContainer(ctx context.Context) error {
	for _, s := range tn.Sidecars {
		if err := s.RemoveContainer(ctx); err != nil {
			return err
		}
	}
	return tn.containerLifecycle.RemoveContainer(ctx)
}

// InitValidatorFiles creates the node files and signs a genesis transaction
func (tn *ChainNode) InitValidatorGenTx(
	ctx context.Context,
	chainType *ibc.ChainConfig,
	genesisAmounts []sdk.Coin,
	genesisSelfDelegation sdk.Coin,
) error {
	//if err := tn.CreateKey(ctx, valKey); err != nil {
	//	return err
	//}
	// Thorchain will only start with 1 validator
	// it must use this mnemonic since the router contracts are created using it (before this chain starts)
	// Otherwise, there is nothing special amoun this mnemonic other than it is what Thornode's sim testing uses.
	// TODO: can we deploy router contracts after thorchain start?

	bech32NodeAddr, err := tn.AccountKeyBech32(ctx, valKey)
	if err != nil {
		return err
	}
	if err := tn.AddGenesisAccount(ctx, bech32NodeAddr, genesisAmounts); err != nil {
		return err
	}
	return nil
	//return tn.Gentx(ctx, valKey, genesisSelfDelegation)
}

func (tn *ChainNode) InitFullNodeFiles(ctx context.Context) error {
	if err := tn.InitHomeFolder(ctx); err != nil {
		return err
	}

	return tn.SetTestConfig(ctx)
}

// NodeID returns the persistent ID of a given node.
func (tn *ChainNode) NodeID(ctx context.Context) (string, error) {
	// This used to call p2p.LoadNodeKey against the file on the host,
	// but because we are transitioning to operating on Docker volumes,
	// we only have to tmjson.Unmarshal the raw content.
	j, err := tn.ReadFile(ctx, "config/node_key.json")
	if err != nil {
		return "", fmt.Errorf("getting node_key.json content: %w", err)
	}

	var nk p2p.NodeKey
	if err := tmjson.Unmarshal(j, &nk); err != nil {
		return "", fmt.Errorf("unmarshaling node_key.json: %w", err)
	}

	return string(nk.ID()), nil
}

// KeyBech32 retrieves the named key's address in bech32 format from the node.
// bech is the bech32 prefix (acc|val|cons). If empty, defaults to the account key (same as "acc").
func (tn *ChainNode) KeyBech32(ctx context.Context, name string, bech string) (string, error) {
	command := []string{
		"keys", "show", "--address", name,
	}

	if bech != "" {
		command = append(command, "--bech", bech)
	}

	stdout, stderr, err := tn.ExecBin(ctx, true, command...)
	if err != nil {
		return "", fmt.Errorf("failed to show key %q (stderr=%q): %w", name, stderr, err)
	}

	return string(bytes.TrimSuffix(stdout, []byte("\n"))), nil
}

// AccountKeyBech32 retrieves the named key's address in bech32 account format.
func (tn *ChainNode) AccountKeyBech32(ctx context.Context, name string) (string, error) {
	return tn.KeyBech32(ctx, name, "")
}

// PeerString returns the string for connecting the nodes passed in
func (nodes ChainNodes) PeerString(ctx context.Context) string {
	addrs := make([]string, len(nodes))
	for i, n := range nodes {
		id, err := n.NodeID(ctx)
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

func (nodes ChainNodes) SidecarBifrostPeers() string {
	addrs := make([]string, len(nodes))
	for i, n := range nodes {
		for _, s := range n.Sidecars {
			if s.startCmd[0] == "bifrost" {
				addrs[i] = s.HostName()
			}
		}
	}
	return strings.Join(addrs, ",")
}

// LogGenesisHashes logs the genesis hashes for the various nodes
func (nodes ChainNodes) LogGenesisHashes(ctx context.Context) error {
	for _, n := range nodes {
		gen, err := n.GenesisFileContent(ctx)
		if err != nil {
			return err
		}

		n.logger().Info("Genesis", zap.String("hash", fmt.Sprintf("%X", sha256.Sum256(gen))))
	}
	return nil
}

func (nodes ChainNodes) logger() *zap.Logger {
	if len(nodes) == 0 {
		return zap.NewNop()
	}
	return nodes[0].logger()
}

func (tn *ChainNode) Exec(ctx context.Context, cmd []string, env []string) ([]byte, []byte, error) {
	job := dockerutil.NewImage(tn.logger(), tn.DockerClient, tn.NetworkID, tn.TestName, tn.Image.Repository, tn.Image.Version)
	opts := dockerutil.ContainerOptions{
		Env:   env,
		Binds: tn.Bind(),
	}
	res := job.Run(ctx, cmd, opts)
	return res.Stdout, res.Stderr, res.Err
}

func (tn *ChainNode) logger() *zap.Logger {
	return tn.log.With(
		zap.String("chain_id", tn.Chain.Config().ChainID),
		zap.String("test", tn.TestName),
	)
}

// QueryICA will query for an interchain account controlled by the specified address on the counterparty chain.
func (tn *ChainNode) QueryICA(ctx context.Context, connectionID, address string) (string, error) {
	stdout, _, err := tn.ExecQuery(ctx,
		"interchain-accounts", "controller", "interchain-account", address, connectionID,
	)
	if err != nil {
		return "", err
	}

	// at this point stdout should look like this:
	// address: cosmos1p76n3mnanllea4d3av0v0e42tjj03cae06xq8fwn9at587rqp23qvxsv0j
	// we split the string at the : and then just grab the address before returning.
	parts := strings.SplitN(string(stdout), ":", 2)
	if len(parts) < 2 {
		return "", fmt.Errorf("malformed stdout from command: %s", stdout)
	}
	return strings.TrimSpace(parts[1]), nil
}

// GetHostAddress returns the host-accessible url for a port in the container.
// This is useful for finding the url & random host port for ports exposed via ChainConfig.ExposeAdditionalPorts
func (tn *ChainNode) GetHostAddress(ctx context.Context, portID string) (string, error) {
	ports, err := tn.containerLifecycle.GetHostPorts(ctx, portID)
	if err != nil {
		return "", err
	}
	if len(ports) == 0 || ports[0] == "" {
		return "", fmt.Errorf("no port with id '%s' found", portID)
	}
	return "http://" + ports[0], nil
}
