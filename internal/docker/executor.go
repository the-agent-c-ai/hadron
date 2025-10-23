// Package docker is internal
package docker

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/rs/zerolog"

	"github.com/the-agent-c-ai/hadron/sdk/hash"
	"github.com/the-agent-c-ai/hadron/sdk/ssh"
)

const (
	checkResultExists  = "exists"
	checkResultMissing = "missing"
	labelFlagFormat    = " --label %s=%s"
)

// Executor executes Docker commands on remote hosts via SSH.
type Executor struct {
	sshPool *ssh.Pool
	logger  zerolog.Logger
}

// NewExecutor creates a new Docker command executor.
func NewExecutor(sshPool *ssh.Pool, logger zerolog.Logger) *Executor {
	return &Executor{
		sshPool: sshPool,
		logger:  logger,
	}
}

// NetworkExists checks if a Docker network exists on the remote host.
func (*Executor) NetworkExists(client ssh.Connection, networkName string) (bool, error) {
	cmd := fmt.Sprintf(
		"docker network inspect %s >/dev/null 2>&1 && echo %s || echo %s",
		networkName,
		checkResultExists,
		checkResultMissing,
	)

	stdout, _, err := client.Execute(cmd)
	if err != nil {
		return false, fmt.Errorf("failed to check network existence: %w", err)
	}

	return strings.TrimSpace(stdout) == checkResultExists, nil
}

// CreateNetwork creates a Docker network on the remote host.
func (e *Executor) CreateNetwork(client ssh.Connection, networkName, driver string, labels map[string]string) error {
	cmd := "docker network create -d " + driver

	// Add labels
	for k, v := range labels {
		cmd += fmt.Sprintf(labelFlagFormat, k, v)
	}

	cmd += " " + networkName

	e.logger.Debug().Str("command", cmd).Msg("Creating network")

	stdout, stderr, err := client.Execute(cmd)
	if err != nil {
		return fmt.Errorf("failed to create network: %w (stderr: %s)", err, stderr)
	}

	e.logger.Info().Str("network", networkName).Str("output", strings.TrimSpace(stdout)).Msg("Network created")

	return nil
}

// RemoveNetwork removes a Docker network from the remote host.
func (e *Executor) RemoveNetwork(client ssh.Connection, networkName string) error {
	cmd := "docker network rm " + networkName

	e.logger.Debug().Str("command", cmd).Msg("Removing network")

	_, stderr, err := client.Execute(cmd)
	if err != nil {
		return fmt.Errorf("failed to remove network: %w (stderr: %s)", err, stderr)
	}

	e.logger.Info().Str("network", networkName).Msg("Network removed")

	return nil
}

// GetNetworkLabel retrieves a label value from a network.
func (*Executor) GetNetworkLabel(client ssh.Connection, networkName, labelKey string) (string, error) {
	cmd := fmt.Sprintf("docker network inspect -f '{{index .Labels \"%s\"}}' %s", labelKey, networkName)

	stdout, stderr, err := client.Execute(cmd)
	if err != nil {
		return "", fmt.Errorf("failed to get network label: %w (stderr: %s)", err, stderr)
	}

	return strings.TrimSpace(stdout), nil
}

// VolumeExists checks if a Docker volume exists on the remote host.
func (*Executor) VolumeExists(client ssh.Connection, volumeName string) (bool, error) {
	cmd := fmt.Sprintf(
		"docker volume inspect %s >/dev/null 2>&1 && echo %s || echo %s",
		volumeName,
		checkResultExists,
		checkResultMissing,
	)

	stdout, _, err := client.Execute(cmd)
	if err != nil {
		return false, fmt.Errorf("failed to check volume existence: %w", err)
	}

	return strings.TrimSpace(stdout) == checkResultExists, nil
}

// CreateVolume creates a Docker volume on the remote host.
func (e *Executor) CreateVolume(client ssh.Connection, volumeName, driver string, labels map[string]string) error {
	cmd := "docker volume create --driver " + driver

	// Add labels
	for k, v := range labels {
		cmd += fmt.Sprintf(labelFlagFormat, k, v)
	}

	cmd += " " + volumeName

	e.logger.Debug().Str("command", cmd).Msg("Creating volume")

	stdout, stderr, err := client.Execute(cmd)
	if err != nil {
		return fmt.Errorf("failed to create volume: %w (stderr: %s)", err, stderr)
	}

	e.logger.Info().Str("volume", volumeName).Str("output", strings.TrimSpace(stdout)).Msg("Volume created")

	return nil
}

// RemoveVolume removes a Docker volume from the remote host.
func (e *Executor) RemoveVolume(client ssh.Connection, volumeName string) error {
	cmd := "docker volume rm " + volumeName

	e.logger.Debug().Str("command", cmd).Msg("Removing volume")

	_, stderr, err := client.Execute(cmd)
	if err != nil {
		return fmt.Errorf("failed to remove volume: %w (stderr: %s)", err, stderr)
	}

	e.logger.Info().Str("volume", volumeName).Msg("Volume removed")

	return nil
}

// GetVolumeLabel retrieves a label value from a volume.
func (*Executor) GetVolumeLabel(client ssh.Connection, volumeName, labelKey string) (string, error) {
	cmd := fmt.Sprintf("docker volume inspect -f '{{index .Labels \"%s\"}}' %s", labelKey, volumeName)

	stdout, stderr, err := client.Execute(cmd)
	if err != nil {
		return "", fmt.Errorf("failed to get volume label: %w (stderr: %s)", err, stderr)
	}

	return strings.TrimSpace(stdout), nil
}

// ContainerExists checks if a Docker container exists on the remote host.
func (*Executor) ContainerExists(client ssh.Connection, containerName string) (bool, error) {
	cmd := fmt.Sprintf(
		"docker container inspect %s >/dev/null 2>&1 && echo %s || echo %s",
		containerName,
		checkResultExists,
		checkResultMissing,
	)

	stdout, _, err := client.Execute(cmd)
	if err != nil {
		return false, fmt.Errorf("failed to check container existence: %w", err)
	}

	return strings.TrimSpace(stdout) == checkResultExists, nil
}

// GetContainerLabel retrieves a label value from a container.
func (*Executor) GetContainerLabel(client ssh.Connection, containerName, labelKey string) (string, error) {
	cmd := fmt.Sprintf("docker container inspect -f '{{index .Config.Labels \"%s\"}}' %s", labelKey, containerName)

	stdout, stderr, err := client.Execute(cmd)
	if err != nil {
		return "", fmt.Errorf("failed to get container label: %w (stderr: %s)", err, stderr)
	}

	return strings.TrimSpace(stdout), nil
}

// PullImage pulls the latest version of an image and returns true if a new image was pulled.
// Returns false if the image was already up to date (nothing to pull).
func (e *Executor) PullImage(client ssh.Connection, image string) (bool, error) {
	e.logger.Debug().
		Str("image", image).
		Msg("Pulling image")

	pullCmd := "docker pull " + image

	stdout, stderr, err := client.Execute(pullCmd)
	if err != nil {
		return false, fmt.Errorf("failed to pull image %s: %w (stderr: %s)", image, err, stderr)
	}

	// Check the Status: line to determine if image was updated
	// Docker outputs one of:
	//   "Status: Image is up to date for ..." - no changes
	//   "Status: Downloaded newer image for ..." - new image pulled
	output := strings.TrimSpace(stdout)

	if strings.Contains(output, "Status: Image is up to date") {
		e.logger.Debug().Str("image", image).Msg("Image already up to date")

		return false, nil
	}

	if strings.Contains(output, "Status: Downloaded newer image") {
		e.logger.Info().Str("image", image).Msg("New image pulled successfully")

		return true, nil
	}

	// If we can't determine from Status line, assume image was pulled
	e.logger.Warn().Str("image", image).Msg("Could not determine if image was updated, assuming yes")

	return true, nil
}

// RunContainer runs a Docker container on the remote host.
func (e *Executor) RunContainer(client ssh.Connection, opts ContainerRunOptions) error {
	// Collect all env files to pass to docker run
	var envFiles []string

	// Handle user-provided env file if specified
	if opts.EnvFile != "" {
		remotePath, err := e.uploadEnvFile(client, opts.EnvFile)
		if err != nil {
			return fmt.Errorf("failed to upload env file: %w", err)
		}

		envFiles = append(envFiles, remotePath)
	}

	// Generate and upload env file from EnvVars map
	if len(opts.EnvVars) > 0 {
		remotePath, err := e.uploadEnvVarsFile(client, opts.EnvVars)
		if err != nil {
			return fmt.Errorf("failed to upload env vars file: %w", err)
		}

		if remotePath != "" {
			envFiles = append(envFiles, remotePath)
		}
	}

	cmd := "docker run -d"

	// Container name
	cmd += " --name " + opts.Name

	// User
	if opts.User != "" {
		cmd += " --user " + opts.User
	}

	// Resource limits
	if opts.Memory != "" {
		cmd += " --memory " + opts.Memory
	}

	if opts.MemoryReservation != "" {
		cmd += " --memory-reservation " + opts.MemoryReservation
	}

	if opts.CPUShares > 0 {
		cmd += fmt.Sprintf(" --cpu-shares %d", opts.CPUShares)
	}

	// Network
	if opts.Network != "" {
		cmd += " --network " + opts.Network
	}

	// Network alias
	if opts.NetworkAlias != "" {
		cmd += " --network-alias " + opts.NetworkAlias
	}

	// Ports
	for _, port := range opts.Ports {
		cmd += " -p " + port
	}

	// Volumes
	for _, vol := range opts.Volumes {
		volStr := fmt.Sprintf("%s:%s", vol.Source, vol.Target)
		if vol.Mode != "" {
			volStr += ":" + vol.Mode
		}

		cmd += " -v " + volStr
	}

	// Tmpfs mounts
	for mountPoint, options := range opts.Tmpfs {
		tmpfsStr := mountPoint
		if options != "" {
			tmpfsStr = fmt.Sprintf("%s:%s", mountPoint, options)
		}

		cmd += " --tmpfs " + tmpfsStr
	}

	// Environment files (both user-provided and generated from EnvVars)
	for _, envFile := range envFiles {
		cmd += " --env-file " + envFile
	}

	// Restart policy
	if opts.Restart != "" {
		cmd += " --restart " + opts.Restart
	}

	// Read-only filesystem
	if opts.ReadOnly {
		cmd += " --read-only"
	}

	// Security options
	for _, opt := range opts.SecurityOpts {
		cmd += " --security-opt " + opt
	}

	// Capabilities
	for _, cap := range opts.CapDrop {
		cmd += " --cap-drop " + cap
	}

	for _, cap := range opts.CapAdd {
		cmd += " --cap-add " + cap
	}

	// Labels
	for k, v := range opts.Labels {
		cmd += fmt.Sprintf(labelFlagFormat, k, v)
	}

	// Image
	cmd += " " + opts.Image

	// Command arguments (appended after image)
	for _, arg := range opts.Command {
		cmd += " " + arg
	}

	e.logger.Debug().Str("command", cmd).Msg("Running container")

	stdout, stderr, err := client.Execute(cmd)
	if err != nil {
		return fmt.Errorf("failed to run container: %w (stderr: %s)", err, stderr)
	}

	e.logger.Info().Str("container", opts.Name).Str("id", strings.TrimSpace(stdout)).Msg("Container started")

	return nil
}

// StopContainer stops a Docker container.
func (e *Executor) StopContainer(client ssh.Connection, containerName string) error {
	cmd := "docker stop " + containerName
	e.logger.Debug().Str("command", cmd).Msg("Stopping container")

	_, stderr, err := client.Execute(cmd)
	if err != nil {
		return fmt.Errorf("failed to stop container: %w (stderr: %s)", err, stderr)
	}

	e.logger.Info().Str("container", containerName).Msg("Container stopped")

	return nil
}

// RemoveContainer removes a Docker container.
//
//revive:disable:flag-parameter
func (e *Executor) RemoveContainer(client ssh.Connection, containerName string, force bool) error {
	cmd := "docker rm " + containerName
	if force {
		cmd += " -f"
	}

	e.logger.Debug().Str("command", cmd).Msg("Removing container")

	_, stderr, err := client.Execute(cmd)
	if err != nil {
		return fmt.Errorf("failed to remove container: %w (stderr: %s)", err, stderr)
	}

	e.logger.Info().Str("container", containerName).Msg("Container removed")

	return nil
}

// RegistryLogin logs into a Docker registry on the remote host.
func (e *Executor) RegistryLogin(client ssh.Connection, registry, username, password string) error {
	// Use echo to pipe password to docker login stdin to avoid password in process list
	cmd := fmt.Sprintf("echo '%s' | docker login -u '%s' --password-stdin '%s'", password, username, registry)

	e.logger.Debug().
		Str("registry", registry).
		Str("username", username).
		Msg("Logging into registry")

	_, stderr, err := client.Execute(cmd)
	if err != nil {
		return fmt.Errorf("failed to login to registry %s: %w (stderr: %s)", registry, err, stderr)
	}

	e.logger.Info().Str("registry", registry).Str("username", username).Msg("Registry login successful")

	return nil
}

// ContainerRunOptions represents options for running a container.
type ContainerRunOptions struct {
	Name              string
	Image             string
	Command           []string // optional command arguments to append after image
	User              string   // user:group or UID:GID
	Memory            string   // memory limit (e.g., "512m", "2g")
	MemoryReservation string   // memory soft limit
	CPUShares         int64    // CPU shares (relative weight)
	Network           string
	NetworkAlias      string
	Ports             []string
	Volumes           []VolumeMount
	Tmpfs             map[string]string // mount point -> options
	EnvFile           string
	EnvVars           map[string]string
	Restart           string
	ReadOnly          bool
	SecurityOpts      []string
	CapDrop           []string
	CapAdd            []string
	Labels            map[string]string
}

// VolumeMount represents a volume mount for docker run.
type VolumeMount struct {
	Source string
	Target string
	Mode   string
}

// uploadContentAddressable uploads data to a content-addressable location on the remote host.
// Uses SHA256 hash of data as filename. Checks if file exists before uploading.
// Optional permissions parameter (defaults to 0600 if not provided).
// Returns the remote file path.
func (e *Executor) uploadContentAddressable(
	client ssh.Connection,
	data []byte,
	permissions ...os.FileMode,
) (string, error) {
	// Determine permissions (default to PermSecretFile)
	perm := PermSecretFile
	if len(permissions) > 0 {
		perm = permissions[0]
	}

	// 1. Hash the data
	dataHashRaw := sha256.Sum256(data)
	dataHash := hex.EncodeToString(dataHashRaw[:])

	// 2. Build remote path
	remotePath := "/var/lib/hadron/files/" + dataHash

	// 3. Check if file exists on remote
	checkCmd := fmt.Sprintf("test -f %s && echo %s || echo %s", remotePath, checkResultExists, checkResultMissing)

	stdout, _, err := client.Execute(checkCmd)
	if err != nil {
		return "", fmt.Errorf("failed to check if file exists on remote: %w", err)
	}

	if strings.TrimSpace(stdout) == checkResultExists {
		e.logger.Debug().Str("remote_path", remotePath).Msg("File already exists on remote")

		return remotePath, nil
	}

	// 4. File doesn't exist, upload it
	e.logger.Debug().Str("remote_path", remotePath).Int("size", len(data)).Msg("Uploading file")

	// Ensure remote directory exists
	mkdirCmd := "mkdir -p /var/lib/hadron/files"
	if _, _, err := client.Execute(mkdirCmd); err != nil {
		return "", fmt.Errorf("failed to create remote files directory: %w", err)
	}

	// Upload data (sets 0600 by default via UploadData)
	if err := client.UploadData(data, remotePath); err != nil {
		return "", fmt.Errorf("failed to upload file: %w", err)
	}

	// Override permissions if not PermSecretFile
	if perm != PermSecretFile {
		chmodCmd := fmt.Sprintf("chmod %o %s", perm, remotePath)
		if _, _, err := client.Execute(chmodCmd); err != nil {
			return "", fmt.Errorf("failed to set file permissions: %w", err)
		}
	}

	e.logger.Info().Str("remote_path", remotePath).Msg("File uploaded")

	return remotePath, nil
}

// uploadEnvFile uploads a local env file to the remote host if it doesn't already exist.
// Returns the remote file path.
func (e *Executor) uploadEnvFile(client ssh.Connection, localPath string) (string, error) {
	// Read local file
	// #nosec G304 -- localPath is controlled by plan author, not external user input
	data, err := os.ReadFile(localPath)
	if err != nil {
		return "", fmt.Errorf("failed to read env file: %w", err)
	}

	// Upload using content-addressable storage (0600 permissions for secrets)
	return e.uploadContentAddressable(client, data)
}

// uploadEnvVarsFile generates an env file from environment variables and uploads it.
// Uses content-addressable storage (content hash) to avoid duplicates.
// Returns the remote file path, or empty string if no env vars.
func (e *Executor) uploadEnvVarsFile(client ssh.Connection, envVars map[string]string) (string, error) {
	if len(envVars) == 0 {
		return "", nil
	}

	// Generate env file content
	var content strings.Builder

	// Sort keys for deterministic output (consistent hashing)
	keys := make([]string, 0, len(envVars))
	for k := range envVars {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	for _, k := range keys {
		// Docker run --env-file format: KEY=VALUE (no quotes - they become part of the value)
		// Replace actual newlines with literal \n text (application must convert back)
		escapedValue := strings.ReplaceAll(envVars[k], "\n", "\\n")

		_, _ = content.WriteString(k)
		_, _ = content.WriteString("=")
		_, _ = content.WriteString(escapedValue)
		_, _ = content.WriteString("\n")
	}

	// Upload using content-addressable storage (0600 permissions for secrets)
	return e.uploadContentAddressable(client, []byte(content.String()))
}

// UploadMount uploads a local file or directory to the remote host if it doesn't already exist.
// Returns the remote path.
func (e *Executor) UploadMount(client ssh.Connection, localPath string) (string, error) {
	// Check if local path is a file or directory
	info, err := os.Stat(localPath)
	if err != nil {
		return "", fmt.Errorf("failed to stat local path: %w", err)
	}

	if info.IsDir() {
		// Directories: use path-based hash (includes directory structure)
		pathHash, err := hash.Path(localPath)
		if err != nil {
			return "", fmt.Errorf("failed to hash mount path: %w", err)
		}

		remotePath := "/var/lib/hadron/files/" + pathHash

		// Check if directory exists on remote
		checkCmd := fmt.Sprintf("test -e %s && echo %s || echo %s", remotePath, checkResultExists, checkResultMissing)

		stdout, _, err := client.Execute(checkCmd)
		if err != nil {
			return "", fmt.Errorf("failed to check if mount exists on remote: %w", err)
		}

		if strings.TrimSpace(stdout) == checkResultExists {
			e.logger.Debug().Str("remote_path", remotePath).Msg("Mount already exists on remote")

			return remotePath, nil
		}

		// Upload directory recursively
		e.logger.Debug().Str("local_path", localPath).Str("remote_path", remotePath).Msg("Uploading mount directory")

		if err := e.uploadDirectory(client, localPath, remotePath); err != nil {
			return "", fmt.Errorf("failed to upload directory: %w", err)
		}

		e.logger.Info().Str("remote_path", remotePath).Msg("Mount uploaded")

		return remotePath, nil
	}

	// Single file: use content-addressable upload with PermPublicFile permissions for container readability
	// #nosec G304 -- localPath is controlled by plan author, not external user input
	data, err := os.ReadFile(localPath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	return e.uploadContentAddressable(client, data, PermPublicFile)
}

// UploadDataMount uploads raw data as a file to the remote host if it doesn't already exist.
// Uses content-addressable storage (SHA256 hash) to avoid duplicates.
// Returns the remote path.
func (e *Executor) UploadDataMount(client ssh.Connection, data []byte) (string, error) {
	// Upload using content-addressable storage with 0644 permissions
	// This allows non-root container users to read mounted files
	// TODO(security): Files are world-readable on host. Future: use ACLs or init containers to set proper ownership
	return e.uploadContentAddressable(client, data, PermPublicFile)
}

// uploadDirectory uploads a directory to the remote host.
// For simplicity, we upload files individually rather than using tar.
func (e *Executor) uploadDirectory(client ssh.Connection, localDir, remotePath string) error {
	return e.uploadDirectoryRecursive(client, localDir, remotePath)
}

// uploadDirectoryRecursive uploads a directory recursively file by file.
func (*Executor) uploadDirectoryRecursive(client ssh.Connection, localDir, remoteBase string) error {
	err := filepath.Walk(localDir, func(localPath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Get relative path
		relPath, err := filepath.Rel(localDir, localPath)
		if err != nil {
			return fmt.Errorf("%w: %w", ErrPathRelative, err)
		}

		remotePath := filepath.Join(remoteBase, relPath)

		if info.IsDir() {
			// Create directory on remote
			mkdirCmd := "mkdir -p " + remotePath
			if _, _, err := client.Execute(mkdirCmd); err != nil {
				return fmt.Errorf("failed to create remote directory %s: %w", remotePath, err)
			}
		} else {
			// Upload file
			if err := client.UploadFile(localPath, remotePath); err != nil {
				return fmt.Errorf("failed to upload file %s: %w", localPath, err)
			}

			// Set permissions
			chmodCmd := "chmod 644 " + remotePath
			if _, _, err := client.Execute(chmodCmd); err != nil {
				return fmt.Errorf("failed to set permissions on %s: %w", remotePath, err)
			}
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("%w: %w", ErrFileSystemWalk, err)
	}

	return nil
}
