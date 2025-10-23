package docker

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/the-agent-c-ai/hadron/sdk/ssh"
)

const (
	daemonConfigPath = "/etc/docker/daemon.json"
	nofileLimit      = 64000
)

// DaemonConfig represents the Docker daemon configuration.
//
//nolint:tagliatelle
type DaemonConfig struct {
	LiveRestore     bool                    `json:"live-restore"`
	UserlandProxy   bool                    `json:"userland-proxy"`
	NoNewPrivileges bool                    `json:"no-new-privileges"`
	ICC             bool                    `json:"icc"`
	LogDriver       string                  `json:"log-driver"`
	LogOpts         map[string]string       `json:"log-opts"`
	DefaultUlimits  map[string]UlimitConfig `json:"default-ulimits"`
}

// UlimitConfig represents a ulimit configuration.
//
//nolint:tagliatelle
type UlimitConfig struct {
	Name string `json:"Name"`
	Hard int    `json:"Hard"`
	Soft int    `json:"Soft"`
}

// GetSecureDefaults returns the recommended secure Docker daemon configuration.
func GetSecureDefaults() *DaemonConfig {
	return &DaemonConfig{
		LiveRestore:     true,
		UserlandProxy:   false,
		NoNewPrivileges: true,
		ICC:             false,
		LogDriver:       "json-file",
		LogOpts: map[string]string{
			"max-size": "10m",
			"max-file": "3",
		},
		DefaultUlimits: map[string]UlimitConfig{
			"nofile": {
				Name: "nofile",
				Hard: nofileLimit,
				Soft: nofileLimit,
			},
		},
	}
}

// DaemonConfigExists checks if /etc/docker/daemon.json exists.
func DaemonConfigExists(client ssh.Connection) (bool, error) {
	cmd := fmt.Sprintf("test -f %s && echo exists || echo missing", daemonConfigPath)

	stdout, _, err := client.Execute(cmd)
	if err != nil {
		return false, fmt.Errorf("failed to check daemon config existence: %w", err)
	}

	return strings.TrimSpace(stdout) == "exists", nil
}

// GetDaemonConfig reads the current daemon configuration.
func GetDaemonConfig(client ssh.Connection) (*DaemonConfig, error) {
	cmd := "cat " + daemonConfigPath

	stdout, _, err := client.Execute(cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to read daemon config: %w", err)
	}

	var config DaemonConfig
	if err := json.Unmarshal([]byte(stdout), &config); err != nil {
		return nil, fmt.Errorf("failed to parse daemon config: %w", err)
	}

	return &config, nil
}

// WriteDaemonConfig writes the daemon configuration to /etc/docker/daemon.json.
func WriteDaemonConfig(client ssh.Connection, config *DaemonConfig) error {
	// Marshal to JSON with indentation
	jsonBytes, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal daemon config: %w", err)
	}

	// Ensure /etc/docker directory exists
	mkdirCmd := "sudo mkdir -p /etc/docker"
	if _, _, err := client.Execute(mkdirCmd); err != nil {
		return fmt.Errorf("failed to create /etc/docker directory: %w", err)
	}

	// Write config via temp file (avoids shell escaping issues)
	tempPath := "/tmp/hadron-daemon.json"
	if err := client.UploadData(jsonBytes, tempPath); err != nil {
		return fmt.Errorf("failed to write temp daemon config: %w", err)
	}

	moveCmd := fmt.Sprintf("sudo mv %s %s", tempPath, daemonConfigPath)
	if _, stderr, err := client.Execute(moveCmd); err != nil {
		return fmt.Errorf("failed to move daemon config: %w (stderr: %s)", err, stderr)
	}

	return nil
}

// ConfigsEqual checks if two daemon configs are equivalent.
func ConfigsEqual(a, b *DaemonConfig) bool {
	return reflect.DeepEqual(a, b)
}

// RestartDockerDaemon restarts the Docker daemon.
func RestartDockerDaemon(client ssh.Connection) error {
	cmd := "sudo systemctl restart docker"

	_, stderr, err := client.Execute(cmd)
	if err != nil {
		return fmt.Errorf("failed to restart docker daemon: %w (stderr: %s)", err, stderr)
	}

	return nil
}

var errDockerNotReady = errors.New("docker daemon did not become ready")

// WaitForDockerReady waits for Docker daemon to be ready after restart.
func WaitForDockerReady(client ssh.Connection, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	checkInterval := 1 * time.Second

	for {
		// Try docker info command
		_, _, err := client.Execute("docker info")
		if err == nil {
			return nil
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("%w within %v", errDockerNotReady, timeout)
		}

		time.Sleep(checkInterval)
	}
}
