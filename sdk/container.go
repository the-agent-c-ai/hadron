package sdk

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"github.com/the-agent-c-ai/hadron/sdk/hash"
)

const (
	commaSeparator = ","
)

// Container represents a Docker container.
type Container struct {
	name              string
	host              *Host
	image             string
	command           []string   // optional command arguments to append to docker run
	user              string     // user:group or UID:GID
	memory            string     // memory limit (e.g., "512m", "2g")
	memoryReservation string     // memory soft limit
	cpuShares         int64      // CPU shares (relative weight)
	cpus              string     // hard CPU limit (e.g., "1.5" for 1.5 CPUs)
	pidsLimit         int64      // maximum number of PIDs (process limit)
	hostname          string     // container hostname
	networks          []*Network // networks to connect to
	networkAlias      string
	ports             []string
	extraHosts        []string // extra host:ip mappings (e.g., "host.docker.internal:host-gateway")
	volumes           []VolumeMount
	mounts            []FileMount
	dataMounts        []DataMount
	tmpfs             map[string]string // mount point -> options (e.g., "noexec,size=100m")
	envFile           string
	envVars           map[string]string
	labels            map[string]string // Docker labels for metadata and service discovery
	healthCheck       *HealthCheck
	dependsOn         []*Container
	readOnly          bool
	securityOpts      []string
	capDrop           []string
	capAdd            []string
	groupAdd          []string // additional groups for the container user
	restart           string
	plan              *Plan
}

// VolumeMount represents a volume mount in a container.
type VolumeMount struct {
	source string // volume name or host path
	target string // container path
	mode   string // ro, rw (optional)
}

// FileMount represents a local file or directory mounted into a container.
type FileMount struct {
	localPath     string // local file or directory path
	containerPath string // container mount path
	mode          string // ro, rw (optional)
}

// DataMount represents raw data mounted as a file into a container.
type DataMount struct {
	data          []byte // raw data to mount
	containerPath string // container mount path
	mode          string // ro, rw (optional)
}

// ContainerBuilder builds a Container with a fluent API.
type ContainerBuilder struct {
	plan              *Plan
	name              string
	host              *Host
	image             string
	command           []string   // optional command arguments to append to docker run
	user              string     // user:group or UID:GID
	memory            string     // memory limit (e.g., "512m", "2g")
	memoryReservation string     // memory soft limit
	cpuShares         int64      // CPU shares (relative weight)
	cpus              string     // hard CPU limit (e.g., "1.5" for 1.5 CPUs)
	pidsLimit         int64      // maximum number of PIDs (process limit)
	hostname          string     // container hostname
	networks          []*Network // networks to connect to
	networkAlias      string
	ports             []string
	extraHosts        []string // extra host:ip mappings (e.g., "host.docker.internal:host-gateway")
	volumes           []VolumeMount
	mounts            []FileMount
	dataMounts        []DataMount
	tmpfs             map[string]string // mount point -> options (e.g., "noexec,size=100m")
	envFile           string
	envVars           map[string]string
	labels            map[string]string // Docker labels for metadata and service discovery
	healthCheck       *HealthCheck
	dependsOn         []*Container
	readOnly          bool
	securityOpts      []string
	capDrop           []string
	capAdd            []string
	groupAdd          []string // additional groups for the container user
	restart           string
}

// Host sets the host where this container will run.
func (cb *ContainerBuilder) Host(host *Host) *ContainerBuilder {
	cb.host = host

	return cb
}

// Image sets the container image (should include digest for immutability).
func (cb *ContainerBuilder) Image(image string) *ContainerBuilder {
	cb.image = image

	return cb
}

// Command sets optional command arguments to append to docker run.
// These arguments are passed after the image name: docker run [OPTIONS] IMAGE [COMMAND...].
func (cb *ContainerBuilder) Command(args ...string) *ContainerBuilder {
	cb.command = args

	return cb
}

// User sets the user to run the container as (user:group or UID:GID).
func (cb *ContainerBuilder) User(user string) *ContainerBuilder {
	cb.user = user

	return cb
}

// Memory sets the memory limit for the container (e.g., "512m", "2g").
func (cb *ContainerBuilder) Memory(limit string) *ContainerBuilder {
	cb.memory = limit

	return cb
}

// MemoryReservation sets the memory soft limit for the container.
func (cb *ContainerBuilder) MemoryReservation(limit string) *ContainerBuilder {
	cb.memoryReservation = limit

	return cb
}

// CPUShares sets the CPU shares (relative weight) for the container.
func (cb *ContainerBuilder) CPUShares(shares int64) *ContainerBuilder {
	cb.cpuShares = shares

	return cb
}

// CPUs sets the hard CPU limit for the container (e.g., "1.5" for 1.5 CPUs).
func (cb *ContainerBuilder) CPUs(cpus string) *ContainerBuilder {
	cb.cpus = cpus

	return cb
}

// PIDsLimit sets the maximum number of PIDs (process limit) for the container.
func (cb *ContainerBuilder) PIDsLimit(limit int64) *ContainerBuilder {
	cb.pidsLimit = limit

	return cb
}

// Hostname sets the hostname for the container.
func (cb *ContainerBuilder) Hostname(hostname string) *ContainerBuilder {
	cb.hostname = hostname

	return cb
}

// Network sets the Docker network for this container.
// Network adds a network to the container. Can be called multiple times to connect to multiple networks.
func (cb *ContainerBuilder) Network(network *Network) *ContainerBuilder {
	cb.networks = append(cb.networks, network)

	return cb
}

// NetworkAlias sets a DNS alias for this container on the network.
func (cb *ContainerBuilder) NetworkAlias(alias string) *ContainerBuilder {
	cb.networkAlias = alias

	return cb
}

// Port adds a port mapping (format: "host:container" or "port").
func (cb *ContainerBuilder) Port(port string) *ContainerBuilder {
	cb.ports = append(cb.ports, port)

	return cb
}

// ExtraHosts adds a custom host-to-IP mapping (format: "hostname:ip").
// Special value "host-gateway" maps to the host's gateway IP.
// Example: ExtraHosts("host.docker.internal:host-gateway").
func (cb *ContainerBuilder) ExtraHosts(mapping string) *ContainerBuilder {
	cb.extraHosts = append(cb.extraHosts, mapping)

	return cb
}

// Volume can be used for bind mounts: ("/host/path", "/container/path", "ro").
func (cb *ContainerBuilder) Volume(source any, target string, mode ...string) *ContainerBuilder {
	mount := VolumeMount{
		target: target,
	}

	switch v := source.(type) {
	case *Volume:
		mount.source = v.Name()
	case string:
		mount.source = v
	default:
		cb.plan.logger.Fatal().Str("container", cb.name).Msg("volume source must be *Volume or string")
	}

	if len(mode) > 0 {
		mount.mode = mode[0]
	}

	cb.volumes = append(cb.volumes, mount)

	return cb
}

// Mount mounts a local file or directory into the container.
// The local path is uploaded to the remote host and mounted into the container.
func (cb *ContainerBuilder) Mount(localPath, containerPath string, mode ...string) *ContainerBuilder {
	mount := FileMount{
		localPath:     localPath,
		containerPath: containerPath,
	}

	if len(mode) > 0 {
		mount.mode = mode[0]
	}

	cb.mounts = append(cb.mounts, mount)

	return cb
}

// MountData mounts raw data as a file into the container.
// The data is content-addressed (SHA256 hash) and uploaded to the remote host.
// This avoids creating temporary files locally with sensitive data.
func (cb *ContainerBuilder) MountData(data []byte, containerPath string, mode ...string) *ContainerBuilder {
	mount := DataMount{
		data:          data,
		containerPath: containerPath,
	}

	if len(mode) > 0 {
		mount.mode = mode[0]
	}

	cb.dataMounts = append(cb.dataMounts, mount)

	return cb
}

// Tmpfs mounts a tmp filesysten ("/tmp", "size=100m") -> results in "noexec,nosuid,nodev,size=100m".
func (cb *ContainerBuilder) Tmpfs(mountPoint string, options ...string) *ContainerBuilder {
	if cb.tmpfs == nil {
		cb.tmpfs = make(map[string]string)
	}

	// ALWAYS enforce security flags
	securityFlags := "noexec,nosuid,nodev"

	// Append any additional options
	opts := securityFlags
	if len(options) > 0 && options[0] != "" {
		opts = securityFlags + "," + options[0]
	}

	cb.tmpfs[mountPoint] = opts

	return cb
}

// EnvFile sets the path to an environment file to load.
func (cb *ContainerBuilder) EnvFile(path string) *ContainerBuilder {
	cb.envFile = path

	return cb
}

// Env sets an environment variable.
func (cb *ContainerBuilder) Env(key, value string) *ContainerBuilder {
	cb.envVars[key] = value

	return cb
}

// Label sets a Docker label for metadata and service discovery.
func (cb *ContainerBuilder) Label(key, value string) *ContainerBuilder {
	cb.labels[key] = value

	return cb
}

// HealthCheck sets the health check for this container.
func (cb *ContainerBuilder) HealthCheck(check *HealthCheck) *ContainerBuilder {
	cb.healthCheck = check

	return cb
}

// DependsOn adds a dependency on another container.
// This container will start after the dependency is healthy.
func (cb *ContainerBuilder) DependsOn(container *Container) *ContainerBuilder {
	cb.dependsOn = append(cb.dependsOn, container)

	return cb
}

// ReadOnly sets the container filesystem to read-only.
func (cb *ContainerBuilder) ReadOnly() *ContainerBuilder {
	cb.readOnly = true

	return cb
}

// SecurityOpt adds a security option.
func (cb *ContainerBuilder) SecurityOpt(opt string) *ContainerBuilder {
	cb.securityOpts = append(cb.securityOpts, opt)

	return cb
}

// CapDrop drops a Linux capability.
func (cb *ContainerBuilder) CapDrop(capability string) *ContainerBuilder {
	cb.capDrop = append(cb.capDrop, capability)

	return cb
}

// CapAdd adds a Linux capability.
func (cb *ContainerBuilder) CapAdd(capability string) *ContainerBuilder {
	cb.capAdd = append(cb.capAdd, capability)

	return cb
}

// GroupAdd adds an additional group for the container user.
// Useful for granting access to host resources (e.g., "docker" for socket access).
func (cb *ContainerBuilder) GroupAdd(group string) *ContainerBuilder {
	cb.groupAdd = append(cb.groupAdd, group)

	return cb
}

// Restart sets the restart policy (default: unless-stopped).
func (cb *ContainerBuilder) Restart(policy string) *ContainerBuilder {
	cb.restart = policy

	return cb
}

// Build creates the Container and registers it with the plan.
func (cb *ContainerBuilder) Build() *Container {
	if cb.host == nil {
		cb.plan.logger.Fatal().Str("container", cb.name).Msg("container must be assigned to a host")
	}

	if cb.image == "" {
		cb.plan.logger.Fatal().Str("container", cb.name).Msg("container image is required")
	}

	if cb.restart == "" {
		cb.restart = "unless-stopped"
	}

	// Enforce mandatory resource limits (CIS Docker Benchmark compliance)
	if cb.memory == "" {
		cb.plan.logger.Fatal().Str("container", cb.name).Msg("memory limit is required (CIS 5.10)")
	}

	if cb.cpuShares == 0 {
		cb.plan.logger.Fatal().Str("container", cb.name).Msg("cpu-shares is required (CIS 5.11)")
	}

	if cb.cpus == "" {
		cb.plan.logger.Fatal().Str("container", cb.name).Msg("cpus limit is required")
	}

	if cb.pidsLimit == 0 {
		cb.plan.logger.Fatal().Str("container", cb.name).Msg("pids-limit is required")
	}

	container := &Container{
		name:              cb.name,
		host:              cb.host,
		image:             cb.image,
		command:           cb.command,
		user:              cb.user,
		memory:            cb.memory,
		memoryReservation: cb.memoryReservation,
		cpuShares:         cb.cpuShares,
		cpus:              cb.cpus,
		pidsLimit:         cb.pidsLimit,
		hostname:          cb.hostname,
		networks:          cb.networks,
		networkAlias:      cb.networkAlias,
		ports:             cb.ports,
		extraHosts:        cb.extraHosts,
		volumes:           cb.volumes,
		mounts:            cb.mounts,
		dataMounts:        cb.dataMounts,
		tmpfs:             cb.tmpfs,
		envFile:           cb.envFile,
		envVars:           cb.envVars,
		labels:            cb.labels,
		healthCheck:       cb.healthCheck,
		dependsOn:         cb.dependsOn,
		readOnly:          cb.readOnly,
		securityOpts:      cb.securityOpts,
		capDrop:           cb.capDrop,
		capAdd:            cb.capAdd,
		groupAdd:          cb.groupAdd,
		restart:           cb.restart,
		plan:              cb.plan,
	}

	cb.plan.containers = append(cb.plan.containers, container)

	return container
}

// Name returns the container name.
func (c *Container) Name() string {
	return c.name
}

// Host returns the host where this container runs.
func (c *Container) Host() *Host {
	return c.host
}

// Image returns the container image.
func (c *Container) Image() string {
	return c.image
}

// NetworkAlias returns the DNS alias for this container.
func (c *Container) NetworkAlias() string {
	return c.networkAlias
}

// HealthCheck returns the health check configuration.
func (c *Container) HealthCheck() *HealthCheck {
	return c.healthCheck
}

// DependsOn returns the containers this container depends on.
func (c *Container) DependsOn() []*Container {
	return c.dependsOn
}

// ConfigHash returns a SHA256 hash of the container configuration.
// Used for idempotent deployments.
func (c *Container) ConfigHash() string {
	var configParts []string

	configParts = append(configParts, c.name, c.image)

	if len(c.command) > 0 {
		configParts = append(configParts, strings.Join(c.command, " "))
	}

	if c.user != "" {
		configParts = append(configParts, c.user)
	}

	if c.memory != "" {
		configParts = append(configParts, c.memory)
	}

	if c.memoryReservation != "" {
		configParts = append(configParts, c.memoryReservation)
	}

	if c.cpuShares > 0 {
		configParts = append(configParts, fmt.Sprintf("cpu-shares:%d", c.cpuShares))
	}

	if c.cpus != "" {
		configParts = append(configParts, "cpus:"+c.cpus)
	}

	if c.pidsLimit > 0 {
		configParts = append(configParts, fmt.Sprintf("pids-limit:%d", c.pidsLimit))
	}

	if c.hostname != "" {
		configParts = append(configParts, c.hostname)
	}

	// Include all networks in config hash (sorted for determinism)
	if len(c.networks) > 0 {
		networkNames := make([]string, len(c.networks))
		for i, net := range c.networks {
			networkNames[i] = net.Name()
		}

		sort.Strings(networkNames)
		configParts = append(configParts, strings.Join(networkNames, commaSeparator))
	}

	if c.networkAlias != "" {
		configParts = append(configParts, c.networkAlias)
	}

	configParts = append(configParts, strings.Join(c.ports, commaSeparator))
	configParts = append(configParts, strings.Join(c.extraHosts, commaSeparator))

	for _, v := range c.volumes {
		configParts = append(configParts, fmt.Sprintf("%s:%s:%s", v.source, v.target, v.mode))
	}

	for _, mount := range c.mounts {
		// Hash the actual file/directory content, not just the path
		contentHash, err := hash.Path(mount.localPath)
		if err != nil {
			// If we can't hash it, fall back to path (better than crashing)
			c.plan.logger.Warn().
				Err(err).
				Str("path", mount.localPath).
				Msg("Failed to hash mount content, using path instead")

			configParts = append(
				configParts,
				fmt.Sprintf("mount:%s:%s:%s", mount.localPath, mount.containerPath, mount.mode),
			)
		} else {
			configParts = append(
				configParts,
				fmt.Sprintf("mount:%s:%s:%s", contentHash, mount.containerPath, mount.mode),
			)
		}
	}

	// Data mounts - hash the data content directly
	for _, mount := range c.dataMounts {
		dataHash := sha256.Sum256(mount.data)
		configParts = append(
			configParts,
			fmt.Sprintf("datamount:%x:%s:%s", dataHash, mount.containerPath, mount.mode),
		)
	}

	// Tmpfs mounts
	for mountPoint, options := range c.tmpfs {
		configParts = append(configParts, fmt.Sprintf("tmpfs:%s:%s", mountPoint, options))
	}

	// Hash the env file content, not just the path
	if c.envFile != "" {
		envFileHash, err := hash.File(c.envFile)
		if err != nil {
			// If we can't hash it, fall back to path (better than crashing)
			c.plan.logger.Warn().
				Err(err).
				Str("path", c.envFile).
				Msg("Failed to hash env file content, using path instead")

			configParts = append(configParts, "envfile:"+c.envFile)
		} else {
			configParts = append(configParts, "envfile:"+envFileHash)
		}
	}

	// Sort env var keys for deterministic hash
	envKeys := make([]string, 0, len(c.envVars))
	for k := range c.envVars {
		envKeys = append(envKeys, k)
	}

	sort.Strings(envKeys)

	for _, k := range envKeys {
		configParts = append(configParts, fmt.Sprintf("%s=%s", k, c.envVars[k]))
	}

	// Sort label keys for deterministic hash
	labelKeys := make([]string, 0, len(c.labels))
	for k := range c.labels {
		labelKeys = append(labelKeys, k)
	}

	sort.Strings(labelKeys)

	for _, k := range labelKeys {
		configParts = append(configParts, fmt.Sprintf("label:%s=%s", k, c.labels[k]))
	}

	configParts = append(configParts, fmt.Sprintf("readonly=%t", c.readOnly))
	configParts = append(configParts, strings.Join(c.securityOpts, commaSeparator))
	configParts = append(configParts, strings.Join(c.capDrop, commaSeparator))
	configParts = append(configParts, strings.Join(c.capAdd, commaSeparator))
	configParts = append(configParts, strings.Join(c.groupAdd, commaSeparator))
	configParts = append(configParts, c.restart)

	config := strings.Join(configParts, "|")
	configHash := sha256.Sum256([]byte(config))

	return hex.EncodeToString(configHash[:])
}
