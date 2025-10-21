package debian

import (
	"errors"
	"fmt"
	"strings"

	"github.com/the-agent-c-ai/hadron/sdk/ssh"
)

const (
	dockerGPGURL   = "https://download.docker.com/linux/debian/gpg"
	dockerGPGPath  = "/etc/apt/keyrings/docker.asc"
	dockerRepoFile = "/etc/apt/sources.list.d/docker.list"
	dockerKeyrings = "/etc/apt/keyrings"
)

var (
	errDockerPrereqFailed   = errors.New("docker prerequisite installation failed")
	errDockerKeyringsDir    = errors.New("failed to create docker keyrings directory")
	errDockerGPGDownload    = errors.New("failed to download docker GPG key")
	errDockerGPGPermissions = errors.New("failed to set docker GPG key permissions")
	errDockerRepoWrite      = errors.New("failed to write docker repository file")
	errDockerRepoUpdate     = errors.New("apt-get update failed after adding docker repository")
	errDockerPackageInstall = errors.New("docker package installation failed")
	errDockerArchFetch      = errors.New("failed to get architecture")
	errDockerCodenameFetch  = errors.New("failed to get debian codename")
)

// InstallDocker installs Docker CE following the official Debian installation procedure.
// See: https://docs.docker.com/engine/install/debian/
func InstallDocker(client ssh.Connection) error {
	// Step 1: Install prerequisites
	if err := installDockerPrerequisites(client); err != nil {
		return fmt.Errorf("failed to install Docker prerequisites: %w", err)
	}

	// Step 2: Add Docker's official GPG key
	if err := addDockerGPGKey(client); err != nil {
		return fmt.Errorf("failed to add Docker GPG key: %w", err)
	}

	// Step 3: Set up Docker repository
	if err := setupDockerRepository(client); err != nil {
		return fmt.Errorf("failed to setup Docker repository: %w", err)
	}

	// Step 4: Install Docker packages
	if err := installDockerPackages(client); err != nil {
		return fmt.Errorf("failed to install Docker packages: %w", err)
	}

	return nil
}

// installDockerPrerequisites installs ca-certificates and curl.
func installDockerPrerequisites(client ssh.Connection) error {
	cmd := "DEBIAN_FRONTEND=noninteractive apt-get update -qq && " +
		"apt-get install -y -qq --no-install-recommends ca-certificates curl"

	_, stderr, err := client.Execute(cmd)
	if err != nil {
		return fmt.Errorf("%w: %s", errDockerPrereqFailed, stderr)
	}

	return nil
}

// addDockerGPGKey downloads and installs Docker's GPG key.
func addDockerGPGKey(client ssh.Connection) error {
	// Create keyrings directory with proper permissions
	createDirCmd := "install -m 0755 -d " + dockerKeyrings

	_, stderr, err := client.Execute(createDirCmd)
	if err != nil {
		return fmt.Errorf("%w: %s", errDockerKeyringsDir, stderr)
	}

	// Download Docker GPG key
	downloadKeyCmd := fmt.Sprintf("curl -fsSL %s -o %s", dockerGPGURL, dockerGPGPath)

	_, stderr, err = client.Execute(downloadKeyCmd)
	if err != nil {
		return fmt.Errorf("%w: %s", errDockerGPGDownload, stderr)
	}

	// Set proper permissions on GPG key
	chmodCmd := "chmod a+r " + dockerGPGPath

	_, stderr, err = client.Execute(chmodCmd)
	if err != nil {
		return fmt.Errorf("%w: %s", errDockerGPGPermissions, stderr)
	}

	return nil
}

// setupDockerRepository adds Docker's apt repository to sources.list.d.
func setupDockerRepository(client ssh.Connection) error {
	// Get architecture
	archCmd := "dpkg --print-architecture"

	arch, _, err := client.Execute(archCmd)
	if err != nil {
		return fmt.Errorf("%w: %w", errDockerArchFetch, err)
	}

	arch = strings.TrimSpace(arch)

	// Get Debian version codename
	codenameCmd := ". /etc/os-release && echo \"$VERSION_CODENAME\""

	codename, _, err := client.Execute(codenameCmd)
	if err != nil {
		return fmt.Errorf("%w: %w", errDockerCodenameFetch, err)
	}

	codename = strings.TrimSpace(codename)

	// Create repository entry
	repoLine := fmt.Sprintf(
		"deb [arch=%s signed-by=%s] https://download.docker.com/linux/debian %s stable",
		arch, dockerGPGPath, codename,
	)

	// Write repository file
	writeRepoCmd := fmt.Sprintf("echo '%s' | tee %s > /dev/null", repoLine, dockerRepoFile)

	_, stderr, err := client.Execute(writeRepoCmd)
	if err != nil {
		return fmt.Errorf("%w: %s", errDockerRepoWrite, stderr)
	}

	// Update apt cache
	updateCmd := "apt-get update -qq"

	_, stderr, err = client.Execute(updateCmd)
	if err != nil {
		return fmt.Errorf("%w: %s", errDockerRepoUpdate, stderr)
	}

	return nil
}

// installDockerPackages installs Docker CE and related packages.
func installDockerPackages(client ssh.Connection) error {
	packages := []string{
		"docker-ce",
		"docker-ce-cli",
		"containerd.io",
		"docker-buildx-plugin",
		"docker-compose-plugin",
	}

	installCmd := "DEBIAN_FRONTEND=noninteractive apt-get install -y -qq --no-install-recommends " + joinPackages(
		packages,
	)

	_, stderr, err := client.Execute(installCmd)
	if err != nil {
		return fmt.Errorf("%w: %s", errDockerPackageInstall, stderr)
	}

	return nil
}

// joinPackages joins package names with spaces for apt-get command.
func joinPackages(packages []string) string {
	result := ""

	for i, pkg := range packages {
		if i > 0 {
			result += " "
		}

		result += pkg
	}

	return result
}
