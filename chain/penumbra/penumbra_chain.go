package penumbra

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	cryptocodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/strangelove-ventures/interchaintest/v7/chain/internal/tendermint"
	"github.com/strangelove-ventures/interchaintest/v7/ibc"
	"github.com/strangelove-ventures/interchaintest/v7/internal/dockerutil"
	"github.com/strangelove-ventures/interchaintest/v7/testutil"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

type PenumbraChain struct {
	log           *zap.Logger
	testName      string
	cfg           ibc.ChainConfig
	numValidators int
	numFullNodes  int
	PenumbraNodes PenumbraNodes
	keyring       keyring.Keyring
}

type PenumbraValidatorDefinition struct {
	SequenceNumber int                              `json:"sequence_number" toml:"sequence_number"`
	Enabled        bool                             `json:"enabled" toml:"enabled"`
	Name           string                           `json:"name" toml:"name"`
	Website        string                           `json:"website" toml:"website"`
	Description    string                           `json:"description" toml:"description"`
	IdentityKey    string                           `json:"identity_key" toml:"identity_key"`
	GovernanceKey  string                           `json:"governance_key" toml:"governance_key"`
	ConsensusKey   PenumbraConsensusKey             `json:"consensus_key" toml:"consensus_key"`
	FundingStreams []PenumbraValidatorFundingStream `json:"funding_streams" toml:"funding_stream"`
}

type PenumbraConsensusKey struct {
	Type  string `json:"type" toml:"type"`
	Value string `json:"value" toml:"value"`
}

type PenumbraValidatorFundingStream struct {
	Address string `json:"address" toml:"address"`
	RateBPS int64  `json:"rate_bps" toml:"rate_bps"`
}

type PenumbraGenesisAppStateAllocation struct {
	Amount  int64  `json:"amount"`
	Denom   string `json:"denom"`
	Address string `json:"address"`
}

func NewPenumbraChain(log *zap.Logger, testName string, chainConfig ibc.ChainConfig, numValidators int, numFullNodes int) *PenumbraChain {
	registry := codectypes.NewInterfaceRegistry()
	cryptocodec.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	kr := keyring.NewInMemory(cdc)

	return &PenumbraChain{
		log:           log,
		testName:      testName,
		cfg:           chainConfig,
		numValidators: numValidators,
		numFullNodes:  numFullNodes,
		keyring:       kr,
	}
}

func (c *PenumbraChain) Acknowledgements(ctx context.Context, height uint64) ([]ibc.PacketAcknowledgement, error) {
	panic("implement me")
}

func (c *PenumbraChain) Timeouts(ctx context.Context, height uint64) ([]ibc.PacketTimeout, error) {
	panic("implement me")
}

// Implements Chain interface
func (c *PenumbraChain) Config() ibc.ChainConfig {
	return c.cfg
}

// Implements Chain interface
func (c *PenumbraChain) Initialize(ctx context.Context, testName string, cli *client.Client, networkID string) error {
	return c.initializeChainNodes(ctx, testName, cli, networkID)
}

// Exec implements chain interface.
func (c *PenumbraChain) Exec(ctx context.Context, cmd []string, env []string) (stdout, stderr []byte, err error) {
	return c.getFullNode().PenumbraAppNode.Exec(ctx, cmd, env)
}

func (c *PenumbraChain) getFullNode() PenumbraNode {
	if len(c.PenumbraNodes) > c.numValidators {
		// use first full node
		return c.PenumbraNodes[c.numValidators]
	}
	// use first validator
	return c.PenumbraNodes[0]
}

// Implements Chain interface
func (c *PenumbraChain) GetRPCAddress() string {
	return fmt.Sprintf("http://%s:26657", c.getFullNode().TendermintNode.HostName())
}

// Implements Chain interface
func (c *PenumbraChain) GetGRPCAddress() string {
	return fmt.Sprintf("%s:9090", c.getFullNode().TendermintNode.HostName())
}

// GetHostRPCAddress returns the address of the RPC server accessible by the host.
// This will not return a valid address until the chain has been started.
func (c *PenumbraChain) GetHostRPCAddress() string {
	return "http://" + c.getFullNode().PenumbraAppNode.hostRPCPort
}

// GetHostGRPCAddress returns the address of the gRPC server accessible by the host.
// This will not return a valid address until the chain has been started.
func (c *PenumbraChain) GetHostGRPCAddress() string {
	return c.getFullNode().PenumbraAppNode.hostGRPCPort
}

func (c *PenumbraChain) HomeDir() string {
	panic(errors.New("HomeDir not implemented yet"))
}

// Implements Chain interface
func (c *PenumbraChain) CreateKey(ctx context.Context, keyName string) error {
	return c.getFullNode().PenumbraAppNode.CreateKey(ctx, keyName)
}

func (c *PenumbraChain) RecoverKey(ctx context.Context, name, mnemonic string) error {
	return c.getFullNode().PenumbraAppNode.RecoverKey(ctx, name, mnemonic)
}

// Implements Chain interface
func (c *PenumbraChain) GetAddress(ctx context.Context, keyName string) ([]byte, error) {
	return c.getFullNode().PenumbraAppNode.GetAddress(ctx, keyName)
}

// BuildWallet will return a Penumbra wallet
// If mnemonic != "", it will restore using that mnemonic
// If mnemonic == "", it will create a new key
func (c *PenumbraChain) BuildWallet(ctx context.Context, keyName string, mnemonic string) (ibc.Wallet, error) {
	if mnemonic != "" {
		if err := c.RecoverKey(ctx, keyName, mnemonic); err != nil {
			return nil, fmt.Errorf("failed to recover key with name %q on chain %s: %w", keyName, c.cfg.Name, err)
		}
	} else {
		if err := c.CreateKey(ctx, keyName); err != nil {
			return nil, fmt.Errorf("failed to create key with name %q on chain %s: %w", keyName, c.cfg.Name, err)
		}
	}

	addrBytes, err := c.GetAddress(ctx, keyName)
	if err != nil {
		return nil, fmt.Errorf("failed to get account address for key %q on chain %s: %w", keyName, c.cfg.Name, err)
	}

	return NewWallet(keyName, addrBytes, mnemonic, c.cfg), nil
}

// BuildRelayerWallet will return a Penumbra wallet populated with the mnemonic so that the wallet can
// be restored in the relayer node using the mnemonic. After it is built, that address is included in
// genesis with some funds.
func (c *PenumbraChain) BuildRelayerWallet(ctx context.Context, keyName string) (ibc.Wallet, error) {
	coinType, err := strconv.ParseUint(c.cfg.CoinType, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid coin type: %w", err)
	}

	info, mnemonic, err := c.keyring.NewMnemonic(
		keyName,
		keyring.English,
		hd.CreateHDPath(uint32(coinType), 0, 0).String(),
		"", // Empty passphrase.
		hd.Secp256k1,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create mnemonic: %w", err)
	}

	addrBytes, err := info.GetAddress()
	if err != nil {
		return nil, fmt.Errorf("failed to get address: %w", err)
	}

	return NewWallet(keyName, addrBytes, mnemonic, c.cfg), nil
}

// Implements Chain interface
func (c *PenumbraChain) SendFunds(ctx context.Context, keyName string, amount ibc.WalletAmount) error {
	fn := c.getFullNode()
	if len(fn.PenumbraClientNodes) == 0 {
		return fmt.Errorf("no pclientd nodes on the fullnode for funds transfer")
	}
	return fn.PenumbraClientNodes[keyName].SendFunds(ctx, amount)
}

// Implements Chain interface
func (c *PenumbraChain) SendIBCTransfer(
	ctx context.Context,
	channelID string,
	keyName string,
	amount ibc.WalletAmount,
	options ibc.TransferOptions,
) (ibc.Tx, error) {
	fn := c.getFullNode()
	if len(fn.PenumbraClientNodes) == 0 {
		return ibc.Tx{}, fmt.Errorf("no pclientd nodes on the fullnode for ibc transfer")
	}
	return fn.PenumbraClientNodes[keyName].SendIBCTransfer(ctx, channelID, amount, options)
}

// Implements Chain interface
func (c *PenumbraChain) ExportState(ctx context.Context, height int64) (string, error) {
	panic("implement me")
}

func (c *PenumbraChain) Height(ctx context.Context) (uint64, error) {
	return c.getFullNode().TendermintNode.Height(ctx)
}

// Implements Chain interface
func (c *PenumbraChain) GetBalance(ctx context.Context, keyName string, denom string) (int64, error) {
	fn := c.getFullNode()
	if len(fn.PenumbraClientNodes) == 0 {
		return 0, fmt.Errorf("no pclientd nodes on the fullnode for balance check")
	}
	return fn.PenumbraClientNodes[keyName].GetBalance(ctx, denom)
}

// Implements Chain interface
func (c *PenumbraChain) GetGasFeesInNativeDenom(gasPaid int64) int64 {
	gasPrice, _ := strconv.ParseFloat(strings.Replace(c.cfg.GasPrices, c.cfg.Denom, "", 1), 64)
	fees := float64(gasPaid) * gasPrice
	return int64(fees)
}

// creates the test node objects required for bootstrapping tests
func (c *PenumbraChain) initializeChainNodes(
	ctx context.Context,
	testName string,
	cli *client.Client,
	networkID string,
) error {
	var penumbraNodes []PenumbraNode
	count := c.numValidators + c.numFullNodes
	chainCfg := c.Config()
	for _, image := range chainCfg.Images {
		rc, err := cli.ImagePull(
			ctx,
			image.Repository+":"+image.Version,
			types.ImagePullOptions{},
		)
		if err != nil {
			c.log.Error("Failed to pull image",
				zap.Error(err),
				zap.String("repository", image.Repository),
				zap.String("tag", image.Version),
			)
		} else {
			_, _ = io.Copy(io.Discard, rc)
			_ = rc.Close()
		}
	}
	for i := 0; i < count; i++ {
		pn, err := NewPenumbraNode(ctx, i, c, cli, networkID, testName, chainCfg.Images[0], chainCfg.Images[1])
		if err != nil {
			return err
		}
		penumbraNodes = append(penumbraNodes, pn)
	}
	c.PenumbraNodes = penumbraNodes

	return nil
}

type GenesisValidatorPubKey struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}
type GenesisValidators struct {
	Address string                 `json:"address"`
	Name    string                 `json:"name"`
	Power   string                 `json:"power"`
	PubKey  GenesisValidatorPubKey `json:"pub_key"`
}
type GenesisFile struct {
	Validators []GenesisValidators `json:"validators"`
}

type ValidatorWithIntPower struct {
	Address      string
	Power        int64
	PubKeyBase64 string
}

func (c *PenumbraChain) Start(testName string, ctx context.Context, additionalGenesisWallets ...ibc.WalletAmount) error {
	validators := c.PenumbraNodes[:c.numValidators]
	fullnodes := c.PenumbraNodes[c.numValidators:]

	chainCfg := c.Config()

	validatorDefinitions := make([]PenumbraValidatorDefinition, len(validators))
	allocations := make([]PenumbraGenesisAppStateAllocation, len(validators)*2)

	eg, egCtx := errgroup.WithContext(ctx)
	for i, v := range validators {
		v := v
		i := i
		eg.Go(func() error {
			if err := v.TendermintNode.InitValidatorFiles(egCtx); err != nil {
				return fmt.Errorf("error initializing validator files: %v", err)
			}
			fr := dockerutil.NewFileRetriever(c.log, v.TendermintNode.DockerClient, v.TendermintNode.TestName)
			privValKeyBytes, err := fr.SingleFileContent(egCtx, v.TendermintNode.VolumeName, "config/priv_validator_key.json")
			if err != nil {
				return fmt.Errorf("error reading tendermint privval key file: %v", err)
			}
			privValKey := tendermint.PrivValidatorKeyFile{}
			if err := json.Unmarshal(privValKeyBytes, &privValKey); err != nil {
				return fmt.Errorf("error unmarshaling tendermint privval key: %v", err)
			}
			if err := v.PenumbraAppNode.CreateKey(egCtx, valKey); err != nil {
				return fmt.Errorf("error generating wallet on penumbra node: %v", err)
			}
			if err := v.PenumbraAppNode.InitValidatorFile(egCtx, valKey); err != nil {
				return fmt.Errorf("error initializing validator template on penumbra node: %v", err)
			}

			// In all likelihood, the PenumbraAppNode and TendermintNode have the same DockerClient and TestName,
			// but instantiate a new FileRetriever to be defensive.
			fr = dockerutil.NewFileRetriever(c.log, v.PenumbraAppNode.DockerClient, v.PenumbraAppNode.TestName)
			validatorTemplateDefinitionFileBytes, err := fr.SingleFileContent(egCtx, v.PenumbraAppNode.VolumeName, "validator.toml")
			if err != nil {
				return fmt.Errorf("error reading validator definition template file: %v", err)
			}
			validatorTemplateDefinition := PenumbraValidatorDefinition{}
			if err := toml.Unmarshal(validatorTemplateDefinitionFileBytes, &validatorTemplateDefinition); err != nil {
				return fmt.Errorf("error unmarshaling validator definition template key: %v", err)
			}
			validatorTemplateDefinition.SequenceNumber = i
			validatorTemplateDefinition.Enabled = true
			validatorTemplateDefinition.ConsensusKey.Value = privValKey.PubKey.Value
			validatorTemplateDefinition.Name = fmt.Sprintf("validator-%d", i)
			validatorTemplateDefinition.Description = fmt.Sprintf("validator-%d description", i)
			validatorTemplateDefinition.Website = fmt.Sprintf("https://validator-%d", i)

			// Assign validatorDefinitions and allocations at fixed indices to avoid data races across the error group's goroutines.
			validatorDefinitions[i] = validatorTemplateDefinition

			// self delegation
			allocations[2*i] = PenumbraGenesisAppStateAllocation{
				Amount:  100_000_000_000,
				Denom:   fmt.Sprintf("udelegation_%s", validatorTemplateDefinition.IdentityKey),
				Address: validatorTemplateDefinition.FundingStreams[0].Address,
			}
			// liquid
			allocations[2*i+1] = PenumbraGenesisAppStateAllocation{
				Amount:  1_000_000_000_000,
				Denom:   chainCfg.Denom,
				Address: validatorTemplateDefinition.FundingStreams[0].Address,
			}

			return nil
		})
	}

	for _, wallet := range additionalGenesisWallets {
		allocations = append(allocations, PenumbraGenesisAppStateAllocation{
			Address: wallet.Address,
			Denom:   wallet.Denom,
			Amount:  wallet.Amount,
		})
	}

	for _, n := range fullnodes {
		n := n
		eg.Go(func() error { return n.TendermintNode.InitFullNodeFiles(egCtx) })
	}

	if err := eg.Wait(); err != nil {
		return fmt.Errorf("waiting to init full nodes' files: %w", err)
	}

	firstVal := c.PenumbraNodes[0]
	if err := firstVal.PenumbraAppNode.GenerateGenesisFile(ctx, chainCfg.ChainID, validatorDefinitions, allocations); err != nil {
		return fmt.Errorf("generating genesis file: %w", err)
	}

	// penumbra generate-testnet right now overwrites new validator keys
	eg, egCtx = errgroup.WithContext(ctx)
	for i, val := range c.PenumbraNodes[:c.numValidators] {
		i := i
		val := val
		// Use an errgroup to save some time doing many concurrent copies inside containers.
		eg.Go(func() error {
			firstValPrivKeyRelPath := fmt.Sprintf(".penumbra/testnet_data/node%d/tendermint/config/priv_validator_key.json", i)

			fr := dockerutil.NewFileRetriever(c.log, firstVal.PenumbraAppNode.DockerClient, firstVal.PenumbraAppNode.TestName)
			pk, err := fr.SingleFileContent(egCtx, firstVal.PenumbraAppNode.VolumeName, firstValPrivKeyRelPath)
			if err != nil {
				return fmt.Errorf("error getting validator private key content: %w", err)
			}

			fw := dockerutil.NewFileWriter(c.log, val.PenumbraAppNode.DockerClient, val.PenumbraAppNode.TestName)
			if err := fw.WriteFile(egCtx, val.TendermintNode.VolumeName, "config/priv_validator_key.json", pk); err != nil {
				return fmt.Errorf("overwriting priv_validator_key.json: %w", err)
			}

			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return err
	}

	return c.start(ctx)
}

// Bootstraps the chain and starts it from genesis
func (c *PenumbraChain) start(ctx context.Context) error {
	// Copy the penumbra genesis to all tendermint nodes.
	genesisContent, err := c.PenumbraNodes[0].PenumbraAppNode.genesisFileContent(ctx)
	if err != nil {
		return err
	}

	tendermintNodes := make([]*tendermint.TendermintNode, len(c.PenumbraNodes))
	for i, node := range c.PenumbraNodes {
		tendermintNodes[i] = node.TendermintNode
		if err := node.TendermintNode.OverwriteGenesisFile(ctx, genesisContent); err != nil {
			return err
		}
	}

	tmNodes := tendermint.TendermintNodes(tendermintNodes)

	if err := tmNodes.LogGenesisHashes(ctx); err != nil {
		return err
	}

	eg, egCtx := errgroup.WithContext(ctx)
	for _, n := range c.PenumbraNodes {
		n := n
		sep, err := n.TendermintNode.GetConfigSeparator()
		if err != nil {
			return err
		}
		eg.Go(func() error {
			return n.TendermintNode.CreateNodeContainer(
				egCtx,
				fmt.Sprintf("--proxy%sapp=tcp://%s:26658", sep, n.PenumbraAppNode.HostName()),
				"--rpc.laddr=tcp://0.0.0.0:26657",
			)
		})
		eg.Go(func() error {
			return n.PenumbraAppNode.CreateNodeContainer(egCtx)
		})
	}
	if err := eg.Wait(); err != nil {
		return err
	}

	eg, egCtx = errgroup.WithContext(ctx)
	for _, n := range c.PenumbraNodes {
		n := n
		c.log.Info("Starting tendermint container", zap.String("container", n.TendermintNode.Name()))
		eg.Go(func() error {
			peers := tmNodes.PeerString(egCtx, n.TendermintNode)
			if err := n.TendermintNode.SetConfigAndPeers(egCtx, peers); err != nil {
				return err
			}
			return n.TendermintNode.StartContainer(egCtx)
		})
		c.log.Info("Starting penumbra container", zap.String("container", n.PenumbraAppNode.Name()))
		eg.Go(func() error {
			return n.PenumbraAppNode.StartContainer(egCtx)
		})
	}
	if err := eg.Wait(); err != nil {
		return err
	}

	// Wait for 5 blocks before considering the chains "started"
	return testutil.WaitForBlocks(ctx, 5, c.getFullNode().TendermintNode)
}
