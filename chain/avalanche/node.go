package avalanche

import (
	"context"
	"errors"
	"fmt"
	"strings"

	dockerclient "github.com/docker/docker/client"
	"github.com/strangelove-ventures/interchaintest/v7/ibc"
)

type (
	AvalancheNodeBootstrapOpts struct {
		ID   string
		Addr string
	}
	AvalancheNodeOpts struct {
		NetworkID   string
		PublicIP    string
		HttpPort    string
		StakingPort string
		Bootstrap   []AvalancheNodeBootstrapOpts
	}
	AvalancheNode struct {
	}
	AvalancheNodes []AvalancheNode
)

func NewAvalancheNode(
	ctx context.Context,
	testName string,
	dockerClient *dockerclient.Client,
	image ibc.DockerImage,
	containerID int,
	options *AvalancheNodeOpts,
) (*AvalancheNode, error) {
	// avalanchego
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
	return nil, nil
}

func (n *AvalancheNode) Exec(ctx context.Context, cmd []string, env []string) (stdout, stderr []byte, err error) {
	// ToDo: exec some command to node
	panic("ToDo: implement me")
}

func (n *AvalancheNode) NodeId() string {
	// Todo: return nodeId, example "NodeID-7Xhw2mDxuDS44j42TCB6U5579esbSt3Lg"
	panic("ToDo: implement me")
}

func (n *AvalancheNode) HostName() string {
	// ToDo: docker hostname
	panic("ToDo: implement me")
}

func (n *AvalancheNode) StackingPort() string {
	// ToDo: return --staking-port
	panic("ToDo: implement me")
}

func (n *AvalancheNode) RPCPort() string {
	// ToDo: return --http-port
	panic("ToDo: implement me")
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
	return 0, fmt.Errorf("address should be have prefix X, P, 0x. current address: %s", address)
}
