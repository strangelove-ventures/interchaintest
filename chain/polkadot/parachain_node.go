package polkadot

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	p2pCrypto "github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/strangelove-ventures/ibctest/ibc"
	"github.com/strangelove-ventures/ibctest/internal/dockerutil"
	"go.uber.org/zap"
)

type ParachainNode struct {
	log      *zap.Logger
	TestName string

	Home  string
	Index int

	NetworkID    string
	containerID  string
	VolumeName   string
	DockerClient *client.Client
	Image        ibc.DockerImage

	Chain           ibc.Chain
	Bin             string
	NodeKey         p2pCrypto.PrivKey
	ChainID         string
	Flags           []string
	RelayChainFlags []string
}

type ParachainNodes []*ParachainNode

// Name of the test node container
func (pn *ParachainNode) Name() string {
	return fmt.Sprintf("%s-%d-%s-%s", pn.Bin, pn.Index, pn.ChainID, dockerutil.SanitizeContainerName(pn.TestName))
}

// Hostname of the test container
func (pn *ParachainNode) HostName() string {
	return dockerutil.CondenseHostName(pn.Name())
}

// Bind returns the home folder bind point for running the node
func (pn *ParachainNode) Bind() []string {
	return []string{fmt.Sprintf("%s:%s", pn.VolumeName, pn.NodeHome())}
}

func (pn *ParachainNode) NodeHome() string {
	return fmt.Sprintf("/home/.%s", pn.Chain.Config().Name)
}

func (pn *ParachainNode) RawChainSpecFilePathFull() string {
	return filepath.Join(pn.NodeHome(), fmt.Sprintf("%s-raw.json", pn.Chain.Config().ChainID))
}

func (pn *ParachainNode) RawChainSpecFilePathRelative() string {
	return fmt.Sprintf("%s-raw.json", pn.Chain.Config().ChainID)
}

func (pn *ParachainNode) PeerID() (string, error) {
	id, err := peer.IDFromPrivateKey(pn.NodeKey)
	if err != nil {
		return "", err
	}
	return peer.Encode(id), nil
}

func (pn *ParachainNode) MultiAddress() (string, error) {
	peerId, err := pn.PeerID()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("/dns4/%s/tcp/%d/p2p/%s", pn.HostName(), rpcPort, peerId), nil
}

type GetParachainIDResponse struct {
	ParachainID int `json:"para_id"`
}

func (pn *ParachainNode) ParachainID(ctx context.Context) (int, error) {
	cmd := []string{
		pn.Bin,
		"build-spec",
		fmt.Sprintf("--chain=%s", pn.ChainID),
	}
	stdout, _, err := pn.Exec(ctx, cmd, nil, dockerutil.LogTailAll)
	if err != nil {
		return -1, err
	}
	res := GetParachainIDResponse{}
	if err := json.Unmarshal([]byte(stdout), &res); err != nil {
		return -1, err
	}
	return res.ParachainID, nil
}

func (pn *ParachainNode) ExportGenesisWasm(ctx context.Context) (string, error) {
	cmd := []string{
		pn.Bin,
		"export-genesis-wasm",
		fmt.Sprintf("--chain=%s", pn.ChainID),
	}
	stdout, _, err := pn.Exec(ctx, cmd, nil, dockerutil.LogTailAll)
	if err != nil {
		return "", err
	}
	return string(stdout), nil
}

func (pn *ParachainNode) ExportGenesisState(ctx context.Context, parachainID int) (string, error) {
	cmd := []string{
		pn.Bin,
		"export-genesis-state",
		fmt.Sprintf("--chain=%s", pn.ChainID),
	}
	stdout, _, err := pn.Exec(ctx, cmd, nil, dockerutil.LogTailAll)
	if err != nil {
		return "", err
	}
	return string(stdout), nil
}

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
		fmt.Sprintf("--ws-port=%d", wsPort),
		"--collator",
		fmt.Sprintf("--node-key=%s", hex.EncodeToString(nodeKey[0:32])),
		fmt.Sprintf("--%s", IndexedName[pn.Index]),
		"--unsafe-ws-external",
		"--unsafe-rpc-external",
		"--prometheus-external",
		"--rpc-cors=all",
		fmt.Sprintf("--prometheus-port=%d", prometheusPort),
		fmt.Sprintf("--listen-addr=/ip4/0.0.0.0/tcp/%d", rpcPort),
		fmt.Sprintf("--public-addr=%s", multiAddress),
		"--base-path", pn.NodeHome(),
		fmt.Sprintf("--chain=%s", pn.ChainID),
	}
	cmd = append(cmd, pn.Flags...)
	cmd = append(cmd, "--", fmt.Sprintf("--chain=%s", pn.RawChainSpecFilePathFull()))
	cmd = append(cmd, pn.RelayChainFlags...)
	fmt.Printf("{%s} -> '%s'\n", pn.Name(), strings.Join(cmd, " "))

	cc, err := pn.DockerClient.ContainerCreate(
		ctx,
		&container.Config{
			Image: pn.Image.Ref(),

			Entrypoint: []string{},
			Cmd:        cmd,

			Hostname: pn.HostName(),
			User:     dockerutil.GetRootUserString(),

			Labels: map[string]string{dockerutil.CleanupLabel: pn.TestName},

			ExposedPorts: exposedPorts,
		},
		&container.HostConfig{
			Binds:           pn.Bind(),
			PublishAllPorts: true,
			AutoRemove:      false,
			DNS:             []string{},
		},
		&network.NetworkingConfig{
			EndpointsConfig: map[string]*network.EndpointSettings{
				pn.NetworkID: {},
			},
		},
		nil,
		pn.Name(),
	)
	if err != nil {
		return err
	}
	pn.containerID = cc.ID
	return nil
}

func (pn *ParachainNode) StopContainer(ctx context.Context) error {
	timeout := 30 * time.Second
	return pn.DockerClient.ContainerStop(ctx, pn.containerID, &timeout)
}

func (pn *ParachainNode) StartContainer(ctx context.Context) error {
	return dockerutil.StartContainer(ctx, pn.DockerClient, pn.containerID)
}

// Exec run a container for a specific job and block until the container exits
func (pn *ParachainNode) Exec(ctx context.Context, cmd []string, env []string, tail uint64) ([]byte, []byte, error) {
	job := dockerutil.NewImage(pn.log, pn.DockerClient, pn.NetworkID, pn.TestName, pn.Image.Repository, pn.Image.Version)
	opts := dockerutil.ContainerOptions{
		Binds: pn.Bind(),
		Env:   env,
		User:  dockerutil.GetRootUserString(),
		Tail:  tail,
	}
	return job.Run(ctx, cmd, opts)
}
