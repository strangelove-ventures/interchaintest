package polkadot

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"go.uber.org/zap"

	"github.com/strangelove-ventures/ibctest/ibc"
	"github.com/strangelove-ventures/ibctest/internal/dockerutil"
)

type ParachainNode struct {
	log             *zap.Logger
	Home            string
	Index           int
	Chain           ibc.Chain
	NetworkID       string
	DockerClient    *client.Client
	containerID     string
	TestName        string
	Image           ibc.DockerImage
	Bin             string
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

// Dir is the directory where the test node files are stored
func (pn *ParachainNode) Dir() string {
	return filepath.Join(pn.Home, pn.Name())
}

// MkDir creates the directory for the testnode
func (pn *ParachainNode) MkDir() {
	if err := os.MkdirAll(pn.Dir(), 0755); err != nil {
		panic(err)
	}
}

// Bind returns the home folder bind point for running the node
func (pn *ParachainNode) Bind() []string {
	return []string{fmt.Sprintf("%s:%s", pn.Dir(), pn.NodeHome())}
}

func (pn *ParachainNode) NodeHome() string {
	return fmt.Sprintf("/root/.%s", pn.Chain.Config().Name)
}

func (pn *ParachainNode) RawChainSpecFilePath() string {
	return filepath.Join(pn.Dir(), fmt.Sprintf("%s-raw.json", pn.Chain.Config().ChainID))
}

func (pn *ParachainNode) RawChainSpecFilePathContainer() string {
	return filepath.Join(pn.NodeHome(), fmt.Sprintf("%s-raw.json", pn.Chain.Config().ChainID))
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
	stdout, _, err := pn.Exec(ctx, cmd, nil)
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
	stdout, _, err := pn.Exec(ctx, cmd, nil)
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
	stdout, _, err := pn.Exec(ctx, cmd, nil)
	if err != nil {
		return "", err
	}
	return string(stdout), nil
}

func (pn *ParachainNode) CreateNodeContainer(ctx context.Context) error {
	cmd := []string{
		pn.Bin,
		fmt.Sprintf("--ws-port=%d", wsPort),
		"--collator",
		"--unsafe-ws-external",
		"--unsafe-rpc-external",
		"--prometheus-external",
		"--rpc-cors=all",
		fmt.Sprintf("--prometheus-port=%d", prometheusPort),
		fmt.Sprintf("--listen-addr=/ip4/0.0.0.0/tcp/%d", rpcPort),
		"--base-path", pn.NodeHome(),
		fmt.Sprintf("--chain=%s", pn.ChainID),
	}
	cmd = append(cmd, pn.Flags...)
	cmd = append(cmd, "--", fmt.Sprintf("--chain=%s", pn.RawChainSpecFilePathContainer()))
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
func (pn *ParachainNode) Exec(ctx context.Context, cmd []string, env []string) ([]byte, []byte, error) {
	job := dockerutil.NewImage(pn.log, pn.DockerClient, pn.NetworkID, pn.TestName, pn.Image.Repository, pn.Image.Version)
	opts := dockerutil.ContainerOptions{
		Binds: pn.Bind(),
		Env:   env,
		User:  dockerutil.GetRootUserString(),
	}
	return job.Run(ctx, cmd, opts)
}

func (pn *ParachainNode) Cleanup(ctx context.Context) error {
	cmd := []string{"find", fmt.Sprintf("%s/.", pn.Home), "-name", ".", "-o", "-prune", "-exec", "rm", "-rf", "--", "{}", "+"}

	// Cleanup should complete instantly,
	// so add a 1-minute timeout in case Docker hangs.
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	_, _, err := pn.Exec(ctx, cmd, nil)
	return err
}
