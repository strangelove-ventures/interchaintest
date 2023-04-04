package avalanche

import (
	"context"
	"errors"

	dockerclient "github.com/docker/docker/client"
	"github.com/strangelove-ventures/interchaintest/v7/ibc"
)

type (
	AvalancheNode struct {
	}
	AvalancheNodes []AvalancheNode
)

func NewAvalancheNode(
	ctx context.Context,
	idx int,
	httpPort int,
	stakingPort int,
	dockerClient *dockerclient.Client,
	networkID string,
	testName string,
	image ibc.DockerImage,
	prevNode *AvalancheNode,
) (*AvalancheNode, error) {
	// ToDo: implement me
	// For first node
	// avalanchego
	// 		--public-ip=127.0.0.1
	//    --http-port=9650
	//    --staking-port=9651
	//    --db-dir=db/node1
	//    --network-id=local
	//    --staking-tls-cert-file=$(pwd)/staking/local/staker1.crt
	//    --staking-tls-key-file=$(pwd)/staking/local/staker1.key
	// For second node
	// avalanchego
	//    --public-ip=127.0.0.1
	//		--http-port=9652
	//		--staking-port=9653
	//		--db-dir=db/node2
	//		--network-id=local
	//		--bootstrap-ips=127.0.0.1:9651
	//		--bootstrap-ids=NodeID-7Xhw2mDxuDS44j42TCB6U5579esbSt3Lg
	//		--staking-tls-cert-file=$(pwd)/staking/local/staker2.crt
	//		--staking-tls-key-file=$(pwd)/staking/local/staker2.key
	// ....
	// For N node
	// avalanchego
	//		--public-ip=127.0.0.1
	//		--http-port=<httpPort>
	//    --staking-port=<stakingPort>
	//		--db-dir=db/node<idx>
	//    --network-id=local
	//    --bootstrap-ips=127.0.0.1:<prevNode.StackingPort()>
	//    --bootstrap-ids=<prevNode.NodeId()>
	//    --staking-tls-cert-file=$(pwd)/staking/local/staker<n>.crt
	//    --staking-tls-key-file=$(pwd)/staking/local/staker<n>.key
	//
	// Example key generation:
	// import "github.com/ava-labs/avalanchego/staking"
	// certBytes, keyBytes, err := staking.NewCertAndKeyBytes()
	//
	// openssl req -x509 -newkey rsa:4096 -keyout staker.key -out staker.crt -days 36500 -nodes -subj '/CN=localhost' -set_serial 0
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
	panic("ToDo: implement me")
}

func (n AvalancheNode) RecoverKey(ctx context.Context, name, mnemonic string) error {
	// ToDo: recover key from mnemonic
	panic("ToDo: implement me")
}

func (n AvalancheNode) GetAddress(ctx context.Context, keyName string) ([]byte, error) {
	// ToDo: get address for keyname
	panic("ToDo: implement me")
}

func (n AvalancheNode) SendFunds(ctx context.Context, keyName string, amount ibc.WalletAmount) error {
	// ToDo: send some amount to keyName from rootAddress
	panic("ToDo: implement me")
}

func (n AvalancheNode) SendIBCTransfer(ctx context.Context, channelID, keyName string, amount ibc.WalletAmount, options ibc.TransferOptions) (ibc.Tx, error) {
	panic("ToDo: implement me")
}

func (c AvalancheNode) Height(ctx context.Context) (uint64, error) {
	panic("ToDo: implement me")
}

func (c AvalancheNode) GetBalance(ctx context.Context, address string, denom string) (int64, error) {
	// ToDo: get balance for given address
	panic("ToDo: implement me")
}
