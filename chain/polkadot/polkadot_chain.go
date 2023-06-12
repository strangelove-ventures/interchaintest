package polkadot

import (
	"context"
	"crypto/rand"
	crand "crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/99designs/keyring"
	"github.com/StirlingMarketingGroup/go-namecase"
	sdktypes "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/go-bip39"
	"github.com/docker/docker/api/types"
	volumetypes "github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
	dockerclient "github.com/docker/docker/client"
	"github.com/icza/dyno"
	p2pcrypto "github.com/libp2p/go-libp2p/core/crypto"
	"github.com/misko9/go-substrate-rpc-client/v4/signature"
	gstypes "github.com/misko9/go-substrate-rpc-client/v4/types"
	"github.com/strangelove-ventures/interchaintest/v7/ibc"
	"github.com/strangelove-ventures/interchaintest/v7/internal/blockdb"
	"github.com/strangelove-ventures/interchaintest/v7/internal/dockerutil"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

// Increase polkadot wallet amount due to their additional precision
const polkadotScaling = int64(1_000)

// PolkadotChain implements the ibc.Chain interface for substrate chains.
type PolkadotChain struct {
	log                *zap.Logger
	testName           string
	cfg                ibc.ChainConfig
	numRelayChainNodes int
	parachainConfig    []ParachainConfig
	RelayChainNodes    RelayChainNodes
	ParachainNodes     []ParachainNodes
	keyring            keyring.Keyring
}

// PolkadotAuthority is used when constructing the validator authorities in the substrate chain spec.
type PolkadotAuthority struct {
	Grandpa            string `json:"grandpa"`
	Babe               string `json:"babe"`
	IMOnline           string `json:"im_online"`
	ParachainValidator string `json:"parachain_validator"`
	AuthorityDiscovery string `json:"authority_discovery"`
	ParaValidator      string `json:"para_validator"`
	ParaAssignment     string `json:"para_assignment"`
	Beefy              string `json:"beefy"`
}

// PolkadotParachainSpec is used when constructing substrate chain spec for parachains.
type PolkadotParachainSpec struct {
	GenesisHead    string `json:"genesis_head"`
	ValidationCode string `json:"validation_code"`
	Parachain      bool   `json:"parachain"`
}

// ParachainConfig is a shared type that allows callers of this module to configure a parachain.
type ParachainConfig struct {
	ChainID         string
	Bin             string
	Image           ibc.DockerImage
	NumNodes        int
	Flags           []string
	RelayChainFlags []string
}

// IndexedName is a slice of the substrate dev key names used for key derivation.
var IndexedName = []string{"alice", "bob", "charlie", "dave", "ferdie"}
var IndexedUri = []string{"//Alice", "//Bob", "//Charlie", "//Dave", "//Ferdie"}

// NewPolkadotChain returns an uninitialized PolkadotChain, which implements the ibc.Chain interface.
func NewPolkadotChain(log *zap.Logger, testName string, chainConfig ibc.ChainConfig, numRelayChainNodes int, parachains []ParachainConfig) *PolkadotChain {
	return &PolkadotChain{
		log:                log,
		testName:           testName,
		cfg:                chainConfig,
		numRelayChainNodes: numRelayChainNodes,
		parachainConfig:    parachains,
		keyring:            keyring.NewArrayKeyring(nil),
	}
}

// Config fetches the chain configuration.
// Implements Chain interface.
func (c *PolkadotChain) Config() ibc.ChainConfig {
	return c.cfg
}

func (c *PolkadotChain) NewRelayChainNode(
	ctx context.Context,
	i int,
	chain *PolkadotChain,
	dockerClient *dockerclient.Client,
	networkID string,
	testName string,
	image ibc.DockerImage,
) (*RelayChainNode, error) {
	seed := make([]byte, 32)
	if _, err := rand.Read(seed); err != nil {
		return nil, err
	}

	nodeKey, _, err := p2pcrypto.GenerateEd25519Key(crand.Reader)
	if err != nil {
		return nil, fmt.Errorf("error generating node key: %w", err)
	}

	nameCased := namecase.New().NameCase(IndexedName[i])

	ed25519PrivKey, err := DeriveEd25519FromName(nameCased)
	if err != nil {
		return nil, err
	}

	accountKeyName := IndexedName[i]
	accountKeyUri := IndexedUri[i]
	stashKeyName := accountKeyName + "stash"
	stashKeyUri := accountKeyUri + "//stash"

	if err := c.RecoverKey(ctx, accountKeyName, accountKeyUri); err != nil {
		return nil, err
	}

	if err := c.RecoverKey(ctx, stashKeyName, stashKeyUri); err != nil {
		return nil, err
	}

	ecdsaPrivKey, err := DeriveSecp256k1FromName(nameCased)
	if err != nil {
		return nil, fmt.Errorf("error generating secp256k1 private key: %w", err)
	}

	pn := &RelayChainNode{
		log:               c.log,
		Index:             i,
		Chain:             c,
		DockerClient:      dockerClient,
		NetworkID:         networkID,
		TestName:          testName,
		Image:             image,
		NodeKey:           nodeKey,
		Ed25519PrivateKey: ed25519PrivKey,
		AccountKeyName:    accountKeyName,
		StashKeyName:      stashKeyName,
		EcdsaPrivateKey:   *ecdsaPrivKey,
	}

	pn.containerLifecycle = dockerutil.NewContainerLifecycle(c.log, dockerClient, pn.Name())

	v, err := dockerClient.VolumeCreate(ctx, volumetypes.CreateOptions{
		Labels: map[string]string{
			dockerutil.CleanupLabel: testName,

			dockerutil.NodeOwnerLabel: pn.Name(),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("creating volume for chain node: %w", err)
	}
	pn.VolumeName = v.Name

	if err := dockerutil.SetVolumeOwner(ctx, dockerutil.VolumeOwnerOptions{
		Log:        c.log,
		Client:     dockerClient,
		VolumeName: v.Name,
		ImageRef:   image.Ref(),
		TestName:   testName,
		UidGid:     image.UidGid,
	}); err != nil {
		return nil, fmt.Errorf("set volume owner: %w", err)
	}

	return pn, nil
}

func (c *PolkadotChain) NewParachainNode(
	ctx context.Context,
	i int,
	dockerClient *dockerclient.Client,
	networkID string,
	testName string,
	parachainConfig ParachainConfig,
) (*ParachainNode, error) {
	nodeKey, _, err := p2pcrypto.GenerateEd25519Key(crand.Reader)
	if err != nil {
		return nil, fmt.Errorf("error generating node key: %w", err)
	}
	pn := &ParachainNode{
		log:             c.log,
		Index:           i,
		Chain:           c,
		DockerClient:    dockerClient,
		NetworkID:       networkID,
		TestName:        testName,
		NodeKey:         nodeKey,
		Image:           parachainConfig.Image,
		Bin:             parachainConfig.Bin,
		ChainID:         parachainConfig.ChainID,
		Flags:           parachainConfig.Flags,
		RelayChainFlags: parachainConfig.RelayChainFlags,
	}

	pn.containerLifecycle = dockerutil.NewContainerLifecycle(c.log, dockerClient, pn.Name())

	v, err := dockerClient.VolumeCreate(ctx, volumetypes.CreateOptions{
		Labels: map[string]string{
			dockerutil.CleanupLabel: testName,

			dockerutil.NodeOwnerLabel: pn.Name(),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("creating volume for chain node: %w", err)
	}
	pn.VolumeName = v.Name

	if err := dockerutil.SetVolumeOwner(ctx, dockerutil.VolumeOwnerOptions{
		Log:        c.log,
		Client:     dockerClient,
		VolumeName: v.Name,
		ImageRef:   parachainConfig.Image.Ref(),
		TestName:   testName,
		UidGid:     parachainConfig.Image.UidGid,
	}); err != nil {
		return nil, fmt.Errorf("set volume owner: %w", err)
	}

	return pn, nil
}

// Initialize initializes node structs so that things like initializing keys can be done before starting the chain.
// Implements Chain interface.
func (c *PolkadotChain) Initialize(ctx context.Context, testName string, cli *client.Client, networkID string) error {
	relayChainNodes := []*RelayChainNode{}
	chainCfg := c.Config()
	images := []ibc.DockerImage{}
	images = append(images, chainCfg.Images...)
	for _, parachain := range c.parachainConfig {
		images = append(images, parachain.Image)
	}
	for _, image := range images {
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
	for i := 0; i < c.numRelayChainNodes; i++ {
		pn, err := c.NewRelayChainNode(ctx, i, c, cli, networkID, testName, chainCfg.Images[0])
		if err != nil {
			return err
		}
		relayChainNodes = append(relayChainNodes, pn)
	}
	c.RelayChainNodes = relayChainNodes
	for _, pc := range c.parachainConfig {
		parachainNodes := []*ParachainNode{}
		for i := 0; i < pc.NumNodes; i++ {
			pn, err := c.NewParachainNode(ctx, i, cli, networkID, testName, pc)
			if err != nil {
				return err
			}
			parachainNodes = append(parachainNodes, pn)
		}
		c.ParachainNodes = append(c.ParachainNodes, parachainNodes)
	}

	return nil
}

func runtimeGenesisPath(path ...interface{}) []interface{} {
	fullPath := []interface{}{"genesis", "runtime", "runtime_genesis_config"}
	fullPath = append(fullPath, path...)
	return fullPath
}

func (c *PolkadotChain) modifyRelayChainGenesis(ctx context.Context, chainSpec interface{}, additionalGenesisWallets []ibc.WalletAmount) error {
	bootNodes := []string{}
	authorities := [][]interface{}{}
	balances := [][]interface{}{}
	var sudoAddress string
	for i, n := range c.RelayChainNodes {
		multiAddress, err := n.MultiAddress()
		if err != nil {
			return err
		}
		bootNodes = append(bootNodes, multiAddress)
		stashAddress, err := c.GetAddress(ctx, n.StashKeyName)
		if err != nil {
			return fmt.Errorf("error getting stash address: %w", err)
		}
		accountAddress, err := c.GetAddress(ctx, n.AccountKeyName)
		if err != nil {
			return fmt.Errorf("error getting account address: %w", err)
		}
		grandpaAddress, err := n.GrandpaAddress()
		if err != nil {
			return fmt.Errorf("error getting grandpa address")
		}
		beefyAddress, err := n.EcdsaAddress()
		if err != nil {
			return fmt.Errorf("error getting beefy address")
		}
		balances = append(balances,
			[]interface{}{string(stashAddress), uint64(1_100_000_000_000_000_000)},
			[]interface{}{string(accountAddress), uint64(1_100_000_000_000_000_000)},
		)
		if i == 0 {
			sudoAddress = string(accountAddress)
		}
		authority := []interface{}{string(stashAddress), string(stashAddress), PolkadotAuthority{
			Grandpa:            grandpaAddress,
			Babe:               string(accountAddress),
			IMOnline:           string(accountAddress),
			ParachainValidator: string(accountAddress),
			AuthorityDiscovery: string(accountAddress),
			ParaValidator:      string(accountAddress),
			ParaAssignment:     string(accountAddress),
			Beefy:              beefyAddress,
		}}
		authorities = append(authorities, authority)
	}
	for _, wallet := range additionalGenesisWallets {
		balances = append(balances,
			[]interface{}{wallet.Address, wallet.Amount * polkadotScaling},
		)
	}

	if err := dyno.Set(chainSpec, bootNodes, "bootNodes"); err != nil {
		return fmt.Errorf("error setting boot nodes: %w", err)
	}
	if err := dyno.Set(chainSpec, authorities, runtimeGenesisPath("session", "keys")...); err != nil {
		return fmt.Errorf("error setting authorities: %w", err)
	}
	if err := dyno.Set(chainSpec, balances, runtimeGenesisPath("balances", "balances")...); err != nil {
		return fmt.Errorf("error setting balances: %w", err)
	}
	if err := dyno.Set(chainSpec, sudoAddress, runtimeGenesisPath("sudo", "key")...); err != nil {
		return fmt.Errorf("error setting sudo key: %w", err)
	}
	/*if err := dyno.Set(chainSpec, sudoAddress, runtimeGenesisPath("bridgeRococoGrandpa", "owner")...); err != nil {
		return fmt.Errorf("error setting bridgeRococoGrandpa owner: %w", err)
	}
	if err := dyno.Set(chainSpec, sudoAddress, runtimeGenesisPath("bridgeWococoGrandpa", "owner")...); err != nil {
		return fmt.Errorf("error setting bridgeWococoGrandpa owner: %w", err)
	}
	if err := dyno.Set(chainSpec, sudoAddress, runtimeGenesisPath("bridgeRococoMessages", "owner")...); err != nil {
		return fmt.Errorf("error setting bridgeRococoMessages owner: %w", err)
	}
	if err := dyno.Set(chainSpec, sudoAddress, runtimeGenesisPath("bridgeWococoMessages", "owner")...); err != nil {
		return fmt.Errorf("error setting bridgeWococoMessages owner: %w", err)
	}
	*/
	if err := dyno.Set(chainSpec, 2, runtimeGenesisPath("configuration", "config", "validation_upgrade_delay")...); err != nil {
		return fmt.Errorf("error setting validation upgrade delay: %w", err)
	}
	parachains := [][]interface{}{}

	for _, parachainNodes := range c.ParachainNodes {
		firstParachainNode := parachainNodes[0]
		parachainID, err := firstParachainNode.ParachainID(ctx)
		if err != nil {
			return fmt.Errorf("error getting parachain ID: %w", err)
		}
		genesisState, err := firstParachainNode.ExportGenesisState(ctx)
		if err != nil {
			return fmt.Errorf("error exporting genesis state: %w", err)
		}
		genesisWasm, err := firstParachainNode.ExportGenesisWasm(ctx)
		if err != nil {
			return fmt.Errorf("error exporting genesis wasm: %w", err)
		}

		composableParachain := []interface{}{parachainID, PolkadotParachainSpec{
			GenesisHead:    genesisState,
			ValidationCode: genesisWasm,
			Parachain:      true,
		}}
		parachains = append(parachains, composableParachain)
	}

	if err := dyno.Set(chainSpec, parachains, runtimeGenesisPath("paras", "paras")...); err != nil {
		return fmt.Errorf("error setting parachains: %w", err)
	}
	if err := dyno.Set(chainSpec, 20, "genesis", "runtime", "session_length_in_blocks"); err != nil {
		return fmt.Errorf("error setting session_length_in_blocks: %w", err)
	}
	return nil
}

func (c *PolkadotChain) logger() *zap.Logger {
	return c.log.With(
		zap.String("chain_id", c.cfg.ChainID),
		zap.String("test", c.testName),
	)
}

// Start sets up everything needed (validators, gentx, fullnodes, peering, additional accounts) for chain to start from genesis.
// Implements Chain interface.
func (c *PolkadotChain) Start(testName string, ctx context.Context, additionalGenesisWallets ...ibc.WalletAmount) error {
	var eg errgroup.Group
	// Generate genesis file for each set of parachains
	for _, parachainNodes := range c.ParachainNodes {
		firstParachainNode := parachainNodes[0]
		parachainChainSpec, err := firstParachainNode.GenerateParachainGenesisFile(ctx, additionalGenesisWallets...)
		if err != nil {
			return fmt.Errorf("error generating parachain genesis file: %w", err)
		}
		for _, n := range parachainNodes {
			n := n
			eg.Go(func() error {
				c.logger().Info("Copying parachain chain spec", zap.String("container", n.Name()))
				fw := dockerutil.NewFileWriter(n.logger(), n.DockerClient, n.TestName)
				return fw.WriteFile(ctx, n.VolumeName, n.ParachainChainSpecFileName(), parachainChainSpec)
			})
		}
	}
	if err := eg.Wait(); err != nil {
		return err
	}

	// generate chain spec
	firstNode := c.RelayChainNodes[0]
	if err := firstNode.GenerateChainSpec(ctx); err != nil {
		return fmt.Errorf("error generating chain spec: %w", err)
	}
	fr := dockerutil.NewFileRetriever(c.logger(), firstNode.DockerClient, c.testName)
	fw := dockerutil.NewFileWriter(c.logger(), firstNode.DockerClient, c.testName)

	chainSpecBytes, err := fr.SingleFileContent(ctx, firstNode.VolumeName, firstNode.ChainSpecFilePathContainer())
	if err != nil {
		return fmt.Errorf("error reading chain spec: %w", err)
	}

	var chainSpec interface{}
	if err := json.Unmarshal(chainSpecBytes, &chainSpec); err != nil {
		return fmt.Errorf("error unmarshaling chain spec: %w", err)
	}

	if err := c.modifyRelayChainGenesis(ctx, chainSpec, additionalGenesisWallets); err != nil {
		return fmt.Errorf("error modifying genesis: %w", err)
	}

	editedChainSpec, err := json.MarshalIndent(chainSpec, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshaling modified chain spec: %w", err)
	}

	if err := fw.WriteFile(ctx, firstNode.VolumeName, firstNode.ChainSpecFilePathContainer(), editedChainSpec); err != nil {
		return fmt.Errorf("error writing modified chain spec: %w", err)
	}

	c.logger().Info("Generating raw chain spec", zap.String("container", firstNode.Name()))

	if err := firstNode.GenerateChainSpecRaw(ctx); err != nil {
		return err
	}

	rawChainSpecBytes, err := fr.SingleFileContent(ctx, firstNode.VolumeName, firstNode.RawChainSpecFilePathRelative())
	if err != nil {
		return fmt.Errorf("error reading chain spec: %w", err)
	}

	for i, n := range c.RelayChainNodes {
		n := n
		i := i
		eg.Go(func() error {
			if i != 0 {
				c.logger().Info("Copying raw chain spec", zap.String("container", n.Name()))
				if err := fw.WriteFile(ctx, n.VolumeName, n.RawChainSpecFilePathRelative(), rawChainSpecBytes); err != nil {
					return fmt.Errorf("error writing raw chain spec: %w", err)
				}
			}
			c.logger().Info("Creating container", zap.String("name", n.Name()))
			if err := n.CreateNodeContainer(ctx); err != nil {
				return err
			}
			c.logger().Info("Starting container", zap.String("name", n.Name()))
			return n.StartContainer(ctx)
		})
	}
	if err := eg.Wait(); err != nil {
		return err
	}
	for _, nodes := range c.ParachainNodes {
		nodes := nodes
		for _, n := range nodes {
			n := n
			eg.Go(func() error {
				c.logger().Info("Copying raw chain spec", zap.String("container", n.Name()))
				if err := fw.WriteFile(ctx, n.VolumeName, n.RawRelayChainSpecFilePathRelative(), rawChainSpecBytes); err != nil {
					return fmt.Errorf("error writing raw chain spec: %w", err)
				}
				//fmt.Print(string(rawChainSpecBytes))
				c.logger().Info("Creating container", zap.String("name", n.Name()))
				if err := n.CreateNodeContainer(ctx); err != nil {
					return err
				}
				c.logger().Info("Starting container", zap.String("name", n.Name()))
				return n.StartContainer(ctx)
			})
		}
	}
	if err := eg.Wait(); err != nil {
		return err
	}

	return nil
}

// Exec runs an arbitrary command using Chain's docker environment.
// Implements Chain interface.
func (c *PolkadotChain) Exec(ctx context.Context, cmd []string, env []string) ([]byte, []byte, error) {
	res := c.RelayChainNodes[0].Exec(ctx, cmd, env)
	return res.Stdout, res.Stderr, res.Err
}

// GetRPCAddress retrieves the rpc address that can be reached by other containers in the docker network.
// Implements Chain interface.
func (c *PolkadotChain) GetRPCAddress() string {
	var parachainHostName string
	port := strings.Split(rpcPort, "/")[0]

	if len(c.ParachainNodes) > 0 && len(c.ParachainNodes[0]) > 0 {
		parachainHostName = c.ParachainNodes[0][0].HostName()
		//return fmt.Sprintf("%s:%s", c.ParachainNodes[0][0].HostName(), strings.Split(rpcPort, "/")[0])
	} else {
		parachainHostName = c.RelayChainNodes[0].HostName()
	}
	relaychainHostName := c.RelayChainNodes[0].HostName()
	parachainUrl := fmt.Sprintf("http://%s:%s", parachainHostName, port)
	relaychainUrl := fmt.Sprintf("http://%s:%s", relaychainHostName, port)
	return fmt.Sprintf("%s,%s", parachainUrl, relaychainUrl)
	//return fmt.Sprintf("%s:%s", c.RelayChainNodes[0].HostName(), strings.Split(rpcPort, "/")[0])
}

// GetGRPCAddress retrieves the grpc address that can be reached by other containers in the docker network.
// Implements Chain interface.
func (c *PolkadotChain) GetGRPCAddress() string {
	if len(c.ParachainNodes) > 0 && len(c.ParachainNodes[0]) > 0 {
		return fmt.Sprintf("%s:%s", c.ParachainNodes[0][0].HostName(), strings.Split(wsPort, "/")[0])
	}
	return fmt.Sprintf("%s:%s", c.RelayChainNodes[0].HostName(), strings.Split(wsPort, "/")[0])
}

// GetHostRPCAddress returns the rpc address that can be reached by processes on the host machine.
// Note that this will not return a valid value until after Start returns.
// Implements Chain interface.
func (c *PolkadotChain) GetHostRPCAddress() string {
	if len(c.ParachainNodes) > 0 && len(c.ParachainNodes[0]) > 0 {
		return c.ParachainNodes[0][0].hostRpcPort
	}
	return c.RelayChainNodes[0].hostRpcPort
}

// GetHostGRPCAddress returns the grpc address that can be reached by processes on the host machine.
// Note that this will not return a valid value until after Start returns.
// Implements Chain interface.
func (c *PolkadotChain) GetHostGRPCAddress() string {
	if len(c.ParachainNodes) > 0 && len(c.ParachainNodes[0]) > 0 {
		return c.ParachainNodes[0][0].hostWsPort
	}
	return c.RelayChainNodes[0].hostWsPort
}

// Height returns the current block height or an error if unable to get current height.
// Implements Chain interface.
func (c *PolkadotChain) Height(ctx context.Context) (uint64, error) {
	if len(c.ParachainNodes) > 0 && len(c.ParachainNodes[0]) > 0 {
		block, err := c.ParachainNodes[0][0].api.RPC.Chain.GetBlockLatest()
		if err != nil {
			return 0, err
		}
		return uint64(block.Block.Header.Number), nil
	}
	block, err := c.RelayChainNodes[0].api.RPC.Chain.GetBlockLatest()
	if err != nil {
		return 0, err
	}
	return uint64(block.Block.Header.Number), nil
}

// ExportState exports the chain state at specific height.
// Implements Chain interface.
func (c *PolkadotChain) ExportState(ctx context.Context, height int64) (string, error) {
	panic("[ExportState] not implemented yet")
}

// HomeDir is the home directory of a node running in a docker container. Therefore, this maps to
// the container's filesystem (not the host).
// Implements Chain interface.
func (c *PolkadotChain) HomeDir() string {
	panic("[HomeDir] not implemented yet")
}

func NewMnemonic() (string, error) {
	// Implementation copied from substrate's go-relayer implementation
	entropySeed, err := bip39.NewEntropy(256)
	if err != nil {
		return "", err
	}
	mnemonic, err := bip39.NewMnemonic(entropySeed)
	if err != nil {
		return "", err
	}

	return mnemonic, nil
}

// CreateKey creates a test key in the "user" node (either the first fullnode or the first validator if no fullnodes).
// Implements Chain interface.
func (c *PolkadotChain) CreateKey(ctx context.Context, keyName string) error {
	_, err := c.keyring.Get(keyName)
	if err == nil {
		return fmt.Errorf("Key already exists: %s", keyName)
	}

	mnemonic, err := NewMnemonic()
	if err != nil {
		return err
	}

	kp, err := signature.KeyringPairFromSecret(mnemonic, Ss58Format)
	if err != nil {
		return fmt.Errorf("failed to create keypair: %w", err)
	}

	serializedKp, err := json.Marshal(kp)
	if err != nil {
		return err
	}
	err = c.keyring.Set(keyring.Item{
		Key:  keyName,
		Data: serializedKp,
	})

	return err
}

// RecoverKey recovers an existing user from a given mnemonic.
// Implements Chain interface.
func (c *PolkadotChain) RecoverKey(ctx context.Context, keyName, mnemonic string) error {
	_, err := c.keyring.Get(keyName)
	if err == nil {
		return fmt.Errorf("Key already exists: %s", keyName)
	}

	kp, err := signature.KeyringPairFromSecret(mnemonic, Ss58Format)
	if err != nil {
		return fmt.Errorf("failed to create keypair: %w", err)
	}

	serializedKp, err := json.Marshal(kp)
	if err != nil {
		return err
	}
	err = c.keyring.Set(keyring.Item{
		Key:  keyName,
		Data: serializedKp,
	})

	return err
}

// GetAddress fetches the address for a test key on the "user" node (either the first fullnode or the first validator if no fullnodes).
// Implements Chain interface.
func (c *PolkadotChain) GetAddress(ctx context.Context, keyName string) ([]byte, error) {
	krItem, err := c.keyring.Get(keyName)
	if err != nil {
		return nil, err
	}

	kp := signature.KeyringPair{}
	err = json.Unmarshal(krItem.Data, &kp)
	if err != nil {
		return nil, err
	}

	return []byte(kp.Address), nil
}

func (c *PolkadotChain) GetPublicKey(keyName string) ([]byte, error) {
	krItem, err := c.keyring.Get(keyName)
	if err != nil {
		return nil, err
	}

	kp := signature.KeyringPair{}
	err = json.Unmarshal(krItem.Data, &kp)
	if err != nil {
		return nil, err
	}

	return kp.PublicKey, nil
}

// BuildWallet will return a Polkadot wallet
// If mnemonic != "", it will restore using that mnemonic
// If mnemonic == "", it will create a new key
func (c *PolkadotChain) BuildWallet(ctx context.Context, keyName string, mnemonic string) (ibc.Wallet, error) {
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

// BuildRelayerWallet will return a Polkadot wallet populated with the mnemonic so that the wallet can
// be restored in the relayer node using the mnemonic. After it is built, that address is included in
// genesis with some funds.
func (c *PolkadotChain) BuildRelayerWallet(ctx context.Context, keyName string) (ibc.Wallet, error) {
	mnemonic, err := NewMnemonic()
	if err != nil {
		return nil, err
	}

	return c.BuildWallet(ctx, keyName, mnemonic)
}

// SendFunds sends funds to a wallet from a user account.
// Implements Chain interface.
func (c *PolkadotChain) SendFunds(ctx context.Context, keyName string, amount ibc.WalletAmount) error {
	// If denom == polkadot denom, it is a relay chain tx, else parachain tx
	if amount.Denom == c.cfg.Denom {
		// If keyName == faucet, also fund parachain's user until relay chain and parachains are their own chains
		if keyName == "faucet" {
			err := c.ParachainNodes[0][0].SendFunds(ctx, keyName, amount)
			if err != nil {
				return err
			}
		}
		return c.RelayChainNodes[0].SendFunds(ctx, keyName, amount)
	}

	return c.ParachainNodes[0][0].SendFunds(ctx, keyName, amount)
}

// SendIBCTransfer sends an IBC transfer returning a transaction or an error if the transfer failed.
// Implements Chain interface.
func (c *PolkadotChain) SendIBCTransfer(
	ctx context.Context,
	channelID string,
	keyName string,
	amount ibc.WalletAmount,
	options ibc.TransferOptions,
) (ibc.Tx, error) {
	return ibc.Tx{}, c.ParachainNodes[0][0].SendIbcFunds(ctx, channelID, keyName, amount, options)
}

// GetBalance fetches the current balance for a specific account address and denom.
// Implements Chain interface.
func (c *PolkadotChain) GetBalance(ctx context.Context, address string, denom string) (int64, error) {
	// If denom == polkadot denom, it is a relay chain query, else parachain query
	if denom == c.cfg.Denom {
		return c.RelayChainNodes[0].GetBalance(ctx, address, denom)
	}

	return c.ParachainNodes[0][0].GetBalance(ctx, address, denom)
}

// AccountInfo contains information of an account
type AccountInfo struct {
	Nonce       gstypes.U32
	Consumers   gstypes.U32
	Providers   gstypes.U32
	Sufficients gstypes.U32
	Data        struct {
		Free       gstypes.U128
		Reserved   gstypes.U128
		MiscFrozen gstypes.U128
		FreeFrozen gstypes.U128
	}
}

// GetGasFeesInNativeDenom gets the fees in native denom for an amount of spent gas.
// Implements Chain interface.
func (c *PolkadotChain) GetGasFeesInNativeDenom(gasPaid int64) int64 {
	panic("[GetGasFeesInNativeDenom] not implemented yet")
}

// Acknowledgements returns all acknowledgements in a block at height.
// Implements Chain interface.
func (c *PolkadotChain) Acknowledgements(ctx context.Context, height uint64) ([]ibc.PacketAcknowledgement, error) {
	panic("[Acknowledgements] not implemented yet")
}

// Timeouts returns all timeouts in a block at height.
// Implements Chain interface.
func (c *PolkadotChain) Timeouts(ctx context.Context, height uint64) ([]ibc.PacketTimeout, error) {
	panic("[Timeouts] not implemented yet")
}

// GetKeyringPair returns the keyring pair from the keyring using keyName
func (c *PolkadotChain) GetKeyringPair(keyName string) (signature.KeyringPair, error) {
	kp := signature.KeyringPair{}
	krItem, err := c.keyring.Get(keyName)
	if err != nil {
		return kp, err
	}

	err = json.Unmarshal(krItem.Data, &kp)
	if err != nil {
		return kp, err
	}

	return kp, nil
}

// FindTxs implements blockdb.BlockSaver (Not implemented yet for polkadot, but we don't want to exit)
func (c *PolkadotChain) FindTxs(ctx context.Context, height uint64) ([]blockdb.Tx, error) {
	return []blockdb.Tx{}, nil
}

// GetIbcBalance returns the Coins type of ibc coins in account
func (c *PolkadotChain) GetIbcBalance(ctx context.Context, address string, denom uint64) (sdktypes.Coin, error) {
	return c.ParachainNodes[0][0].GetIbcBalance(ctx, address, denom)
}

// MintFunds mints an asset for a user on parachain, keyName must be the owner of the asset
func (c *PolkadotChain) MintFunds(keyName string, amount ibc.WalletAmount) error {
	return c.ParachainNodes[0][0].MintFunds(keyName, amount)
}
