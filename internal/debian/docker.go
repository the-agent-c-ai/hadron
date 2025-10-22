package debian

import (
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

// installDocker installs Docker CE following the official Debian installation procedure.
// See: https://docs.docker.com/engine/install/debian/
func installDocker(client ssh.Connection) error {
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
	cmd := "sudo DEBIAN_FRONTEND=noninteractive apt-get update -qq && " +
		"sudo apt-get install -qq --no-install-recommends ca-certificates curl"

	_, stderr, err := client.Execute(cmd)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrDockerPrereqFailed, stderr)
	}

	return nil
}

// addDockerGPGKey downloads and installs Docker's GPG key.
func addDockerGPGKey(client ssh.Connection) error {
	// Create keyrings directory with proper permissions
	createDirCmd := "sudo install -m 0755 -d " + dockerKeyrings

	_, stderr, err := client.Execute(createDirCmd)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrDockerKeyringsDir, stderr)
	}

	// Download Docker GPG key to temp location, then move with sudo
	downloadKeyCmd := fmt.Sprintf(
		"curl -fsSL %s -o /tmp/docker.asc && sudo mv /tmp/docker.asc %s",
		dockerGPGURL,
		dockerGPGPath,
	)

	_, stderr, err = client.Execute(downloadKeyCmd)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrDockerGPGDownload, stderr)
	}

	// Set proper permissions on GPG key
	chmodCmd := "sudo chmod a+r " + dockerGPGPath

	_, stderr, err = client.Execute(chmodCmd)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrDockerGPGPermissions, stderr)
	}

	return nil
}

// setupDockerRepository adds Docker's apt repository to sources.list.d.
func setupDockerRepository(client ssh.Connection) error {
	// Get architecture
	archCmd := "sudo dpkg --print-architecture"

	arch, _, err := client.Execute(archCmd)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrDockerArchFetch, err)
	}

	arch = strings.TrimSpace(arch)

	// Get Debian version codename (parse file safely without executing it)
	codenameCmd := "grep '^VERSION_CODENAME=' /etc/os-release | cut -d= -f2 | tr -d '\"'"

	codename, _, err := client.Execute(codenameCmd)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrDockerCodenameFetch, err)
	}

	codename = strings.TrimSpace(codename)

	// Create repository entry
	repoLine := fmt.Sprintf(
		"deb [arch=%s signed-by=%s] https://download.docker.com/linux/debian %s stable",
		arch, dockerGPGPath, codename,
	)

	// Write repository file
	writeRepoCmd := fmt.Sprintf("echo '%s' | sudo tee %s > /dev/null", repoLine, dockerRepoFile)

	_, stderr, err := client.Execute(writeRepoCmd)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrDockerRepoWrite, stderr)
	}

	// Update apt cache
	updateCmd := "sudo apt-get update -qq"

	_, stderr, err = client.Execute(updateCmd)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrDockerRepoUpdate, stderr)
	}

	return nil
}

// installDockerPackages installs Docker CE and related packages.
func installDockerPackages(client ssh.Connection) error {
	packages := []string{
		"docker-ce",
		"docker-ce-cli",
		"containerd.io",
	}

	installCmd := "sudo DEBIAN_FRONTEND=noninteractive apt-get install -qq --no-install-recommends " + strings.Join(
		packages,
		" ",
	)

	_, stderr, err := client.Execute(installCmd)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrDockerPackageInstall, stderr)
	}

	return nil
}
