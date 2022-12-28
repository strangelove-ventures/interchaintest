package dockerutil

import (
	"context"
	"errors"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

// StartContainer attempts to start the container with the given ID.
// If the request times out, it retries a certain number of times before failing.
// Any other failure modes stop immediately.
func StartContainer(ctx context.Context, cli *client.Client, id string) error {
	return retry.Do(
		func() error {
			retryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()

			err := cli.ContainerStart(retryCtx, id, types.ContainerStartOptions{})
			if err != nil {
				// One special case: retryCtx timed out and the outer ctx didn't.
				if errors.Is(err, context.DeadlineExceeded) && ctx.Err() == nil {
					return err
				}

				// Otherwise, assume the error cannot be retried.
				return retry.Unrecoverable(err)
			}

			return nil
		},
		retry.Context(ctx),
		retry.DelayType(retry.FixedDelay),
	)
}
