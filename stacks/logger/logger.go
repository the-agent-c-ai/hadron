// Package logger provides Vector log collection and forwarding infrastructure.
package logger

import (
	_ "embed"
	"time"

	"github.com/the-agent-c-ai/hadron/sdk"
)

//go:embed agent.yaml
var agentYAML string

// Config contains configuration for the Vector logging stack.
type Config struct {
	Image        string
	LogLevel     string
	Environment  string
	LokiEndpoint string
	LokiUsername string
	LokiPassword string
}

// Logger deploys a Vector agent container for log collection and forwarding.
// It creates an isolated network, volume for buffering, and configures Vector
// to collect Docker container logs and forward them to Loki.
func Logger(plan *sdk.Plan, host *sdk.Host, cnf *Config) {
	// vector: Isolated observability network for Vector (outbound-only + docker socket)
	// Segregated to limit lateral movement from compromised observability components
	vectorNet := plan.Network("vector").
		Host(host).
		Driver("bridge").
		Build()

	// Vector agent data directory (for buffering to aggregator)
	vectorAgentData := plan.Volume("vector-agent-data").
		Host(host).
		Build()

	// Vector Agent
	// Collects Docker container logs and forwards to local aggregator
	// Isolated on vector network - only docker socket access and outbound to aggregator
	// Requires: access to Docker socket (read-only)
	// Exposes: 8686 (health/API), 9598 (metrics)
	// Image: Synced to GHCR via sync.go for production stability
	plan.Container("vector").
		Host(host).
		Image(cnf.Image).
		Network(vectorNet).
		Volume("/var/run/docker.sock", "/var/run/docker.sock", "ro"). // Docker socket (read-only)
		Volume(vectorAgentData, "/var/lib/vector").                   // Writable: buffering, state
		MountData([]byte(agentYAML), "/etc/vector/vector.yaml", "ro").
		Env("LOKI_ENDPOINT", cnf.LokiEndpoint).
		Env("LOKI_USERNAME", cnf.LokiUsername).
		Env("LOKI_PASSWORD", cnf.LokiPassword).
		Env("ENVIRONMENT", cnf.Environment).
		Env("VECTOR_LOG", cnf.LogLevel).
		Restart("unless-stopped").
		ReadOnly().
		CapDrop("ALL").
		SecurityOpt("no-new-privileges").
		Memory("1024m").
		MemoryReservation("512m").
		CPUShares(1024).
		HealthCheck(sdk.HTTPCheck("/health", 8686).
			WithTimeout(30 * time.Second).
			WithInterval(30 * time.Second).
			WithRetries(3)).
		Build()
}
