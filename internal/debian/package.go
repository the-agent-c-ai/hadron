// Package debian provides Debian package management operations.
package debian

import (
	"errors"
	"fmt"
	"strings"

	"github.com/the-agent-c-ai/hadron/sdk/ssh"
)

var (
	errPackageInstallFailed = errors.New("package installation failed")
	errPackageRemoveFailed  = errors.New("package removal failed")
)

// CustomInstaller is a function that performs custom package installation.
type CustomInstaller func(client ssh.Connection) error

// getCustomInstaller returns a custom installer for the given package name, if one exists.
func getCustomInstaller(packageName string) (CustomInstaller, bool) {
	// Map of package names to custom installation functions
	customInstallers := map[string]CustomInstaller{
		"docker-ce": InstallDocker,
	}

	installer, exists := customInstallers[packageName]

	return installer, exists
}

// IsInstalled checks if a Debian package is installed on the system.
func IsInstalled(client ssh.Connection, packageName string) (bool, error) {
	// Check if package is installed using dpkg
	// Output format: "ii  package-name  version  architecture  description"
	cmd := fmt.Sprintf("dpkg -l %s 2>/dev/null | grep '^ii' | grep -q '%s'", packageName, packageName)

	_, _, err := client.Execute(cmd)
	if err != nil {
		// grep returns non-zero if no match found (package not installed)
		// This is expected behavior, not an error condition
		return false, nil //nolint:nilerr // exit code used for logic, not error indication
	}

	return true, nil
}

// Install installs a Debian package using apt-get.
// If a custom installer exists for the package, it will be used instead.
func Install(client ssh.Connection, packageName string) error {
	// Check if custom installer exists
	if installer, exists := getCustomInstaller(packageName); exists {
		return installer(client)
	}

	// Default apt-get installation
	return installWithApt(client, packageName)
}

// installWithApt installs a package using apt-get with standard flags.
func installWithApt(client ssh.Connection, packageName string) error {
	// Update package lists
	updateCmd := "apt-get update -qq"

	_, stderr, err := client.Execute(updateCmd)
	if err != nil {
		return fmt.Errorf("%w: apt-get update failed: %s", errPackageInstallFailed, stderr)
	}

	// Install package with:
	// -y: assume yes to all prompts
	// -qq: very quiet output
	// --no-install-recommends: only install dependencies, not recommended packages
	installCmd := "DEBIAN_FRONTEND=noninteractive apt-get install -y -qq --no-install-recommends " + packageName

	_, stderr, err = client.Execute(installCmd)
	if err != nil {
		return fmt.Errorf("%w: %s: %s", errPackageInstallFailed, packageName, stderr)
	}

	return nil
}

// Remove removes a Debian package using apt-get.
func Remove(client ssh.Connection, packageName string) error {
	// Remove package with:
	// -y: assume yes to all prompts
	// -qq: very quiet output
	removeCmd := "DEBIAN_FRONTEND=noninteractive apt-get remove -y -qq " + packageName

	_, stderr, err := client.Execute(removeCmd)
	if err != nil {
		return fmt.Errorf("%w: %s: %s", errPackageRemoveFailed, packageName, stderr)
	}

	// Clean up unused dependencies
	autoremoveCmd := "apt-get autoremove -y -qq"

	_, stderr, err = client.Execute(autoremoveCmd)
	if err != nil {
		return fmt.Errorf("%w: autoremove failed: %s", errPackageRemoveFailed, stderr)
	}

	return nil
}

// EnsureInstalled ensures a package is installed, installing it if necessary.
func EnsureInstalled(client ssh.Connection, packageName string) error {
	installed, err := IsInstalled(client, packageName)
	if err != nil {
		return fmt.Errorf("failed to check if %s is installed: %w", packageName, err)
	}

	if installed {
		return nil // Already installed
	}

	return Install(client, packageName)
}

// EnsureRemoved ensures a package is not installed, removing it if necessary.
func EnsureRemoved(client ssh.Connection, packageName string) error {
	installed, err := IsInstalled(client, packageName)
	if err != nil {
		return fmt.Errorf("failed to check if %s is installed: %w", packageName, err)
	}

	if !installed {
		return nil // Already removed
	}

	return Remove(client, packageName)
}

// GetInstalledVersion returns the installed version of a package, or empty string if not installed.
func GetInstalledVersion(client ssh.Connection, packageName string) (string, error) {
	cmd := fmt.Sprintf("dpkg -s %s 2>/dev/null | grep '^Version:' | awk '{print $2}'", packageName)

	stdout, _, err := client.Execute(cmd)
	if err != nil {
		return "", nil //nolint:nilerr // exit code used for logic, not error indication
	}

	return strings.TrimSpace(stdout), nil
}
