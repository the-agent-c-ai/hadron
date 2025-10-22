package debian

import "errors"

// Package management errors.
var (
	// ErrPackageInstallFailed indicates a package installation failed.
	ErrPackageInstallFailed = errors.New("package installation failed")
	// ErrPackageRemoveFailed indicates a package removal failed.
	ErrPackageRemoveFailed = errors.New("package removal failed")
)

// Docker installation errors.
var (
	// ErrDockerPrereqFailed indicates Docker prerequisite installation failed.
	ErrDockerPrereqFailed = errors.New("docker prerequisite installation failed")
	// ErrDockerKeyringsDir indicates failed to create Docker keyrings directory.
	ErrDockerKeyringsDir = errors.New("failed to create docker keyrings directory")
	// ErrDockerGPGDownload indicates failed to download Docker GPG key.
	ErrDockerGPGDownload = errors.New("failed to download docker GPG key")
	// ErrDockerGPGPermissions indicates failed to set Docker GPG key permissions.
	ErrDockerGPGPermissions = errors.New("failed to set docker GPG key permissions")
	// ErrDockerRepoWrite indicates failed to write Docker repository file.
	ErrDockerRepoWrite = errors.New("failed to write docker repository file")
	// ErrDockerRepoUpdate indicates apt-get update failed after adding Docker repository.
	ErrDockerRepoUpdate = errors.New("apt-get update failed after adding docker repository")
	// ErrDockerPackageInstall indicates Docker package installation failed.
	ErrDockerPackageInstall = errors.New("docker package installation failed")
	// ErrDockerArchFetch indicates failed to get system architecture.
	ErrDockerArchFetch = errors.New("failed to get architecture")
	// ErrDockerCodenameFetch indicates failed to get Debian codename.
	ErrDockerCodenameFetch = errors.New("failed to get debian codename")
)
