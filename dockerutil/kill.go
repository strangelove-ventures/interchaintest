package dockerutil

import (
	"context"
	"os"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/moby/moby/client"
)

// KillAllInterchaintestContainers kills all containers that are prefixed with interchaintest specific container names.
// This can be called during cleanup, which is especially useful when running tests which fail to cleanup after themselves.
// Specifically, on failed ic.Build(...) calls.
func KillAllInterchaintestContainers(ctx context.Context) []string {
	if os.Getenv("KEEP_CONTAINERS") != "" {
		return nil
	}

	removedContainers := []string{}

	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		panic(err)
	}

	containers, err := cli.ContainerList(ctx, container.ListOptions{
		// grabs only running containers
		All: false,
		Filters: filters.NewArgs(
			filters.Arg("label", CleanupLabel),
		),
	})
	if err != nil {
		panic(err)
	}

	for _, container := range containers {
		if len(container.Names) == 0 {
			continue
		}

		name := strings.ToLower(container.Names[0])
		name = strings.TrimPrefix(name, "/")

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
