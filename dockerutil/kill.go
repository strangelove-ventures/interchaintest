package dockerutil

import (
	"context"
	"strings"

	dockerapitypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

// KillAllInterchaintestContainers kills all containers that are prefixed with interchaintest specific container names.
// This can be called during cleanup, which is especially useful when running tests which fail to cleanup after themselves.
// Specifically, on failed ic.Build(...) calls.
func KillAllInterchaintestContainers(ctx context.Context) []string {
	removedContainers := []string{}

	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		panic(err)
	}

	containers, err := cli.ContainerList(ctx, dockerapitypes.ContainerListOptions{})
	if err != nil {
		panic(err)
	}

	for _, container := range containers {
		if len(container.Names) == 0 {
			continue
		}

		name := strings.ToLower(container.Names[0])
		name = strings.TrimPrefix(name, "/")

		// leave non ict relayed containers running
		if !(strings.HasPrefix(name, ICTDockerPrefix) || strings.HasPrefix(name, RelayerDockerPrefix)) {
			continue
		}

		inspected, err := cli.ContainerInspect(ctx, container.ID)
		if err != nil {
			panic(err)
		}

		if inspected.State.Running {
			if err := cli.ContainerKill(ctx, container.ID, "SIGKILL"); err != nil {
				panic(err)
			}
			removedContainers = append(removedContainers, name)
		}
	}

	return removedContainers
}
