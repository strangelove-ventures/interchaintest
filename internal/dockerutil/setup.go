package dockerutil

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/docker/docker/errdefs"
)

// CleanupLabel is a docker label key targeted by DockerSetup when it cleans up docker resources.
//
// "ibctest" is perhaps a better name. However, for backwards compatability we preserve the original name of "ibc-test"
// with the hyphen. Otherwise, we run the risk of causing "container already exists" errors because DockerSetup
// is unable to clean old resources from docker engine.
const CleanupLabel = "ibc-test"

// DockerSetup returns a new Docker Client and the ID of a configured network, associated with t.
//
// If any part of the setup fails, t.Fatal is called.
func DockerSetup(t *testing.T) (*client.Client, string) {
	t.Helper()

	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		t.Fatalf("failed to create docker client: %v", err)
	}

	// Clean up docker resources at end of test.
	t.Cleanup(dockerCleanup(t, cli))

	// Also eagerly clean up any leftover resources from a previous test run,
	// e.g. if the test was interrupted.
	dockerCleanup(t, cli)()

	name := fmt.Sprintf("ibctest-%s", RandLowerCaseLetterString(8))
	network, err := cli.NetworkCreate(context.TODO(), name, types.NetworkCreate{
		CheckDuplicate: true,

		Labels: map[string]string{CleanupLabel: t.Name()},
	})
	if err != nil {
		t.Fatalf("failed to create docker network: %v", err)
	}

	return cli, network.ID
}

// dockerCleanup will clean up Docker containers, networks, and the other various config files generated in testing
func dockerCleanup(t *testing.T, client *client.Client) func() {
	return func() {
		ctx := context.TODO()

		cs, err := client.ContainerList(ctx, types.ContainerListOptions{
			All:     true,
			Filters: filters.NewArgs(filters.Arg("name", t.Name())),
		})
		if err != nil {
			t.Logf("Failed to list containers during docker cleanup: %v", err)
			return
		}

		for _, c := range cs {
			stopTimeout := 10 * time.Second
			deadline := time.Now().Add(stopTimeout)
			if err := client.ContainerStop(ctx, c.ID, &stopTimeout); isLoggableStopError(err) {
				t.Logf("Failed to stop container %s during docker cleanup: %v", c.ID, err)
			}

			waitCtx, cancel := context.WithDeadline(ctx, deadline.Add(500*time.Millisecond))
			waitCh, errCh := client.ContainerWait(waitCtx, c.ID, container.WaitConditionNotRunning)
			select {
			case <-waitCtx.Done():
				t.Logf("Timed out waiting for container %s", c.ID)
			case err := <-errCh:
				t.Logf("Failed to wait for container %s during docker cleanup: %v", c.ID, err)
			case res := <-waitCh:
				if res.Error != nil {
					t.Logf("Error while waiting for container %s during docker cleanup: %s", c.ID, res.Error.Message)
				}
				// Ignoring statuscode for now.
			}
			cancel()

			if err := client.ContainerRemove(ctx, c.ID, types.ContainerRemoveOptions{
				RemoveVolumes: true,
				Force:         true,
			}); err != nil {
				t.Logf("Failed to remove container %s during docker cleanup: %v", c.ID, err)
			}
		}

		res, err := client.NetworksPrune(ctx, filters.NewArgs(filters.Arg("label", CleanupLabel+"="+t.Name())))
		if err != nil {
			// TODO: some errors should not be logged, use errdefs to check.
			t.Logf("Failed to prune networks during docker cleanup: %v", err)
		}
		if len(res.NetworksDeleted) > 0 {
			t.Logf("Pruned unused networks: %v", res.NetworksDeleted)
		}
	}
}

func isLoggableStopError(err error) bool {
	if err == nil {
		return false
	}
	return !(errdefs.IsNotModified(err) || errdefs.IsNotFound(err))
}
