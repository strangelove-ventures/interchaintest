package avalanche

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/volume"
	dockerclient "github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/strangelove-ventures/interchaintest/v7/chain/avalanche/utils"
	"github.com/strangelove-ventures/interchaintest/v7/chain/avalanche/utils/crypto/secp256k1"
	"github.com/strangelove-ventures/interchaintest/v7/chain/avalanche/utils/ids"
	"github.com/strangelove-ventures/interchaintest/v7/ibc"
	"github.com/strangelove-ventures/interchaintest/v7/internal/dockerutil"
	"go.uber.org/zap"
)

var (
	RPCPort     = "9650/tcp"
	StakingPort = "9651/tcp"
)

type (
	AvalancheNode struct {
		chain *AvalancheChain

		logger             *zap.Logger
		containerLifecycle *dockerutil.ContainerLifecycle
		dockerClient       *dockerclient.Client
		image              ibc.DockerImage
		volume             types.Volume

		networkID string
		testName  string
		index     int
		options   AvalancheNodeOpts
	}

	AvalancheNodes []*AvalancheNode

	AvalancheNodeCredentials struct {
		PK      *secp256k1.PrivateKey
		ID      ids.NodeID
		TLSCert []byte
		TLSKey  []byte
	}

	AvalancheNodeSubnetOpts struct {
		Name string
		VM   []byte
	}

	AvalancheNodeOpts struct {
		PublicIP    string
		Subnets     []AvalancheNodeSubnetOpts
		Bootstrap   []*AvalancheNode
		Credentials AvalancheNodeCredentials
		ChainID     utils.ChainID
	}
)

func NewAvalancheNode(
	ctx context.Context,
	chain *AvalancheChain,
	networkID string,
	testName string,
	dockerClient *dockerclient.Client,
	image ibc.DockerImage,
	containerIdx int,
	log *zap.Logger,
	genesis Genesis,
	options *AvalancheNodeOpts,
) (*AvalancheNode, error) {
	node := &AvalancheNode{
		chain:        chain,
		index:        containerIdx,
		logger:       log,
		dockerClient: dockerClient,
		image:        image,
		networkID:    networkID,
		testName:     testName,
		options:      *options,
	}

	// avalanchego
	//   --plugin-dir=<Sets the directory for VM plugins. The default value is $HOME/.avalanchego/plugins>
	//   --vm-aliases-file=<Path to JSON file that defines aliases for Virtual Machine IDs. Defaults to ~/.avalanchego/configs/vms/aliases.json>
	//   --public-ip=<options.PublicIP>
	//   --http-port=<options.HttpPort>
	//   --staking-port=<options.StakingPort>
	//   --db-dir=db/node<idx>
	//   --network-id=<options.NetworkID>
	//   [--bootstrap-ips=<options.Bootstrap[0].Addr>]
	//   [--bootstrap-ids=<options.Bootstrap[0].ID>]
	//   --staking-tls-cert-file=$(pwd)/staking/local/staker<n>.crt
	//   --staking-tls-key-file=$(pwd)/staking/local/staker<n>.key
	// staking-tls-cert-file and staking-tls-key-file can be generated using NewCertAndKeyBytes
	//
	// links to genesis config https://github.com/ava-labs/avalanche-docs/blob/c136e8752af23db5214ff82c2153aac55542781b/docs/nodes/maintain/avalanchego-config-flags.md#genesis
	// https://github.com/ava-labs/avalanchego/blob/master/genesis/genesis_local.json
	//
	// Vm ID can be generated as zero-extended in a 32 byte array and encoded in CB58([32]byte(subnet.Name))

	name := node.Name()

	volume, err := dockerClient.VolumeCreate(ctx, volume.VolumeCreateBody{
		Name: name,
		Labels: map[string]string{
			dockerutil.CleanupLabel:   testName,
			dockerutil.NodeOwnerLabel: name,
		},
	})
	if err != nil {
		return nil, err
	}

	if err := dockerutil.SetVolumeOwner(ctx, dockerutil.VolumeOwnerOptions{
		Log:        log,
		Client:     dockerClient,
		VolumeName: name,
		ImageRef:   image.Ref(),
		TestName:   testName,
		UidGid:     image.UidGid,
	}); err != nil {
		return nil, fmt.Errorf("set volume owner: %w", err)
	}

	node.volume = volume

	fmt.Printf("creating container lifecycle, name: %s\n", name)

	node.containerLifecycle = dockerutil.NewContainerLifecycle(log, dockerClient, name)

	genesisBz, err := json.MarshalIndent(genesis, "", "  ")
	if err != nil {
		return nil, err
	}

	if err := node.WriteFile(ctx, genesisBz, "genesis.json"); err != nil {
		return nil, fmt.Errorf("failed to write genesis file: %w", err)
	}

	if err := node.WriteFile(ctx, options.Credentials.TLSCert, "tls.cert"); err != nil {
		return nil, fmt.Errorf("failed to write TLS certificate: %w", err)
	}

	if err := node.WriteFile(ctx, options.Credentials.TLSKey, "tls.key"); err != nil {
		return nil, fmt.Errorf("failed to write TLS key: %w", err)
	}

	return node, node.CreateContainer(ctx)
}

func (n *AvalancheNode) HomeDir() string {
	return "/home/heighliner/ava"
}

func (n *AvalancheNode) Bind() []string {
	return []string{
		fmt.Sprintf("%s:%s", n.volume.Name, n.HomeDir()),
	}
}

func (n *AvalancheNode) WriteFile(ctx context.Context, content []byte, relPath string) error {
	fw := dockerutil.NewFileWriter(n.logger, n.dockerClient, n.testName)
	return fw.WriteFile(ctx, n.volume.Name, relPath, content)
}

func (n *AvalancheNode) Exec(ctx context.Context, cmd []string, env []string) ([]byte, []byte, error) {
	job := dockerutil.NewImage(n.logger, n.dockerClient, n.networkID, n.testName, n.image.Repository, n.image.Version)
	opts := dockerutil.ContainerOptions{
		Binds: n.Bind(),
		Env:   env,
		User:  n.image.UidGid,
	}
	res := job.Run(ctx, cmd, opts)
	return res.Stdout, res.Stderr, res.Err
}

func (n *AvalancheNode) NodeId() string {
	return n.options.Credentials.ID.String()
}

func (n *AvalancheNode) Name() string {
	return fmt.Sprintf(
		"av-%s-%d",
		dockerutil.SanitizeContainerName(n.testName),
		n.index,
	)
}

func (n *AvalancheNode) HostName() string {
	return dockerutil.CondenseHostName(n.Name())
}

func (n *AvalancheNode) PublicStakingAddr(ctx context.Context) (string, error) {
	netinfo, err := n.dockerClient.NetworkInspect(ctx, n.networkID, types.NetworkInspectOptions{})
	if err != nil {
		return "", err
	}
	info, err := n.dockerClient.ContainerInspect(ctx, n.containerLifecycle.ContainerID())
	if err != nil {
		return "", err
	}

	return fmt.Sprintf(
		"%s:9651",
		info.NetworkSettings.Networks[netinfo.Name].IPAddress,
	), nil
}

func (n *AvalancheNode) StakingPort() string {
	info, err := n.dockerClient.ContainerInspect(context.Background(), n.containerLifecycle.ContainerID())
	if err != nil {
		panic(err)
	}
	return info.HostConfig.PortBindings[nat.Port(StakingPort)][0].HostPort
}

func (n *AvalancheNode) RPCPort() string {
	info, err := n.dockerClient.ContainerInspect(context.Background(), n.containerLifecycle.ContainerID())
	if err != nil {
		panic(err)
	}
	return info.HostConfig.PortBindings[nat.Port(RPCPort)][0].HostPort
}

func (n *AvalancheNode) GRPCPort() string {
	panic(errors.New("doesn't support grpc"))
}

func (n *AvalancheNode) CreateKey(ctx context.Context, keyName string) error {
	// ToDo: create key
	// https://github.com/ava-labs/avalanche-docs/blob/c136e8752af23db5214ff82c2153aac55542781b/docs/quickstart/fund-a-local-test-network.md
	// https://github.com/ava-labs/avalanche-docs/blob/c136e8752af23db5214ff82c2153aac55542781b/docs/quickstart/multisig-utxos-with-avalanchejs.md#setup-keychains-with-private-keys
	panic("ToDo: implement me")
}

func (n *AvalancheNode) RecoverKey(ctx context.Context, name, mnemonic string) error {
	// ToDo: recover key from mnemonic
	// https://github.com/ava-labs/avalanche-docs/blob/c136e8752af23db5214ff82c2153aac55542781b/docs/quickstart/fund-a-local-test-network.md
	// https://github.com/ava-labs/avalanche-docs/blob/c136e8752af23db5214ff82c2153aac55542781b/docs/quickstart/multisig-utxos-with-avalanchejs.md#setup-keychains-with-private-keys
	panic("ToDo: implement me")
}

func (n *AvalancheNode) GetAddress(ctx context.Context, keyName string) ([]byte, error) {
	// ToDo: get address for keyname
	// https://github.com/ava-labs/avalanche-docs/blob/c136e8752af23db5214ff82c2153aac55542781b/docs/quickstart/fund-a-local-test-network.md
	panic("ToDo: implement me")
}

func (n *AvalancheNode) SendFunds(ctx context.Context, keyName string, amount ibc.WalletAmount) error {
	// ToDo: send some amount to keyName from rootAddress
	// https://github.com/ava-labs/avalanche-docs/blob/c136e8752af23db5214ff82c2153aac55542781b/docs/quickstart/fund-a-local-test-network.md
	// https://github.com/ava-labs/avalanche-docs/blob/c136e8752af23db5214ff82c2153aac55542781b/docs/quickstart/cross-chain-transfers.md
	// IF allocated chain subnet config:
	//   - Blockchain Handlers: /ext/bc/[chainID]
	//   - VM Handlers: /ext/vm/[vmID]
	panic("ToDo: implement me")
}

func (n *AvalancheNode) SendIBCTransfer(ctx context.Context, channelID, keyName string, amount ibc.WalletAmount, options ibc.TransferOptions) (ibc.Tx, error) {
	return ibc.Tx{}, errors.New("not yet implemented")
}

func (n *AvalancheNode) Height(ctx context.Context) (uint64, error) {
	panic("ToDo: implement me")
}

func (n *AvalancheNode) GetBalance(ctx context.Context, address string, denom string) (int64, error) {
	if strings.HasPrefix(address, "X-") {
		// ToDo: call /ext/bc/X (method avm.getBalance)
		// https://github.com/ava-labs/avalanche-docs/blob/c136e8752af23db5214ff82c2153aac55542781b/docs/quickstart/fund-a-local-test-network.md#check-x-chain-balance
		panic("ToDo: implement me")
	} else if strings.HasPrefix(address, "P-") {
		// ToDo: call /ext/bc/P (method platform.getBalance)
		// https://github.com/ava-labs/avalanche-docs/blob/c136e8752af23db5214ff82c2153aac55542781b/docs/quickstart/fund-a-local-test-network.md#check-p-chain-balance
		panic("ToDo: implement me")
	} else if strings.HasPrefix(address, "0x") {
		// ToDo: call /ext/bc/C/rpc (method eth_getBalance)
		// https://github.com/ava-labs/avalanche-docs/blob/c136e8752af23db5214ff82c2153aac55542781b/docs/quickstart/fund-a-local-test-network.md#check-the-c-chain-balance
		panic("ToDo: implement me")
	}
	// if allocated subnet, we must call /ext/bc/[chainID]
	return 0, fmt.Errorf("address should be have prefix X, P, 0x. current address: %s", address)
}

func (n *AvalancheNode) IP() string {
	return n.options.PublicIP
}

func (n *AvalancheNode) CreateContainer(ctx context.Context) error {
	netinfo, err := n.dockerClient.NetworkInspect(ctx, n.networkID, types.NetworkInspectOptions{})
	if err != nil {
		return fmt.Errorf("failed to inspect network: %w", err)
	}

	bootstrapIps, bootstrapIds := "", ""
	if len(n.options.Bootstrap) > 0 {
		for i := range n.options.Bootstrap {
			sep := ""
			if i > 0 {
				sep = ","
			}
			stakingAddr, err := n.options.Bootstrap[i].PublicStakingAddr(ctx)
			if err != nil {
				return fmt.Errorf("failed to get public staking address for index %d: %w", i, err)
			}
			bootstrapIps += sep + stakingAddr
			bootstrapIds += sep + n.options.Bootstrap[i].NodeId()
		}
	}

	cmd := []string{
		n.chain.cfg.Bin,
		"--http-host", "0.0.0.0",
		"--data-dir", n.HomeDir(),
		"--public-ip", n.options.PublicIP,
		"--network-id", n.options.ChainID.String(),
		"--genesis", filepath.Join(n.HomeDir(), "genesis.json"),
		"--staking-tls-cert-file", filepath.Join(n.HomeDir(), "tls.cert"),
		"--staking-tls-key-file", filepath.Join(n.HomeDir(), "tls.key"),
	}
	if bootstrapIps != "" && bootstrapIds != "" {
		cmd = append(
			cmd,
			"--bootstrap-ips", bootstrapIps,
			"--bootstrap-ids", bootstrapIds,
		)
	}
	port1, _ := nat.NewPort("tcp", "9650")
	port2, _ := nat.NewPort("tcp", "9651")
	ports := nat.PortSet{
		port1: {},
		port2: {},
	}
	return n.containerLifecycle.CreateContainerInNetwork(
		ctx,
		n.testName,
		n.networkID,
		n.image,
		ports,
		n.Bind(),
		&network.NetworkingConfig{
			EndpointsConfig: map[string](*network.EndpointSettings){
				netinfo.Name: &network.EndpointSettings{
					NetworkID: netinfo.ID,
					IPAddress: n.options.PublicIP,
				},
			},
		},
		n.HostName(),
		cmd,
	)
}

func (n *AvalancheNode) StartContainer(ctx context.Context, testName string, additionalGenesisWallets []ibc.WalletAmount) error {
	return n.containerLifecycle.StartContainer(ctx)
}
