package utils

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"net"
	"os"
	"regexp"
	"runtime"
	"strings"

	"github.com/ory/dockertest/docker"
)

// GetHostPort returns a resource's published port with an address.
func GetHostPort(cont *docker.Container, portID string) string {
	if cont == nil || cont.NetworkSettings == nil {
		return ""
	}

	m, ok := cont.NetworkSettings.Ports[docker.Port(portID)]
	if !ok || len(m) == 0 {
		return ""
	}

	ip := m[0].HostIP
	if ip == "0.0.0.0" {
		ip = "localhost"
	}
	return net.JoinHostPort(ip, m[0].HostPort)
}

// RandLowerCaseLetterString returns a lowercase letter string of given length
func RandLowerCaseLetterString(length int) string {
	chars := []rune("abcdefghijklmnopqrstuvwxyz")
	var b strings.Builder
	for i := 0; i < length; i++ {
		i, _ := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		b.WriteRune(chars[i.Int64()])
	}
	return b.String()
}

func HandleNodeJobError(exitCode int, stdout, stderr string, err error) error {
	if err != nil {
		return err
	}
	if exitCode != 0 {
		return fmt.Errorf("container returned non-zero error code: %d\n", exitCode)
	}
	return nil
}

func GetDockerUserString() string {
	uid := os.Getuid()
	var usr string
	userOS := runtime.GOOS
	if userOS == "darwin" {
		usr = ""
	} else {
		usr = fmt.Sprintf("%d:%d", uid, uid)
	}
	return usr
}

// condenseHostName truncates the middle of the given name
// if it is 64 characters or longer.
//
// Without this helper, you may see an error like:
//     API error (500): failed to create shim: OCI runtime create failed: container_linux.go:380: starting container process caused: process_linux.go:545: container init caused: sethostname: invalid argument: unknown
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
