package debian

import (
	"errors"
	"fmt"
	"strings"

	"github.com/the-agent-c-ai/hadron/sdk/ssh"
)

const (
	nodeExporterPackage    = "prometheus-node-exporter"
	nodeExporterConfigFile = "/etc/default/prometheus-node-exporter"
	nodeExporterService    = "prometheus-node-exporter.service"
	// docker0 bridge default IP address.
	docker0IP = "172.17.0.1"
	// Default node_exporter port.
	nodeExporterPort = "9100"
)

var (
	errDockerZeroIP      = errors.New("failed to get docker0 IP")
	errNoIPv4            = errors.New("docker0 interface has no IPv4 address")
	errMoveConfig        = errors.New("failed to move config")
	errConfigPermissions = errors.New("failed to set config permissions")
	errAddFirewallRule   = errors.New("failed to add UFW rule")
	errEnableService     = errors.New("failed to enable service")
	errRestartService    = errors.New("failed to restart service")
)

// installNodeExporter installs prometheus-node-exporter and configures it to listen on docker0.
// This allows containers to scrape host metrics without exposing node_exporter publicly.
func installNodeExporter(client ssh.Connection) error {
	// Step 1: Install the package via apt
	if err := installWithApt(client, nodeExporterPackage); err != nil {
		return fmt.Errorf("failed to install %s: %w", nodeExporterPackage, err)
	}

	// Step 2: Configure node_exporter to listen on docker0 interface
	if err := configureNodeExporter(client); err != nil {
		return fmt.Errorf("failed to configure node_exporter: %w", err)
	}

	// Step 3: Configure firewall to allow Docker containers to reach node_exporter
	if err := configureNodeExporterFirewall(client); err != nil {
		return fmt.Errorf("failed to configure node_exporter firewall: %w", err)
	}

	// Step 4: Enable and start the service
	if err := enableNodeExporter(client); err != nil {
		return fmt.Errorf("failed to enable node_exporter service: %w", err)
	}

	return nil
}

// configureNodeExporter creates the configuration file to make node_exporter listen on docker0.
func configureNodeExporter(client ssh.Connection) error {
	// Get docker0 IP address dynamically
	getIPCmd := "ip -4 addr show docker0 | grep -oP '(?<=inet\\s)\\d+(\\.\\d+){3}'"

	docker0IPAddr, stderr, err := client.Execute(getIPCmd)
	if err != nil {
		return fmt.Errorf("%w: %s", errDockerZeroIP, stderr)
	}

	docker0IPAddr = strings.TrimSpace(docker0IPAddr)
	if docker0IPAddr == "" {
		return errNoIPv4
	}

	// Configuration: listen on docker0 interface only
	// This prevents exposing metrics to the public internet while allowing container access
	listenAddr := fmt.Sprintf("%s:%s", docker0IPAddr, nodeExporterPort)
	config := fmt.Sprintf("ARGS=\"--web.listen-address=%s\"\n", listenAddr)

	// Upload config to temp location
	tempPath := "/tmp/hadron-node-exporter-config"
	if err := client.UploadData([]byte(config), tempPath); err != nil {
		return fmt.Errorf("failed to upload config: %w", err)
	}

	// Move to final location with sudo
	moveCmd := fmt.Sprintf("sudo mv %s %s", tempPath, nodeExporterConfigFile)

	_, moveStderr, err := client.Execute(moveCmd)
	if err != nil {
		return fmt.Errorf("%w: %s", errMoveConfig, moveStderr)
	}

	// Set proper permissions (readable by prometheus-node-exporter user)
	chmodCmd := "sudo chmod 644 " + nodeExporterConfigFile

	_, chmodStderr, err := client.Execute(chmodCmd)
	if err != nil {
		return fmt.Errorf("%w: %s", errConfigPermissions, chmodStderr)
	}

	return nil
}

// configureNodeExporterFirewall configures UFW to allow Docker containers to reach node_exporter.
// Uses Docker's default IPAM pool (172.16.0.0/12) to support dynamically allocated network subnets.
// This function is idempotent - safe to run multiple times.
func configureNodeExporterFirewall(client ssh.Connection) error {
	// Check if the rule already exists
	checkCmd := "sudo ufw status | grep -q '172.16.0.0/12.*172.17.0.1.*9100'"
	_, _, err := client.Execute(checkCmd)

	if err == nil {
		// Rule already exists (grep found it, exit code 0)
		return nil
	}

	// Allow all Docker networks (172.16.0.0/12 covers 172.16.0.0 - 172.31.255.255)
	// to reach docker0 gateway IP on port 9100
	// This supports dynamic network allocation while restricting destination to docker0 only
	addRuleCmd := fmt.Sprintf(
		"sudo ufw allow from 172.16.0.0/12 to %s port %s comment 'node_exporter from Docker networks'",
		docker0IP,
		nodeExporterPort,
	)

	_, stderr, err := client.Execute(addRuleCmd)
	if err != nil {
		return fmt.Errorf("%w: %s", errAddFirewallRule, stderr)
	}

	return nil
}

// enableNodeExporter enables and starts the systemd service.
func enableNodeExporter(client ssh.Connection) error {
	// Enable service to start on boot
	enableCmd := "sudo systemctl enable " + nodeExporterService

	_, stderr, err := client.Execute(enableCmd)
	if err != nil {
		return fmt.Errorf("%w: %s", errEnableService, stderr)
	}

	// Restart service to apply new configuration
	restartCmd := "sudo systemctl restart " + nodeExporterService

	_, stderr, err = client.Execute(restartCmd)
	if err != nil {
		return fmt.Errorf("%w: %s", errRestartService, stderr)
	}

	return nil
}
