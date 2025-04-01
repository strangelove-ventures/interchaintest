package dockerutil

import (
	"math/rand"
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
	"github.com/stretchr/testify/require"
)

func TestGetHostPort(t *testing.T) {
	tests := []struct {
		name     string
		cont     container.InspectResponse
		portID   string
		expected string
	}{
		{
			name: "valid port",
			cont: container.InspectResponse{
				NetworkSettings: &container.NetworkSettings{
					NetworkSettingsBase: container.NetworkSettingsBase{
						Ports: nat.PortMap{
							"8080/tcp": []nat.PortBinding{
								{HostIP: "0.0.0.0", HostPort: "8081"},
							},
						},
					},
				},
			},
			portID:   "8080/tcp",
			expected: "0.0.0.0:8081",
		},
		{
			name: "no port",
			cont: container.InspectResponse{
				NetworkSettings: &container.NetworkSettings{
					NetworkSettingsBase: container.NetworkSettingsBase{
						Ports: nat.PortMap{},
					},
				},
			},
			portID:   "8080/tcp",
			expected: "",
		},
		{
			name:     "nil network settings",
			cont:     container.InspectResponse{},
			portID:   "8080/tcp",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetHostPort(tt.cont, tt.portID)
			if got != tt.expected {
				t.Errorf("GetHostPort() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestRandLowerCaseLetterString(t *testing.T) {
	require.Empty(t, RandLowerCaseLetterString(0))

	rand.Seed(1) // nolint:staticcheck
	require.Equal(t, "xvlbzgbaicmr", RandLowerCaseLetterString(12))

	rand.Seed(1) // nolint:staticcheck
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
