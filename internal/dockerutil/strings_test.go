package dockerutil

import (
	"math/rand"
	"testing"

	"github.com/ory/dockertest/v3/docker"
	"github.com/stretchr/testify/require"
)

func TestGetHostPort(t *testing.T) {
	for _, tt := range []struct {
		Container *docker.Container
		PortID    string
		Want      string
	}{
		{&docker.Container{
			NetworkSettings: &docker.NetworkSettings{
				Ports: map[docker.Port][]docker.PortBinding{"test": {
					{HostIP: "1.2.3.4", HostPort: "8080"}, {HostIP: "0.0.0.0", HostPort: "9999"}},
				},
			},
		}, "test", "1.2.3.4:8080"},

		{&docker.Container{
			NetworkSettings: &docker.NetworkSettings{
				Ports: map[docker.Port][]docker.PortBinding{"test": {
					{HostIP: "0.0.0.0", HostPort: "3000"}},
				},
			},
		}, "test", "localhost:3000"},

		{nil, "", ""},
		{&docker.Container{}, "", ""},
		{&docker.Container{NetworkSettings: &docker.NetworkSettings{}}, "does-not-matter", ""},
	} {
		require.Equal(t, tt.Want, GetHostPort(tt.Container, tt.PortID), tt)
	}
}

func TestRandLowerCaseLetterString(t *testing.T) {
	require.Empty(t, RandLowerCaseLetterString(0))

	rand.Seed(1)
	require.Equal(t, "xvlbzgbaicmr", RandLowerCaseLetterString(12))

	rand.Seed(1)
	require.Equal(t, "xvlbzgbaicmrajwwhthctcuaxhxkqf", RandLowerCaseLetterString(30))
}

func TestCondenseHostName(t *testing.T) {
	for _, tt := range []struct {
		HostName, Want string
	}{
		{"", ""},
		{"test", "test"},
		{"some-really-very-incredibly-long-hostname-that-is-greater-than-64-characters", "some-really-very-incredibly-lo_._-is-greater-than-64-characters"},
	} {
		require.Equal(t, tt.Want, CondenseHostName(tt.HostName), tt)
	}
}

func TestSanitizeContainerName(t *testing.T) {
	for _, tt := range []struct {
		Name, Want string
	}{
		{"hello-there", "hello-there"},
		{"hello@there", "hello_there"},
		{"hello@/there", "hello__there"},
		// edge cases
		{"?", "_"},
		{"", ""},
	} {
		require.Equal(t, tt.Want, SanitizeContainerName(tt.Name), tt)
	}
}
