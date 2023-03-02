package dockerutil

import (
	"context"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

// StartContainer attempts to start the container with the given ID.
// If the request times out, it retries a certain number of times before failing.
// Any other failure modes stop immediately.
func StartContainer(ctx context.Context, cli *client.Client, id string) error {
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel func()
		ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
	}

	err := cli.ContainerStart(ctx, id, types.ContainerStartOptions{})
	if err != nil {
		return retry.Unrecoverable(err)
	}

	return nil
}
