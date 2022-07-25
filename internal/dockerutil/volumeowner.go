package dockerutil

import (
	"context"
	"fmt"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"go.uber.org/zap"
)

// VolumeOwnerOptions contain the configuration for the SetVolumeOwner function.
type VolumeOwnerOptions struct {
	Log *zap.Logger

	Client *client.Client

	VolumeName string
	ImageRef   string
	TestName   string
}

// SetVolumeOwner configures the owner of a volume to match the default user in the supplied image reference.
func SetVolumeOwner(ctx context.Context, opts VolumeOwnerOptions) error {
	ii, _, err := opts.Client.ImageInspectWithRaw(ctx, opts.ImageRef)
	if err != nil {
		return fmt.Errorf("inspecting image %q: %w", opts.ImageRef, err)
	}

	// Unclear guidance on the difference between the Config and ContainerConfig fields:
	// https://forums.docker.com/t/what-is-the-difference-between-containerconfig-and-config-in-image/83232
	// https://stackoverflow.com/q/36216220
	// Assuming Config is more pertinent.
	u := ii.Config.User
	if u == "" {
		// The inline script expects a real user, and some images
		// may not have an explicit user set. When they don't,
		// it should be safe to assume the user is root.
		u = "root"
	}

	// Start a one-off container to chmod and chown the volume.

	containerName := fmt.Sprintf("ibctest-volumeowner-%d-%s", time.Now().UnixNano(), RandLowerCaseLetterString(5))

	const mountPath = "/mnt/dockervolume"
	cc, err := opts.Client.ContainerCreate(
		ctx,
		&container.Config{
			Image: opts.ImageRef, // Using the original image so the owner is present.

			Entrypoint: []string{"sh", "-c"},
			Cmd: []string{
				`chown "$2" "$1" && chmod 0700 "$1"`,
				"_", // Meaningless arg0 for sh -c with positional args.
				mountPath,
				u,
			},

			// Root user so we have permissions to set ownership and mode.
			User: GetRootUserString(),

			Labels: map[string]string{CleanupLabel: opts.TestName},
		},
		&container.HostConfig{
			Binds:      []string{opts.VolumeName + ":" + mountPath},
			AutoRemove: true,
		},
		nil, // No networking necessary.
		nil,
		containerName,
	)
	if err != nil {
		return fmt.Errorf("creating container: %w", err)
	}

	autoRemoved := false
	defer func() {
		if autoRemoved {
			// No need to attempt removing the container if we successfully started and waited for it to complete.
			return
		}

		if err := opts.Client.ContainerRemove(ctx, cc.ID, types.ContainerRemoveOptions{
			Force: true,
		}); err != nil {
			opts.Log.Warn("Failed to remove volume-owner container", zap.String("container_id", cc.ID), zap.Error(err))
		}
	}()

	if err := opts.Client.ContainerStart(ctx, cc.ID, types.ContainerStartOptions{}); err != nil {
		return fmt.Errorf("starting volume-owner container: %w", err)
	}

	waitCh, errCh := opts.Client.ContainerWait(ctx, cc.ID, container.WaitConditionNotRunning)
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errCh:
		return err
	case res := <-waitCh:
		autoRemoved = true

		if res.Error != nil {
			return fmt.Errorf("waiting for volume-owner container: %s", res.Error.Message)
		}

		if res.StatusCode != 0 {
			return fmt.Errorf("configuring volume exited %d", res.StatusCode)
		}
	}

	return nil
}
