package polkadot

import (
	"context"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"

	schnorrkel "github.com/ChainSafe/go-schnorrkel/1"
	p2pCrypto "github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	"go.uber.org/zap"

	"github.com/decred/dcrd/dcrec/secp256k1/v2"
	"github.com/strangelove-ventures/ibctest/ibc"
	"github.com/strangelove-ventures/ibctest/internal/dockerutil"
)

type RelayChainNode struct {
	log      *zap.Logger
	TestName string

	Home  string
	Index int

	NetworkID    string
	containerID  string
	VolumeName   string
	DockerClient *client.Client
	Image        ibc.DockerImage

	Chain             ibc.Chain
	NodeKey           p2pCrypto.PrivKey
	AccountKey        *schnorrkel.MiniSecretKey
	StashKey          *schnorrkel.MiniSecretKey
	Ed25519PrivateKey p2pCrypto.PrivKey
	EcdsaPrivateKey   secp256k1.PrivateKey
}

type RelayChainNodes []*RelayChainNode

const (
	wsPort         = 27451
	rpcPort        = 27452
	prometheusPort = 27453
)

var exposedPorts = map[nat.Port]struct{}{
	nat.Port(fmt.Sprint(wsPort)):         {},
	nat.Port(fmt.Sprint(rpcPort)):        {},
	nat.Port(fmt.Sprint(prometheusPort)): {},
}

// Name of the test node container
func (p *RelayChainNode) Name() string {
	return fmt.Sprintf("relaychain-%d-%s-%s", p.Index, p.Chain.Config().ChainID, dockerutil.SanitizeContainerName(p.TestName))
}

// Hostname of the test container
func (p *RelayChainNode) HostName() string {
	return dockerutil.CondenseHostName(p.Name())
}

// Bind returns the home folder bind point for running the node
func (p *RelayChainNode) Bind() []string {
	return []string{fmt.Sprintf("%s:%s", p.VolumeName, p.NodeHome())}
}

func (p *RelayChainNode) NodeHome() string {
	return fmt.Sprintf("/home/.%s", p.Chain.Config().Name)
}

func (p *RelayChainNode) PeerID() (string, error) {
	id, err := peer.IDFromPrivateKey(p.NodeKey)
	if err != nil {
		return "", err
	}
	return peer.Encode(id), nil
}

func (p *RelayChainNode) GrandpaAddress() (string, error) {
	pubKey, err := p.Ed25519PrivateKey.GetPublic().Raw()
	if err != nil {
		return "", fmt.Errorf("error fetching pubkey bytes: %w", err)
	}
	return EncodeAddressSS58(pubKey)
}

func (p *RelayChainNode) AccountAddress() (string, error) {
	pubKey := make([]byte, 32)
	for i, mkByte := range p.AccountKey.Public().Encode() {
		pubKey[i] = mkByte
	}
	return EncodeAddressSS58(pubKey)
}

func (p *RelayChainNode) StashAddress() (string, error) {
	pubKey := make([]byte, 32)
	for i, mkByte := range p.StashKey.Public().Encode() {
		pubKey[i] = mkByte
	}
	return EncodeAddressSS58(pubKey)
}

func (p *RelayChainNode) EcdsaAddress() (string, error) {
	pubKey := []byte{}
	y := p.EcdsaPrivateKey.PublicKey.Y.Bytes()
	if y[len(y)-1]%2 == 0 {
		pubKey = append(pubKey, 0x02)
	} else {
		pubKey = append(pubKey, 0x03)
	}
	pubKey = append(pubKey, p.EcdsaPrivateKey.PublicKey.X.Bytes()...)
	return EncodeAddressSS58(pubKey)
}

func (p *RelayChainNode) MultiAddress() (string, error) {
	peerId, err := p.PeerID()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("/dns4/%s/tcp/%d/p2p/%s", p.HostName(), rpcPort, peerId), nil
}

func (c *RelayChainNode) logger() *zap.Logger {
	return c.log.With(
		zap.String("chain_id", c.Chain.Config().ChainID),
		zap.String("test", c.TestName),
	)
}

func (p *RelayChainNode) ChainSpecFilePathContainer() string {
	return fmt.Sprintf("%s.json", p.Chain.Config().ChainID)
}

func (p *RelayChainNode) RawChainSpecFilePathFull() string {
	return filepath.Join(p.NodeHome(), fmt.Sprintf("%s-raw.json", p.Chain.Config().ChainID))
}

func (p *RelayChainNode) RawChainSpecFilePathRelative() string {
	return fmt.Sprintf("%s-raw.json", p.Chain.Config().ChainID)
}

func (p *RelayChainNode) GenerateChainSpec(ctx context.Context) error {
	chainCfg := p.Chain.Config()
	cmd := []string{
		chainCfg.Bin,
		"build-spec",
		fmt.Sprintf("--chain=%s", chainCfg.ChainID),
		"--disable-default-bootnode",
	}
	stdout, _, err := p.Exec(ctx, cmd, nil)
	if err != nil {
		return err
	}
	fw := dockerutil.NewFileWriter(p.logger(), p.DockerClient, p.TestName)
	return fw.WriteFile(ctx, p.VolumeName, p.ChainSpecFilePathContainer(), stdout)
}

func (p *RelayChainNode) GenerateChainSpecRaw(ctx context.Context) error {
	chainCfg := p.Chain.Config()
	cmd := []string{
		chainCfg.Bin,
		"build-spec",
		fmt.Sprintf("--chain=%s.json", filepath.Join(p.NodeHome(), chainCfg.ChainID)),
		"--raw",
	}
	stdout, _, err := p.Exec(ctx, cmd, nil)
	if err != nil {
		return err
	}
	fw := dockerutil.NewFileWriter(p.logger(), p.DockerClient, p.TestName)
	return fw.WriteFile(ctx, p.VolumeName, p.RawChainSpecFilePathRelative(), stdout)
}

func (p *RelayChainNode) CreateNodeContainer(ctx context.Context) error {
	nodeKey, err := p.NodeKey.Raw()
	if err != nil {
		return fmt.Errorf("error getting ed25519 node key: %w", err)
	}
	multiAddress, err := p.MultiAddress()
	if err != nil {
		return err
	}
	chainCfg := p.Chain.Config()
	cmd := []string{
		chainCfg.Bin,
		fmt.Sprintf("--chain=%s", p.RawChainSpecFilePathFull()),
		fmt.Sprintf("--ws-port=%d", wsPort),
		fmt.Sprintf("--%s", IndexedName[p.Index]),
		fmt.Sprintf("--node-key=%s", hex.EncodeToString(nodeKey[0:32])),
		"--beefy",
		"--rpc-cors=all",
		"--unsafe-ws-external",
		"--unsafe-rpc-external",
		"--prometheus-external",
		fmt.Sprintf("--prometheus-port=%d", prometheusPort),
		fmt.Sprintf("--listen-addr=/ip4/0.0.0.0/tcp/%d", rpcPort),
		fmt.Sprintf("--public-addr=%s", multiAddress),
		"--base-path", p.NodeHome(),
	}
	fmt.Printf("{%s} -> '%s'\n", p.Name(), strings.Join(cmd, " "))

	cc, err := p.DockerClient.ContainerCreate(
		ctx,
		&container.Config{
			Image: p.Image.Ref(),

			Entrypoint: []string{},
			Cmd:        cmd,

			Hostname: p.HostName(),
			User:     dockerutil.GetRootUserString(),

			Labels: map[string]string{dockerutil.CleanupLabel: p.TestName},

			ExposedPorts: exposedPorts,
		},
		&container.HostConfig{
			Binds:           p.Bind(),
			PublishAllPorts: true,
			AutoRemove:      false,
			DNS:             []string{},
		},
		&network.NetworkingConfig{
			EndpointsConfig: map[string]*network.EndpointSettings{
				p.NetworkID: {},
			},
		},
		nil,
		p.Name(),
	)
	if err != nil {
		return err
	}
	p.containerID = cc.ID
	return nil
}

func (p *RelayChainNode) StopContainer(ctx context.Context) error {
	timeout := 30 * time.Second
	return p.DockerClient.ContainerStop(ctx, p.containerID, &timeout)
}

func (p *RelayChainNode) StartContainer(ctx context.Context) error {
	return dockerutil.StartContainer(ctx, p.DockerClient, p.containerID)
}

// Exec run a container for a specific job and block until the container exits
func (p *RelayChainNode) Exec(ctx context.Context, cmd []string, env []string) ([]byte, []byte, error) {
	job := dockerutil.NewImage(p.log, p.DockerClient, p.NetworkID, p.TestName, p.Image.Repository, p.Image.Version)
	opts := dockerutil.ContainerOptions{
		Binds: p.Bind(),
		Env:   env,
		User:  dockerutil.GetRootUserString(),
		Tail:  dockerutil.LogTailAll,
	}
	return job.Run(ctx, cmd, opts)
}
