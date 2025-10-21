package debian

import (
	"fmt"
	"strings"

	"github.com/the-agent-c-ai/hadron/sdk/ssh"
)

const (
	unattendedUpgradesPackage = "unattended-upgrades"
	autoUpgradesConfigPath    = "/etc/apt/apt.conf.d/20auto-upgrades"
)

// IsUnattendedUpgradesInstalled checks if unattended-upgrades package is installed.
func IsUnattendedUpgradesInstalled(client ssh.Connection) (bool, error) {
	return IsInstalled(client, unattendedUpgradesPackage)
}

// IsAutoUpdatesConfigured checks if automatic updates are properly configured.
// Verifies that /etc/apt/apt.conf.d/20auto-upgrades exists and contains proper settings.
func IsAutoUpdatesConfigured(client ssh.Connection) (bool, error) {
	// Check if config file exists
	cmd := fmt.Sprintf("test -f %s && echo exists || echo missing", autoUpgradesConfigPath)

	stdout, _, err := client.Execute(cmd)
	if err != nil {
		return false, fmt.Errorf("failed to check auto-updates config: %w", err)
	}

	if strings.TrimSpace(stdout) != "exists" {
		return false, nil
	}

	// Check if config file has proper content
	// Should contain: APT::Periodic::Update-Package-Lists "1";
	// Should contain: APT::Periodic::Unattended-Upgrade "1";
	cmd = fmt.Sprintf("grep -q 'APT::Periodic::Update-Package-Lists \"1\"' %s && "+
		"grep -q 'APT::Periodic::Unattended-Upgrade \"1\"' %s && "+
		"echo configured || echo not-configured", autoUpgradesConfigPath, autoUpgradesConfigPath)

	stdout, _, err = client.Execute(cmd)
	if err != nil {
		return false, fmt.Errorf("failed to check auto-updates config content: %w", err)
	}

	return strings.TrimSpace(stdout) == "configured", nil
}

// InstallUnattendedUpgrades installs the unattended-upgrades package.
func InstallUnattendedUpgrades(client ssh.Connection) error {
	return EnsureInstalled(client, unattendedUpgradesPackage)
}

// ConfigureAutoUpdates enables automatic security updates via dpkg-reconfigure.
func ConfigureAutoUpdates(client ssh.Connection) error {
	// Use -plow for non-interactive configuration (low priority = enable auto-updates)
	cmd := "sudo DEBIAN_FRONTEND=noninteractive dpkg-reconfigure -plow unattended-upgrades"

	_, stderr, err := client.Execute(cmd)
	if err != nil {
		return fmt.Errorf("failed to configure automatic updates: %w (stderr: %s)", err, stderr)
	}

	return nil
}
