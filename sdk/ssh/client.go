// Package ssh provides SSH client and connection pool utilities.
package ssh

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/kevinburke/ssh_config"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
)

var (
	errNotConnected        = errors.New("not connected")
	errHostKeyMismatch     = errors.New("host key verification failed: key mismatch (possible MITM attack)")
	errHostNotInKnownHosts = errors.New("host key verification failed: host not found in known_hosts")
	errNoSSHAgent          = errors.New("SSH agent not available: ensure SSH_AUTH_SOCK is set and ssh-agent is running")
	errInvalidPort         = errors.New("invalid port in SSH config")
)

const (
	defaultSSHPort = 22
	dirPermission  = 0o700
	filePermission = 0o600
)

// Connection represents an active SSH connection.
// All methods are safe for use within the context managed by Pool.
type Connection interface {
	Execute(command string) (stdout, stderr string, err error)
	UploadFile(localPath, remotePath string) error
	UploadData(data []byte, remotePath string) error
}

// client represents an SSH client with connection pooling.
// This type is intentionally unexported - use Pool.GetClient() to obtain connections.
type client struct {
	endpoint   string
	hostname   string
	user       string
	port       int
	sshClient  *ssh.Client
	sftpClient *sftp.Client
	agentConn  net.Conn
	mu         sync.Mutex
}

// newClient creates a new SSH client for the given endpoint.
// The endpoint can be an IP address, hostname, or SSH config alias.
// Connection parameters (User, Port, Hostname) are resolved from ~/.ssh/config.
func newClient(endpoint string) *client {
	return &client{
		endpoint: endpoint,
	}
}

// connect establishes an SSH connection to the remote host using SSH agent.
// Connection parameters are resolved from ~/.ssh/config based on the endpoint.
func (c *client) connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.sshClient != nil {
		return nil // already connected
	}

	// Resolve connection parameters from SSH config
	if err := c.resolveConfig(); err != nil {
		return fmt.Errorf("failed to resolve SSH config: %w", err)
	}

	// Get SSH agent auth method
	agentAuth, err := c.getSSHAgentAuth()
	if err != nil {
		return err
	}

	// Load host key callback
	hostKeyCallback, err := c.loadHostKeyCallback()
	if err != nil {
		return fmt.Errorf("failed to load host key callback: %w", err)
	}

	// Create SSH client config
	config := &ssh.ClientConfig{
		User: c.user,
		Auth: []ssh.AuthMethod{
			agentAuth,
		},
		HostKeyCallback: hostKeyCallback,
		// Only accept Ed25519 host keys (most secure, modern standard)
		HostKeyAlgorithms: []string{
			ssh.KeyAlgoED25519,
		},
	}

	// Connect to remote host
	addr := fmt.Sprintf("%s:%d", c.hostname, c.port)

	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %w", addr, err)
	}

	c.sshClient = client

	// Initialize SFTP client for file operations
	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		_ = client.Close()
		c.sshClient = nil

		return fmt.Errorf("failed to initialize SFTP client: %w", err)
	}

	c.sftpClient = sftpClient

	return nil
}

// resolveConfig resolves SSH connection parameters from ~/.ssh/config.
func (c *client) resolveConfig() error {
	// Parse endpoint to extract user@hostname if present
	endpointUser := ""
	endpointHost := c.endpoint

	// Check if endpoint contains user@ prefix
	if user, host, found := strings.Cut(c.endpoint, "@"); found {
		endpointUser = user
		endpointHost = host
	}

	// Get current user as default
	currentUser := os.Getenv("USER")
	if currentUser == "" {
		currentUser = os.Getenv("LOGNAME")
	}

	if currentUser == "" {
		currentUser = "root"
	}

	// Resolve User: endpoint user takes precedence, then SSH config, then current user
	user := endpointUser
	if user == "" {
		user = ssh_config.Get(c.endpoint, "User")
	}

	if user == "" {
		user = currentUser
	}

	c.user = user

	// Resolve Port from SSH config
	portStr := ssh_config.Get(c.endpoint, "Port")
	if portStr == "" {
		c.port = defaultSSHPort
	} else {
		port, err := strconv.Atoi(portStr)
		if err != nil {
			return fmt.Errorf("%w: %s", errInvalidPort, portStr)
		}

		c.port = port
	}

	// Resolve Hostname from SSH config, using parsed endpoint host as fallback
	hostname := ssh_config.Get(c.endpoint, "Hostname")
	if hostname == "" {
		hostname = endpointHost // Use parsed hostname (without user@ prefix)
	}

	c.hostname = hostname

	return nil
}

// close closes the SSH connection.
func (c *client) close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Close SFTP client first
	if c.sftpClient != nil {
		_ = c.sftpClient.Close()
		c.sftpClient = nil
	}

	// Then close SSH connection
	if c.sshClient == nil {
		return nil
	}

	err := c.sshClient.Close()
	c.sshClient = nil

	// Close SSH agent connection to prevent file descriptor leak
	if c.agentConn != nil {
		_ = c.agentConn.Close()
		c.agentConn = nil
	}

	if err != nil {
		return fmt.Errorf("%w: %w", ErrConnectionClose, err)
	}

	return nil
}

// Execute runs a command on the remote host and returns stdout, stderr, and error.
func (c *client) Execute(command string) (stdout, stderr string, err error) {
	if c.sshClient == nil {
		return "", "", errNotConnected
	}

	// Create a new session for this command
	session, err := c.sshClient.NewSession()
	if err != nil {
		return "", "", fmt.Errorf("failed to create session: %w", err)
	}

	defer func() { _ = session.Close() }()

	// Capture stdout and stderr
	stdoutPipe, err := session.StdoutPipe()
	if err != nil {
		return "", "", fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	stderrPipe, err := session.StderrPipe()
	if err != nil {
		return "", "", fmt.Errorf("failed to get stderr pipe: %w", err)
	}

	// Start command
	if err := session.Start(command); err != nil {
		return "", "", fmt.Errorf("failed to start command: %w", err)
	}

	// Read output
	stdoutBytes, _ := io.ReadAll(stdoutPipe)
	stderrBytes, _ := io.ReadAll(stderrPipe)

	// Wait for command to complete
	if err := session.Wait(); err != nil {
		return string(stdoutBytes), string(stderrBytes), fmt.Errorf("command failed: %w", err)
	}

	return string(stdoutBytes), string(stderrBytes), nil
}

// getSSHAgentAuth returns an SSH auth method using the SSH agent.
func (c *client) getSSHAgentAuth() (ssh.AuthMethod, error) {
	// Get SSH_AUTH_SOCK environment variable
	socket := os.Getenv("SSH_AUTH_SOCK")
	if socket == "" {
		return nil, errNoSSHAgent
	}

	// Connect to SSH agent
	conn, err := net.Dial("unix", socket)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to connect to agent socket: %w", errNoSSHAgent, err)
	}

	// Store connection for cleanup in Close()
	c.agentConn = conn

	// Create agent client
	agentClient := agent.NewClient(conn)

	// Return auth method that uses the agent
	return ssh.PublicKeysCallback(agentClient.Signers), nil
}

// loadHostKeyCallback loads the host key callback for SSH host verification.
// It uses the standard ~/.ssh/known_hosts file for verification.
func (c *client) loadHostKeyCallback() (ssh.HostKeyCallback, error) {
	// Get home directory
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	// Standard known_hosts path
	knownHostsPath := filepath.Join(home, ".ssh", "known_hosts")

	// Check if known_hosts exists
	if _, err := os.Stat(knownHostsPath); err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to check known_hosts: %w", err)
		}

		// If known_hosts doesn't exist, create it with proper permissions
		sshDir := filepath.Join(home, ".ssh")
		if err := os.MkdirAll(sshDir, dirPermission); err != nil {
			return nil, fmt.Errorf("failed to create .ssh directory: %w", err)
		}

		// Create empty known_hosts file
		//nolint:gosec
		file, err := os.OpenFile(knownHostsPath, os.O_CREATE|os.O_WRONLY, filePermission)
		if err != nil {
			return nil, fmt.Errorf("failed to create known_hosts: %w", err)
		}

		_ = file.Close()
	}

	// Load known_hosts
	hostKeyCallback, err := knownhosts.New(knownHostsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load known_hosts: %w", err)
	}

	// Wrap the callback to provide better error messages
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		err := hostKeyCallback(hostname, remote, key)
		if err != nil {
			// Check if this is a key mismatch error
			var keyErr *knownhosts.KeyError
			if errors.As(err, &keyErr) && len(keyErr.Want) > 0 {
				return fmt.Errorf(
					"%w for %s. If you trust this host, remove the old key from %s and retry",
					errHostKeyMismatch,
					hostname,
					knownHostsPath,
				)
			}

			// Check if this is an unknown host error
			if errors.As(err, &keyErr) && len(keyErr.Want) == 0 {
				return fmt.Errorf(
					"%w: %s. To add this host, run: ssh-keyscan -H %s >> %s",
					errHostNotInKnownHosts,
					hostname,
					c.hostname,
					knownHostsPath,
				)
			}

			return fmt.Errorf("host key verification failed: %w", err)
		}

		return nil
	}, nil
}

// String returns a string representation of the client.
func (c *client) String() string {
	if c.hostname != "" {
		return fmt.Sprintf("%s@%s:%d", c.user, c.hostname, c.port)
	}

	return c.endpoint
}

// UploadFile uploads a local file to the remote host using SFTP protocol.
func (c *client) UploadFile(localPath, remotePath string) error {
	if c.sshClient == nil {
		return errNotConnected
	}

	// Read local file
	//nolint:gosec // Path is from user config, not user input
	localFile, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("failed to open local file: %w", err)
	}

	defer func() { _ = localFile.Close() }()

	// Create remote file using SFTP
	remoteFile, err := c.sftpClient.Create(remotePath)
	if err != nil {
		return fmt.Errorf("failed to create remote file: %w", err)
	}

	// Copy content from local to remote
	if _, err := io.Copy(remoteFile, localFile); err != nil {
		_ = remoteFile.Close()

		return fmt.Errorf("failed to write file content: %w", err)
	}

	// Close remote file
	if err := remoteFile.Close(); err != nil {
		return fmt.Errorf("failed to close remote file: %w", err)
	}

	// Set file permissions to 0600 (owner read/write only)
	if err := c.sftpClient.Chmod(remotePath, filePermission); err != nil {
		return fmt.Errorf("failed to set file permissions: %w", err)
	}

	return nil
}

// UploadData uploads raw data as a file to the remote host.
// Data is uploaded directly without creating a local temporary file.
func (c *client) UploadData(data []byte, remotePath string) error {
	if c.sshClient == nil {
		return errNotConnected
	}

	// Create remote file using SFTP (truncate if exists)
	remoteFile, err := c.sftpClient.OpenFile(remotePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC)
	if err != nil {
		return fmt.Errorf("failed to create remote file: %w", err)
	}

	// Write data
	if _, err := remoteFile.Write(data); err != nil {
		_ = remoteFile.Close()

		return fmt.Errorf("failed to write file content: %w", err)
	}

	// Close remote file
	if err := remoteFile.Close(); err != nil {
		return fmt.Errorf("failed to close remote file: %w", err)
	}

	// Set file permissions to 0600 (owner read/write only)
	if err := c.sftpClient.Chmod(remotePath, filePermission); err != nil {
		return fmt.Errorf("failed to set file permissions: %w", err)
	}

	return nil
}
