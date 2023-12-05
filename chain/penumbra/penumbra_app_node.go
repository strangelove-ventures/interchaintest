package penumbra

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	volumetypes "github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
	dockerclient "github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"github.com/strangelove-ventures/interchaintest/v8/internal/dockerutil"
	"go.uber.org/zap"
)

type PenumbraAppNode struct {
	log *zap.Logger

	Index        int
	VolumeName   string
	Chain        *PenumbraChain
	TestName     string
	NetworkID    string
	DockerClient *client.Client
	Image        ibc.DockerImage

	containerLifecycle *dockerutil.ContainerLifecycle

	// Set during StartContainer.
	hostRPCPort  string
	hostGRPCPort string
}

func NewPenumbraAppNode(
	ctx context.Context,
	log *zap.Logger,
	chain *PenumbraChain,
	index int,
	testName string,
	dockerClient *dockerclient.Client,
	networkID string,
	image ibc.DockerImage,
) (*PenumbraAppNode, error) {
	pn := &PenumbraAppNode{log: log, Index: index, Chain: chain,
		DockerClient: dockerClient, NetworkID: networkID, TestName: testName, Image: image}

	pn.containerLifecycle = dockerutil.NewContainerLifecycle(log, dockerClient, pn.Name())

	pv, err := dockerClient.VolumeCreate(ctx, volumetypes.CreateOptions{
		Labels: map[string]string{
			dockerutil.CleanupLabel:   testName,
			dockerutil.NodeOwnerLabel: pn.Name(),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("creating penumbra volume: %w", err)
	}
	pn.VolumeName = pv.Name
	if err := dockerutil.SetVolumeOwner(ctx, dockerutil.VolumeOwnerOptions{
		Log: log,

		Client: dockerClient,

		VolumeName: pn.VolumeName,
		ImageRef:   pn.Image.Ref(),
		TestName:   pn.TestName,
		UidGid:     image.UidGid,
	}); err != nil {
		return nil, fmt.Errorf("set penumbra volume owner: %w", err)
	}

	return pn, nil
}

const (
	valKey      = "validator"
	rpcPort     = "26657/tcp"
	abciPort    = "26658/tcp"
	grpcPort    = "8080/tcp"
	metricsPort = "9000/tcp"
)

var exposedPorts = nat.PortSet{
	nat.Port(abciPort):    {},
	nat.Port(grpcPort):    {},
	nat.Port(metricsPort): {},
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
	pdUrl := fmt.Sprintf("http://%s:8080", p.HostName())
	cmd := []string{"pcli", "--home", keyPath, "init", "--grpc-url", pdUrl, "soft-kms", "generate"}
	_, stderr, err := p.Exec(ctx, cmd, nil)
	// already exists error is okay
	if err != nil && !strings.Contains(string(stderr), "not empty;, refusing to initialize") {
		return err
	}
	return nil
}

type PcliConfig struct {
	GrpcURL        string `toml:"grpc_url"`
	FullViewingKey string `toml:"full_viewing_key"`
	Custody        struct {
		Backend  string `toml:"backend"`
		SpendKey string `toml:"spend_key"`
	} `toml:"custody"`
}

// ReadFile attempts to read a file from the Docker filesystem at the specified path.
// relPath describes the location of the file in the Docker volume relative to the home directory.
func (p *PenumbraAppNode) ReadFile(ctx context.Context, relPath string) ([]byte, error) {
	fr := dockerutil.NewFileRetriever(p.log, p.DockerClient, p.TestName)
	fileBz, err := fr.SingleFileContent(ctx, p.VolumeName, relPath)
	if err != nil {
		return nil, err
	}

	return fileBz, nil
}

func (p *PenumbraAppNode) FullViewingKey(ctx context.Context, keyName string) (string, error) {
	keyPath := filepath.Join(p.HomeDir(), "keys", keyName)
	fileBz, err := p.ReadFile(ctx, keyPath+"config.toml")

	c := PcliConfig{}
	err = toml.Unmarshal(fileBz, &c)
	if err != nil {
		return "", err
	}

	return c.FullViewingKey, nil
}

// RecoverKey restores a key from a given mnemonic.
func (p *PenumbraAppNode) RecoverKey(ctx context.Context, keyName, mnemonic string) error {
	keyPath := filepath.Join(p.HomeDir(), "keys", keyName)
	pdUrl := fmt.Sprintf("http://%s:8080", p.HostName())
	cmd := []string{
		"sh",
		"-c",
		fmt.Sprintf(`echo %q | pcli --home %s -n %s keys import phrase`, mnemonic, keyPath, pdUrl),
	}

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
		"--home", keyPath,
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
	gen, err := fr.SingleFileContent(ctx, p.VolumeName, ".penumbra/testnet_data/node0/cometbft/config/genesis.json")
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

	allocationsCsv := []byte(`"amount","denom","address"` + "\n")
	for _, allocation := range allocations {
		allocationsCsv = append(allocationsCsv, []byte(fmt.Sprintf(`"%s","%s","%s"`+"\n", allocation.Amount.String(), allocation.Denom, allocation.Address))...)
	}

	if err := fw.WriteFile(ctx, p.VolumeName, "allocations.csv", allocationsCsv); err != nil {
		return fmt.Errorf("error writing allocations to file: %w", err)
	}

	cmd := []string{
		"pd",
		"testnet",
		"generate",
		"--chain-id", chainID,
		"--preserve-chain-id",
		"--validators-input-file", p.ValidatorsInputFileContainer(),
		"--allocations-input-file", p.AllocationsInputFileContainer(),
	}
	_, _, err = p.Exec(ctx, cmd, nil)
	if err != nil {
		return fmt.Errorf("failed to exec testnet generate: %w", err)
	}

	return err
}

func (p *PenumbraAppNode) GetAddress(ctx context.Context, keyName string) ([]byte, error) {
	keyPath := filepath.Join(p.HomeDir(), "keys", keyName)
	//pdUrl := fmt.Sprintf("http://%s:8080", p.HostName())
	cmd := []string{"pcli", "--home", keyPath, "view", "address"}

	stdout, _, err := p.Exec(ctx, cmd, nil)
	if err != nil {
		return nil, err
	}

	if len(stdout) == 0 {
		return []byte{}, errors.New("address not found")
	}

	addr := strings.TrimSpace(string(stdout))
	return []byte(addr), nil
}

// TODO we need to change the func sig to take a denom then filter out the target denom bal from stdout
func (p *PenumbraAppNode) GetBalance(ctx context.Context, keyName string) (int64, error) {
	keyPath := filepath.Join(p.HomeDir(), "keys", keyName)
	//pdUrl := fmt.Sprintf("http://%s:8080", p.HostName())
	cmd := []string{"pcli", "--home", keyPath, "view", "balance"}

	stdout, _, err := p.Exec(ctx, cmd, nil)
	if err != nil {
		return 0, err
	}

	fmt.Printf("STDOUT BAL: '%s'\n", string(stdout))
	return 0, nil
}

func (p *PenumbraAppNode) GetAddressBech32m(ctx context.Context, keyName string) (string, error) {
	cmd := []string{"pcli", "--home", p.HomeDir(), "addr", "list"}
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

func (p *PenumbraAppNode) CreateNodeContainer(ctx context.Context, tendermintAddress string) error {
	cmd := []string{
		"pd", "start",
		"--abci-bind", "0.0.0.0:" + strings.Split(abciPort, "/")[0],
		"--grpc-bind", "0.0.0.0:" + strings.Split(grpcPort, "/")[0],
		"--metrics-bind", "0.0.0.0:" + strings.Split(metricsPort, "/")[0],
		"--tendermint-addr", "http://" + tendermintAddress,
		"--home", p.HomeDir(),
	}

	// TODO: remove after debugging
	env := []string{
		"RUST_LOG=debug",
	}

	return p.containerLifecycle.CreateContainer(ctx, p.TestName, p.NetworkID, p.Image, exposedPorts, p.Bind(), p.HostName(), cmd, env)
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

func (p *PenumbraAppNode) SendIBCTransfer(ctx context.Context, channelID, keyName string, amount ibc.WalletAmount, opts ibc.TransferOptions) (ibc.Tx, error) {
	keyPath := filepath.Join(p.HomeDir(), "keys", keyName)
	//pdUrl := fmt.Sprintf("http://%s:8080", p.HostName())

	parts := strings.Split(channelID, "-")
	chanNum := parts[1]

	cmd := []string{"pcli", "--home", keyPath, "tx", "withdraw",
		"--to", amount.Address,
		"--channel", chanNum,
		"--timeout-height", fmt.Sprintf("0-%d", opts.Timeout.Height),
		fmt.Sprintf("%s%s", amount.Amount.String(), amount.Denom),
	}

	stdout, stderr, err := p.Exec(ctx, cmd, nil)
	if err != nil {
		return ibc.Tx{}, err
	}

	fmt.Printf("STDOUT: %s \n", string(stdout))
	fmt.Printf("STDERR: %s \n", string(stderr))

	tx := ibc.Tx{
		Height:   0,
		TxHash:   "",
		GasSpent: 0,
		Packet: ibc.Packet{
			Sequence:         0,
			SourcePort:       "",
			SourceChannel:    "",
			DestPort:         "",
			DestChannel:      "",
			Data:             nil,
			TimeoutHeight:    "",
			TimeoutTimestamp: 0,
		},
	}

	return tx, nil
}
