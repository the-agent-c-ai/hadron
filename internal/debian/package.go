// Package debian provides Debian package management operations.
package debian

import (
	"fmt"

	"github.com/the-agent-c-ai/hadron/sdk/ssh"
)

// customInstaller is a function that performs custom package installation.
type customInstaller func(client ssh.Connection) error

// getCustomInstaller returns a custom installer for the given package name, if one exists.
func getCustomInstaller(packageName string) (customInstaller, bool) {
	// Map of package names to custom installation functions
	customInstallers := map[string]customInstaller{
		"docker-ce": installDocker,
	}

	installer, exists := customInstallers[packageName]

	return installer, exists
}

// isInstalled checks if a Debian package is installed on the system.
func isInstalled(client ssh.Connection, packageName string) bool {
	// Check if package is installed using dpkg
	// Output format: "ii  package-name  version  architecture  description"
	cmd := fmt.Sprintf("sudo dpkg -l %s 2>/dev/null | grep '^ii' | grep -q '%s'", packageName, packageName)

	_, _, err := client.Execute(cmd)
	if err != nil {
		// grep returns non-zero if no match found (package not installed)
		// This is expected behavior, not an error condition
		return false
	}

	return true
}

// install installs a Debian package using apt-get.
// If a custom installer exists for the package, it will be used instead.
func install(client ssh.Connection, packageName string) error {
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
	updateCmd := "sudo apt-get update -qq"

	_, stderr, err := client.Execute(updateCmd)
	if err != nil {
		return fmt.Errorf("%w: apt-get update failed: %s", ErrPackageInstallFailed, stderr)
	}

	// Install package with:
	// -y: assume yes to all prompts
	// -qq: very quiet output
	// --no-install-recommends: only install dependencies, not recommended packages
	installCmd := "sudo DEBIAN_FRONTEND=noninteractive apt-get install -qq --no-install-recommends " + packageName

	_, stderr, err = client.Execute(installCmd)
	if err != nil {
		return fmt.Errorf("%w: %s: %s", ErrPackageInstallFailed, packageName, stderr)
	}

	return nil
}

// remove removes a Debian package using apt-get.
func remove(client ssh.Connection, packageName string) error {
	// Remove package with:
	// -y: assume yes to all prompts
	// -qq: very quiet output
	removeCmd := "sudo DEBIAN_FRONTEND=noninteractive apt-get remove -qq " + packageName

	_, stderr, err := client.Execute(removeCmd)
	if err != nil {
		return fmt.Errorf("%w: %s: %s", ErrPackageRemoveFailed, packageName, stderr)
	}

	// Clean up unused dependencies
	autoremoveCmd := "sudo apt-get autoremove -qq"

	_, stderr, err = client.Execute(autoremoveCmd)
	if err != nil {
		return fmt.Errorf("%w: autoremove failed: %s", ErrPackageRemoveFailed, stderr)
	}

	return nil
}

// EnsureInstalled ensures a package is installed, installing it if necessary.
func EnsureInstalled(client ssh.Connection, packageName string) error {
	if isInstalled(client, packageName) {
		return nil // Already installed
	}

	return install(client, packageName)
}

// EnsureRemoved ensures a package is not installed, removing it if necessary.
func EnsureRemoved(client ssh.Connection, packageName string) error {
	if !isInstalled(client, packageName) {
		return nil // Already removed
	}

	return remove(client, packageName)
}
