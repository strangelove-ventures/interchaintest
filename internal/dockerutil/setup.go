package dockerutil

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
)

// CleanupLabel is a docker label key targeted by DockerSetup when it cleans up docker resources.
//
// "ibctest" is perhaps a better name. However, for backwards compatability we preserve the original name of "ibc-test"
// with the hyphen. Otherwise, we run the risk of causing "container already exists" errors because DockerSetup
// is unable to clean old resources from docker engine.
const CleanupLabel = "ibc-test"

// DockerSetup sets up a new dockertest.Pool (which is a client connection
// to a Docker engine) and configures a network associated with t.
// Returns a pool and the network id.
//
// If any part of the setup fails, t.Fatal is called.
func DockerSetup(t *testing.T) (*dockertest.Pool, string) {
	t.Helper()

	pool, err := dockertest.NewPool("")
	if err != nil {
		t.Fatalf("failed to create dockertest pool: %v", err)
	}

	// Clean up docker resources at end of test.
	t.Cleanup(dockerCleanup(t.Name(), pool))

	// Also eagerly clean up any leftover resources from a previous test run,
	// e.g. if the test was interrupted.
	dockerCleanup(t.Name(), pool)()

	name := fmt.Sprintf("ibctest-%s", RandLowerCaseLetterString(8))
	network, err := pool.CreateNetwork(name, func(cfg *docker.CreateNetworkOptions) {
		cfg.Labels = map[string]string{CleanupLabel: t.Name()}
		cfg.CheckDuplicate = true
		cfg.Context = context.Background() // TODO (nix - 6/24/22) Pass in context from function call.
	})
	if err != nil {
		t.Fatalf("failed to create docker network: %v", err)
	}

	return pool, network.Network.ID
}

// dockerCleanup will clean up Docker containers, networks, and the other various config files generated in testing
func dockerCleanup(testName string, pool *dockertest.Pool) func() {
	return func() {
		cont, _ := pool.Client.ListContainers(docker.ListContainersOptions{All: true})
		for _, c := range cont {
			for k, v := range c.Labels {
				if k == CleanupLabel && v == testName {
					_ = pool.Client.StopContainer(c.ID, 10)
					ctxWait, cancelWait := context.WithTimeout(context.Background(), time.Second*5)
					_, _ = pool.Client.WaitContainerWithContext(c.ID, ctxWait)
					_ = pool.Client.RemoveContainer(docker.RemoveContainerOptions{ID: c.ID, Force: true})
					cancelWait() // prevent deferring in a loop
					break
				}
			}
		}
		nets, _ := pool.Client.ListNetworks()
		for _, n := range nets {
			for k, v := range n.Labels {
				if k == CleanupLabel && v == testName {
					_ = pool.Client.RemoveNetwork(n.ID)
					break
				}
			}
		}
	}
}
