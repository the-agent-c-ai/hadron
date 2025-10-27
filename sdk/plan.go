package sdk

import (
	"context"
	"errors"

	"github.com/rs/zerolog"
)

var (
	errDryRunNotImplemented  = errors.New("dry run not yet implemented")
	errDestroyNotImplemented = errors.New("destroy not yet implemented")
)

// Plan represents a deployment plan containing hosts and resources.
type Plan struct {
	name       string
	hosts      []*Host
	networks   []*Network
	volumes    []*Volume
	containers []*Container
	logger     zerolog.Logger
}

// NewPlan creates a new deployment plan with the given name.
func NewPlan(name string) *Plan {
	return &Plan{
		name:       name,
		hosts:      make([]*Host, 0),
		networks:   make([]*Network, 0),
		volumes:    make([]*Volume, 0),
		containers: make([]*Container, 0),
	}
}

// WithLogger sets the logger for the plan.
func (p *Plan) WithLogger(logger zerolog.Logger) *Plan {
	p.logger = logger

	return p
}

// Host creates a new host builder.
// The endpoint can be an IP address, hostname, or SSH config alias.
func (p *Plan) Host(endpoint string) *HostBuilder {
	return &HostBuilder{
		plan:     p,
		endpoint: endpoint,
	}
}

// Network creates a new network builder.
func (p *Plan) Network(name string) *NetworkBuilder {
	return &NetworkBuilder{
		plan:   p,
		name:   name,
		driver: "bridge", // default driver
	}
}

// Volume creates a new volume builder.
func (p *Plan) Volume(name string) *VolumeBuilder {
	return &VolumeBuilder{
		plan: p,
		name: name,
	}
}

// Container creates a new container builder.
func (p *Plan) Container(name string) *ContainerBuilder {
	return &ContainerBuilder{
		plan:         p,
		name:         name,
		ports:        make([]string, 0),
		volumes:      make([]VolumeMount, 0),
		tmpfs:        make(map[string]string),
		envVars:      make(map[string]string),
		labels:       make(map[string]string),
		securityOpts: make([]string, 0),
		capDrop:      make([]string, 0),
		capAdd:       make([]string, 0),
	}
}

// Execute executes the plan by deploying all resources to their respective hosts.
// Execute runs the plan with the given context.
func (p *Plan) Execute(ctx context.Context) error {
	exec := newExecutor(p)

	return exec.execute(ctx)
}

// DryRun shows what would be deployed without actually deploying.
func (p *Plan) DryRun() error {
	p.logger.Info().Str("plan", p.name).Msg("Dry run - showing planned changes")

	// TODO: Implement dry run logic
	// Show what docker commands would be executed

	return errDryRunNotImplemented
}

// Destroy removes all resources defined in the plan.
func (p *Plan) Destroy() error {
	p.logger.Info().Str("plan", p.name).Msg("Destroying resources")

	// TODO: Implement destroy logic
	// Remove in reverse order: Containers -> Volumes -> Networks

	return errDestroyNotImplemented
}
