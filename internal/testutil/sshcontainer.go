// Package testutil provides reusable test infrastructure for integration tests.
package testutil

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"github.com/the-agent-c-ai/hadron/sdk/ssh"
)

const (
	sshWaitRetries   = 30
	sshRetryDelaySec = 1
)

// getTestKeyPath returns the absolute path to the test SSH key.
func getTestKeyPath() string {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		panic("failed to get caller information")
	}

	dir := filepath.Dir(filename)

	return filepath.Join(dir, "ssh", "test_key")
}

// SSHContainer represents a test container with SSH access.
type SSHContainer struct {
	ContainerID string
	Endpoint    string
	client      ssh.Connection
}

// Client returns the SSH connection to the container.
func (c *SSHContainer) Client() ssh.Connection {
	return c.client
}

// StartDebianSSHContainer starts an ephemeral Debian container with SSH enabled.
// Returns a configured SSHContainer ready for testing.
func StartDebianSSHContainer(t *testing.T) *SSHContainer {
	t.Helper()

	// Container configuration
	containerName := fmt.Sprintf("hadron-test-debian-%d", time.Now().Unix())

	// Read test SSH public key
	testKeyPath := getTestKeyPath()
	pubKeyPath := testKeyPath + ".pub"

	pubKeyBytes, err := os.ReadFile(pubKeyPath) //nolint:gosec // Test key path from source file location
	if err != nil {
		t.Fatalf("failed to read test SSH public key: %v", err)
	}

	pubKey := string(pubKeyBytes)

	// Start Debian container with SSH server
	// Using debian image, install openssh-server and sudo, inject public key
	startCmd := exec.Command("docker", "run", "-d", "--rm", //nolint:gosec // Test container with test-generated key
		"--name", containerName,
		"debian:bookworm-slim",
		"sh", "-c",
		"apt-get update -qq && "+
			"apt-get install -y -qq openssh-server sudo && "+
			"mkdir -p /run/sshd /root/.ssh && "+
			"chmod 700 /root/.ssh && "+
			"echo '"+pubKey+"' > /root/.ssh/authorized_keys && "+
			"chmod 600 /root/.ssh/authorized_keys && "+
			"/usr/sbin/sshd -D",
	)

	output, err := startCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to start container: %v\noutput: %s", err, output)
	}

	// Setup cleanup
	t.Cleanup(func() {
		stopCmd := exec.Command("docker", "stop", containerName) //nolint:gosec // containerName is test-generated
		_ = stopCmd.Run()                                        // Best effort cleanup
	})

	// Get container IP address
	ipCmd := exec.Command( //nolint:gosec // containerName is test-generated
		"docker", "inspect", "-f",
		"{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}",
		containerName,
	)

	ipOutput, err := ipCmd.Output()
	if err != nil {
		t.Fatalf("failed to get container IP: %v", err)
	}

	containerIP := string(ipOutput)
	containerIP = containerIP[:len(containerIP)-1] // Remove trailing newline

	// Wait for SSH port to be open before scanning keys
	sshReady := false

	for range sshWaitRetries {
		ncCmd := exec.Command("nc", "-z", containerIP, "22") //nolint:gosec // containerIP is from Docker inspect
		if ncCmd.Run() == nil {
			sshReady = true

			break
		}

		time.Sleep(sshRetryDelaySec * time.Second)
	}

	if !sshReady {
		t.Fatalf("SSH port never became ready on %s:22", containerIP)
	}

	// Ensure ~/.ssh directory exists and scan host keys
	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("failed to get home directory: %v", err)
	}

	sshDir := filepath.Join(homeDir, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil { //nolint:revive // Standard SSH directory permissions
		t.Fatalf("failed to create .ssh directory: %v", err)
	}

	knownHostsPath := filepath.Join(sshDir, "known_hosts")

	// Scan and add host keys to known_hosts
	keyscanCmd := exec.Command("ssh-keyscan", "-H", containerIP) //nolint:gosec // containerIP from Docker inspect

	keyscanOutput, err := keyscanCmd.Output()
	if err != nil {
		t.Fatalf("failed to scan host key: %v", err)
	}

	// Append to known_hosts
	//nolint:gosec,revive // Path from os.UserHomeDir, standard SSH file permissions
	knownHostsFile, err := os.OpenFile(knownHostsPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatalf("failed to open known_hosts: %v", err)
	}
	defer knownHostsFile.Close()

	if _, err := knownHostsFile.Write(keyscanOutput); err != nil {
		t.Fatalf("failed to write to known_hosts: %v", err)
	}

	// Cleanup: remove this host from known_hosts when test ends
	t.Cleanup(func() {
		_ = exec.Command("ssh-keygen", "-R", containerIP).Run() //nolint:gosec // containerIP from Docker
	})

	// Add test key to SSH agent for this session
	// First, ensure the private key has correct permissions (SSH requires 0600)
	if err := os.Chmod(testKeyPath, 0o600); err != nil { //nolint:revive // Standard SSH key permissions
		t.Fatalf("failed to set test key permissions: %v", err)
	}

	addKeyCmd := exec.Command("ssh-add", testKeyPath) //nolint:gosec // Test key path from source file location

	addKeyOutput, err := addKeyCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to add test key to agent: %v\noutput: %s", err, addKeyOutput)
	}

	// Wait for SSH to be ready with retries
	pool := ssh.NewPool(zerolog.Nop())
	endpoint := "root@" + containerIP

	var client ssh.Connection
	for range sshWaitRetries {
		client, err = pool.GetClient(endpoint)
		if err == nil {
			// Connection successful, verify SSH works
			_, _, testErr := client.Execute("echo test")
			if testErr == nil {
				break
			}
		}

		time.Sleep(sshRetryDelaySec * time.Second)
	}

	if client == nil || err != nil {
		t.Fatalf("failed to connect to container after %d retries: %v", sshWaitRetries, err)
	}

	return &SSHContainer{
		ContainerID: containerName,
		Endpoint:    endpoint,
		client:      client,
	}
}
