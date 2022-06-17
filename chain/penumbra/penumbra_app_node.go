package penumbra

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"github.com/strangelove-ventures/ibctest/ibc"
	"github.com/strangelove-ventures/ibctest/internal/dockerutil"
)

type PenumbraAppNode struct {
	Index     int
	Home      string
	Chain     ibc.Chain
	TestName  string
	NetworkID string
	Pool      *dockertest.Pool
	Container *docker.Container
	Image     ibc.DockerImage
}

const (
	valKey         = "validator"
	rpcPort        = "26657/tcp"
	tendermintPort = "26658/tcp"
	grpcPort       = "9090/tcp"
)

var exposedPorts = map[docker.Port]struct{}{
	docker.Port(tendermintPort): {},
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
	exitCode, stdout, stderr, err := p.NodeJob(ctx, cmd)
	// already exists error is okay
	if err != nil && !strings.Contains(stderr, "already exists, refusing to overwrite it") {
		return dockerutil.HandleNodeJobError(exitCode, stdout, stderr, err)
	}
	cmd = []string{"pcli", "-w", p.WalletPathContainer(), "addr", "new", keyName}
	return dockerutil.HandleNodeJobError(p.NodeJob(ctx, cmd))
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
	return dockerutil.HandleNodeJobError(p.NodeJob(ctx, cmd))
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

	return dockerutil.HandleNodeJobError(p.NodeJob(ctx, cmd))
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
	return dockerutil.HandleNodeJobError(p.NodeJob(ctx, cmd))
}

func (p *PenumbraAppNode) GetAddress(ctx context.Context, keyName string) ([]byte, error) {
	cmd := []string{"pcli", "-w", p.WalletPathContainer(), "addr", "list"}
	exitCode, stdout, stderr, err := p.NodeJob(ctx, cmd)
	if err != nil {
		return []byte{}, dockerutil.HandleNodeJobError(exitCode, stdout, stderr, err)
	}
	addresses := strings.Split(stdout, "\n")
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
	exitCode, stdout, stderr, err := p.NodeJob(ctx, cmd)
	if err != nil {
		return "", dockerutil.HandleNodeJobError(exitCode, stdout, stderr, err)
	}
	addresses := strings.Split(stdout, "\n")
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

func (p *PenumbraAppNode) CreateNodeContainer() error {
	cmd := []string{"pd", "start", "--host", "0.0.0.0", "-r", p.NodeHome()}
	fmt.Printf("{%s} -> '%s'\n", p.Name(), strings.Join(cmd, " "))

	cont, err := p.Pool.Client.CreateContainer(docker.CreateContainerOptions{
		Name: p.Name(),
		Config: &docker.Config{
			User:         dockerutil.GetRootUserString(),
			Cmd:          cmd,
			Hostname:     p.HostName(),
			ExposedPorts: exposedPorts,
			DNS:          []string{},
			// Env:          []string{"RUST_BACKTRACE=full"},
			Image:  fmt.Sprintf("%s:%s", p.Image.Repository, p.Image.Version),
			Labels: map[string]string{"ibc-test": p.TestName},
		},
		HostConfig: &docker.HostConfig{
			Binds:           p.Bind(),
			PublishAllPorts: true,
			AutoRemove:      false,
		},
		NetworkingConfig: &docker.NetworkingConfig{
			EndpointsConfig: map[string]*docker.EndpointConfig{
				p.NetworkID: {},
			},
		},
		Context: nil,
	})
	if err != nil {
		return err
	}
	p.Container = cont
	return nil
}

func (p *PenumbraAppNode) StopContainer() error {
	const timeoutSeconds = 30 // StopContainer expects whole seconds as a uint.
	return p.Pool.Client.StopContainer(p.Container.ID, timeoutSeconds)
}

func (p *PenumbraAppNode) StartContainer(ctx context.Context) error {
	if err := p.Pool.Client.StartContainerWithContext(p.Container.ID, nil, ctx); err != nil {
		return err
	}

	c, err := p.Pool.Client.InspectContainerWithContext(p.Container.ID, ctx)
	if err != nil {
		return err
	}
	p.Container = c
	return nil
}

// NodeJob run a container for a specific job and block until the container exits
// NOTE: on job containers generate random name
func (p *PenumbraAppNode) NodeJob(ctx context.Context, cmd []string) (int, string, string, error) {
	counter, _, _, _ := runtime.Caller(1)
	caller := runtime.FuncForPC(counter).Name()
	funcName := strings.Split(caller, ".")
	container := fmt.Sprintf("%s-%s-%s", p.Name(), funcName[len(funcName)-1], dockerutil.RandLowerCaseLetterString(3))
	fmt.Printf("{%s} -> '%s'\n", container, strings.Join(cmd, " "))
	cont, err := p.Pool.Client.CreateContainer(docker.CreateContainerOptions{
		Name: container,
		Config: &docker.Config{
			User: dockerutil.GetRootUserString(),
			// random hostname is okay here
			Hostname:     dockerutil.CondenseHostName(container),
			ExposedPorts: exposedPorts,
			DNS:          []string{},
			// Env:          []string{"RUST_BACKTRACE=full"},
			Image:  fmt.Sprintf("%s:%s", p.Image.Repository, p.Image.Version),
			Cmd:    cmd,
			Labels: map[string]string{"ibc-test": p.TestName},
		},
		HostConfig: &docker.HostConfig{
			Binds:           p.Bind(),
			PublishAllPorts: true,
			AutoRemove:      false,
		},
		NetworkingConfig: &docker.NetworkingConfig{
			EndpointsConfig: map[string]*docker.EndpointConfig{
				p.NetworkID: {},
			},
		},
		Context: ctx,
	})
	if err != nil {
		return 1, "", "", err
	}
	if err := p.Pool.Client.StartContainerWithContext(cont.ID, nil, ctx); err != nil {
		return 1, "", "", err
	}

	exitCode, err := p.Pool.Client.WaitContainerWithContext(cont.ID, ctx)
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	_ = p.Pool.Client.Logs(docker.LogsOptions{Context: ctx, Container: cont.ID, OutputStream: stdout, ErrorStream: stderr, Stdout: true, Stderr: true, Tail: "50", Follow: false, Timestamps: false})
	_ = p.Pool.Client.RemoveContainer(docker.RemoveContainerOptions{ID: cont.ID})
	fmt.Printf("{%s} - stdout:\n%s\n{%s} - stderr:\n%s\n", container, stdout.String(), container, stderr.String())
	return exitCode, stdout.String(), stderr.String(), err
}
