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
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	dockertypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	dockerclient "github.com/docker/docker/client"
	"github.com/docker/docker/errdefs"
	"github.com/docker/go-connections/nat"
	"github.com/strangelove-ventures/ibctest/v5/ibc"
	"github.com/strangelove-ventures/ibctest/v5/internal/blockdb"
	"github.com/strangelove-ventures/ibctest/v5/internal/configutil"
	"github.com/strangelove-ventures/ibctest/v5/internal/dockerutil"
	"github.com/strangelove-ventures/ibctest/v5/test"
	tmjson "github.com/tendermint/tendermint/libs/json"
	"github.com/tendermint/tendermint/p2p"
	rpcclient "github.com/tendermint/tendermint/rpc/client"
	rpchttp "github.com/tendermint/tendermint/rpc/client/http"
	coretypes "github.com/tendermint/tendermint/rpc/core/types"
	libclient "github.com/tendermint/tendermint/rpc/jsonrpc/client"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

// ChainNode represents a node in the test network that is being created
type ChainNode struct {
	VolumeName   string
	Index        int
	Chain        ibc.Chain
	Validator    bool
	NetworkID    string
	DockerClient *dockerclient.Client
	Client       rpcclient.Client
	TestName     string
	Image        ibc.DockerImage

	lock sync.Mutex
	log  *zap.Logger

	containerID string

	// Ports set during StartContainer.
	hostRPCPort  string
	hostGRPCPort string
}

// ChainNodes is a collection of ChainNode
type ChainNodes []*ChainNode

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
	sentryPorts = nat.PortSet{
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
	return nil
}

// CliContext creates a new Cosmos SDK client context
func (tn *ChainNode) CliContext() client.Context {
	cfg := tn.Chain.Config()
	return client.Context{
		Client:            tn.Client,
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
	var nodeType string
	if tn.Validator {
		nodeType = "val"
	} else {
		nodeType = "fn"
	}
	return fmt.Sprintf("%s-%s-%d-%s", tn.Chain.Config().ChainID, nodeType, tn.Index, dockerutil.SanitizeContainerName(tn.TestName))
}

// hostname of the test node container
func (tn *ChainNode) HostName() string {
	return dockerutil.CondenseHostName(tn.Name())
}

func (tn *ChainNode) genesisFileContent(ctx context.Context) ([]byte, error) {
	fr := dockerutil.NewFileRetriever(tn.logger(), tn.DockerClient, tn.TestName)
	gen, err := fr.SingleFileContent(ctx, tn.VolumeName, "config/genesis.json")
	if err != nil {
		return nil, fmt.Errorf("getting genesis.json content: %w", err)
	}

	return gen, nil
}

func (tn *ChainNode) overwriteGenesisFile(ctx context.Context, content []byte) error {
	fw := dockerutil.NewFileWriter(tn.logger(), tn.DockerClient, tn.TestName)
	if err := fw.WriteFile(ctx, tn.VolumeName, "config/genesis.json", content); err != nil {
		return fmt.Errorf("overwriting genesis.json: %w", err)
	}

	return nil
}

func (tn *ChainNode) copyGentx(ctx context.Context, destVal *ChainNode) error {
	nid, err := tn.NodeID(ctx)
	if err != nil {
		return fmt.Errorf("getting node ID: %w", err)
	}

	relPath := fmt.Sprintf("config/gentx/gentx-%s.json", nid)

	fr := dockerutil.NewFileRetriever(tn.logger(), tn.DockerClient, tn.TestName)
	gentx, err := fr.SingleFileContent(ctx, tn.VolumeName, relPath)
	if err != nil {
		return fmt.Errorf("getting gentx content: %w", err)
	}

	fw := dockerutil.NewFileWriter(destVal.logger(), destVal.DockerClient, destVal.TestName)
	if err := fw.WriteFile(ctx, destVal.VolumeName, relPath, gentx); err != nil {
		return fmt.Errorf("overwriting gentx: %w", err)
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
	return path.Join("/var/cosmos-chain", tn.Chain.Config().Name)
}

// SetTestConfig modifies the config to reasonable values for use within ibctest.
func (tn *ChainNode) SetTestConfig(ctx context.Context) error {
	c := make(configutil.Toml)

	// Set Log Level to info
	c["log_level"] = "info"

	p2p := make(configutil.Toml)

	// Allow p2p strangeness
	p2p["allow_duplicate_ip"] = true
	p2p["addr_book_strict"] = false

	c["p2p"] = p2p

	consensus := make(configutil.Toml)

	blockT := (time.Duration(blockTime) * time.Second).String()
	consensus["timeout_commit"] = blockT
	consensus["timeout_propose"] = blockT

	c["consensus"] = consensus

	rpc := make(configutil.Toml)

	// Enable public RPC
	rpc["laddr"] = "tcp://0.0.0.0:26657"

	c["rpc"] = rpc

	return configutil.ModifyTomlConfigFile(
		ctx,
		tn.logger(),
		tn.DockerClient,
		tn.TestName,
		tn.VolumeName,
		"config/config.toml",
		c,
	)
}

// SetPeers modifies the config persistent_peers for a node
func (tn *ChainNode) SetPeers(ctx context.Context, peers string) error {
	c := make(configutil.Toml)
	p2p := make(configutil.Toml)

	// Set peers
	p2p["persistent_peers"] = peers
	c["p2p"] = p2p

	return configutil.ModifyTomlConfigFile(
		ctx,
		tn.logger(),
		tn.DockerClient,
		tn.TestName,
		tn.VolumeName,
		"config/config.toml",
		c,
	)
}

func (tn *ChainNode) Height(ctx context.Context) (uint64, error) {
	res, err := tn.Client.Status(ctx)
	if err != nil {
		return 0, fmt.Errorf("tendermint rpc client status: %w", err)
	}
	height := res.SyncInfo.LatestBlockHeight
	return uint64(height), nil
}

// FindTxs implements blockdb.BlockSaver.
func (tn *ChainNode) FindTxs(ctx context.Context, height uint64) ([]blockdb.Tx, error) {
	h := int64(height)
	var eg errgroup.Group
	var blockRes *coretypes.ResultBlockResults
	var block *coretypes.ResultBlock
	eg.Go(func() (err error) {
		blockRes, err = tn.Client.BlockResults(ctx, &h)
		return err
	})
	eg.Go(func() (err error) {
		block, err = tn.Client.Block(ctx, &h)
		return err
	})
	if err := eg.Wait(); err != nil {
		return nil, err
	}
	interfaceRegistry := tn.Chain.Config().EncodingConfig.InterfaceRegistry
	txs := make([]blockdb.Tx, 0, len(block.Block.Txs)+2)
	for i, tx := range block.Block.Txs {
		var newTx blockdb.Tx
		newTx.Data = []byte(fmt.Sprintf(`{"data":"%s"}`, hex.EncodeToString(tx)))

		sdkTx, err := decodeTX(interfaceRegistry, tx)
		if err != nil {
			tn.logger().Info("Failed to decode tx", zap.Uint64("height", height), zap.Error(err))
			continue
		}
		b, err := encodeTxToJSON(interfaceRegistry, sdkTx)
		if err != nil {
			tn.logger().Info("Failed to marshal tx to json", zap.Uint64("height", height), zap.Error(err))
			continue
		}
		newTx.Data = b

		rTx := blockRes.TxsResults[i]

		newTx.Events = make([]blockdb.Event, len(rTx.Events))
		for j, e := range rTx.Events {
			attrs := make([]blockdb.EventAttribute, len(e.Attributes))
			for k, attr := range e.Attributes {
				attrs[k] = blockdb.EventAttribute{
					Key:   string(attr.Key),
					Value: string(attr.Value),
				}
			}
			newTx.Events[j] = blockdb.Event{
				Type:       e.Type,
				Attributes: attrs,
			}
		}
		txs = append(txs, newTx)
	}
	if len(blockRes.BeginBlockEvents) > 0 {
		beginBlockTx := blockdb.Tx{
			Data: []byte(`{"data":"begin_block","note":"this is a transaction artificially created for debugging purposes"}`),
		}
		beginBlockTx.Events = make([]blockdb.Event, len(blockRes.BeginBlockEvents))
		for i, e := range blockRes.BeginBlockEvents {
			attrs := make([]blockdb.EventAttribute, len(e.Attributes))
			for j, attr := range e.Attributes {
				attrs[j] = blockdb.EventAttribute{
					Key:   string(attr.Key),
					Value: string(attr.Value),
				}
			}
			beginBlockTx.Events[i] = blockdb.Event{
				Type:       e.Type,
				Attributes: attrs,
			}
		}
		txs = append(txs, beginBlockTx)
	}
	if len(blockRes.EndBlockEvents) > 0 {
		endBlockTx := blockdb.Tx{
			Data: []byte(`{"data":"end_block","note":"this is a transaction artificially created for debugging purposes"}`),
		}
		endBlockTx.Events = make([]blockdb.Event, len(blockRes.EndBlockEvents))
		for i, e := range blockRes.EndBlockEvents {
			attrs := make([]blockdb.EventAttribute, len(e.Attributes))
			for j, attr := range e.Attributes {
				attrs[j] = blockdb.EventAttribute{
					Key:   string(attr.Key),
					Value: string(attr.Value),
				}
			}
			endBlockTx.Events[i] = blockdb.Event{
				Type:       e.Type,
				Attributes: attrs,
			}
		}
		txs = append(txs, endBlockTx)
	}

	return txs, nil
}

// TxCommand is a helper to retrieve a full command for broadcasting a tx
// with the chain node binary.
func (tn *ChainNode) TxCommand(keyName string, command ...string) []string {
	command = append([]string{"tx"}, command...)
	return tn.NodeCommand(append(command,
		"--from", keyName,
		"--gas-prices", tn.Chain.Config().GasPrices,
		"--gas-adjustment", fmt.Sprint(tn.Chain.Config().GasAdjustment),
		"--keyring-backend", keyring.BackendTest,
		"--output", "json",
		"-y",
	)...)
}

// ExecTx executes a transaction, waits for 2 blocks if successful, then returns the tx hash.
func (tn *ChainNode) ExecTx(ctx context.Context, keyName string, command ...string) (string, error) {
	tn.lock.Lock()
	defer tn.lock.Unlock()

	stdout, _, err := tn.Exec(ctx, tn.TxCommand(keyName, command...), nil)
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
	if err := test.WaitForBlocks(ctx, 2, tn); err != nil {
		return "", err
	}
	return output.TxHash, nil
}

// NodeCommand is a helper to retrieve a full command for a chain node binary.
// when interactions with the RPC endpoint are necessary.
// For example, if chain node binary is `gaiad`, and desired command is `gaiad keys show key1`,
// pass ("keys", "show", "key1") for command to return the full command.
// Will include additional flags for node URL, home directory, and chain ID.
func (tn *ChainNode) NodeCommand(command ...string) []string {
	command = tn.BinCommand(command...)
	return append(command,
		"--node", fmt.Sprintf("tcp://%s:26657", tn.HostName()),
		"--chain-id", tn.Chain.Config().ChainID,
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
func (tn *ChainNode) ExecBin(ctx context.Context, command ...string) ([]byte, []byte, error) {
	return tn.Exec(ctx, tn.BinCommand(command...), nil)
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
	return tn.Exec(ctx, tn.QueryCommand(command...), nil)
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

	_, _, err := tn.ExecBin(ctx,
		"init", CondenseMoniker(tn.Name()),
		"--chain-id", tn.Chain.Config().ChainID,
	)
	return err
}

// CreateKey creates a key in the keyring backend test for the given node
func (tn *ChainNode) CreateKey(ctx context.Context, name string) error {
	tn.lock.Lock()
	defer tn.lock.Unlock()

	_, _, err := tn.ExecBin(ctx,
		"keys", "add", name,
		"--keyring-backend", keyring.BackendTest,
	)
	return err
}

// RecoverKey restores a key from a given mnemonic.
func (tn *ChainNode) RecoverKey(ctx context.Context, keyName, mnemonic string) error {
	command := []string{
		"sh",
		"-c",
		fmt.Sprintf(`echo %q | %s keys add %s --recover --keyring-backend %s --home %s --output json`, mnemonic, tn.Chain.Config().Bin, keyName, keyring.BackendTest, tn.HomeDir()),
	}

	tn.lock.Lock()
	defer tn.lock.Unlock()

	_, _, err := tn.Exec(ctx, command, nil)
	return err
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

	tn.lock.Lock()
	defer tn.lock.Unlock()

	// Adding a genesis account should complete instantly,
	// so use a 1-minute timeout to more quickly detect if Docker has locked up.
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	_, _, err := tn.ExecBin(ctx, "add-genesis-account", address, amount)
	return err
}

// Gentx generates the gentx for a given node
func (tn *ChainNode) Gentx(ctx context.Context, name string, genesisSelfDelegation types.Coin) error {
	tn.lock.Lock()
	defer tn.lock.Unlock()

	_, _, err := tn.ExecBin(ctx,
		"gentx", valKey, fmt.Sprintf("%d%s", genesisSelfDelegation.Amount.Int64(), genesisSelfDelegation.Denom),
		"--keyring-backend", keyring.BackendTest,
		"--chain-id", tn.Chain.Config().ChainID,
	)
	return err
}

// CollectGentxs runs collect gentxs on the node's home folders
func (tn *ChainNode) CollectGentxs(ctx context.Context) error {
	command := []string{tn.Chain.Config().Bin, "collect-gentxs",
		"--home", tn.HomeDir(),
	}

	tn.lock.Lock()
	defer tn.lock.Unlock()

	_, _, err := tn.Exec(ctx, command, nil)
	return err
}

type CosmosTx struct {
	TxHash string `json:"txhash"`
	Code   int    `json:"code"`
	RawLog string `json:"raw_log"`
}

func (tn *ChainNode) SendIBCTransfer(ctx context.Context, channelID string, keyName string, amount ibc.WalletAmount, timeout *ibc.IBCTimeout) (string, error) {
	command := []string{
		"ibc-transfer", "transfer", "transfer", channelID,
		amount.Address, fmt.Sprintf("%d%s", amount.Amount, amount.Denom),
	}
	if timeout != nil {
		if timeout.NanoSeconds > 0 {
			command = append(command, "--packet-timeout-timestamp", fmt.Sprint(timeout.NanoSeconds))
		} else if timeout.Height > 0 {
			command = append(command, "--packet-timeout-height", fmt.Sprintf("0-%d", timeout.Height))
		}
	}
	return tn.ExecTx(ctx, keyName, command...)
}

func (tn *ChainNode) SendFunds(ctx context.Context, keyName string, amount ibc.WalletAmount) error {
	_, err := tn.ExecTx(ctx,
		keyName, "bank", "send", keyName,
		amount.Address, fmt.Sprintf("%d%s", amount.Amount, amount.Denom),
	)
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

func (tn *ChainNode) InstantiateContract(
    ctx context.Context,
	keyName string,
	amount ibc.WalletAmount,
	fileName,
	initMessage string,
	needsNoAdminFlag bool,
) (QueryContractInfoResponse, error) {
	content, err := os.ReadFile(fileName)
	if err != nil {
		return QueryContractInfoResponse{}, err
	}

	_, file := filepath.Split(fileName)
	fw := dockerutil.NewFileWriter(tn.logger(), tn.DockerClient, tn.TestName)
	if err := fw.WriteFile(ctx, tn.VolumeName, file, content); err != nil {
		return QueryContractInfoResponse{}, fmt.Errorf("writing contract file to docker volume: %w", err)
	}

	if _, err := tn.ExecTx(ctx, "wasm", "store", path.Join(tn.HomeDir(), file), "--gas", fmt.Sprintf("%d", amount.Amount)); err != nil {
		return QueryContractInfoResponse{}, err
	}

	err = test.WaitForBlocks(ctx, 5, tn)
	if err != nil {
		return QueryContractInfoResponse{}, fmt.Errorf("wait for blocks: %w", err)
	}

	stdout, _, err := tn.ExecQuery(ctx, "wasm", "list-code", "--reverse")
	if err != nil {
		return QueryContractInfoResponse{}, err
	}
        tn.logger().Info(string(stdout))

	res := CodeInfosResponse{}
	if err := json.Unmarshal([]byte(stdout), &res); err != nil {
		return QueryContractInfoResponse{}, err
	}

	codeID := res.CodeInfos[0].CodeID
	command := []string{"wasm", "instantiate", codeID, initMessage}
	if needsNoAdminFlag {
		command = append(command, "--no-admin")
	}
	_, err = tn.ExecTx(ctx, keyName, command...)
	if err != nil {
		return QueryContractInfoResponse{}, err
	}

	stdout, _, err = tn.ExecQuery(ctx, "wasm", "list-contract-by-code", codeID)
	if err != nil {
		return QueryContractInfoResponse{}, err
	}

	contractsRes := QueryContractResponse{}
	if err := json.Unmarshal([]byte(stdout), &contractsRes); err != nil {
		return QueryContractInfoResponse{}, err
	}

	contractAddress := contractsRes.Contracts[len(contractsRes.Contracts)-1]

	command = []string{tn.Chain.Config().Bin,
		"query", "wasm", "contract", contractAddress,
		"--node", fmt.Sprintf("tcp://%s:26657", tn.HostName()),
		"--output", "json",
		"--home", tn.HomeDir(),
		"--chain-id", tn.Chain.Config().ChainID,
	}

	stdout, _, err = tn.Exec(ctx, command, nil)
	if err != nil {
		return QueryContractInfoResponse{}, err
	}
        tn.logger().Info(string(stdout))

        contractInfoRes := QueryContractInfoResponse{}
	if err := json.Unmarshal([]byte(stdout), &contractInfoRes); err != nil {
		return QueryContractInfoResponse{}, err
	}

	return contractInfoRes, nil
}

func (tn *ChainNode) ExecuteContract(ctx context.Context, keyName string, contractAddress string, message string) error {
	_, err := tn.ExecTx(ctx, keyName,
		"wasm", "execute", contractAddress, message,
	)
	return err
}

// VoteOnProposal submits a vote for the specified proposal.
func (tn *ChainNode) VoteOnProposal(ctx context.Context, keyName string, proposalID string, vote string) error {
	_, err := tn.ExecTx(ctx, keyName,
		"gov", "vote",
		proposalID, vote,
	)
	return err
}

// QueryProposal returns the state and details of a governance proposal.
func (tn *ChainNode) QueryProposal(ctx context.Context, proposalID string) (*ProposalResponse, error) {
	stdout, _, err := tn.ExecQuery(ctx, "gov", "proposal", proposalID)
	if err != nil {
		return nil, err
	}
	var proposal ProposalResponse
	err = json.Unmarshal(stdout, &proposal)
	if err != nil {
		return nil, err
	}
	return &proposal, nil
}

// UpgradeProposal submits a software-upgrade governance proposal to the chain.
func (tn *ChainNode) UpgradeProposal(ctx context.Context, keyName string, prop SoftwareUpgradeProposal) (string, error) {
	command := []string{
		"gov", "submit-proposal",
		"software-upgrade", prop.Name,
		"--upgrade-height", strconv.FormatUint(prop.Height, 10),
		"--title", prop.Title,
		"--description", prop.Description,
		"--deposit", prop.Deposit,
	}

	if prop.Info != "" {
		command = append(command, "--upgrade-info", prop.Info)
	}

	return tn.ExecTx(ctx, keyName, command...)
}

// TextProposal submits a text governance proposal to the chain.
func (tn *ChainNode) TextProposal(ctx context.Context, keyName string, prop TextProposal) (string, error) {
	command := []string{
		"gov", "submit-proposal",
		"--type", "text",
		"--title", prop.Title,
		"--description", prop.Description,
		"--deposit", prop.Deposit,
	}
	if prop.Expedited {
		command = append(command, "--is-expedited=true")
	}
	return tn.ExecTx(ctx, keyName, command...)
}

// DumpContractState dumps the state of a contract at a block height.
func (tn *ChainNode) DumpContractState(ctx context.Context, contractAddress string, height int64) (*DumpContractStateResponse, error) {
	stdout, _, err := tn.ExecQuery(ctx,
		"wasm", "contract-state", "all", contractAddress,
		"--height", fmt.Sprint(height),
	)
	if err != nil {
		return nil, err
	}

	res := new(DumpContractStateResponse)
	if err := json.Unmarshal([]byte(stdout), res); err != nil {
		return nil, err
	}
	return res, nil
}

func (tn *ChainNode) ExportState(ctx context.Context, height int64) (string, error) {
	tn.lock.Lock()
	defer tn.lock.Unlock()

	_, stderr, err := tn.ExecBin(ctx, "export", "--height", fmt.Sprint(height))
	if err != nil {
		return "", err
	}
	// output comes to stderr for some reason
	return string(stderr), nil
}

func (tn *ChainNode) UnsafeResetAll(ctx context.Context) error {
	tn.lock.Lock()
	defer tn.lock.Unlock()

	_, _, err := tn.ExecBin(ctx, "unsafe-reset-all")
	return err
}

func (tn *ChainNode) CreateNodeContainer(ctx context.Context) error {
	chainCfg := tn.Chain.Config()
	cmd := []string{chainCfg.Bin, "start", "--home", tn.HomeDir(), "--x-crisis-skip-assert-invariants"}
	if chainCfg.NoHostMount {
		cmd = []string{"sh", "-c", fmt.Sprintf("cp -r %s %s_nomnt && %s start --home %s_nomnt --x-crisis-skip-assert-invariants", tn.HomeDir(), tn.HomeDir(), chainCfg.Bin, tn.HomeDir())}
	}
	imageRef := tn.Image.Ref()
	tn.logger().
		Info("Running command",
			zap.String("command", strings.Join(cmd, " ")),
			zap.String("container", tn.Name()),
			zap.String("image", imageRef),
		)

	cc, err := tn.DockerClient.ContainerCreate(
		ctx,
		&container.Config{
			Image: imageRef,

			Entrypoint: []string{},
			Cmd:        cmd,

			Hostname: tn.HostName(),

			Labels: map[string]string{dockerutil.CleanupLabel: tn.TestName},

			ExposedPorts: sentryPorts,
		},
		&container.HostConfig{
			Binds:           tn.Bind(),
			PublishAllPorts: true,
			AutoRemove:      false,
			DNS:             []string{},
		},
		&network.NetworkingConfig{
			EndpointsConfig: map[string]*network.EndpointSettings{
				tn.NetworkID: {},
			},
		},
		nil,
		tn.Name(),
	)
	if err != nil {
		return err
	}
	tn.containerID = cc.ID
	return nil
}

func (tn *ChainNode) StartContainer(ctx context.Context) error {
	if err := dockerutil.StartContainer(ctx, tn.DockerClient, tn.containerID); err != nil {
		return err
	}

	c, err := tn.DockerClient.ContainerInspect(ctx, tn.containerID)
	if err != nil {
		return err
	}

	// Set the host ports once since they will not change after the container has started.
	tn.hostRPCPort = dockerutil.GetHostPort(c, rpcPort)
	tn.hostGRPCPort = dockerutil.GetHostPort(c, grpcPort)

	tn.logger().Info("Cosmos chain node started", zap.String("container", tn.Name()), zap.String("rpc_port", tn.hostRPCPort))

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
		// TODO: reenable this check, having trouble with it for some reason
		if stat != nil && stat.SyncInfo.CatchingUp {
			return fmt.Errorf("still catching up: height(%d) catching-up(%t)",
				stat.SyncInfo.LatestBlockHeight, stat.SyncInfo.CatchingUp)
		}
		return nil
	}, retry.Context(ctx), retry.Attempts(40), retry.Delay(3*time.Second), retry.DelayType(retry.FixedDelay))
}

func (tn *ChainNode) StopContainer(ctx context.Context) error {
	timeout := 30 * time.Second
	return tn.DockerClient.ContainerStop(ctx, tn.containerID, &timeout)
}

func (tn *ChainNode) RemoveContainer(ctx context.Context) error {
	err := tn.DockerClient.ContainerRemove(ctx, tn.containerID, dockertypes.ContainerRemoveOptions{
		Force:         true,
		RemoveVolumes: true,
	})
	if err != nil && !errdefs.IsNotFound(err) {
		return fmt.Errorf("remove container %s: %w", tn.Name(), err)
	}
	return nil
}

// InitValidatorFiles creates the node files and signs a genesis transaction
func (tn *ChainNode) InitValidatorGenTx(
	ctx context.Context,
	chainType *ibc.ChainConfig,
	genesisAmounts []types.Coin,
	genesisSelfDelegation types.Coin,
) error {
	if err := tn.CreateKey(ctx, valKey); err != nil {
		return err
	}
	bech32, err := tn.KeyBech32(ctx, valKey)
	if err != nil {
		return err
	}
	if err := tn.AddGenesisAccount(ctx, bech32, genesisAmounts); err != nil {
		return err
	}
	return tn.Gentx(ctx, valKey, genesisSelfDelegation)
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

	fr := dockerutil.NewFileRetriever(tn.logger(), tn.DockerClient, tn.TestName)
	j, err := fr.SingleFileContent(ctx, tn.VolumeName, "config/node_key.json")
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
func (tn *ChainNode) KeyBech32(ctx context.Context, name string) (string, error) {
	command := []string{tn.Chain.Config().Bin, "keys", "show", "--address", name,
		"--home", tn.HomeDir(),
		"--keyring-backend", keyring.BackendTest,
	}

	stdout, stderr, err := tn.Exec(ctx, command, nil)
	if err != nil {
		return "", fmt.Errorf("failed to show key %q (stderr=%q): %w", name, stderr, err)
	}

	return string(bytes.TrimSuffix(stdout, []byte("\n"))), nil
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

// LogGenesisHashes logs the genesis hashes for the various nodes
func (nodes ChainNodes) LogGenesisHashes(ctx context.Context) error {
	for _, n := range nodes {
		gen, err := n.genesisFileContent(ctx)
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

// RegisterICA will attempt to register an interchain account on the counterparty chain.
func (tn *ChainNode) RegisterICA(ctx context.Context, keyName, connectionID string) (string, error) {
	return tn.ExecTx(ctx, keyName,
		"intertx", "register",
		"--connection-id", connectionID,
	)
}

// QueryICA will query for an interchain account controlled by the specified address on the counterparty chain.
func (tn *ChainNode) QueryICA(ctx context.Context, connectionID, address string) (string, error) {
	stdout, _, err := tn.ExecQuery(ctx,
		"intertx", "interchainaccounts", connectionID, address,
	)
	if err != nil {
		return "", err
	}

	// at this point stdout should look like this:
	// interchain_account_address: cosmos1p76n3mnanllea4d3av0v0e42tjj03cae06xq8fwn9at587rqp23qvxsv0j
	// we split the string at the : and then just grab the address before returning.
	parts := strings.SplitN(string(stdout), ":", 2)
	if len(parts) < 2 {
		return "", fmt.Errorf("malformed stdout from command: %s", stdout)
	}
	return strings.TrimSpace(parts[1]), nil
}

// SendICABankTransfer builds a bank transfer message for a specified address and sends it to the specified
// interchain account.
func (tn *ChainNode) SendICABankTransfer(ctx context.Context, connectionID, fromAddr string, amount ibc.WalletAmount) error {
	msg, err := json.Marshal(map[string]any{
		"@type":        "/cosmos.bank.v1beta1.MsgSend",
		"from_address": fromAddr,
		"to_address":   amount.Address,
		"amount": []map[string]any{
			{
				"denom":  amount.Denom,
				"amount": amount.Amount,
			},
		},
	})
	if err != nil {
		return err
	}

	_, err = tn.ExecTx(ctx, fromAddr,
		"intertx", "submit", string(msg),
		"--connection-id", connectionID,
	)
	return err
}
