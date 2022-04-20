package penumbra

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"github.com/strangelove-ventures/ibc-test-framework/dockerutil"
	"github.com/strangelove-ventures/ibc-test-framework/ibc"
)

type PenumbraAppNode struct {
	Index     int
	Home      string
	Chain     ibc.Chain
	TestName  string
	NetworkID string
	Pool      *dockertest.Pool
	Container *docker.Container
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

// Name is the hostname of the test node container
func (p *PenumbraAppNode) Name() string {
	return fmt.Sprintf("node-%d-%s-%s", p.Index, p.Chain.Config().ChainID, p.TestName)
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
	return fmt.Sprintf("/tmp/.%s", p.Chain.Config().Name)
}

func (p *PenumbraAppNode) CreateKey(ctx context.Context, keyName string) error {
	cmd := []string{"pcli", "wallet", "generate"}
	exitCode, stdout, stderr, err := p.NodeJob(ctx, cmd)
	// already exists error is okay
	if err != nil && !strings.Contains(stderr, "already exists, refusing to overwrite it") {
		return dockerutil.HandleNodeJobError(exitCode, stdout, stderr, err)
	}
	cmd = []string{"pcli", "addr", "new", keyName}
	return dockerutil.HandleNodeJobError(p.NodeJob(ctx, cmd))
}

func (p *PenumbraAppNode) GetAddress(ctx context.Context, keyName string) ([]byte, error) {
	cmd := []string{"pcli", "addr", "list"}
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
			// TODO looks like penumbra address is bech64? need to decode to bytes here
			return []byte(fields[2]), nil
		}
	}
	return []byte{}, errors.New("address not found")
}

func (p *PenumbraAppNode) GetAddressBech64(ctx context.Context, keyName string) (string, error) {
	cmd := []string{"pcli", "addr", "list"}
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
			// TODO looks like penumbra address is bech64? need to decode to bytes here
			return fields[2], nil
		}
	}
	return "", errors.New("address not found")
}

func (p *PenumbraAppNode) SendFunds(ctx context.Context, keyName string, amount ibc.WalletAmount) error {
	return errors.New("not yet implemented")
}

func (p *PenumbraAppNode) SendIBCTransfer(ctx context.Context, channelID, keyName string, amount ibc.WalletAmount, timeout *ibc.IBCTimeout) (string, error) {
	return "", errors.New("not yet implemented")
}

func (p *PenumbraAppNode) CreateNodeContainer() error {
	chainCfg := p.Chain.Config()
	cmd := []string{"pd", "start", "--host", "0.0.0.0", "-r", p.NodeHome()}
	fmt.Printf("{%s} -> '%s'\n", p.Name(), strings.Join(cmd, " "))

	cont, err := p.Pool.Client.CreateContainer(docker.CreateContainerOptions{
		Name: p.Name(),
		Config: &docker.Config{
			User:         dockerutil.GetDockerUserString(),
			Cmd:          cmd,
			Hostname:     p.Name(),
			ExposedPorts: exposedPorts,
			DNS:          []string{},
			Image:        fmt.Sprintf("%s:%s", chainCfg.Meta[0], chainCfg.Meta[1]),
			Labels:       map[string]string{"ibc-test": p.TestName},
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
	return p.Pool.Client.StopContainer(p.Container.ID, uint(time.Second*30))
}

func (p *PenumbraAppNode) StartContainer(ctx context.Context) error {
	if err := p.Pool.Client.StartContainer(p.Container.ID, nil); err != nil {
		return err
	}

	c, err := p.Pool.Client.InspectContainer(p.Container.ID)
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
	chainCfg := p.Chain.Config()
	cont, err := p.Pool.Client.CreateContainer(docker.CreateContainerOptions{
		Name: container,
		Config: &docker.Config{
			User:         dockerutil.GetDockerUserString(),
			Hostname:     dockerutil.CondenseHostName(container),
			ExposedPorts: exposedPorts,
			DNS:          []string{},
			Image:        fmt.Sprintf("%s:%s", chainCfg.Repository, chainCfg.Version),
			Cmd:          cmd,
			Labels:       map[string]string{"ibc-test": p.TestName},
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
		return 1, "", "", err
	}
	if err := p.Pool.Client.StartContainer(cont.ID, nil); err != nil {
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
