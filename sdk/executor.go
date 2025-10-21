package sdk

import (
	"fmt"
	"time"

	"github.com/the-agent-c-ai/hadron/internal/debian"
	"github.com/the-agent-c-ai/hadron/internal/docker"
	"github.com/the-agent-c-ai/hadron/internal/firewall"
	"github.com/the-agent-c-ai/hadron/sdk/ssh"
)

const (
	labelConfigSHA     = "hadron.config.sha"
	labelPlan          = "hadron.plan"
	errFailedSSHClient = "failed to get SSH client for %s: %w"
	dockerReadyTimeout = 30 * time.Second
)

// deployableResource represents a resource that can be deployed (Network, Volume).
type deployableResource interface {
	Name() string
	Host() *Host
	Driver() string
	ConfigHash() string
}

// resourceOperations defines the operations needed to deploy a resource.
type resourceOperations struct {
	resourceType string
	exists       func(ssh.Connection, string) (bool, error)
	getLabel     func(ssh.Connection, string, string) (string, error)
	remove       func(ssh.Connection, string) error
	create       func(ssh.Connection, string, string, map[string]string) error
	existsError  error
	createError  error
}

// executor implements plan execution logic.
type executor struct {
	plan       *Plan
	sshPool    *ssh.Pool
	dockerExec *docker.Executor
}

// newExecutor creates a new plan executor.
func newExecutor(plan *Plan) *executor {
	sshPool := ssh.NewPool(plan.logger)
	dockerExec := docker.NewExecutor(sshPool, plan.logger)

	return &executor{
		plan:       plan,
		sshPool:    sshPool,
		dockerExec: dockerExec,
	}
}

// execute performs the actual deployment.
func (e *executor) execute() error {
	defer func() {
		if err := e.sshPool.CloseAll(); err != nil {
			e.plan.logger.Warn().Err(err).Msg("Failed to close SSH connections")
		}
	}()

	e.plan.logger.Info().Msg("Starting deployment")

	// Deploy packages first (install then remove)
	if err := e.deployPackages(); err != nil {
		return fmt.Errorf("failed to deploy packages: %w", err)
	}

	// Configure Docker daemon after packages (Docker must be installed)
	if err := e.deployDockerDaemon(); err != nil {
		return fmt.Errorf("failed to deploy docker daemon config: %w", err)
	}

	// Configure automatic security updates (always enabled)
	if err := e.deployAutoUpdates(); err != nil {
		return fmt.Errorf("failed to deploy automatic updates: %w", err)
	}

	// Configure firewalls after packages (ufw may need to be installed)
	if err := e.deployFirewalls(); err != nil {
		return fmt.Errorf("failed to deploy firewalls: %w", err)
	}

	// Login to registries after Docker is available
	if err := e.loginRegistries(); err != nil {
		return fmt.Errorf("failed to login to registries: %w", err)
	}

	// Deploy networks
	if err := e.deployNetworks(); err != nil {
		return fmt.Errorf("failed to deploy networks: %w", err)
	}

	// Deploy volumes
	if err := e.deployVolumes(); err != nil {
		return fmt.Errorf("failed to deploy volumes: %w", err)
	}

	// Deploy containers (respecting dependencies)
	if err := e.deployContainers(); err != nil {
		return fmt.Errorf("failed to deploy containers: %w", err)
	}

	e.plan.logger.Info().Msg("Deployment completed successfully")

	return nil
}

// deployNetworks deploys all networks in the plan.
func (e *executor) deployNetworks() error {
	for _, network := range e.plan.networks {
		if err := e.deployNetwork(network); err != nil {
			return err
		}
	}

	return nil
}

// deployResource is a generic function to deploy a resource (network or volume).
// This eliminates code duplication between deployNetwork and deployVolume.
func (e *executor) deployResource(resource deployableResource, ops resourceOperations) error {
	client, err := e.sshPool.GetClient(resource.Host().Endpoint())
	if err != nil {
		return fmt.Errorf(errFailedSSHClient, resource.Host(), err)
	}

	// Check if resource exists
	exists, err := ops.exists(client, resource.Name())
	if err != nil {
		return fmt.Errorf("%w: %w", ops.existsError, err)
	}

	if exists {
		// Check config hash to see if update needed
		existingHash, err := ops.getLabel(client, resource.Name(), labelConfigSHA)
		if err != nil {
			e.plan.logger.Warn().
				Str(ops.resourceType, resource.Name()).
				Msg("Could not get existing config hash, will recreate")
		} else if existingHash == resource.ConfigHash() {
			e.plan.logger.Info().Str(ops.resourceType, resource.Name()).Msg(ops.resourceType + " unchanged, skipping")

			return nil
		}

		// Config changed or missing, need to recreate
		e.plan.logger.Info().
			Str(ops.resourceType, resource.Name()).
			Msg(ops.resourceType + " config changed, recreating")

		if err := ops.remove(client, resource.Name()); err != nil {
			return fmt.Errorf("failed to remove old %s: %w", ops.resourceType, err)
		}
	}

	labels := map[string]string{
		labelConfigSHA: resource.ConfigHash(),
		labelPlan:      e.plan.name,
	}

	if err := ops.create(client, resource.Name(), resource.Driver(), labels); err != nil {
		return fmt.Errorf("%w: %w", ops.createError, err)
	}

	return nil
}

// deployNetwork deploys a single network.
func (e *executor) deployNetwork(network *Network) error {
	return e.deployResource(network, resourceOperations{
		resourceType: "network",
		exists:       e.dockerExec.NetworkExists,
		getLabel:     e.dockerExec.GetNetworkLabel,
		remove:       e.dockerExec.RemoveNetwork,
		create:       e.dockerExec.CreateNetwork,
		existsError:  ErrNetworkCheck,
		createError:  ErrNetworkCreate,
	})
}

// deployVolumes deploys all volumes in the plan.
func (e *executor) deployVolumes() error {
	for _, volume := range e.plan.volumes {
		if err := e.deployVolume(volume); err != nil {
			return err
		}
	}

	return nil
}

// deployVolume deploys a single volume.
func (e *executor) deployVolume(volume *Volume) error {
	return e.deployResource(volume, resourceOperations{
		resourceType: "volume",
		exists:       e.dockerExec.VolumeExists,
		getLabel:     e.dockerExec.GetVolumeLabel,
		remove:       e.dockerExec.RemoveVolume,
		create:       e.dockerExec.CreateVolume,
		existsError:  ErrVolumeCheck,
		createError:  ErrVolumeCreate,
	})
}

// deployContainers deploys all containers in the plan, respecting dependencies.
func (e *executor) deployContainers() error {
	// TODO: Implement dependency resolution and ordering
	// For MVP, deploy in order defined in plan
	for _, container := range e.plan.containers {
		if err := e.deployContainer(container); err != nil {
			return err
		}
	}

	return nil
}

// deployContainer deploys a single container.
func (e *executor) deployContainer(container *Container) error {
	client, err := e.sshPool.GetClient(container.host.Endpoint())
	if err != nil {
		return fmt.Errorf(errFailedSSHClient, container.host, err)
	}

	// Always pull the latest image to detect updates
	// This ensures that even if the config hash is unchanged, we redeploy if the image changed
	e.plan.logger.Info().
		Str("container", container.Name()).
		Str("image", container.Image()).
		Msg("Pulling latest image")

	imagePulled, err := e.dockerExec.PullImage(client, container.Image())
	if err != nil {
		return fmt.Errorf("failed to pull image: %w", err)
	}

	// Check if container exists
	exists, err := e.dockerExec.ContainerExists(client, container.Name())
	if err != nil {
		return fmt.Errorf("%w: %w", ErrContainerCheck, err)
	}

	if exists {
		// Check config hash
		existingHash, err := e.dockerExec.GetContainerLabel(client, container.Name(), labelConfigSHA)

		switch {
		case err != nil:
			e.plan.logger.Warn().Str("container", container.Name()).Msg("Could not get existing config hash")
		case existingHash == container.ConfigHash() && !imagePulled:
			// Config unchanged AND image wasn't updated (already had latest)
			e.plan.logger.Info().Str("container", container.Name()).Msg("Container unchanged, skipping")

			return nil
		case imagePulled:
			e.plan.logger.Info().
				Str("container", container.Name()).
				Msg("Image updated, redeploying container")
		}

		// Need to update container
		e.plan.logger.Info().Str("container", container.Name()).Msg("Container config changed, updating")

		// For MVP, stop and remove old container
		// TODO: Implement zero-downtime updates with network aliases
		if err := e.dockerExec.StopContainer(client, container.Name()); err != nil {
			e.plan.logger.Warn().Err(err).Msg("Failed to stop old container")
		}

		if err := e.dockerExec.RemoveContainer(client, container.Name(), true); err != nil {
			return fmt.Errorf("failed to remove old container: %w", err)
		}
	}

	// Prepare volumes - pre-allocate capacity for all volume types to avoid reallocations
	totalCapacity := len(container.volumes) + len(container.mounts) + len(container.dataMounts)
	volumes := make([]docker.VolumeMount, 0, totalCapacity)

	// Add container volumes
	for _, v := range container.volumes {
		volumes = append(volumes, docker.VolumeMount{
			Source: v.source,
			Target: v.target,
			Mode:   v.mode,
		})
	}

	// Handle file mounts - upload local files/directories and add to volumes
	for _, mount := range container.mounts {
		e.plan.logger.Info().
			Str("container", container.Name()).
			Str("local_path", mount.localPath).
			Str("container_path", mount.containerPath).
			Msg("Uploading mount")

		remotePath, err := e.dockerExec.UploadMount(client, mount.localPath)
		if err != nil {
			return fmt.Errorf("failed to upload mount %s: %w", mount.localPath, err)
		}

		// Add to volumes list
		volumes = append(volumes, docker.VolumeMount{
			Source: remotePath,
			Target: mount.containerPath,
			Mode:   mount.mode,
		})

		e.plan.logger.Info().
			Str("container", container.Name()).
			Str("remote_path", remotePath).
			Str("container_path", mount.containerPath).
			Msg("Mount uploaded successfully")
	}

	// Handle data mounts - upload raw data as files and add to volumes
	for _, mount := range container.dataMounts {
		e.plan.logger.Info().
			Str("container", container.Name()).
			Int("data_size", len(mount.data)).
			Str("container_path", mount.containerPath).
			Msg("Uploading data mount")

		remotePath, err := e.dockerExec.UploadDataMount(client, mount.data)
		if err != nil {
			return fmt.Errorf("failed to upload data mount to %s: %w", mount.containerPath, err)
		}

		// Add to volumes list
		volumes = append(volumes, docker.VolumeMount{
			Source: remotePath,
			Target: mount.containerPath,
			Mode:   mount.mode,
		})

		e.plan.logger.Info().
			Str("container", container.Name()).
			Str("remote_path", remotePath).
			Str("container_path", mount.containerPath).
			Msg("Data mount uploaded successfully")
	}

	// Prepare run options
	opts := docker.ContainerRunOptions{
		Name:              container.name,
		Image:             container.image,
		Command:           container.command,
		User:              container.user,
		Memory:            container.memory,
		MemoryReservation: container.memoryReservation,
		CPUShares:         container.cpuShares,
		Network:           "",
		NetworkAlias:      container.networkAlias,
		Ports:             container.ports,
		Volumes:           volumes,
		Tmpfs:             container.tmpfs,
		EnvFile:           container.envFile,
		EnvVars:           container.envVars,
		Restart:           container.restart,
		ReadOnly:          container.readOnly,
		SecurityOpts:      container.securityOpts,
		CapDrop:           container.capDrop,
		CapAdd:            container.capAdd,
		Labels: map[string]string{
			labelConfigSHA: container.ConfigHash(),
			labelPlan:      e.plan.name,
		},
	}

	if container.network != nil {
		opts.Network = container.network.Name()
	}

	// Run container
	if err := e.dockerExec.RunContainer(client, opts); err != nil {
		return fmt.Errorf("failed to run container: %w", err)
	}

	// TODO: Perform health check if configured

	return nil
}

// deployPackages manages package installation and removal on all hosts.
func (e *executor) deployPackages() error {
	// Process each host's package requirements
	for _, host := range e.plan.hosts {
		if err := e.deployHostPackages(host); err != nil {
			return err
		}
	}

	return nil
}

// deployHostPackages manages packages for a single host (install then remove).
func (e *executor) deployHostPackages(host *Host) error {
	// Skip if no package operations needed
	if len(host.packages) == 0 && len(host.removePackages) == 0 {
		return nil
	}

	// Get SSH client for this host
	client, err := e.sshPool.GetClient(host.Endpoint())
	if err != nil {
		return fmt.Errorf(errFailedSSHClient, host, err)
	}

	// Phase 1: Install packages
	for _, packageName := range host.packages {
		e.plan.logger.Info().
			Str("host", host.String()).
			Str("package", packageName).
			Msg("Ensuring package is installed")

		if err := debian.EnsureInstalled(client, packageName); err != nil {
			return fmt.Errorf("failed to install package %s on %s: %w", packageName, host, err)
		}

		e.plan.logger.Info().
			Str("host", host.String()).
			Str("package", packageName).
			Msg("Package installed successfully")
	}

	// Phase 2: Remove packages
	for _, packageName := range host.removePackages {
		e.plan.logger.Info().
			Str("host", host.String()).
			Str("package", packageName).
			Msg("Ensuring package is removed")

		if err := debian.EnsureRemoved(client, packageName); err != nil {
			return fmt.Errorf("failed to remove package %s from %s: %w", packageName, host, err)
		}

		e.plan.logger.Info().
			Str("host", host.String()).
			Str("package", packageName).
			Msg("Package removed successfully")
	}

	return nil
}

// deployDockerDaemon configures Docker daemon on all hosts.
func (e *executor) deployDockerDaemon() error {
	// Process each host's Docker daemon configuration
	for _, host := range e.plan.hosts {
		if err := e.deployHostDockerDaemon(host); err != nil {
			return err
		}
	}

	return nil
}

// deployHostDockerDaemon configures the Docker daemon for a single host.
func (e *executor) deployHostDockerDaemon(host *Host) error {
	// Skip if hardening not requested
	if !host.hardenDocker {
		return nil
	}

	// Get SSH client for this host
	client, err := e.sshPool.GetClient(host.Endpoint())
	if err != nil {
		return fmt.Errorf(errFailedSSHClient, host, err)
	}

	e.plan.logger.Info().
		Str("host", host.String()).
		Msg("Configuring Docker daemon security hardening")

	// Get desired configuration
	desiredConfig := docker.GetSecureDefaults()

	// Check if config exists
	exists, err := docker.DaemonConfigExists(client)
	if err != nil {
		return fmt.Errorf("failed to check daemon config on %s: %w", host, err)
	}

	var needsRestart bool

	switch {
	case !exists:
		e.plan.logger.Info().
			Str("host", host.String()).
			Msg("Docker daemon config not found, creating")

		needsRestart = true
	case exists:
		// Read current config
		currentConfig, err := docker.GetDaemonConfig(client)

		switch {
		case err != nil:
			e.plan.logger.Warn().
				Err(err).
				Str("host", host.String()).
				Msg("Could not read current daemon config, will overwrite")

			needsRestart = true
		case docker.ConfigsEqual(currentConfig, desiredConfig):
			e.plan.logger.Info().
				Str("host", host.String()).
				Msg("Docker daemon config unchanged, skipping")

			return nil
		default:
			e.plan.logger.Info().
				Str("host", host.String()).
				Msg("Docker daemon config changed, updating")

			needsRestart = true
		}
	}

	// Write new config
	if err := docker.WriteDaemonConfig(client, desiredConfig); err != nil {
		return fmt.Errorf("failed to write daemon config on %s: %w", host, err)
	}

	if needsRestart {
		e.plan.logger.Info().
			Str("host", host.String()).
			Msg("Restarting Docker daemon to apply configuration")

		if err := docker.RestartDockerDaemon(client); err != nil {
			return fmt.Errorf("failed to restart docker daemon on %s: %w", host, err)
		}

		e.plan.logger.Info().
			Str("host", host.String()).
			Msg("Waiting for Docker daemon to be ready")

		if err := docker.WaitForDockerReady(client, dockerReadyTimeout); err != nil {
			return fmt.Errorf("docker daemon failed to become ready on %s: %w", host, err)
		}

		e.plan.logger.Info().
			Str("host", host.String()).
			Msg("Docker daemon restarted successfully")
	}

	e.plan.logger.Info().
		Str("host", host.String()).
		Msg("Docker daemon hardening complete")

	return nil
}

// deployAutoUpdates configures automatic security updates on all hosts.
func (e *executor) deployAutoUpdates() error {
	// Process each host's automatic updates configuration
	for _, host := range e.plan.hosts {
		if err := e.deployHostAutoUpdates(host); err != nil {
			return err
		}
	}

	return nil
}

// deployHostAutoUpdates configures automatic security updates for a single host.
// This is always enabled for all hosts - no opt-out.
func (e *executor) deployHostAutoUpdates(host *Host) error {
	// Get SSH client for this host
	client, err := e.sshPool.GetClient(host.Endpoint())
	if err != nil {
		return fmt.Errorf(errFailedSSHClient, host, err)
	}

	e.plan.logger.Info().
		Str("host", host.String()).
		Msg("Configuring automatic security updates")

	// Check if unattended-upgrades is installed
	installed, err := debian.IsUnattendedUpgradesInstalled(client)
	if err != nil {
		return fmt.Errorf("failed to check if unattended-upgrades is installed on %s: %w", host, err)
	}

	if !installed {
		e.plan.logger.Info().
			Str("host", host.String()).
			Msg("unattended-upgrades not installed, installing")

		if err := debian.InstallUnattendedUpgrades(client); err != nil {
			return fmt.Errorf("failed to install unattended-upgrades on %s: %w", host, err)
		}

		e.plan.logger.Info().
			Str("host", host.String()).
			Msg("unattended-upgrades installed successfully")
	}

	// Check if automatic updates are configured
	configured, err := debian.IsAutoUpdatesConfigured(client)
	if err != nil {
		return fmt.Errorf("failed to check auto-updates configuration on %s: %w", host, err)
	}

	if !configured {
		e.plan.logger.Info().
			Str("host", host.String()).
			Msg("Automatic updates not configured, enabling")

		if err := debian.ConfigureAutoUpdates(client); err != nil {
			return fmt.Errorf("failed to configure automatic updates on %s: %w", host, err)
		}

		e.plan.logger.Info().
			Str("host", host.String()).
			Msg("Automatic updates configured successfully")
	} else {
		e.plan.logger.Info().
			Str("host", host.String()).
			Msg("Automatic updates already configured, skipping")
	}

	e.plan.logger.Info().
		Str("host", host.String()).
		Msg("Automatic security updates configuration complete")

	return nil
}

// deployFirewalls configures firewalls on all hosts.
func (e *executor) deployFirewalls() error {
	// Process each host's firewall configuration
	for _, host := range e.plan.hosts {
		if err := e.deployHostFirewall(host); err != nil {
			return err
		}
	}

	return nil
}

// deployHostFirewall configures the firewall for a single host.
func (e *executor) deployHostFirewall(host *Host) error {
	// Skip if no firewall configuration
	if host.firewallConfig == nil || !host.firewallConfig.Enabled {
		return nil
	}

	config := host.firewallConfig

	// Get SSH client for this host
	client, err := e.sshPool.GetClient(host.Endpoint())
	if err != nil {
		return fmt.Errorf(errFailedSSHClient, host, err)
	}

	e.plan.logger.Info().
		Str("host", host.String()).
		Msg("Configuring firewall")

	// Check if ufw is installed
	installed, err := firewall.IsInstalled(client)
	if err != nil {
		return fmt.Errorf("failed to check if ufw is installed on %s: %w", host, err)
	}

	if !installed {
		e.plan.logger.Info().
			Str("host", host.String()).
			Msg("ufw not installed, installing")

		if err := firewall.Install(client); err != nil {
			return fmt.Errorf("failed to install ufw on %s: %w", host, err)
		}

		e.plan.logger.Info().
			Str("host", host.String()).
			Msg("ufw installed successfully")
	}

	// Check current defaults
	currentIncoming, currentOutgoing, err := firewall.GetDefaults(client)
	if err != nil {
		e.plan.logger.Warn().
			Err(err).
			Str("host", host.String()).
			Msg("Could not get current firewall defaults, will set them")

		currentIncoming, currentOutgoing = "", ""
	}

	// Set defaults if they differ
	if currentIncoming != config.DefaultIncoming || currentOutgoing != config.DefaultOutgoing {
		e.plan.logger.Info().
			Str("host", host.String()).
			Str("incoming", config.DefaultIncoming).
			Str("outgoing", config.DefaultOutgoing).
			Msg("Setting firewall defaults")

		if err := firewall.SetDefaults(client, config.DefaultIncoming, config.DefaultOutgoing); err != nil {
			return fmt.Errorf("failed to set firewall defaults on %s: %w", host, err)
		}
	}

	// Get current rules
	currentRules, err := firewall.GetRules(client)
	if err != nil {
		return fmt.Errorf("failed to get current firewall rules on %s: %w", host, err)
	}

	// Convert config rules to firewall.Rule format
	desiredRules := make([]firewall.Rule, len(config.Rules))
	for i, rule := range config.Rules {
		desiredRules[i] = firewall.Rule{
			Port:      rule.Port,
			Protocol:  rule.Protocol,
			Comment:   rule.Comment,
			RateLimit: rule.RateLimit,
		}
	}

	// Sync rules (add/update)
	if err := e.syncFirewallRules(client, host, currentRules, desiredRules); err != nil {
		return err
	}

	// Remove rules not in desired configuration
	for _, current := range currentRules {
		if firewall.FindRule(desiredRules, current.Port, current.Protocol) == nil {
			e.plan.logger.Info().
				Str("host", host.String()).
				Int("port", current.Port).
				Str("protocol", current.Protocol).
				Msg("Removing unwanted firewall rule")

			if err := firewall.RemoveRule(client, current.Port, current.Protocol); err != nil {
				return fmt.Errorf("failed to remove firewall rule %d/%s on %s: %w",
					current.Port, current.Protocol, host, err)
			}
		}
	}

	// Check if firewall is enabled
	enabled, err := firewall.IsEnabled(client)
	if err != nil {
		return fmt.Errorf("failed to check if firewall is enabled on %s: %w", host, err)
	}

	if !enabled {
		e.plan.logger.Info().
			Str("host", host.String()).
			Msg("Enabling firewall")

		if err := firewall.Enable(client); err != nil {
			return fmt.Errorf("failed to enable firewall on %s: %w", host, err)
		}
	}

	e.plan.logger.Info().
		Str("host", host.String()).
		Msg("Firewall configuration complete")

	return nil
}

// syncFirewallRules adds or updates firewall rules to match desired state.
func (e *executor) syncFirewallRules(
	client ssh.Connection,
	host *Host,
	currentRules, desiredRules []firewall.Rule,
) error {
	for _, desired := range desiredRules {
		existing := firewall.FindRule(currentRules, desired.Port, desired.Protocol)

		switch {
		case existing == nil:
			e.plan.logger.Info().
				Str("host", host.String()).
				Int("port", desired.Port).
				Str("protocol", desired.Protocol).
				Str("comment", desired.Comment).
				Bool("rate_limit", desired.RateLimit).
				Msg("Adding firewall rule")

			if err := firewall.AddRule(client, desired); err != nil {
				return fmt.Errorf("failed to add firewall rule %d/%s on %s: %w",
					desired.Port, desired.Protocol, host, err)
			}
		case !firewall.RulesEqual(*existing, desired):
			// Rule exists but differs (e.g., rate limit changed)
			e.plan.logger.Info().
				Str("host", host.String()).
				Int("port", desired.Port).
				Str("protocol", desired.Protocol).
				Msg("Firewall rule changed, recreating")

			// Remove old rule
			if err := firewall.RemoveRule(client, existing.Port, existing.Protocol); err != nil {
				return fmt.Errorf("failed to remove old firewall rule %d/%s on %s: %w",
					existing.Port, existing.Protocol, host, err)
			}

			// Add new rule
			if err := firewall.AddRule(client, desired); err != nil {
				return fmt.Errorf("failed to add firewall rule %d/%s on %s: %w",
					desired.Port, desired.Protocol, host, err)
			}
		default:
			e.plan.logger.Debug().
				Str("host", host.String()).
				Int("port", desired.Port).
				Str("protocol", desired.Protocol).
				Msg("Firewall rule unchanged")
		}
	}

	return nil
}

// loginRegistries logs into Docker registries on all hosts.
func (e *executor) loginRegistries() error {
	// Process each host's registry credentials
	for _, host := range e.plan.hosts {
		if err := e.loginHostRegistries(host); err != nil {
			return err
		}
	}

	return nil
}

// loginHostRegistries logs into registries for a single host.
func (e *executor) loginHostRegistries(host *Host) error {
	// Skip if no registries configured
	if len(host.registries) == 0 {
		return nil
	}

	// Get SSH client for this host
	client, err := e.sshPool.GetClient(host.Endpoint())
	if err != nil {
		return fmt.Errorf(errFailedSSHClient, host, err)
	}

	// Login to each registry
	for _, registry := range host.registries {
		e.plan.logger.Info().
			Str("host", host.String()).
			Str("registry", registry.Registry).
			Str("username", registry.Username).
			Msg("Logging into registry")

		if err := e.dockerExec.RegistryLogin(client, registry.Registry, registry.Username, registry.Password); err != nil {
			return fmt.Errorf("failed to login to registry %s on %s: %w", registry.Registry, host, err)
		}

		e.plan.logger.Info().
			Str("host", host.String()).
			Str("registry", registry.Registry).
			Msg("Registry login successful")
	}

	return nil
}
