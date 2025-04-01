package dockerutil

import (
	"bytes"
	"context"
	"fmt"
	"math/rand"
	"net"
	"os"
	"strings"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/moby/moby/client"
	"github.com/moby/moby/errdefs"
)

// DockerSetupTestingT is a subset of testing.T required for DockerSetup.
type DockerSetupTestingT interface {
	Helper()

	Name() string

	Failed() bool
	Cleanup(func())

	Logf(format string, args ...any)
}

// CleanupLabel is a docker label key targeted by DockerSetup when it cleans up docker resources.
//
// "interchaintest" is perhaps a better name. However, for backwards compatibility we preserve the original name of "ibc-test"
// with the hyphen. Otherwise, we run the risk of causing "container already exists" errors because DockerSetup
// is unable to clean old resources from docker engine.
const CleanupLabel = "ibc-test"

// CleanupLabel is the "old" format.
// Note that any new labels should follow the reverse DNS format suggested at
// https://docs.docker.com/config/labels-custom-metadata/#key-format-recommendations.

const (
	// LabelPrefix is the reverse DNS format "namespace" for interchaintest Docker labels.
	LabelPrefix = "ventures.strangelove.interchaintest."

	// NodeOwnerLabel indicates the logical node owning a particular object (probably a volume).
	NodeOwnerLabel = LabelPrefix + "node-owner"
)

// KeepVolumesOnFailure determines whether volumes associated with a test
// using DockerSetup are retained or deleted following a test failure.
//
// The value is false by default, but can be initialized to true by setting the
// environment variable ICTEST_SKIP_FAILURE_CLEANUP to a non-empty value.
// Alternatively, importers of the dockerutil package may set the variable to true.
// Because dockerutil is an internal package, the public API for setting this value
// is interchaintest.KeepDockerVolumesOnFailure(bool).
var KeepVolumesOnFailure = os.Getenv("ICTEST_SKIP_FAILURE_CLEANUP") != ""

// DockerSetup returns a new Docker Client and the ID of a configured network, associated with t.
//
// If any part of the setup fails, DockerSetup panics because the test cannot continue.
func DockerSetup(t DockerSetupTestingT) (*client.Client, string) {
	t.Helper()

	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		panic(fmt.Errorf("failed to create docker client: %v", err))
	}

	// Clean up docker resources at end of test.
	t.Cleanup(DockerCleanup(t, cli))

	// Also eagerly clean up any leftover resources from a previous test run,
	// e.g. if the test was interrupted.
	DockerCleanup(t, cli)()

	name := fmt.Sprintf("%s-%s", ICTDockerPrefix, RandLowerCaseLetterString(8))
	octet := uint8(rand.Intn(256))
	baseSubnet := fmt.Sprintf("172.%d.0.0/16", octet)
	usedSubnets, err := getUsedSubnets(cli)
	if err != nil {
		panic(fmt.Errorf("failed to get used subnets: %v", err))
	}
	subnet, err := findAvailableSubnet(baseSubnet, usedSubnets)
	if err != nil {
		panic(fmt.Errorf("failed to find an available subnet: %v", err))
	}
	network, err := cli.NetworkCreate(context.TODO(), name, network.CreateOptions{
		Driver: "bridge",
		IPAM: &network.IPAM{
			Config: []network.IPAMConfig{
				{
					Subnet: subnet,
				},
			},
		},

		Labels: map[string]string{CleanupLabel: t.Name()},
	})
	if err != nil {
		panic(fmt.Errorf("failed to create docker network: %v", err))
	}

	return cli, network.ID
}

func getUsedSubnets(cli *client.Client) (map[string]bool, error) {
	usedSubnets := make(map[string]bool)
	networks, err := cli.NetworkList(context.TODO(), network.ListOptions{})
	if err != nil {
		return nil, err
	}

	for _, net := range networks {
		for _, config := range net.IPAM.Config {
			if config.Subnet != "" {
				usedSubnets[config.Subnet] = true
			}
		}
	}
	return usedSubnets, nil
}

func findAvailableSubnet(baseSubnet string, usedSubnets map[string]bool) (string, error) {
	ip, ipNet, err := net.ParseCIDR(baseSubnet)
	if err != nil {
		return "", fmt.Errorf("invalid base subnet: %v", err)
	}

	for {
		if isSubnetUsed(ipNet.String(), usedSubnets) {
			incrementIP(ip, 2)
			ipNet.IP = ip
			continue
		}

		for subIP := ip.Mask(ipNet.Mask); ipNet.Contains(subIP); incrementIP(subIP, 1) {
			subnet := fmt.Sprintf("%s/24", subIP)

			if !isSubnetUsed(subnet, usedSubnets) {
				return subnet, nil
			}
		}

		incrementIP(ip, 2)
		ipNet.IP = ip
	}
}

func isSubnetUsed(subnet string, usedSubnets map[string]bool) bool {
	_, targetNet, err := net.ParseCIDR(subnet)
	if err != nil {
		return true
	}

	for usedSubnet := range usedSubnets {
		_, usedNet, err := net.ParseCIDR(usedSubnet)
		if err != nil {
			continue
		}

		if usedNet.Contains(targetNet.IP) || targetNet.Contains(usedNet.IP) {
			return true
		}
	}
	return false
}

func incrementIP(ip net.IP, incrementLevel int) {
	for j := len(ip) - incrementLevel; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}

// DockerCleanup will clean up Docker containers, networks, and the other various config files generated in testing.
func DockerCleanup(t DockerSetupTestingT, cli *client.Client) func() {
	return func() {
		showContainerLogs := os.Getenv("SHOW_CONTAINER_LOGS")
		containerLogTail := os.Getenv("CONTAINER_LOG_TAIL")
		keepContainers := os.Getenv("KEEP_CONTAINERS") != ""

		ctx := context.TODO()
		cli.NegotiateAPIVersion(ctx)
		cs, err := cli.ContainerList(ctx, container.ListOptions{
			All: true,
			Filters: filters.NewArgs(
				filters.Arg("label", CleanupLabel+"="+t.Name()),
			),
		})
		if err != nil {
			t.Logf("Failed to list containers during docker cleanup: %v", err)
			return
		}

		for _, c := range cs {
			if (t.Failed() && showContainerLogs == "") || showContainerLogs == "always" {
				logTail := "50"
				if containerLogTail != "" {
					logTail = containerLogTail
				}
				rc, err := cli.ContainerLogs(ctx, c.ID, container.LogsOptions{
					ShowStdout: true,
					ShowStderr: true,
					Tail:       logTail,
				})
				if err == nil {
					b := new(bytes.Buffer)
					_, err := b.ReadFrom(rc)
					if err == nil {
						t.Logf("\n\nContainer logs - {%s}\n%s", strings.Join(c.Names, " "), b.String())
					}
				}
			}
			if !keepContainers {
				var stopTimeout container.StopOptions
				timeout := 10
				timeoutDur := time.Duration(timeout * int(time.Second))
				deadline := time.Now().Add(timeoutDur)
				stopTimeout.Timeout = &timeout
				if err := cli.ContainerStop(ctx, c.ID, stopTimeout); IsLoggableStopError(err) {
					t.Logf("Failed to stop container %s during docker cleanup: %v", c.ID, err)
				}

				waitCtx, cancel := context.WithDeadline(ctx, deadline.Add(500*time.Millisecond))
				waitCh, errCh := cli.ContainerWait(waitCtx, c.ID, container.WaitConditionNotRunning)
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

				if err := cli.ContainerRemove(ctx, c.ID, container.RemoveOptions{
					// Not removing volumes with the container, because we separately handle them conditionally.
					Force: true,
				}); err != nil {
					t.Logf("Failed to remove container %s during docker cleanup: %v", c.ID, err)
				}
			}
		}

		if !keepContainers {
			PruneVolumesWithRetry(ctx, t, cli)
			PruneNetworksWithRetry(ctx, t, cli)
		} else {
			t.Logf("Keeping containers - Docker cleanup skipped")
		}
	}
}

func PruneVolumesWithRetry(ctx context.Context, t DockerSetupTestingT, cli *client.Client) {
	if KeepVolumesOnFailure && t.Failed() {
		return
	}

	var msg string
	err := retry.Do(
		func() error {
			res, err := cli.VolumesPrune(ctx, filters.NewArgs(filters.Arg("label", CleanupLabel+"="+t.Name())))
			if err != nil {
				if errdefs.IsConflict(err) {
					// Prune is already in progress; try again.
					return err
				}

				// Give up on any other error.
				return retry.Unrecoverable(err)
			}

			if len(res.VolumesDeleted) > 0 {
				msg = fmt.Sprintf("Pruned %d volumes, reclaiming approximately %.1f MB", len(res.VolumesDeleted), float64(res.SpaceReclaimed)/(1024*1024))
			}

			return nil
		},
		retry.Context(ctx),
		retry.DelayType(retry.FixedDelay),
	)
	if err != nil {
		t.Logf("Failed to prune volumes during docker cleanup: %v", err)
		return
	}

	if msg != "" {
		// Odd to Logf %s, but this is a defensive way to keep the DockerSetupTestingT interface
		// with only Logf and not need to add Log.
		t.Logf("%s", msg)
	}
}

func PruneNetworksWithRetry(ctx context.Context, t DockerSetupTestingT, cli *client.Client) {
	var deleted []string
	err := retry.Do(
		func() error {
			res, err := cli.NetworksPrune(ctx, filters.NewArgs(filters.Arg("label", CleanupLabel+"="+t.Name())))
			if err != nil {
				if errdefs.IsConflict(err) {
					// Prune is already in progress; try again.
					return err
				}

				// Give up on any other error.
				return retry.Unrecoverable(err)
			}

			deleted = res.NetworksDeleted
			return nil
		},
		retry.Context(ctx),
		retry.DelayType(retry.FixedDelay),
	)
	if err != nil {
		t.Logf("Failed to prune networks during docker cleanup: %v", err)
		return
	}

	if len(deleted) > 0 {
		t.Logf("Pruned unused networks: %v", deleted)
	}
}

func IsLoggableStopError(err error) bool {
	if err == nil {
		return false
	}
	return !(errdefs.IsNotModified(err) || errdefs.IsNotFound(err))
}
