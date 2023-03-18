package penumbra

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/strangelove-ventures/interchaintest/v7/ibc"
	"github.com/strangelove-ventures/interchaintest/v7/internal/dockerutil"
	"go.uber.org/zap"
)

type PenumbraAppNode struct {
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

// Bind returns the home folder bind point for running the node
func (p *PenumbraAppNode) Bind() []string {
	return []string{fmt.Sprintf("%s:%s", p.VolumeName, p.HomeDir())}
}

func (p *PenumbraAppNode) HomeDir() string {
	return "/home/heighliner"
}

func (p *PenumbraAppNode) CreateKey(ctx context.Context, keyName string) error {
	keyPath := filepath.Join(p.HomeDir(), "keys", keyName)
	cmd := []string{"pcli", "-d", keyPath, "keys", "generate"}
	_, stderr, err := p.Exec(ctx, cmd, nil)
	// already exists error is okay
	if err != nil && !strings.Contains(string(stderr), "already exists, refusing to overwrite it") {
		return err
	}
	return nil
}

// RecoverKey restores a key from a given mnemonic.
func (p *PenumbraAppNode) RecoverKey(ctx context.Context, keyName, mnemonic string) error {
	keyPath := filepath.Join(p.HomeDir(), "keys", keyName)
	cmd := []string{"pcli", "-d", keyPath, "keys", "import", "phrase", mnemonic}
	_, stderr, err := p.Exec(ctx, cmd, nil)
	// already exists error is okay
	if err != nil && !strings.Contains(string(stderr), "already exists, refusing to overwrite it") {
		return err
	}
	return nil
}

// initializes validator definition template file
// wallet must be generated first
func (p *PenumbraAppNode) InitValidatorFile(ctx context.Context, valKeyName string) error {
	keyPath := filepath.Join(p.HomeDir(), "keys", valKeyName)
	cmd := []string{
		"pcli",
		"-d", keyPath,
		"validator", "definition", "template",
		"--file", p.ValidatorDefinitionTemplateFilePathContainer(),
	}
	_, _, err := p.Exec(ctx, cmd, nil)
	return err
}

func (p *PenumbraAppNode) ValidatorDefinitionTemplateFilePathContainer() string {
	return filepath.Join(p.HomeDir(), "validator.toml")
}

func (p *PenumbraAppNode) ValidatorsInputFileContainer() string {
	return filepath.Join(p.HomeDir(), "validators.json")
}

func (p *PenumbraAppNode) AllocationsInputFileContainer() string {
	return filepath.Join(p.HomeDir(), "allocations.csv")
}

func (p *PenumbraAppNode) genesisFileContent(ctx context.Context) ([]byte, error) {
	fr := dockerutil.NewFileRetriever(p.log, p.DockerClient, p.TestName)
	gen, err := fr.SingleFileContent(ctx, p.VolumeName, ".penumbra/testnet_data/node0/tendermint/config/genesis.json")
	if err != nil {
		return nil, fmt.Errorf("error getting genesis.json content: %w", err)
	}

	return gen, nil
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
	fw := dockerutil.NewFileWriter(p.log, p.DockerClient, p.TestName)
	if err := fw.WriteFile(ctx, p.VolumeName, "validators.json", validatorsJson); err != nil {
		return fmt.Errorf("error writing validators to file: %w", err)
	}
	allocationsCsv := []byte(`"amount","denom","address"\n`)
	for _, allocation := range allocations {
		allocationsCsv = append(allocationsCsv, []byte(fmt.Sprintf(`"%d","%s","%s"\n`, allocation.Amount, allocation.Denom, allocation.Address))...)
	}
	if err := fw.WriteFile(ctx, p.VolumeName, "allocations.csv", allocationsCsv); err != nil {
		return fmt.Errorf("error writing allocations to file: %w", err)
	}
	cmd := []string{
		"pd",
		"testnet",
		"generate",
		"--chain-id", chainID,
		"--validators-input-file", p.ValidatorsInputFileContainer(),
		"--allocations-input-file", p.AllocationsInputFileContainer(),
	}
	_, _, err = p.Exec(ctx, cmd, nil)
	return err
}

func (p *PenumbraAppNode) GetAddress(ctx context.Context, keyName string) ([]byte, error) {
	cmd := []string{"pcli", "-d", p.HomeDir(), "addr", "list"}
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
	cmd := []string{"pcli", "-d", p.HomeDir(), "addr", "list"}
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

func (p *PenumbraAppNode) SendIBCTransfer(
	ctx context.Context,
	channelID string,
	keyName string,
	amount ibc.WalletAmount,
	options ibc.TransferOptions,
) (ibc.Tx, error) {
	return ibc.Tx{}, errors.New("not yet implemented")
}

func (p *PenumbraAppNode) CreateNodeContainer(ctx context.Context) error {
	cmd := []string{"pd", "start", "--host", "0.0.0.0", "--home", p.HomeDir()}

	return p.containerLifecycle.CreateContainer(ctx, p.TestName, p.NetworkID, p.Image, exposedPorts, p.Bind(), p.HostName(), cmd)
}

func (p *PenumbraAppNode) StopContainer(ctx context.Context) error {
	return p.containerLifecycle.StopContainer(ctx)
}

func (p *PenumbraAppNode) StartContainer(ctx context.Context) error {
	if err := p.containerLifecycle.StartContainer(ctx); err != nil {
		return err
	}

	hostPorts, err := p.containerLifecycle.GetHostPorts(ctx, rpcPort, grpcPort)
	if err != nil {
		return err
	}

	p.hostRPCPort, p.hostGRPCPort = hostPorts[0], hostPorts[1]

	return nil
}

// Exec run a container for a specific job and block until the container exits
func (p *PenumbraAppNode) Exec(ctx context.Context, cmd []string, env []string) ([]byte, []byte, error) {
	job := dockerutil.NewImage(p.log, p.DockerClient, p.NetworkID, p.TestName, p.Image.Repository, p.Image.Version)
	opts := dockerutil.ContainerOptions{
		Binds: p.Bind(),
		Env:   env,
		User:  p.Image.UidGid,
	}
	res := job.Run(ctx, cmd, opts)
	return res.Stdout, res.Stderr, res.Err
}
