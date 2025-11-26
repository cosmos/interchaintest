package dockerutil

import (
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
	"github.com/stretchr/testify/require"
)

func TestGetHostPort(t *testing.T) {
	for _, tt := range []struct {
		Container container.InspectResponse
		PortID    string
		Want      string
	}{
		{
			func() container.InspectResponse {
				resp := container.InspectResponse{
					NetworkSettings: &container.NetworkSettings{},
				}
				resp.NetworkSettings.Ports = nat.PortMap{
					nat.Port("test"): []nat.PortBinding{
						{HostIP: "1.2.3.4", HostPort: "8080"},
						{HostIP: "0.0.0.0", HostPort: "9999"},
					},
				}
				return resp
			}(), "test", "1.2.3.4:8080",
		},
		{
			func() container.InspectResponse {
				resp := container.InspectResponse{
					NetworkSettings: &container.NetworkSettings{},
				}
				resp.NetworkSettings.Ports = nat.PortMap{
					nat.Port("test"): []nat.PortBinding{
						{HostIP: "0.0.0.0", HostPort: "3000"},
					},
				}
				return resp
			}(), "test", "0.0.0.0:3000",
		},

		{container.InspectResponse{}, "", ""},
		{container.InspectResponse{NetworkSettings: &container.NetworkSettings{}}, "does-not-matter", ""},
	} {
		require.Equal(t, tt.Want, GetHostPort(tt.Container, tt.PortID), tt)
	}
}

func TestRandLowerCaseLetterString(t *testing.T) {
	require.Empty(t, RandLowerCaseLetterString(0))

	result12 := RandLowerCaseLetterString(12)
	require.Len(t, result12, 12)
	require.Regexp(t, "^[a-z]+$", result12)

	result30 := RandLowerCaseLetterString(30)
	require.Len(t, result30, 30)
	require.Regexp(t, "^[a-z]+$", result30)
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
