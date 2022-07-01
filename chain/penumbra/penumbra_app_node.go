package penumbra

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/strangelove-ventures/ibctest/ibc"
	"github.com/strangelove-ventures/ibctest/internal/dockerutil"
	"go.uber.org/zap"
)

type PenumbraAppNode struct {
	log *zap.Logger

	Index        int
	Home         string
	Chain        ibc.Chain
	TestName     string
	NetworkID    string
	DockerClient *client.Client
	Image        ibc.DockerImage

	containerID string

	// Set during StartContainer.
	hostRPCPort  string
	hostGRPCPort string
}

const (
	valKey         = "validator"
	rpcPort        = "26657/tcp"
	tendermintPort = "26658/tcp"
	grpcPort       = "9090/tcp"
)

var exposedPorts = nat.PortSet{
	nat.Port(tendermintPort): {},
}

// Name of the test node container
func (p *PenumbraAppNode) Name() string {
	return fmt.Sprintf("pd-%d-%s-%s", p.Index, p.Chain.Config().ChainID, p.TestName)
}

// the hostname of the test node container
func (p *PenumbraAppNode) HostName() string {
	return dockerutil.CondenseHostName(p.Name())
}

// Dir is the directory where the test node files are stored
func (p *PenumbraAppNode) Dir() string {
	return fmt.Sprintf("%s/%s/", p.Home, p.Name())
}

// MkDir creates the directory for the testnode
func (p *PenumbraAppNode) MkDir() {
	if err := os.MkdirAll(p.Dir(), 0755); err != nil {
		panic(err)
	}
}

// Bind returns the home folder bind point for running the node
func (p *PenumbraAppNode) Bind() []string {
	return []string{fmt.Sprintf("%s:%s", p.Dir(), p.NodeHome())}
}

func (p *PenumbraAppNode) NodeHome() string {
	return fmt.Sprintf("/root/.%s", p.Chain.Config().Name)
}

func (p *PenumbraAppNode) CreateKey(ctx context.Context, keyName string) error {
	cmd := []string{"pcli", "-w", p.WalletPathContainer(), "wallet", "generate"}
	_, stderr, err := p.Exec(ctx, cmd, nil)
	// already exists error is okay
	if err != nil && !strings.Contains(string(stderr), "already exists, refusing to overwrite it") {
		return err
	}
	cmd = []string{"pcli", "-w", p.WalletPathContainer(), "addr", "new", keyName}
	_, _, err = p.Exec(ctx, cmd, nil)
	return err
}

// initializes validator definition template file
// wallet must be generated first
func (p *PenumbraAppNode) InitValidatorFile(ctx context.Context) error {
	cmd := []string{
		"pcli",
		"-w", p.WalletPathContainer(),
		"validator", "template-definition",
		"--file", p.ValidatorDefinitionTemplateFilePathContainer(),
	}
	_, _, err := p.Exec(ctx, cmd, nil)
	return err
}

func (p *PenumbraAppNode) ValidatorDefinitionTemplateFilePath() string {
	return filepath.Join(p.Dir(), "validator.json")
}

func (p *PenumbraAppNode) ValidatorDefinitionTemplateFilePathContainer() string {
	return filepath.Join(p.NodeHome(), "validator.json")
}

func (p *PenumbraAppNode) WalletPathContainer() string {
	return filepath.Join(p.NodeHome(), "wallet")
}

func (p *PenumbraAppNode) ValidatorsInputFile() string {
	return filepath.Join(p.Dir(), "validators.json")
}

func (p *PenumbraAppNode) ValidatorsInputFileContainer() string {
	return filepath.Join(p.NodeHome(), "validators.json")
}

func (p *PenumbraAppNode) AllocationsInputFile() string {
	return filepath.Join(p.Dir(), "allocations.csv")
}

func (p *PenumbraAppNode) AllocationsInputFileContainer() string {
	return filepath.Join(p.NodeHome(), "allocations.csv")
}

func (p *PenumbraAppNode) GenesisFile() string {
	return filepath.Join(p.Dir(), "node0", "tendermint", "config", "genesis.json")
}

func (p *PenumbraAppNode) ValidatorPrivateKeyFile(nodeNum int) string {
	return filepath.Join(p.Dir(), fmt.Sprintf("node%d", nodeNum), "tendermint", "config", "priv_validator_key.json")
}

func (p *PenumbraAppNode) Cleanup(ctx context.Context) error {
	cmd := []string{"find", fmt.Sprintf("%s/.", p.NodeHome()), "-name", ".", "-o", "-prune", "-exec", "rm", "-rf", "--", "{}", "+"}

	// Cleanup should complete instantly,
	// so add a 1-minute timeout in case Docker hangs.
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	_, _, err := p.Exec(ctx, cmd, nil)
	return err
}

func (p *PenumbraAppNode) GenerateGenesisFile(
	ctx context.Context,
	chainID string,
	validators []PenumbraValidatorDefinition,
	allocations []PenumbraGenesisAppStateAllocation,
) error {
	validatorsJson, err := json.Marshal(validators)
	if err != nil {
		return fmt.Errorf("error marshalling validators to json: %w", err)
	}
	if err := os.WriteFile(p.ValidatorsInputFile(), validatorsJson, 0644); err != nil {
		return fmt.Errorf("error writing validators to file: %w", err)
	}
	allocationsCsv := []byte(`"amount","denom","address"\n`)
	for _, allocation := range allocations {
		allocationsCsv = append(allocationsCsv, []byte(fmt.Sprintf(`"%d","%s","%s"\n`, allocation.Amount, allocation.Denom, allocation.Address))...)
	}
	if err := os.WriteFile(p.AllocationsInputFile(), allocationsCsv, 0644); err != nil {
		return fmt.Errorf("error writing allocations to file: %w", err)
	}
	cmd := []string{
		"pd",
		"generate-testnet",
		"--chain-id", chainID,
		"--validators-input-file", p.ValidatorsInputFileContainer(),
		"--allocations-input-file", p.AllocationsInputFileContainer(),
		"--output-dir", p.NodeHome(),
	}
	_, _, err = p.Exec(ctx, cmd, nil)
	return err
}

func (p *PenumbraAppNode) GetAddress(ctx context.Context, keyName string) ([]byte, error) {
	cmd := []string{"pcli", "-w", p.WalletPathContainer(), "addr", "list"}
	stdout, _, err := p.Exec(ctx, cmd, nil)
	if err != nil {
		return nil, err
	}
	addresses := strings.Split(string(stdout), "\n")
	for _, address := range addresses {
		fields := strings.Fields(address)
		if len(fields) < 3 {
			continue
		}
		if fields[1] == keyName {
			// TODO penumbra address is bech32m. need to decode to bytes here
			return []byte(fields[2]), nil
		}
	}
	return []byte{}, errors.New("address not found")
}

func (p *PenumbraAppNode) GetAddressBech32m(ctx context.Context, keyName string) (string, error) {
	cmd := []string{"pcli", "-w", p.WalletPathContainer(), "addr", "list"}
	stdout, _, err := p.Exec(ctx, cmd, nil)
	if err != nil {
		return "", err
	}
	addresses := strings.Split(string(stdout), "\n")
	for _, address := range addresses {
		fields := strings.Fields(address)
		if len(fields) < 3 {
			continue
		}
		if fields[1] == keyName {
			return fields[2], nil
		}
	}
	return "", errors.New("address not found")
}

func (p *PenumbraAppNode) SendFunds(ctx context.Context, keyName string, amount ibc.WalletAmount) error {
	return errors.New("not yet implemented")
}

func (p *PenumbraAppNode) SendIBCTransfer(ctx context.Context, channelID, keyName string, amount ibc.WalletAmount, timeout *ibc.IBCTimeout) (ibc.Tx, error) {
	return ibc.Tx{}, errors.New("not yet implemented")
}

func (p *PenumbraAppNode) CreateNodeContainer(ctx context.Context) error {
	cmd := []string{"pd", "start", "--host", "0.0.0.0", "-r", p.NodeHome()}
	fmt.Printf("{%s} -> '%s'\n", p.Name(), strings.Join(cmd, " "))

	cc, err := p.DockerClient.ContainerCreate(
		ctx,
		&container.Config{
			Image: p.Image.Ref(),

			Cmd: cmd,

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

func (p *PenumbraAppNode) StopContainer(ctx context.Context) error {
	timeout := 30 * time.Second
	return p.DockerClient.ContainerStop(ctx, p.containerID, &timeout)
}

func (p *PenumbraAppNode) StartContainer(ctx context.Context) error {
	if err := dockerutil.StartContainer(ctx, p.DockerClient, p.containerID); err != nil {
		return err
	}

	c, err := p.DockerClient.ContainerInspect(ctx, p.containerID)
	if err != nil {
		return err
	}

	p.hostRPCPort = dockerutil.GetHostPort(c, rpcPort)
	p.hostGRPCPort = dockerutil.GetHostPort(c, grpcPort)

	return nil
}

// Exec run a container for a specific job and block until the container exits
func (p *PenumbraAppNode) Exec(ctx context.Context, cmd []string, env []string) ([]byte, []byte, error) {
	job := dockerutil.NewImage(p.log, p.DockerClient, p.NetworkID, p.TestName, p.Image.Repository, p.Image.Version)
	opts := dockerutil.ContainerOptions{
		Binds: p.Bind(),
		Env:   env,
		User:  dockerutil.GetRootUserString(),
	}
	return job.Run(ctx, cmd, opts)
}
