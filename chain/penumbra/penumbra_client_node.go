package penumbra

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/strangelove-ventures/interchaintest/v7/ibc"
	"github.com/strangelove-ventures/interchaintest/v7/internal/dockerutil"
	"github.com/strangelove-ventures/interchaintest/v7/testutil"
	"go.uber.org/zap"
)

type PenumbraClientNode struct {
	log *zap.Logger

	Index        int
	VolumeName   string
	Chain        ibc.Chain
	TestName     string
	NetworkID    string
	DockerClient *client.Client
	Image        ibc.DockerImage

	containerLifecycle *dockerutil.ContainerLifecycle

	// Set during StartContainer.
	hostGRPCPort string
}

func NewClientNode(log *zap.Logger, chain *PenumbraChain, index int, testName string, image ibc.DockerImage) *PenumbraClientNode {
	return &PenumbraClientNode{
		log:      log,
		Index:    index,
		Chain:    chain,
		TestName: testName,
		Image:    image,
	}
}

const (
	pclientdPort = "8081/tcp"
)

var pclientdPorts = nat.PortSet{
	nat.Port(pclientdPort): {},
}

// Name of the test node container
func (p *PenumbraClientNode) Name() string {
	return fmt.Sprintf("pd-%d-%s-%s", p.Index, p.Chain.Config().ChainID, p.TestName)
}

// the hostname of the test node container
func (p *PenumbraClientNode) HostName() string {
	return dockerutil.CondenseHostName(p.Name())
}

// Bind returns the home folder bind point for running the node
func (p *PenumbraClientNode) Bind() []string {
	return []string{fmt.Sprintf("%s:%s", p.VolumeName, p.HomeDir())}
}

func (p *PenumbraClientNode) HomeDir() string {
	return "/home/heighliner"
}

func (p *PenumbraClientNode) GetAddress(ctx context.Context) ([]byte, error) {
	// TODO make grpc call to pclientd to get address
	panic("not yet implemented")
}

func (p *PenumbraClientNode) SendFunds(ctx context.Context, amount ibc.WalletAmount) error {
	// TODO make grpc call to pclientd to send transfer
	panic("not yet implemented")
}

func (p *PenumbraClientNode) SendIBCTransfer(
	ctx context.Context,
	channelID string,
	amount ibc.WalletAmount,
	options ibc.TransferOptions,
) (ibc.Tx, error) {
	// TODO make grpc call to pclientd to send ibc transfer
	panic("not yet implemented")
}

func (p *PenumbraClientNode) GetBalance(ctx context.Context, denom string) (int64, error) {
	// TODO make grpc call to pclientd to get balance
	panic("not yet implemented")
}

// WriteFile accepts file contents in a byte slice and writes the contents to
// the docker filesystem. relPath describes the location of the file in the
// docker volume relative to the home directory
func (p *PenumbraClientNode) WriteFile(ctx context.Context, content []byte, relPath string) error {
	fw := dockerutil.NewFileWriter(p.log, p.DockerClient, p.TestName)
	return fw.WriteFile(ctx, p.VolumeName, relPath, content)
}

// Initialize loads the view and spend keys into the pclientd config.
func (p *PenumbraClientNode) Initialize(ctx context.Context, spendKey string, fullViewingKey []byte) error {
	c := make(testutil.Toml)

	kmsConfig := make(testutil.Toml)
	kmsConfig["spend_key"] = spendKey
	c["kms_config"] = kmsConfig

	fvk := make(testutil.Toml)
	fvk["inner"] = base64.StdEncoding.EncodeToString(fullViewingKey)
	c["fvk"] = fvk

	buf := new(bytes.Buffer)
	if err := toml.NewEncoder(buf).Encode(c); err != nil {
		return err
	}

	return p.WriteFile(ctx, buf.Bytes(), "config.toml")
}

func (p *PenumbraClientNode) CreateNodeContainer(ctx context.Context, pdAddress, tendermintRPCAddress string) error {
	cmd := []string{
		"pclientd",
		"--home", p.HomeDir(),
		"--node", pdAddress,
		// "--tm-node", tendermintRPCAddress, // This flag does not exist but we need something like this.
		"start",
		"--host", "0.0.0.0",
		"--view-port", strings.Split(pclientdPort, "/")[0],
	}

	return p.containerLifecycle.CreateContainer(ctx, p.TestName, p.NetworkID, p.Image, pclientdPorts, p.Bind(), p.HostName(), cmd)
}

func (p *PenumbraClientNode) StopContainer(ctx context.Context) error {
	return p.containerLifecycle.StopContainer(ctx)
}

func (p *PenumbraClientNode) StartContainer(ctx context.Context) error {
	if err := p.containerLifecycle.StartContainer(ctx); err != nil {
		return err
	}

	hostPorts, err := p.containerLifecycle.GetHostPorts(ctx, pclientdPort)
	if err != nil {
		return err
	}

	p.hostGRPCPort = hostPorts[0]

	return nil
}

// Exec run a container for a specific job and block until the container exits
func (p *PenumbraClientNode) Exec(ctx context.Context, cmd []string, env []string) ([]byte, []byte, error) {
	job := dockerutil.NewImage(p.log, p.DockerClient, p.NetworkID, p.TestName, p.Image.Repository, p.Image.Version)
	opts := dockerutil.ContainerOptions{
		Binds: p.Bind(),
		Env:   env,
		User:  p.Image.UidGid,
	}
	res := job.Run(ctx, cmd, opts)
	return res.Stdout, res.Stderr, res.Err
}
