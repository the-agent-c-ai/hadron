package logger

import (
	_ "embed"
	"time"

	"github.com/the-agent-c-ai/hadron/sdk"
)

//go:embed agent.yaml
var agentYAML string

type Config struct {
	VectorImage  string
	LokiEndpoint string
	LokiUsername string
	LokiPassword string
}

func Logger(plan *sdk.Plan, strawHost *sdk.Host, cnf *Config) {
	// vector: Isolated observability network for Vector (outbound-only + docker socket)
	// Segregated to limit lateral movement from compromised observability components
	vectorNet := plan.Network("vector").
		Host(strawHost).
		Driver("bridge").
		Build()

	// Vector aggregator data directory (for buffering to Loki)
	vectorAggData := plan.Volume("vector-aggregator-data").
		Host(strawHost).
		Build()

	// Vector Aggregator
	// Receives logs from local agent and forwards to Grafana Cloud Loki
	// Isolated on vector network - only outbound communication to Loki
	// Exposes: 9000 (vector protocol, local only), 8686 (health/API), 9598 (metrics)
	// Image: Synced to GHCR via sync.go for production stability
	vectorAgg := plan.Container("vector-aggregator").
		Host(strawHost).
		Image(cnf.VectorImage).
		Network(vectorNet).
		NetworkAlias("vector-aggregator").                                  // For agent to connect
		Volume(vectorAggData, "/var/lib/vector").                           // Writable: buffering, state
		MountData([]byte(aggregatorYAML), "/etc/vector/vector.yaml", "ro"). // Mount config
		Env("LOKI_ENDPOINT", cnf.LokiEndpoint).
		Env("LOKI_USERNAME", cnf.LokiUsername).
		Env("LOKI_PASSWORD", cnf.LokiPassword).
		Env("VECTOR_LOG", "info").
		Env("ENVIRONMENT", "production").
		Restart("unless-stopped").
		ReadOnly(). // Read-only root filesystem
		CapDrop("ALL").
		SecurityOpt("no-new-privileges").
		HealthCheck(sdk.HTTPCheck("/health", 8686).
			WithTimeout(30 * time.Second).
			WithInterval(30 * time.Second).
			WithRetries(3)).
		Build()

	// Vector agent data directory (for buffering to aggregator)
	vectorAgentData := plan.Volume("vector-agent-data").
		Host(strawHost).
		Build()

	// Vector Agent
	// Collects Docker container logs and forwards to local aggregator
	// Isolated on vector network - only docker socket access and outbound to aggregator
	// Requires: access to Docker socket (read-only)
	// Exposes: 8686 (health/API), 9598 (metrics)
	// Image: Synced to GHCR via sync.go for production stability
	plan.Container("vector-agent").
		Host(strawHost).
		Image(cnf.VectorImage).
		Network(vectorNet).
		DependsOn(vectorAgg).                                         // Wait for aggregator to be healthy
		Volume("/var/run/docker.sock", "/var/run/docker.sock", "ro"). // Docker socket (read-only)
		Volume(vectorAgentData, "/var/lib/vector").                   // Writable: buffering, state
		MountData([]byte(agentYAML), "/etc/vector/vector.yaml", "ro").
		Env("VECTOR_LOG", "info").
		Env("ENVIRONMENT", "production").
		Env("AGGREGATOR_ENDPOINT", "vector-aggregator:9000").
		Restart("unless-stopped").
		ReadOnly(). // Read-only root filesystem
		CapDrop("ALL").
		SecurityOpt("no-new-privileges").
		HealthCheck(sdk.HTTPCheck("/health", 8686).
			WithTimeout(30 * time.Second).
			WithInterval(30 * time.Second).
			WithRetries(3)).
		Build()

}
