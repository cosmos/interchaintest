package relayer

import (
	"github.com/cosmos/interchaintest/v10/ibc"
)

// RelayerOpt is a functional option for configuring a relayer.
type RelayerOpt func(relayer *DockerRelayer)

// DockerImage overrides the default relayer docker image.
func DockerImage(image *ibc.DockerImage) RelayerOpt {
	return func(r *DockerRelayer) {
		r.customImage = image
	}
}

// CustomDockerImage overrides the default relayer docker image.
// uidGid is the uid:gid format owner that should be used within the container.
// If uidGid is empty, root user will be assumed.
func CustomDockerImage(repository string, version string, uidGID string) RelayerOpt {
	return DockerImage(&ibc.DockerImage{
		Repository: repository,
		Version:    version,
		UIDGID:     uidGID,
	})
}

// HomeDir overrides the default relayer home directory.
func HomeDir(homeDir string) RelayerOpt {
	return func(r *DockerRelayer) {
		r.homeDir = homeDir
	}
}

// ImagePull overrides whether the relayer image should be pulled on startup.
func ImagePull(pull bool) RelayerOpt {
	return func(r *DockerRelayer) {
		r.pullImage = pull
	}
}

// StartupFlags overrides the default relayer startup flags.
func StartupFlags(flags ...string) RelayerOpt {
	return func(r *DockerRelayer) {
		r.extraStartupFlags = flags
	}
}
