package avalanche

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/docker/docker/api/types"
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

type (
	AvalancheNodeCredentials struct {
		PK      *secp256k1.PrivateKey
		ID      ids.NodeID
		TLSCert []byte
		TLSKey  []byte
	}
	AvalancheNodeBootstrapOpts struct {
		ID   string
		Addr string
	}
	AvalancheNodeSubnetOpts struct {
		Name string
		VM   []byte
	}
	AvalancheNodeOpts struct {
		PublicIP    string
		HttpPort    string
		StakingPort string
		Subnets     []AvalancheNodeSubnetOpts
		Bootstrap   []AvalancheNodeBootstrapOpts
		Credentials AvalancheNodeCredentials
		Genesis     Genesis
		ChainID     utils.ChainID
	}
	AvalancheNode struct {
		name               string
		containerLifecycle *dockerutil.ContainerLifecycle
		logger             *zap.Logger
		dockerClient       *dockerclient.Client
		image              ibc.DockerImage
		volume             types.Volume

		networkID   string
		testName    string
		containerID int
		options     AvalancheNodeOpts
	}
	AvalancheNodes []AvalancheNode
)

func NewAvalancheNode(
	ctx context.Context,
	networkID string,
	testName string,
	dockerClient *dockerclient.Client,
	image ibc.DockerImage,
	containerID int,
	log *zap.Logger,
	options *AvalancheNodeOpts,
) (*AvalancheNode, error) {
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
	name := fmt.Sprintf(
		"av-%s-%d",
		testName,
		containerID,
	)

	containerLifecycle := dockerutil.NewContainerLifecycle(log, dockerClient, name)

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

	rawGenesis, err := json.MarshalIndent(options.Genesis, "", "  ")
	if err != nil {
		return nil, err
	}

	node := AvalancheNode{
		name:               name,
		containerLifecycle: containerLifecycle,
		logger:             log,
		dockerClient:       dockerClient,
		image:              image,
		volume:             volume,
		networkID:          networkID,
		containerID:        containerID,
		options:            *options,
	}

	if err := node.WriteFile(ctx, rawGenesis, "genesis.json"); err != nil {
		return nil, err
	}

	if err := node.WriteFile(ctx, options.Credentials.TLSCert, "tls.cert"); err != nil {
		return nil, err
	}

	if err := node.WriteFile(ctx, options.Credentials.TLSKey, "tls.key"); err != nil {
		return nil, err
	}

	ports := nat.PortSet{nat.Port(fmt.Sprintf("%s/tcp", options.StakingPort)): {}}
	cmd := []string{
		"/bin/avalanchego",
		"--http-port", options.HttpPort,
		"--staking-port", options.StakingPort,
		"--network-id", fmt.Sprintf("%s-%d", options.ChainID.Name, options.ChainID.Number),
		"--genesis", fmt.Sprintf("%s/config/genesis.json", node.HomeDir()),
		"--staking-tls-cert-file", fmt.Sprintf("%s/config/tls.cert", node.HomeDir()),
		"--staking-tls-key-file", fmt.Sprintf("%s/config/tls.key", node.HomeDir()),
	}
	if len(options.Bootstrap) > 0 {
		bootstapIps := make([]string, len(options.Bootstrap))
		bootstapIds := make([]string, len(options.Bootstrap))
		for i := range options.Bootstrap {
			bootstapIps[i] = options.Bootstrap[i].Addr
			bootstapIds[i] = options.Bootstrap[i].ID
		}
		cmd = append(
			cmd,
			"--bootstrap-ips", `"`+strings.Join(bootstapIps, ",")+`"`,
			"--bootstrap-ids", `"`+strings.Join(bootstapIds, ",")+`"`,
		)
	}

	if err := node.containerLifecycle.CreateContainer(ctx, testName, networkID, image, ports, node.Bind(), node.HostName(), cmd); err != nil {
		return nil, err
	}

	return &node, nil
}

func (n *AvalancheNode) HomeDir() string {
	return "/home/heighliner"
}

func (n *AvalancheNode) Bind() []string {
	return []string{
		fmt.Sprintf("%s:%s", n.volume.Name, n.HomeDir()+"/config"),
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

func (n *AvalancheNode) HostName() string {
	return n.name
}

func (n *AvalancheNode) StackingPort() string {
	return n.options.StakingPort
}

func (n *AvalancheNode) RPCPort() string {
	return n.options.HttpPort
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

func (n AvalancheNode) RecoverKey(ctx context.Context, name, mnemonic string) error {
	// ToDo: recover key from mnemonic
	// https://github.com/ava-labs/avalanche-docs/blob/c136e8752af23db5214ff82c2153aac55542781b/docs/quickstart/fund-a-local-test-network.md
	// https://github.com/ava-labs/avalanche-docs/blob/c136e8752af23db5214ff82c2153aac55542781b/docs/quickstart/multisig-utxos-with-avalanchejs.md#setup-keychains-with-private-keys
	panic("ToDo: implement me")
}

func (n AvalancheNode) GetAddress(ctx context.Context, keyName string) ([]byte, error) {
	// ToDo: get address for keyname
	// https://github.com/ava-labs/avalanche-docs/blob/c136e8752af23db5214ff82c2153aac55542781b/docs/quickstart/fund-a-local-test-network.md
	panic("ToDo: implement me")
}

func (n AvalancheNode) SendFunds(ctx context.Context, keyName string, amount ibc.WalletAmount) error {
	// ToDo: send some amount to keyName from rootAddress
	// https://github.com/ava-labs/avalanche-docs/blob/c136e8752af23db5214ff82c2153aac55542781b/docs/quickstart/fund-a-local-test-network.md
	// https://github.com/ava-labs/avalanche-docs/blob/c136e8752af23db5214ff82c2153aac55542781b/docs/quickstart/cross-chain-transfers.md
	// IF allocated chain subnet config:
	//   - Blockchain Handlers: /ext/bc/[chainID]
	//   - VM Handlers: /ext/vm/[vmID]
	panic("ToDo: implement me")
}

func (n AvalancheNode) SendIBCTransfer(ctx context.Context, channelID, keyName string, amount ibc.WalletAmount, options ibc.TransferOptions) (ibc.Tx, error) {
	return ibc.Tx{}, errors.New("not yet implemented")
}

func (c AvalancheNode) Height(ctx context.Context) (uint64, error) {
	panic("ToDo: implement me")
}

func (c AvalancheNode) GetBalance(ctx context.Context, address string, denom string) (int64, error) {
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

func (c AvalancheNode) StartContainer(ctx context.Context) error {
	return nil
}
