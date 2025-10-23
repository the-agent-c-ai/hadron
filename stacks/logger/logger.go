// Package logger provides Vector log collection and forwarding infrastructure.
package logger

import (
	_ "embed"
	"time"

	"github.com/the-agent-c-ai/hadron/sdk"
)

const (
	// Webhook to select the webhook variant.
	Webhook = "webhook"
	// Host to select the host variant.
	Host = "host"
)

//go:embed agent.yaml
var agentYAML string

//go:embed webhooks.yaml
var hooksYAML string

// Config contains configuration for the Vector logging stack.
type Config struct {
	Image         string
	LogLevel      string
	Environment   string
	LokiEndpoint  string
	LokiUsername  string
	LokiPassword  string
	Mode          string
	WebhookSecret string
}

// Logger deploys a Vector agent container for log collection and forwarding.
// It creates an isolated network, volume for buffering, and configures Vector
// to collect Docker container logs and forward them to Loki.
func Logger(plan *sdk.Plan, host *sdk.Host, vectorNet *sdk.Network, cnf *Config) *sdk.Container {
	alias := "vector-webhooks"
	config := []byte(hooksYAML)

	if cnf.Mode != Webhook {
		alias = "vector-host"
		config = []byte(agentYAML)
		// vector: Isolated observability network for Vector (outbound-only + docker socket)
		// Segregated to limit lateral movement from compromised observability components
		vectorNet = plan.Network("vector-isolated").
			Host(host).
			Driver("bridge").
			Build()
	}

	// Vector agent data directory (for buffering to aggregator)
	vectorAgentData := plan.Volume(alias).
		Host(host).
		Build()

	// Vector Agent
	// Collects Docker container logs and forwards to local aggregator
	// Isolated on vector network - only docker socket access and outbound to aggregator
	// Requires: access to Docker socket (read-only)
	// Exposes: 8686 (health/API), 9598 (metrics)
	// Image: Synced to GHCR via sync.go for production stability
	con := plan.Container(alias).
		Host(host).
		Image(cnf.Image).
		Network(vectorNet).
		NetworkAlias(alias).
		Volume(vectorAgentData, "/var/lib/vector"). // Writable: buffering, state
		MountData(config, "/etc/vector/vector.yaml", "ro").
		Env("LOKI_ENDPOINT", cnf.LokiEndpoint).
		Env("LOKI_USERNAME", cnf.LokiUsername).
		Env("LOKI_PASSWORD", cnf.LokiPassword).
		Env("ENVIRONMENT", cnf.Environment).
		Env("VECTOR_LOG", cnf.LogLevel).
		Env("GENERIC_WEBHOOK_SECRET", cnf.WebhookSecret).
		Env("AUTH0_WEBHOOK_SECRET", cnf.WebhookSecret).
		Restart("unless-stopped").
		ReadOnly().
		CapDrop("ALL").
		SecurityOpt("no-new-privileges").
		Memory("1024m").
		MemoryReservation("256m").
		CPUShares(512).
		HealthCheck(sdk.HTTPCheck("/health", 8686).
			WithTimeout(30 * time.Second).
			WithInterval(30 * time.Second).
			WithRetries(3))

	if cnf.Mode != Webhook {
		con = con.Volume("/var/run/docker.sock", "/var/run/docker.sock", "ro"). // Docker socket (read-only)
											Volume("/var/log/ufw.log", "/host/ufw.log", "ro").
			// Docker socket (read-only)
			Volume("/var/log/auth.log", "/host/auth.log", "ro")
		// Docker socket (read-only)
	}

	return con.Build()
}
