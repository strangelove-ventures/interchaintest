package thorchain

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"

	dockerimage "github.com/docker/docker/api/types/image"
	volumetypes "github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	sdkmath "cosmossdk.io/math"

	clienttypes "github.com/cosmos/ibc-go/v10/modules/core/02-client/types" // nolint:staticcheck
	chanTypes "github.com/cosmos/ibc-go/v10/modules/core/04-channel/types"

	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	cryptocodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/types"

	"github.com/strangelove-ventures/interchaintest/v8/blockdb"
	"github.com/strangelove-ventures/interchaintest/v8/dockerutil"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"github.com/strangelove-ventures/interchaintest/v8/testutil"
)

type Thorchain struct {
	testName      string
	cfg           ibc.ChainConfig
	NumValidators int
	numFullNodes  int
	Validators    ChainNodes
	FullNodes     ChainNodes

	// preStartNodes is able to mutate the node containers before
	// they are all started
	preStartNodes func(*Thorchain)

	// Additional processes that need to be run on a per-chain basis.
	Sidecars SidecarProcesses

	cdc      *codec.ProtoCodec
	log      *zap.Logger
	keyring  keyring.Keyring
	findTxMu sync.Mutex
}

func NewThorchainHeighlinerChainConfig(
	name string,
	binary string,
	bech32Prefix string,
	denom string,
	gasPrices string,
	gasAdjustment float64,
	trustingPeriod string,
	noHostMount bool,
) ibc.ChainConfig {
	return ibc.ChainConfig{
		Type:           "thorchain",
		Name:           name,
		Bech32Prefix:   bech32Prefix,
		Denom:          denom,
		GasPrices:      gasPrices,
		GasAdjustment:  gasAdjustment,
		TrustingPeriod: trustingPeriod,
		NoHostMount:    noHostMount,
		Images: []ibc.DockerImage{
			{
				Repository: "ghcr.io/strangelove-ventures/heighliner/thorchain",
				UIDGID:     dockerutil.GetHeighlinerUserString(),
			},
		},
		Bin: binary,
	}
}

func NewThorchain(testName string, chainConfig ibc.ChainConfig, numValidators int, numFullNodes int, log *zap.Logger) *Thorchain {
	// if numValidators != 1 {
	// 	panic(fmt.Sprintf("Thorchain must start with 1 validators for vault and router contract setup"))
	// }
	if chainConfig.EncodingConfig == nil {
		cfg := DefaultEncoding()
		chainConfig.EncodingConfig = &cfg
	}

	if chainConfig.UsesCometMock() {
		if numValidators != 1 {
			panic(fmt.Sprintf("CometMock only supports 1 validator. Set `NumValidators` to 1 in %s's ChainSpec", chainConfig.Name))
		}
		if numFullNodes != 0 {
			panic(fmt.Sprintf("CometMock only supports 1 validator. Set `NumFullNodes` to 0 in %s's ChainSpec", chainConfig.Name))
		}
	}

	registry := codectypes.NewInterfaceRegistry()
	cryptocodec.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	kr := keyring.NewInMemory(cdc)

	return &Thorchain{
		testName:      testName,
		cfg:           chainConfig,
		NumValidators: numValidators,
		numFullNodes:  numFullNodes,
		log:           log,
		cdc:           cdc,
		keyring:       kr,
	}
}

// WithPreStartNodes sets the preStartNodes function.
func (c *Thorchain) WithPreStartNodes(preStartNodes func(*Thorchain)) {
	c.preStartNodes = preStartNodes
}

// GetCodec returns the codec for the chain.
func (c *Thorchain) GetCodec() *codec.ProtoCodec {
	return c.cdc
}

// Nodes returns all nodes, including validators and fullnodes.
func (c *Thorchain) Nodes() ChainNodes {
	return append(c.Validators, c.FullNodes...)
}

// AddValidators adds new validators to the network, peering with the existing nodes.
func (c *Thorchain) AddValidators(ctx context.Context, configFileOverrides map[string]any, inc int) error {
	// Get peer string for existing nodes
	peers := c.Nodes().PeerString(ctx)

	// Get genesis.json
	genbz, err := c.Validators[0].GenesisFileContent(ctx)
	if err != nil {
		return err
	}

	prevCount := c.NumValidators
	c.NumValidators += inc
	if err := c.initializeChainNodes(ctx, c.testName, c.getFullNode().DockerClient, c.getFullNode().NetworkID); err != nil {
		return err
	}

	// Create full node, validator keys, and start up
	var eg errgroup.Group
	for i := prevCount; i < c.NumValidators; i++ {
		eg.Go(func() error {
			val := c.Validators[i]
			if err := val.InitFullNodeFiles(ctx); err != nil {
				return err
			}
			if err := val.SetPeers(ctx, peers); err != nil {
				return err
			}
			if err := val.OverwriteGenesisFile(ctx, genbz); err != nil {
				return err
			}
			for configFile, modifiedConfig := range configFileOverrides {
				modifiedToml, ok := modifiedConfig.(testutil.Toml)
				if !ok {
					return fmt.Errorf("provided toml override for file %s is of type (%T). Expected (DecodedToml)", configFile, modifiedConfig)
				}
				if err := testutil.ModifyTomlConfigFile(
					ctx,
					val.logger(),
					val.DockerClient,
					val.TestName,
					val.VolumeName,
					configFile,
					modifiedToml,
				); err != nil {
					return err
				}
			}
			if err := val.CreateKey(ctx, valKey); err != nil {
				return fmt.Errorf("failed to create key: %w", err)
			}
			if err := val.GetNodeAccount(ctx); err != nil {
				return fmt.Errorf("failed to get node account info: %w", err)
			}
			if err := val.CreateNodeContainer(ctx); err != nil {
				return err
			}
			return val.StartContainer(ctx)
		})
	}

	if err := eg.Wait(); err != nil {
		return err
	}

	// Fund validator address and register for next churn
	decimalPow := int64(math.Pow10(int(*c.cfg.CoinDecimals)))
	for i := prevCount; i < c.NumValidators; i++ {
		// Fund validator from faucet
		if err := c.SendFunds(ctx, "faucet", ibc.WalletAmount{
			Address: c.Validators[i].NodeAccount.NodeAddress,
			Amount:  sdkmath.NewInt(100).MulRaw(decimalPow), // 100e8 rune
			Denom:   c.cfg.Denom,
		}); err != nil {
			return fmt.Errorf("failed to fund val %d, %w", i, err)
		}

		eg.Go(func() error {
			val := c.Validators[i]
			// thornode tx thorchain deposit 1e8 RUNE "bond:$NODE_ADDRESS"
			// Bond 2 rune since the next 3 txs will deduct .02 rune/tx and we need > 1 rune bonded
			if err := val.Bond(ctx, sdkmath.NewInt(2).MulRaw(decimalPow)); err != nil {
				return fmt.Errorf("failed to set val %d node keys, %w", i, err)
			}
			// thornode tx thorchain set-node-keys "$NODE_PUB_KEY" "$NODE_PUB_KEY_ED25519" "$VALIDATOR"
			if err := val.SetNodeKeys(ctx); err != nil {
				return fmt.Errorf("failed to set val %d node keys, %w", i, err)
			}
			// thornode tx thorchain set-ip-address "192.168.0.10"
			if err := val.SetIPAddress(ctx); err != nil {
				return fmt.Errorf("failed to set val %d ip address, %w", i, err)
			}
			// thornode tx thorchain set-version
			if err := val.SetVersion(ctx); err != nil {
				return fmt.Errorf("failed to set val %d version, %w", i, err)
			}
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return err
	}

	// start sidecar/bifrost
	return c.StartAllValSidecars(ctx)
}

// AddDuplicateValidator spins up a duplicate validator node to test double signing.
func (c *Thorchain) AddDuplicateValidator(ctx context.Context, configFileOverrides map[string]any, originalVal *ChainNode) (*ChainNode, error) {
	// Get peer string for existing nodes
	peers := c.Nodes().PeerString(ctx)

	// Get genesis.json
	genbz, err := c.Validators[0].GenesisFileContent(ctx)
	if err != nil {
		return nil, err
	}

	c.NumValidators += 1
	if err := c.initializeChainNodes(ctx, c.testName, c.getFullNode().DockerClient, c.getFullNode().NetworkID); err != nil {
		return nil, err
	}

	// Create full node, validator keys, and start up
	val := c.Validators[c.NumValidators-1]
	if err := val.InitFullNodeFiles(ctx); err != nil {
		return nil, err
	}
	if err := val.SetPeers(ctx, peers); err != nil {
		return nil, err
	}
	if err := val.OverwriteGenesisFile(ctx, genbz); err != nil {
		return nil, err
	}
	for configFile, modifiedConfig := range configFileOverrides {
		modifiedToml, ok := modifiedConfig.(testutil.Toml)
		if !ok {
			return nil, fmt.Errorf("provided toml override for file %s is of type (%T). Expected (DecodedToml)", configFile, modifiedConfig)
		}
		if err := testutil.ModifyTomlConfigFile(
			ctx,
			val.logger(),
			val.DockerClient,
			val.TestName,
			val.VolumeName,
			configFile,
			modifiedToml,
		); err != nil {
			return nil, err
		}
	}
	privValFile, err := originalVal.PrivValFileContent(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get priv_validator_key.json, %w", err)
	}
	if err := val.OverwritePrivValFile(ctx, privValFile); err != nil {
		return nil, fmt.Errorf("failed to overwrite priv_validator_key.json, %w", err)
	}
	if err := val.RecoverKey(ctx, valKey, originalVal.ValidatorMnemonic); err != nil {
		return nil, fmt.Errorf("failed to create key: %w", err)
	}
	val.ValidatorMnemonic = originalVal.ValidatorMnemonic
	if err := val.GetNodeAccount(ctx); err != nil {
		return nil, fmt.Errorf("failed to get node account info: %w", err)
	}
	if err := val.CreateNodeContainer(ctx); err != nil {
		return nil, err
	}
	return val, val.StartContainer(ctx)
}

// AddFullNodes adds new fullnodes to the network, peering with the existing nodes.
func (c *Thorchain) AddFullNodes(ctx context.Context, configFileOverrides map[string]any, inc int) error {
	// Get peer string for existing nodes
	peers := c.Nodes().PeerString(ctx)

	// Get genesis.json
	genbz, err := c.Validators[0].GenesisFileContent(ctx)
	if err != nil {
		return err
	}

	prevCount := c.numFullNodes
	c.numFullNodes += inc
	if err := c.initializeChainNodes(ctx, c.testName, c.getFullNode().DockerClient, c.getFullNode().NetworkID); err != nil {
		return err
	}

	var eg errgroup.Group
	for i := prevCount; i < c.numFullNodes; i++ {
		eg.Go(func() error {
			fn := c.FullNodes[i]
			if err := fn.InitFullNodeFiles(ctx); err != nil {
				return err
			}
			if err := fn.SetPeers(ctx, peers); err != nil {
				return err
			}
			if err := fn.OverwriteGenesisFile(ctx, genbz); err != nil {
				return err
			}
			for configFile, modifiedConfig := range configFileOverrides {
				modifiedToml, ok := modifiedConfig.(testutil.Toml)
				if !ok {
					return fmt.Errorf("provided toml override for file %s is of type (%T). Expected (DecodedToml)", configFile, modifiedConfig)
				}
				if err := testutil.ModifyTomlConfigFile(
					ctx,
					fn.logger(),
					fn.DockerClient,
					fn.TestName,
					fn.VolumeName,
					configFile,
					modifiedToml,
				); err != nil {
					return err
				}
			}
			if err := fn.CreateNodeContainer(ctx); err != nil {
				return err
			}
			return fn.StartContainer(ctx)
		})
	}
	return eg.Wait()
}

// Implements Chain interface.
func (c *Thorchain) Config() ibc.ChainConfig {
	return c.cfg
}

// Implements Chain interface.
func (c *Thorchain) Initialize(ctx context.Context, testName string, cli *client.Client, networkID string) error {
	if err := c.initializeSidecars(ctx, testName, cli, networkID); err != nil {
		return err
	}
	return c.initializeChainNodes(ctx, testName, cli, networkID)
}

func (c *Thorchain) getFullNode() *ChainNode {
	return c.GetNode()
}

func (c *Thorchain) GetNode() *ChainNode {
	return c.Validators[0]
}

// Exec implements ibc.Chain.
func (c *Thorchain) Exec(ctx context.Context, cmd []string, env []string) (stdout, stderr []byte, err error) {
	return c.getFullNode().Exec(ctx, cmd, env)
}

// Implements Chain interface.
func (c *Thorchain) GetRPCAddress() string {
	if c.Config().UsesCometMock() {
		return fmt.Sprintf("http://%s:22331", c.getFullNode().HostnameCometMock())
	}

	return fmt.Sprintf("http://%s:26657", c.getFullNode().HostName())
}

// Implements Chain interface.
func (c *Thorchain) GetAPIAddress() string {
	return fmt.Sprintf("http://%s:1317", "127.0.0.1")
	// return fmt.Sprintf("http://%s:1317", c.getFullNode().HostName())
}

// Implements Chain interface.
func (c *Thorchain) GetGRPCAddress() string {
	return fmt.Sprintf("%s:9090", c.getFullNode().HostName())
}

// GetHostRPCAddress returns the address of the RPC server accessible by the host.
// This will not return a valid address until the chain has been started.
func (c *Thorchain) GetHostRPCAddress() string {
	return "http://" + c.getFullNode().hostRPCPort
}

// GetHostAPIAddress returns the address of the REST API server accessible by the host.
// This will not return a valid address until the chain has been started.
func (c *Thorchain) GetHostAPIAddress() string {
	return "http://" + c.getFullNode().hostAPIPort
}

// GetHostGRPCAddress returns the address of the gRPC server accessible by the host.
// This will not return a valid address until the chain has been started.
func (c *Thorchain) GetHostGRPCAddress() string {
	return c.getFullNode().hostGRPCPort
}

// GetHostP2PAddress returns the address of the P2P server accessible by the host.
// This will not return a valid address until the chain has been started.
func (c *Thorchain) GetHostPeerAddress() string {
	return c.getFullNode().hostP2PPort
}

// HomeDir implements ibc.Chain.
func (c *Thorchain) HomeDir() string {
	return c.getFullNode().HomeDir()
}

// Implements Chain interface.
func (c *Thorchain) CreateKey(ctx context.Context, keyName string) error {
	return c.getFullNode().CreateKey(ctx, keyName)
}

// Implements Chain interface.
func (c *Thorchain) RecoverKey(ctx context.Context, keyName, mnemonic string) error {
	return c.getFullNode().RecoverKey(ctx, keyName, mnemonic)
}

// Implements Chain interface.
func (c *Thorchain) GetAddress(ctx context.Context, keyName string) ([]byte, error) {
	b32Addr, err := c.getFullNode().AccountKeyBech32(ctx, keyName)
	if err != nil {
		return nil, err
	}

	return types.GetFromBech32(b32Addr, c.Config().Bech32Prefix)
}

// BuildWallet will return a Cosmos wallet
// If mnemonic != "", it will restore using that mnemonic
// If mnemonic == "", it will create a new key.
func (c *Thorchain) BuildWallet(ctx context.Context, keyName string, mnemonic string) (ibc.Wallet, error) {
	if mnemonic != "" {
		c.log.Info("BuildWallet recovering key", zap.String("key_name", keyName))
		if err := c.RecoverKey(ctx, keyName, mnemonic); err != nil {
			return nil, fmt.Errorf("failed to recover key with name %q on chain %s: %w", keyName, c.cfg.Name, err)
		}
	} else {
		c.log.Info("BuildWallet creating key", zap.String("key_name", keyName))
		if err := c.CreateKey(ctx, keyName); err != nil {
			return nil, fmt.Errorf("failed to create key with name %q on chain %s: %w", keyName, c.cfg.Name, err)
		}
	}

	addrBytes, err := c.GetAddress(ctx, keyName)
	if err != nil {
		return nil, fmt.Errorf("failed to get account address for key %q on chain %s: %w", keyName, c.cfg.Name, err)
	}

	c.log.Info("BuildWallet got address", zap.String("key_name", keyName), zap.String("address", hex.EncodeToString(addrBytes)), zap.String("bech32addr", types.MustBech32ifyAddressBytes(c.cfg.Bech32Prefix, addrBytes)))

	return NewWallet(keyName, addrBytes, mnemonic, c.cfg), nil
}

// BuildRelayerWallet will return a Cosmos wallet populated with the mnemonic so that the wallet can
// be restored in the relayer node using the mnemonic. After it is built, that address is included in
// genesis with some funds.
func (c *Thorchain) BuildRelayerWallet(ctx context.Context, keyName string) (ibc.Wallet, error) {
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

// Implements Chain interface.
func (c *Thorchain) SendFunds(ctx context.Context, keyName string, amount ibc.WalletAmount) error {
	return c.getFullNode().BankSend(ctx, keyName, amount)
}

// Implements Chain interface.
func (c *Thorchain) SendFundsWithNote(ctx context.Context, keyName string, amount ibc.WalletAmount, note string) (string, error) {
	return c.getFullNode().BankSendWithNote(ctx, keyName, amount, note)
}

// Implements Chain interface.
func (c *Thorchain) SendIBCTransfer(
	ctx context.Context,
	channelID string,
	keyName string,
	amount ibc.WalletAmount,
	options ibc.TransferOptions,
) (tx ibc.Tx, _ error) {
	panic("SendIBCTransfer unimplemented")
	// txHash, err := c.getFullNode().SendIBCTransfer(ctx, channelID, keyName, amount, options)
	// if err != nil {
	// 	return tx, fmt.Errorf("send ibc transfer: %w", err)
	// }
	// txResp, err := c.GetTransaction(txHash)
	// if err != nil {
	// 	return tx, fmt.Errorf("failed to get transaction %s: %w", txHash, err)
	// }
	// if txResp.Code != 0 {
	// 	return tx, fmt.Errorf("error in transaction (code: %d): %s", txResp.Code, txResp.RawLog)
	// }
	// tx.Height = txResp.Height
	// tx.TxHash = txHash
	// // In cosmos, user is charged for entire gas requested, not the actual gas used.
	// tx.GasSpent = txResp.GasWanted

	// const evType = "send_packet"
	// events := txResp.Events

	// var (
	// 	seq, _           = tendermint.AttributeValue(events, evType, "packet_sequence")
	// 	srcPort, _       = tendermint.AttributeValue(events, evType, "packet_src_port")
	// 	srcChan, _       = tendermint.AttributeValue(events, evType, "packet_src_channel")
	// 	dstPort, _       = tendermint.AttributeValue(events, evType, "packet_dst_port")
	// 	dstChan, _       = tendermint.AttributeValue(events, evType, "packet_dst_channel")
	// 	timeoutHeight, _ = tendermint.AttributeValue(events, evType, "packet_timeout_height")
	// 	timeoutTs, _     = tendermint.AttributeValue(events, evType, "packet_timeout_timestamp")
	// 	dataHex, _       = tendermint.AttributeValue(events, evType, "packet_data_hex")
	// )
	// tx.Packet.SourcePort = srcPort
	// tx.Packet.SourceChannel = srcChan
	// tx.Packet.DestPort = dstPort
	// tx.Packet.DestChannel = dstChan
	// tx.Packet.TimeoutHeight = timeoutHeight

	// data, err := hex.DecodeString(dataHex)
	// if err != nil {
	// 	return tx, fmt.Errorf("malformed data hex %s: %w", dataHex, err)
	// }
	// tx.Packet.Data = data

	// seqNum, err := strconv.ParseUint(seq, 10, 64)
	// if err != nil {
	// 	return tx, fmt.Errorf("invalid packet sequence from events %s: %w", seq, err)
	// }
	// tx.Packet.Sequence = seqNum

	// timeoutNano, err := strconv.ParseUint(timeoutTs, 10, 64)
	// if err != nil {
	// 	return tx, fmt.Errorf("invalid packet timestamp timeout %s: %w", timeoutTs, err)
	// }
	// tx.Packet.TimeoutTimestamp = ibc.Nanoseconds(timeoutNano)

	// return tx, nil
}

// QueryParam returns the param state of a given key.
func (c *Thorchain) QueryParam(ctx context.Context, subspace, key string) (*ParamChange, error) {
	return c.getFullNode().QueryParam(ctx, subspace, key)
}

// QueryBankMetadata returns the metadata of a given token denomination.
func (c *Thorchain) QueryBankMetadata(ctx context.Context, denom string) (*BankMetaData, error) {
	return c.getFullNode().QueryBankMetadata(ctx, denom)
}

// ExportState exports the chain state at specific height.
// Implements Chain interface.
func (c *Thorchain) ExportState(ctx context.Context, height int64) (string, error) {
	return c.getFullNode().ExportState(ctx, height)
}

func (c *Thorchain) GetTransaction(txhash string) (*types.TxResponse, error) {
	fn := c.getFullNode()
	return fn.GetTransaction(fn.CliContext(), txhash)
}

func (c *Thorchain) GetGasFeesInNativeDenom(gasPaid int64) int64 {
	gasPrice, _ := strconv.ParseFloat(strings.Replace(c.cfg.GasPrices, c.cfg.Denom, "", 1), 64)
	fees := float64(gasPaid) * gasPrice
	return int64(math.Ceil(fees))
}

func (c *Thorchain) pullImages(ctx context.Context, cli *client.Client) {
	for _, image := range c.Config().Images {
		if image.Version == "local" {
			continue
		}
		rc, err := cli.ImagePull(
			ctx,
			image.Repository+":"+image.Version,
			dockerimage.PullOptions{},
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
}

// NewChainNode constructs a new cosmos chain node with a docker volume.
func (c *Thorchain) NewChainNode(
	ctx context.Context,
	testName string,
	cli *client.Client,
	networkID string,
	image ibc.DockerImage,
	validator bool,
	index int,
) (*ChainNode, error) {
	// Construct the ChainNode first so we can access its name.
	// The ChainNode's VolumeName cannot be set until after we create the volume.
	tn := NewChainNode(c.log, validator, c, cli, networkID, testName, image, index)

	tn.logger().Info("Creating volume")

	v, err := cli.VolumeCreate(ctx, volumetypes.CreateOptions{
		Labels: map[string]string{
			dockerutil.CleanupLabel: testName,

			dockerutil.NodeOwnerLabel: tn.Name(),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("creating volume for chain node: %w", err)
	}

	tn.logger().Info("Setting volume owner", zap.String("volume", v.Name))

	tn.VolumeName = v.Name

	if err := dockerutil.SetVolumeOwner(ctx, dockerutil.VolumeOwnerOptions{
		Log: c.log,

		Client: cli,

		VolumeName: v.Name,
		ImageRef:   image.Ref(),
		TestName:   testName,
		UidGid:     image.UIDGID,
	}); err != nil {
		return nil, fmt.Errorf("set volume owner: %w", err)
	}

	tn.logger().Info("Created docker volume and set owner", zap.String("volume", v.Name))

	for _, cfg := range c.cfg.SidecarConfigs {
		if !cfg.ValidatorProcess {
			continue
		}

		tn.logger().Info("Creating sidecar process", zap.String("process", cfg.ProcessName))

		err = tn.NewSidecarProcess(ctx, cfg.PreStart, cfg.ProcessName, cli, networkID, cfg.Image, cfg.HomeDir, cfg.Ports, cfg.StartCmd, cfg.Env)
		if err != nil {
			return nil, err
		}
	}

	tn.logger().Info("Done initializing chain node")

	return tn, nil
}

// NewSidecarProcess constructs a new sidecar process with a docker volume.
func (c *Thorchain) NewSidecarProcess(
	ctx context.Context,
	preStart bool,
	processName string,
	testName string,
	cli *client.Client,
	networkID string,
	image ibc.DockerImage,
	homeDir string,
	index int,
	ports []string,
	startCmd []string,
	env []string,
) error {
	// Construct the SidecarProcess first so we can access its name.
	// The SidecarProcess's VolumeName cannot be set until after we create the volume.
	s := NewSidecar(c.log, false, preStart, c, cli, networkID, processName, testName, image, homeDir, index, ports, startCmd, env)

	v, err := cli.VolumeCreate(ctx, volumetypes.CreateOptions{
		Labels: map[string]string{
			dockerutil.CleanupLabel:   testName,
			dockerutil.NodeOwnerLabel: s.Name(),
		},
	})
	if err != nil {
		return fmt.Errorf("creating volume for sidecar process: %w", err)
	}
	s.VolumeName = v.Name

	if err := dockerutil.SetVolumeOwner(ctx, dockerutil.VolumeOwnerOptions{
		Log: c.log,

		Client: cli,

		VolumeName: v.Name,
		ImageRef:   image.Ref(),
		TestName:   testName,
		UidGid:     image.UIDGID,
	}); err != nil {
		return fmt.Errorf("set volume owner: %w", err)
	}

	c.Sidecars = append(c.Sidecars, s)

	return nil
}

// creates the test node objects required for bootstrapping tests.
func (c *Thorchain) initializeChainNodes(
	ctx context.Context,
	testName string,
	cli *client.Client,
	networkID string,
) error {
	chainCfg := c.Config()
	c.pullImages(ctx, cli)
	image := chainCfg.Images[0]

	newVals := make(ChainNodes, c.NumValidators)
	copy(newVals, c.Validators)
	newFullNodes := make(ChainNodes, c.numFullNodes)
	copy(newFullNodes, c.FullNodes)

	eg, egCtx := errgroup.WithContext(ctx)
	for i := len(c.Validators); i < c.NumValidators; i++ {
		eg.Go(func() error {
			val, err := c.NewChainNode(egCtx, testName, cli, networkID, image, true, i)
			if err != nil {
				return err
			}
			newVals[i] = val
			return nil
		})
	}
	for i := len(c.FullNodes); i < c.numFullNodes; i++ {
		eg.Go(func() error {
			fn, err := c.NewChainNode(egCtx, testName, cli, networkID, image, false, i)
			if err != nil {
				return err
			}
			newFullNodes[i] = fn
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return err
	}

	c.findTxMu.Lock()
	defer c.findTxMu.Unlock()

	c.Validators = newVals
	c.FullNodes = newFullNodes

	c.log.Info("Initialized chain nodes", zap.Int("validators", len(newVals)), zap.Int("fullnodes", len(newFullNodes)))

	return nil
}

// initializeSidecars creates the sidecar processes that exist at the chain level.
func (c *Thorchain) initializeSidecars(
	ctx context.Context,
	testName string,
	cli *client.Client,
	networkID string,
) error {
	eg, egCtx := errgroup.WithContext(ctx)
	for i, cfg := range c.cfg.SidecarConfigs {
		if cfg.ValidatorProcess {
			continue
		}

		eg.Go(func() error {
			err := c.NewSidecarProcess(egCtx, cfg.PreStart, cfg.ProcessName, testName, cli, networkID, cfg.Image, cfg.HomeDir, i, cfg.Ports, cfg.StartCmd, cfg.Env)
			if err != nil {
				return err
			}
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return err
	}
	return nil
}

type GenesisValidatorPubKey struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

type GenesisFile struct {
	AppState struct {
		Thorchain struct {
			NodeAccounts []NodeAccount `json:"node_accounts"`
		} `json:"thorchain"`
	} `json:"app_state"`
}

type ValidatorWithIntPower struct {
	Address string
	Power   int64
	PubKey  string
}

func (c *Thorchain) prepNodes(ctx context.Context, genesisAmounts [][]types.Coin, genesisSelfDelegation []types.Coin) error {
	chainCfg := c.Config()
	configFileOverrides := chainCfg.ConfigFileOverrides

	eg, egCtx := errgroup.WithContext(ctx)
	// Initialize config and sign gentx for each validator.
	for i, v := range c.Validators {
		v.Validator = true
		eg.Go(func() error {
			if err := v.InitFullNodeFiles(egCtx); err != nil {
				return err
			}
			for configFile, modifiedConfig := range configFileOverrides {
				modifiedToml, ok := modifiedConfig.(testutil.Toml)
				if !ok {
					return fmt.Errorf("provided toml override for file %s is of type (%T). Expected (DecodedToml)", configFile, modifiedConfig)
				}
				if err := testutil.ModifyTomlConfigFile(
					egCtx,
					v.logger(),
					v.DockerClient,
					v.TestName,
					v.VolumeName,
					configFile,
					modifiedToml,
				); err != nil {
					return fmt.Errorf("failed to modify toml config file: %w", err)
				}
			}

			if v.Index == 0 {
				v.ValidatorMnemonic = strings.Repeat("dog ", 23) + "fossil"
				v.logger().Debug("Recover validator key")

				if err := v.RecoverKey(ctx, valKey, v.ValidatorMnemonic); err != nil {
					return err
				}
			} else {
				v.logger().Debug("Create validator key")
				if err := v.CreateKey(ctx, valKey); err != nil {
					return fmt.Errorf("failed to create key: %w", err)
				}
			}

			if !c.cfg.SkipGenTx && c.cfg.Genesis == nil {
				v.logger().Info("Initializing gentx")
				if err := v.InitValidatorGenTx(egCtx, &chainCfg, genesisAmounts[i], genesisSelfDelegation[i]); err != nil {
					return fmt.Errorf("failed to init validator gentx for validator %d: %w", i, err)
				}
			}

			v.logger().Info("Getting node account info")

			if err := v.GetNodeAccount(egCtx); err != nil {
				return fmt.Errorf("failed to get node account info: %w", err)
			}

			v.logger().Info("Adding node account info")
			if err := v.AddNodeAccount(egCtx, *v.NodeAccount); err != nil {
				return fmt.Errorf("failed to add node account: %w", err)
			}

			return nil
		})
	}

	// Initialize config for each full node.
	for _, n := range c.FullNodes {
		n.Validator = false
		eg.Go(func() error {
			if err := n.InitFullNodeFiles(egCtx); err != nil {
				return err
			}
			for configFile, modifiedConfig := range configFileOverrides {
				modifiedToml, ok := modifiedConfig.(testutil.Toml)
				if !ok {
					return fmt.Errorf("provided toml override for file %s is of type (%T). Expected (testutil.Toml)", configFile, modifiedConfig)
				}
				if err := testutil.ModifyTomlConfigFile(
					egCtx,
					n.logger(),
					n.DockerClient,
					n.TestName,
					n.VolumeName,
					configFile,
					modifiedToml,
				); err != nil {
					return err
				}
			}
			return nil
		})
	}

	// wait for this to finish
	return eg.Wait()
}

// Bootstraps the chain and starts it from genesis.
func (c *Thorchain) Start(testName string, ctx context.Context, additionalGenesisWallets ...ibc.WalletAmount) error {
	c.log.Info("Starting", zap.String("chain", c.Config().Name))
	chainCfg := c.Config()

	var genBz []byte

	genesisAmounts := make([][]types.Coin, len(c.Validators))
	genesisSelfDelegation := make([]types.Coin, len(c.Validators))

	var activeVals []NodeAccount
	var genBzReplace func(find, replace []byte)

	if chainCfg.Genesis == nil {
		decimalPow := int64(math.Pow10(int(*chainCfg.CoinDecimals)))

		for i := range c.Validators {
			genesisAmounts[i] = []types.Coin{{Amount: sdkmath.NewInt(1).MulRaw(decimalPow), Denom: chainCfg.Denom}}
			genesisSelfDelegation[i] = types.Coin{Amount: sdkmath.NewInt(1).MulRaw(decimalPow), Denom: chainCfg.Denom}
			if chainCfg.ModifyGenesisAmounts != nil {
				amount, selfDelegation := chainCfg.ModifyGenesisAmounts(i)
				genesisAmounts[i] = []types.Coin{amount}
				genesisSelfDelegation[i] = selfDelegation
			}
		}
	} else {
		genBz = chainCfg.Genesis.Contents
		var genesisFile GenesisFile
		if err := json.Unmarshal(genBz, &genesisFile); err != nil {
			return fmt.Errorf("failed to unmarshal genesis file: %w", err)
		}

		genesisValidators := genesisFile.AppState.Thorchain.NodeAccounts
		totalBond := uint64(0)

		validatorsWithPower := make([]NodeAccount, 0)

		for _, genesisValidator := range genesisValidators {
			if genesisValidator.Status != "Active" {
				continue
			}
			bond, err := strconv.ParseUint(genesisValidator.Bond, 10, 64)
			if err != nil {
				return err
			}
			totalBond += bond
			genesisValidator.BondUInt = bond
			validatorsWithPower = append(validatorsWithPower, genesisValidator)
		}

		sort.Slice(validatorsWithPower, func(i, j int) bool {
			return validatorsWithPower[i].BondUInt > validatorsWithPower[j].BondUInt
		})

		var mu sync.Mutex
		genBzReplace = func(find, replace []byte) {
			mu.Lock()
			defer mu.Unlock()
			genBz = bytes.ReplaceAll(genBz, find, replace)
		}

		twoThirdsConsensus := uint64(math.Ceil(float64(totalBond) * 2 / 3))
		totalConsensus := uint64(0)

		for _, validator := range validatorsWithPower {
			activeVals = append(activeVals, validator)

			totalConsensus += validator.BondUInt

			if !chainCfg.Genesis.AllValidators && totalConsensus > twoThirdsConsensus {
				break
			}
		}

		if chainCfg.Genesis.MaxVals == 0 {
			chainCfg.Genesis.MaxVals = 10
		}

		if len(activeVals) > chainCfg.Genesis.MaxVals {
			c.log.Warn("Not enough validators to meet 2/3 bond threshold, increase GenesisConfig.MaxVals to reach consensus", zap.Int("required", len(activeVals)), zap.Int("max", chainCfg.Genesis.MaxVals))
			// return fmt.Errorf("too many validators required to meet bond threshold: %d, max allowed: %d: increase this limit to proceed", len(activeVals), chainCfg.Genesis.MaxVals)
			c.NumValidators = chainCfg.Genesis.MaxVals
		} else {
			c.NumValidators = len(activeVals)
		}

		c.log.Info("Will launch validators", zap.Int("count", c.NumValidators))
	}

	fn := c.getFullNode()

	if err := c.initializeChainNodes(ctx, testName, fn.DockerClient, fn.NetworkID); err != nil {
		return err
	}

	if err := c.prepNodes(ctx, genesisAmounts, genesisSelfDelegation); err != nil {
		return fmt.Errorf("failed to prep nodes: %w", err)
	}

	if c.preStartNodes != nil {
		c.preStartNodes(c)
	}

	if c.cfg.PreGenesis != nil {
		err := c.cfg.PreGenesis(c)
		if err != nil {
			return err
		}
	}

	var err error

	if chainCfg.Genesis == nil {
		// for the validators we need to collect the gentxs and the accounts
		// to the first node's genesis file
		validator0 := c.Validators[0]
		for i := 1; i < len(c.Validators); i++ {
			validatorN := c.Validators[i]

			bech32, err := validatorN.AccountKeyBech32(ctx, valKey)
			if err != nil {
				return err
			}

			if err := validator0.AddGenesisAccount(ctx, bech32, genesisAmounts[0]); err != nil {
				return err
			}

			if !c.cfg.SkipGenTx {
				if err := validator0.AddNodeAccount(ctx, *validatorN.NodeAccount); err != nil {
					return fmt.Errorf("failed to add node account to val0: %w", err)
				}
			}
		}

		for _, wallet := range additionalGenesisWallets {
			if err := validator0.AddGenesisAccount(ctx, wallet.Address, []types.Coin{{Denom: wallet.Denom, Amount: wallet.Amount}}); err != nil {
				return err
			}
		}

		if err := validator0.AddBondModule(ctx); err != nil {
			return err
		}

		genBz, err = validator0.GenesisFileContent(ctx)
		if err != nil {
			return err
		}

		genBz = bytes.ReplaceAll(genBz, []byte(`"stake"`), []byte(fmt.Sprintf(`"%s"`, chainCfg.Denom)))
	} else {
		var eg errgroup.Group
		for i, validator := range activeVals {
			if i >= chainCfg.Genesis.MaxVals {
				break
			}
			v := c.Validators[i]
			c.log.Info(
				"Will emulate validator",
				zap.String("bond_address", validator.BondAddress),
				zap.String("node_address", validator.NodeAddress),
			)
			eg.Go(func() error {
				na := v.NodeAccount

				// modify genesis file overwriting validators info with the one generated for this test node
				genBzReplace([]byte(validator.ValidatorConsPubKey), []byte(na.ValidatorConsPubKey))
				genBzReplace([]byte(validator.NodeAddress), []byte(na.NodeAddress))
				genBzReplace([]byte(validator.BondAddress), []byte(na.BondAddress))
				genBzReplace([]byte(validator.PubKeySet.Secp256k1), []byte(na.PubKeySet.Secp256k1))
				genBzReplace([]byte(validator.PubKeySet.Ed25519), []byte(na.PubKeySet.Ed25519))

				return nil
			})
		}

		if err := eg.Wait(); err != nil {
			return err
		}
	}

	if c.cfg.ModifyGenesis != nil {
		genBz, err = c.cfg.ModifyGenesis(chainCfg, genBz)
		if err != nil {
			return err
		}
	}

	// Provide EXPORT_GENESIS_FILE_PATH and EXPORT_GENESIS_CHAIN to help debug genesis file
	exportGenesis := os.Getenv("EXPORT_GENESIS_FILE_PATH")
	exportGenesisChain := os.Getenv("EXPORT_GENESIS_CHAIN")
	if exportGenesis != "" && exportGenesisChain == c.cfg.Name {
		c.log.Debug("Exporting genesis file",
			zap.String("chain", exportGenesisChain),
			zap.String("path", exportGenesis),
		)
		_ = os.WriteFile(exportGenesis, genBz, 0o600)
	}

	chainNodes := c.Nodes()

	for _, cn := range chainNodes {
		if err := cn.OverwriteGenesisFile(ctx, genBz); err != nil {
			return err
		}
	}

	if err := chainNodes.LogGenesisHashes(ctx); err != nil {
		return err
	}

	// Start any sidecar processes that should be running before the chain starts
	eg, egCtx := errgroup.WithContext(ctx)
	for _, s := range c.Sidecars {
		err = s.containerLifecycle.Running(ctx)
		if s.preStart && err != nil {
			eg.Go(func() error {
				if err := s.CreateContainer(egCtx, nil); err != nil {
					return err
				}

				if err := s.StartContainer(egCtx); err != nil {
					return err
				}

				return nil
			})
		}
	}
	if err := eg.Wait(); err != nil {
		return err
	}

	eg, egCtx = errgroup.WithContext(ctx)
	for _, n := range chainNodes {
		eg.Go(func() error {
			return n.CreateNodeContainer(egCtx)
		})
	}
	if err := eg.Wait(); err != nil {
		return err
	}

	peers := chainNodes.PeerString(ctx)

	eg, egCtx = errgroup.WithContext(ctx)
	for _, n := range chainNodes {
		c.log.Info("Starting container", zap.String("container", n.Name()))
		eg.Go(func() error {
			if err := n.SetPeers(egCtx, peers); err != nil {
				return err
			}
			return n.StartContainer(egCtx)
		})
	}
	if err := eg.Wait(); err != nil {
		return err
	}

	// Wait for blocks before considering the chains "started"
	return testutil.WaitForBlocks(ctx, 2, c.getFullNode())
}

// Height implements ibc.Chain.
func (c *Thorchain) Height(ctx context.Context) (int64, error) {
	return c.getFullNode().Height(ctx)
}

// Acknowledgements implements ibc.Chain, returning all acknowledgments in block at height.
func (c *Thorchain) Acknowledgements(ctx context.Context, height int64) ([]ibc.PacketAcknowledgement, error) {
	var acks []*chanTypes.MsgAcknowledgement
	err := RangeBlockMessages(ctx, c.cfg.EncodingConfig.InterfaceRegistry, c.getFullNode().Client, height, func(msg types.Msg) bool {
		found, ok := msg.(*chanTypes.MsgAcknowledgement)
		if ok {
			acks = append(acks, found)
		}
		return false
	})
	if err != nil {
		return nil, fmt.Errorf("find acknowledgements at height %d: %w", height, err)
	}
	ibcAcks := make([]ibc.PacketAcknowledgement, len(acks))
	for i, ack := range acks {
		ibcAcks[i] = ibc.PacketAcknowledgement{
			Acknowledgement: ack.Acknowledgement,
			Packet: ibc.Packet{
				Sequence:         ack.Packet.Sequence,
				SourcePort:       ack.Packet.SourcePort,
				SourceChannel:    ack.Packet.SourceChannel,
				DestPort:         ack.Packet.DestinationPort,
				DestChannel:      ack.Packet.DestinationChannel,
				Data:             ack.Packet.Data,
				TimeoutHeight:    ack.Packet.TimeoutHeight.String(),
				TimeoutTimestamp: ibc.Nanoseconds(ack.Packet.TimeoutTimestamp),
			},
		}
	}
	return ibcAcks, nil
}

// Timeouts implements ibc.Chain, returning all timeouts in block at height.
func (c *Thorchain) Timeouts(ctx context.Context, height int64) ([]ibc.PacketTimeout, error) {
	var timeouts []*chanTypes.MsgTimeout
	err := RangeBlockMessages(ctx, c.cfg.EncodingConfig.InterfaceRegistry, c.getFullNode().Client, height, func(msg types.Msg) bool {
		found, ok := msg.(*chanTypes.MsgTimeout)
		if ok {
			timeouts = append(timeouts, found)
		}
		return false
	})
	if err != nil {
		return nil, fmt.Errorf("find timeouts at height %d: %w", height, err)
	}
	ibcTimeouts := make([]ibc.PacketTimeout, len(timeouts))
	for i, ack := range timeouts {
		ibcTimeouts[i] = ibc.PacketTimeout{
			Packet: ibc.Packet{
				Sequence:         ack.Packet.Sequence,
				SourcePort:       ack.Packet.SourcePort,
				SourceChannel:    ack.Packet.SourceChannel,
				DestPort:         ack.Packet.DestinationPort,
				DestChannel:      ack.Packet.DestinationChannel,
				Data:             ack.Packet.Data,
				TimeoutHeight:    ack.Packet.TimeoutHeight.String(),
				TimeoutTimestamp: ibc.Nanoseconds(ack.Packet.TimeoutTimestamp),
			},
		}
	}
	return ibcTimeouts, nil
}

// FindTxs implements blockdb.BlockSaver.
func (c *Thorchain) FindTxs(ctx context.Context, height int64) ([]blockdb.Tx, error) {
	fn := c.getFullNode()
	c.findTxMu.Lock()
	defer c.findTxMu.Unlock()
	return fn.FindTxs(ctx, height)
}

// StopAllNodes stops and removes all long running containers (validators and full nodes).
func (c *Thorchain) StopAllNodes(ctx context.Context) error {
	var eg errgroup.Group
	for _, n := range c.Nodes() {
		eg.Go(func() error {
			if err := n.StopContainer(ctx); err != nil {
				return err
			}
			return n.RemoveContainer(ctx)
		})
	}
	return eg.Wait()
}

// StopAllSidecars stops and removes all long-running containers for sidecar processes.
func (c *Thorchain) StopAllSidecars(ctx context.Context) error {
	var eg errgroup.Group
	for _, s := range c.Sidecars {
		eg.Go(func() error {
			if err := s.StopContainer(ctx); err != nil {
				return err
			}
			return s.RemoveContainer(ctx)
		})
	}
	return eg.Wait()
}

// StartAllNodes creates and starts new containers for each node.
// Should only be used if the chain has previously been started with .Start.
func (c *Thorchain) StartAllNodes(ctx context.Context) error {
	// prevent client calls during this time
	c.findTxMu.Lock()
	defer c.findTxMu.Unlock()
	var eg errgroup.Group
	for _, n := range c.Nodes() {
		eg.Go(func() error {
			if err := n.CreateNodeContainer(ctx); err != nil {
				return err
			}
			return n.StartContainer(ctx)
		})
	}
	return eg.Wait()
}

// StartAllSidecars creates and starts new containers for each sidecar process.
// Should only be used if the chain has previously been started with .Start.
func (c *Thorchain) StartAllSidecars(ctx context.Context) error {
	// prevent client calls during this time
	c.findTxMu.Lock()
	defer c.findTxMu.Unlock()
	var eg errgroup.Group
	for _, s := range c.Sidecars {
		err := s.containerLifecycle.Running(ctx)
		if err == nil {
			continue
		}

		eg.Go(func() error {
			if err := s.CreateContainer(ctx, nil); err != nil {
				return err
			}
			return s.StartContainer(ctx)
		})
	}
	return eg.Wait()
}

// StartAllValSidecars creates and starts new containers for each validator sidecar process.
// Should only be used if the chain has previously been started with .Start.
func (c *Thorchain) StartAllValSidecars(ctx context.Context) error {
	// prevent client calls during this time
	c.findTxMu.Lock()
	defer c.findTxMu.Unlock()
	var eg errgroup.Group

	for _, v := range c.Validators {
		for _, s := range v.Sidecars {
			err := s.containerLifecycle.Running(ctx)
			if err == nil {
				continue
			}

			eg.Go(func() error {
				env := s.env
				env = append(env, fmt.Sprintf("NODES=%d", c.NumValidators))
				env = append(env, fmt.Sprintf("SIGNER_SEED_PHRASE=\"%s\"", v.ValidatorMnemonic))
				env = append(env, fmt.Sprintf("CHAIN_API=%s:1317", v.HostName()))
				env = append(env, fmt.Sprintf("CHAIN_RPC=%s:26657", v.HostName()))
				env = append(env, fmt.Sprintf("PEER=%s", c.Validators.SidecarBifrostPeers()))
				s.env = env
				if err := s.CreateContainer(ctx, v.Bind()); err != nil {
					return err
				}
				return s.StartContainer(ctx)
			})
		}
	}

	return eg.Wait()
}

// GetTimeoutHeight returns a timeout height of 1000 blocks above the current block height.
// This function should be used when the timeout is never expected to be reached.
func (c *Thorchain) GetTimeoutHeight(ctx context.Context) (clienttypes.Height, error) {
	height, err := c.Height(ctx)
	if err != nil {
		c.log.Error("Failed to get chain height", zap.Error(err))
		return clienttypes.Height{}, fmt.Errorf("failed to get chain height: %w", err)
	}

	return clienttypes.NewHeight(clienttypes.ParseChainID(c.Config().ChainID), uint64(height)+1000), nil
}
