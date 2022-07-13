package tendermint

// this package applies to chains that use tendermint >= v0.35.0, likely separate from the abci app

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	dockerclient "github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/strangelove-ventures/ibctest/ibc"
	"github.com/strangelove-ventures/ibctest/internal/dockerutil"
	"github.com/tendermint/tendermint/p2p"
	rpcclient "github.com/tendermint/tendermint/rpc/client"
	rpchttp "github.com/tendermint/tendermint/rpc/client/http"
	libclient "github.com/tendermint/tendermint/rpc/jsonrpc/client"
	"go.uber.org/zap"
)

// TendermintNode represents a node in the test network that is being created
type TendermintNode struct {
	Log *zap.Logger

	Home         string
	Index        int
	Chain        ibc.Chain
	NetworkID    string
	DockerClient *dockerclient.Client
	Client       rpcclient.Client
	TestName     string
	Image        ibc.DockerImage

	containerID string
}

// TendermintNodes is a collection of TendermintNode
type TendermintNodes []*TendermintNode

const (
	// BlockTimeSeconds (in seconds) is approx time to create a block
	BlockTimeSeconds = 2

	p2pPort     = "26656/tcp"
	rpcPort     = "26657/tcp"
	grpcPort    = "9090/tcp"
	apiPort     = "1317/tcp"
	privValPort = "1234/tcp"
)

var (
	sentryPorts = nat.PortSet{
		nat.Port(p2pPort):     {},
		nat.Port(rpcPort):     {},
		nat.Port(grpcPort):    {},
		nat.Port(apiPort):     {},
		nat.Port(privValPort): {},
	}
)

// NewClient creates and assigns a new Tendermint RPC client to the TendermintNode
func (tn *TendermintNode) NewClient(addr string) error {
	httpClient, err := libclient.DefaultHTTPClient(addr)
	if err != nil {
		return err
	}

	httpClient.Timeout = 10 * time.Second
	rpcClient, err := rpchttp.NewWithClient(addr, "/websocket", httpClient)
	if err != nil {
		return err
	}

	tn.Client = rpcClient
	return nil
}

// Name is the hostname of the test node container
func (tn *TendermintNode) Name() string {
	return fmt.Sprintf("node-%d-%s-%s", tn.Index, tn.Chain.Config().ChainID, dockerutil.SanitizeContainerName(tn.TestName))
}

func (tn *TendermintNode) HostName() string {
	return dockerutil.CondenseHostName(tn.Name())
}

// Dir is the directory where the test node files are stored
func (tn *TendermintNode) Dir() string {
	return filepath.Join(tn.Home, tn.Name())
}

// MkDir creates the directory for the testnode
func (tn *TendermintNode) MkDir() {
	if err := os.MkdirAll(tn.Dir(), 0755); err != nil {
		panic(err)
	}
}

// GentxPath returns the path to the gentx for a node
func (tn *TendermintNode) GentxPath() (string, error) {
	id, err := tn.NodeID()
	return filepath.Join(tn.Dir(), "config", "gentx", fmt.Sprintf("gentx-%s.json", id)), err
}

func (tn *TendermintNode) GenesisFilePath() string {
	return filepath.Join(tn.Dir(), "config", "genesis.json")
}

type PrivValidatorKey struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

type PrivValidatorKeyFile struct {
	Address string           `json:"address"`
	PubKey  PrivValidatorKey `json:"pub_key"`
	PrivKey PrivValidatorKey `json:"priv_key"`
}

func (tn *TendermintNode) PrivValKeyFilePath() string {
	return filepath.Join(tn.Dir(), "config", "priv_validator_key.json")
}

func (tn *TendermintNode) TMConfigPath() string {
	return filepath.Join(tn.Dir(), "config", "config.toml")
}

func (tn *TendermintNode) TMConfigPathContainer() string {
	return filepath.Join(tn.HomeDir(), "config", "config.toml")
}

// Bind returns the home folder bind point for running the node
func (tn *TendermintNode) Bind() []string {
	return []string{fmt.Sprintf("%s:%s", tn.Dir(), tn.HomeDir())}
}

func (tn *TendermintNode) HomeDir() string {
	return filepath.Join("/tmp", tn.Chain.Config().Name)
}

func (tn *TendermintNode) sedCommandForConfigFile(key, newValue string) string {
	return fmt.Sprintf("sed -i \"/^%s = .*/ s//%s = %s/\" %s", key, key, newValue, tn.TMConfigPathContainer())
}

// SetConfigAndPeers modifies the config for a validator node to start a chain
func (tn *TendermintNode) SetConfigAndPeers(ctx context.Context, peers string) error {
	timeoutCommitPropose := fmt.Sprintf("\\\"%ds\\\"", BlockTimeSeconds)
	cmds := []string{
		tn.sedCommandForConfigFile("timeout-commit", timeoutCommitPropose),
		tn.sedCommandForConfigFile("timeout-propose", timeoutCommitPropose),
		tn.sedCommandForConfigFile("allow-duplicate-ip", "true"),
		tn.sedCommandForConfigFile("addr-book-strict", "false"),
		tn.sedCommandForConfigFile("persistent-peers", fmt.Sprintf("\\\"%s\\\"", peers)),
	}
	cmd := []string{"sh", "-c", strings.Join(cmds, " && ")}
	_, _, err := tn.Exec(ctx, cmd, nil)
	return err
}

func (tn *TendermintNode) Height(ctx context.Context) (uint64, error) {
	stat, err := tn.Client.Status(ctx)
	if err != nil {
		return 0, fmt.Errorf("tendermint client status: %w", err)
	}
	return uint64(stat.SyncInfo.LatestBlockHeight), nil
}

// InitHomeFolder initializes a home folder for the given node
func (tn *TendermintNode) InitHomeFolder(ctx context.Context, mode string) error {
	command := []string{tn.Chain.Config().Bin, "init", mode,
		"--home", tn.HomeDir(),
	}
	_, _, err := tn.Exec(ctx, command, nil)
	return err
}

func (tn *TendermintNode) CreateNodeContainer(ctx context.Context, additionalFlags ...string) error {
	chainCfg := tn.Chain.Config()
	cmd := []string{chainCfg.Bin, "start", "--home", tn.HomeDir()}
	cmd = append(cmd, additionalFlags...)
	fmt.Printf("{%s} -> '%s'\n", tn.Name(), strings.Join(cmd, " "))

	cc, err := tn.DockerClient.ContainerCreate(
		ctx,
		&container.Config{
			Image: tn.Image.Ref(),

			Entrypoint: []string{},
			Cmd:        cmd,

			Hostname: tn.HostName(),
			User:     dockerutil.GetDockerUserString(),

			Labels: map[string]string{dockerutil.CleanupLabel: tn.TestName},

			ExposedPorts: sentryPorts,
		},
		&container.HostConfig{
			Binds:           tn.Bind(),
			PublishAllPorts: true,
			AutoRemove:      false,
			DNS:             []string{},
		},
		&network.NetworkingConfig{
			EndpointsConfig: map[string]*network.EndpointSettings{
				tn.NetworkID: {},
			},
		},
		nil,
		tn.Name(),
	)
	if err != nil {
		return err
	}
	tn.containerID = cc.ID
	return nil
}

func (tn *TendermintNode) StopContainer(ctx context.Context) error {
	timeout := 30 * time.Second
	return tn.DockerClient.ContainerStop(ctx, tn.containerID, &timeout)
}

func (tn *TendermintNode) StartContainer(ctx context.Context) error {
	if err := dockerutil.StartContainer(ctx, tn.DockerClient, tn.containerID); err != nil {
		return err
	}

	c, err := tn.DockerClient.ContainerInspect(ctx, tn.containerID)
	if err != nil {
		return err
	}

	port := dockerutil.GetHostPort(c, rpcPort)
	fmt.Printf("{%s} RPC => %s\n", tn.Name(), port)

	err = tn.NewClient(fmt.Sprintf("tcp://%s", port))
	if err != nil {
		return err
	}

	time.Sleep(5 * time.Second)
	return retry.Do(func() error {
		stat, err := tn.Client.Status(ctx)
		if err != nil {
			// tn.t.Log(err)
			return err
		}
		// TODO: reenable this check, having trouble with it for some reason
		if stat != nil && stat.SyncInfo.CatchingUp {
			return fmt.Errorf("still catching up: height(%d) catching-up(%t)",
				stat.SyncInfo.LatestBlockHeight, stat.SyncInfo.CatchingUp)
		}
		return nil
	}, retry.Context(ctx), retry.DelayType(retry.BackOffDelay))
}

// InitValidatorFiles creates the node files and signs a genesis transaction
func (tn *TendermintNode) InitValidatorFiles(ctx context.Context) error {
	return tn.InitHomeFolder(ctx, "validator")
}

func (tn *TendermintNode) InitFullNodeFiles(ctx context.Context) error {
	return tn.InitHomeFolder(ctx, "full")
}

// NodeID returns the node of a given node
func (tn *TendermintNode) NodeID() (string, error) {
	nodeKey, err := p2p.LoadNodeKey(filepath.Join(tn.Dir(), "config", "node_key.json"))
	if err != nil {
		return "", err
	}
	return string(nodeKey.ID()), nil
}

// PeerString returns the string for connecting the nodes passed in
func (tn TendermintNodes) PeerString(node *TendermintNode) string {
	addrs := make([]string, len(tn))
	for i, n := range tn {
		if n == node {
			// don't peer with ourself
			continue
		}
		id, err := n.NodeID()
		if err != nil {
			// TODO: would this be better to panic?
			// When would NodeId return an error?
			break
		}
		hostName := n.HostName()
		ps := fmt.Sprintf("%s@%s:26656", id, hostName)
		fmt.Printf("{%s} peering (%s)\n", hostName, ps)
		addrs[i] = ps
	}
	return strings.Join(addrs, ",")
}

// LogGenesisHashes logs the genesis hashes for the various nodes
func (tn TendermintNodes) LogGenesisHashes() error {
	for _, n := range tn {
		gen, err := os.ReadFile(filepath.Join(n.Dir(), "config", "genesis.json"))
		if err != nil {
			return err
		}
		fmt.Printf("{%s} genesis hash %x\n", n.Name(), sha256.Sum256(gen))
	}
	return nil
}

func (tn *TendermintNode) Exec(ctx context.Context, cmd []string, env []string) ([]byte, []byte, error) {
	job := dockerutil.NewImage(tn.Log, tn.DockerClient, tn.NetworkID, tn.TestName, tn.Image.Repository, tn.Image.Version)
	opts := dockerutil.ContainerOptions{
		Env:   env,
		Binds: tn.Bind(),
	}
	err := job.Run(ctx, cmd, opts)
	return err.Stdout, err.Stderr, err.Err
}
