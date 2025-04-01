package dockerutil

import (
	"fmt"
	"math/rand"
	"net"
	"os"
	"regexp"
	"runtime"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
)

const (
	ICTDockerPrefix = "interchaintest"
)

// GetHostPort returns a resource's published port with an address.
// cont is the type returned by the Docker client's ContainerInspect method.
func GetHostPort(cont container.InspectResponse, portID string) string {
	if cont.NetworkSettings == nil {
		return ""
	}

	m, ok := cont.NetworkSettings.Ports[nat.Port(portID)]
	if !ok || len(m) == 0 {
		return ""
	}

	return net.JoinHostPort(m[0].HostIP, m[0].HostPort)
}

var chars = []byte("abcdefghijklmnopqrstuvwxyz")

// RandLowerCaseLetterString returns a lowercase letter string of given length.
func RandLowerCaseLetterString(length int) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = chars[rand.Intn(len(chars))]
	}
	return string(b)
}

func GetDockerUserString() string {
	uid := os.Getuid()
	var usr string
	if runtime.GOOS == "darwin" {
		usr = ""
	} else {
		usr = fmt.Sprintf("%d:%d", uid, uid)
	}
	return usr
}

func GetHeighlinerUserString() string {
	return "1025:1025"
}

func GetRootUserString() string {
	return "0:0"
}

// CondenseHostName truncates the middle of the given name
// if it is 64 characters or longer.
//
// Without this helper, you may see an error like:
//
//	API error (500): failed to create shim: OCI runtime create failed: container_linux.go:380: starting container process caused: process_linux.go:545: container init caused: sethostname: invalid argument: unknown
func CondenseHostName(name string) string {
	if len(name) < 64 {
		return name
	}

	// I wanted to use ... as the middle separator,
	// but that causes resolution problems for other hosts.
	// Instead, use _._ which will be okay if there is a . on either end.
	return name[:30] + "_._" + name[len(name)-30:]
}

var validContainerCharsRE = regexp.MustCompile(`[^a-zA-Z0-9_.-]`)

// SanitizeContainerName returns name with any
// invalid characters replaced with underscores.
// Subtests will include slashes, and there may be other
// invalid characters too.
func SanitizeContainerName(name string) string {
	return validContainerCharsRE.ReplaceAllLiteralString(name, "_")
}
