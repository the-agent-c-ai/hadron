// Package testutil provides reusable test infrastructure for integration tests.
package testutil

import (
	"fmt"
	"os/exec"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"github.com/the-agent-c-ai/hadron/sdk/ssh"
)

const (
	sshWaitRetries   = 30
	sshRetryDelaySec = 1
	testKeyBasePath  = "/Users/dmp/Projects/go/the-agent-c-ai/hadron/internal/testutil/.ssh/test_key"
)

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
	pubKeyPath := testKeyBasePath + ".pub"

	pubKeyBytes, err := exec.Command("cat", pubKeyPath).Output()
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

	// Scan and add host keys to user's known_hosts (will cleanup later)
	keyscanCmd := exec.Command( //nolint:gosec // containerIP from Docker inspect
		"sh",
		"-c",
		fmt.Sprintf("ssh-keyscan -H %s >> ~/.ssh/known_hosts 2>/dev/null", containerIP),
	)
	if err := keyscanCmd.Run(); err != nil {
		t.Logf("warning: failed to add host key: %v", err)
	}

	// Cleanup: remove this host from known_hosts when test ends
	t.Cleanup(func() {
		_ = exec.Command("ssh-keygen", "-R", containerIP).Run() //nolint:gosec // containerIP from Docker
	})

	// Add test key to SSH agent for this session
	addKeyCmd := exec.Command("ssh-add", testKeyBasePath)
	if err := addKeyCmd.Run(); err != nil {
		t.Logf("warning: failed to add test key to agent (may already be added): %v", err)
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
