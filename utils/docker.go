package utils

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"net"
	"os"
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
