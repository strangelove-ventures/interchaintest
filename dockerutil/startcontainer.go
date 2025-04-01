package dockerutil

import (
	"context"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/moby/moby/client"
)

// StartContainer attempts to start the container with the given ID.
func StartContainer(ctx context.Context, cli *client.Client, id string) error {
	// add a deadline for the request if the calling context does not provide one
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel func()
		ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
	}

	err := cli.ContainerStart(ctx, id, container.StartOptions{})
	if err != nil {
		return err
	}

	return nil
}
