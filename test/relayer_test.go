package test

import (
	"os"
	"testing"

	"github.com/ory/dockertest"
	"github.com/ory/dockertest/docker"
)

func TestChainSpinUp(t *testing.T) {
	ctx, home, pool, network, validators := SetupTestRun(t, 4)

	t.Cleanup(Cleanup(pool, t.Name(), home))

	// TODO(desa): I think these need to be different from the existing validators
	fullnodes := []*TestNode{}
	// start validators and sentry nodes
	StartNodeContainers(t, ctx, network, validators, fullnodes)

	// Wait for all nodes to get to given block height
	validators.WaitForHeight(5)
}

// Cleanup will clean up Docker containers, networks, and the other various config files generated in testing
func Cleanup(pool *dockertest.Pool, testName, testDir string) func() {
	return func() {
		cont, _ := pool.Client.ListContainers(docker.ListContainersOptions{All: true})
		for _, c := range cont {
			for k, v := range c.Labels {
				if k == "horcrux-test" && v == testName {
					_ = pool.Client.StopContainer(c.ID, 10)
				}
			}
		}
		nets, _ := pool.Client.ListNetworks()
		for _, n := range nets {
			for k, v := range n.Labels {
				if k == "horcrux-test" && v == testName {
					_ = pool.Client.RemoveNetwork(n.ID)
				}
			}
		}
		_ = os.RemoveAll(testDir)
	}
}
