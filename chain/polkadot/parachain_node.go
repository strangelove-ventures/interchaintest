package polkadot

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/avast/retry-go/v4"
	gsrpc "github.com/centrifuge/go-substrate-rpc-client/v4"
	"github.com/docker/docker/client"
	p2pcrypto "github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/strangelove-ventures/ibctest/v5/ibc"
	"github.com/strangelove-ventures/ibctest/v5/internal/dockerutil"
	"go.uber.org/zap"
	"path/filepath"
	"strings"
)

// ParachainNode defines the properties required for running a polkadot parachain node.
type ParachainNode struct {
	log      *zap.Logger
	TestName string
	Index    int

	NetworkID          string
	containerLifecycle *dockerutil.ContainerLifecycle
	VolumeName         string
	DockerClient       *client.Client
	Image              ibc.DockerImage

	Chain           ibc.Chain
	Bin             string
	NodeKey         p2pcrypto.PrivKey
	ChainID         string
	Flags           []string
	RelayChainFlags []string

	api         *gsrpc.SubstrateAPI
	hostWsPort  string
	hostRpcPort string
}

type ParachainNodes []*ParachainNode

// Name returns the name of the test node container.
func (pn *ParachainNode) Name() string {
	return fmt.Sprintf("%s-%d-%s-%s", pn.Bin, pn.Index, pn.ChainID, dockerutil.SanitizeContainerName(pn.TestName))
}

// HostName returns the docker hostname of the test container.
func (pn *ParachainNode) HostName() string {
	return dockerutil.CondenseHostName(pn.Name())
}

// Bind returns the home folder bind point for running the node.
func (pn *ParachainNode) Bind() []string {
	return []string{fmt.Sprintf("%s:%s", pn.VolumeName, pn.NodeHome())}
}

// NodeHome returns the working directory within the docker image,
// the path where the docker volume is mounted.
func (pn *ParachainNode) NodeHome() string {
	return fmt.Sprintf("/home/.%s", pn.Chain.Config().Name)
}

// RawChainSpecFilePathFull returns the full path to the raw chain spec file
// within the container.
func (pn *ParachainNode) RawChainSpecFilePathFull() string {
	return filepath.Join(pn.NodeHome(), fmt.Sprintf("%s-raw.json", pn.Chain.Config().ChainID))
}

// RawChainSpecFilePathRelative returns the relative path to the raw chain spec file
// within the container.
func (pn *ParachainNode) RawChainSpecFilePathRelative() string {
	return fmt.Sprintf("%s-raw.json", pn.Chain.Config().ChainID)
}

// PeerID returns the public key of the node key for p2p.
func (pn *ParachainNode) PeerID() (string, error) {
	id, err := peer.IDFromPrivateKey(pn.NodeKey)
	if err != nil {
		return "", err
	}
	return peer.Encode(id), nil
}

// MultiAddress returns the p2p multiaddr of the node.
func (pn *ParachainNode) MultiAddress() (string, error) {
	peerId, err := pn.PeerID()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("/dns4/%s/tcp/%s/p2p/%s", pn.HostName(), strings.Split(rpcPort, "/")[0], peerId), nil
}

type GetParachainIDResponse struct {
	ParachainID int `json:"para_id"`
}

// ParachainID retrieves the node parachain ID.
func (pn *ParachainNode) ParachainID(ctx context.Context) (int, error) {
	cmd := []string{
		pn.Bin,
		"build-spec",
		fmt.Sprintf("--chain=%s", pn.ChainID),
	}
	res := pn.Exec(ctx, cmd, nil)
	if res.Err != nil {
		return -1, res.Err
	}
	out := GetParachainIDResponse{}
	if err := json.Unmarshal([]byte(res.Stdout), &out); err != nil {
		return -1, err
	}
	return out.ParachainID, nil
}

// ExportGenesisWasm exports the genesis wasm json for the configured chain ID.
func (pn *ParachainNode) ExportGenesisWasm(ctx context.Context) (string, error) {
	cmd := []string{
		pn.Bin,
		"export-genesis-wasm",
		fmt.Sprintf("--chain=%s", pn.ChainID),
	}
	res := pn.Exec(ctx, cmd, nil)
	if res.Err != nil {
		return "", res.Err
	}
	return string(res.Stdout), nil
}

// ExportGenesisState exports the genesis state json for the configured chain ID.
func (pn *ParachainNode) ExportGenesisState(ctx context.Context) (string, error) {
	cmd := []string{
		pn.Bin,
		"export-genesis-state",
		fmt.Sprintf("--chain=%s", pn.ChainID),
	}
	res := pn.Exec(ctx, cmd, nil)
	if res.Err != nil {
		return "", res.Err
	}
	return string(res.Stdout), nil
}

func (pn *ParachainNode) logger() *zap.Logger {
	return pn.log.With(
		zap.String("chain_id", pn.ChainID),
		zap.String("test", pn.TestName),
	)
}

// CreateNodeContainer assembles a parachain node docker container ready to launch.
func (pn *ParachainNode) CreateNodeContainer(ctx context.Context) error {
	nodeKey, err := pn.NodeKey.Raw()
	if err != nil {
		return fmt.Errorf("error getting ed25519 node key: %w", err)
	}
	multiAddress, err := pn.MultiAddress()
	if err != nil {
		return err
	}
	cmd := []string{
		pn.Bin,
		fmt.Sprintf("--ws-port=%s", strings.Split(wsPort, "/")[0]),
		"--collator",
		fmt.Sprintf("--node-key=%s", hex.EncodeToString(nodeKey[0:32])),
		fmt.Sprintf("--%s", IndexedName[pn.Index]),
		"--unsafe-ws-external",
		"--unsafe-rpc-external",
		"--prometheus-external",
		"--rpc-cors=all",
		fmt.Sprintf("--prometheus-port=%s", strings.Split(prometheusPort, "/")[0]),
		fmt.Sprintf("--listen-addr=/ip4/0.0.0.0/tcp/%s", strings.Split(rpcPort, "/")[0]),
		fmt.Sprintf("--public-addr=%s", multiAddress),
		"--base-path", pn.NodeHome(),
		fmt.Sprintf("--chain=%s", pn.ChainID),
	}
	cmd = append(cmd, pn.Flags...)
	cmd = append(cmd, "--", fmt.Sprintf("--chain=%s", pn.RawChainSpecFilePathFull()))
	cmd = append(cmd, pn.RelayChainFlags...)
	return pn.containerLifecycle.CreateContainer(ctx, pn.TestName, pn.NetworkID, pn.Image, exposedPorts, pn.Bind(), pn.HostName(), cmd)
}

// StopContainer stops the relay chain node container, waiting at most 30 seconds.
func (pn *ParachainNode) StopContainer(ctx context.Context) error {
	return pn.containerLifecycle.StopContainer(ctx)
}

// StartContainer starts the container after it is built by CreateNodeContainer.
func (pn *ParachainNode) StartContainer(ctx context.Context) error {
	if err := pn.containerLifecycle.StartContainer(ctx); err != nil {
		return err
	}

	hostPorts, err := pn.containerLifecycle.GetHostPorts(ctx, wsPort, rpcPort)
	if err != nil {
		return err
	}

	// Set the host ports once since they will not change after the container has started.
	pn.hostWsPort, pn.hostRpcPort = hostPorts[0], hostPorts[1]

	explorerUrl := fmt.Sprintf("\033[4;34mhttps://polkadot.js.org/apps?rpc=ws://%s#/explorer\033[0m",
		strings.Replace(pn.hostWsPort, "localhost", "127.0.0.1", 1))
	pn.log.Info(explorerUrl, zap.String("container", pn.Name()))
	var api *gsrpc.SubstrateAPI
	if err = retry.Do(func() error {
		var err error
		api, err = gsrpc.NewSubstrateAPI("ws://" + pn.hostWsPort)
		return err
	}, retry.Context(ctx), RtyAtt, RtyDel, RtyErr); err != nil {
		return err
	}
	pn.api = api
	return nil
}

// Exec run a container for a specific job and block until the container exits.
func (pn *ParachainNode) Exec(ctx context.Context, cmd []string, env []string) dockerutil.ContainerExecResult {
	job := dockerutil.NewImage(pn.log, pn.DockerClient, pn.NetworkID, pn.TestName, pn.Image.Repository, pn.Image.Version)
	opts := dockerutil.ContainerOptions{
		Binds: pn.Bind(),
		Env:   env,
		User:  dockerutil.GetRootUserString(),
	}
	return job.Run(ctx, cmd, opts)
}
