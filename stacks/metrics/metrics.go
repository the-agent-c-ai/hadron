// Package metrics provides Grafana Alloy metrics collection infrastructure.
package metrics

import (
	_ "embed"
	"time"

	"github.com/the-agent-c-ai/hadron/sdk"
)

//go:embed alloy.alloy
var alloyConfig string

const (
	// Maximum number of PIDs allowed in the Alloy container.
	maxPIDs = 50
)

// Config contains configuration for the Grafana Alloy metrics collector.
type Config struct {
	Image                 string         // GHCR image with digest
	Networks              []*sdk.Network // Networks to join for scraping targets
	Environment           string         // Environment label (production, staging, etc.)
	Instance              string
	LogLevel              string // Log level (debug, info, warn, error)
	PrometheusEndpoint    string // Grafana Cloud Prometheus URL
	PrometheusUsername    string // Grafana Cloud username
	PrometheusPassword    string // Grafana Cloud API token
	PrometheusBearerToken string // Bearer token for authenticated scrape targets (optional)
}

// Metrics deploys a Grafana Alloy container for collecting Prometheus metrics
// from Black containers (Caddy, Vector) and forwarding to Grafana Cloud.
//
// The Alloy container connects to all networks specified in Config.Networks
// to enable scraping metrics from targets on different network segments.
func Metrics(
	plan *sdk.Plan,
	host *sdk.Host,
	cnf *Config,
) *sdk.Container {
	// Alloy data (persistent storage for metrics buffers)
	alloyData := plan.Volume("alloy-data").
		Host(host).
		Build()

	builder := plan.Container("alloy").
		Host(host).
		Image(cnf.Image)

	// Connect to all specified networks
	for _, network := range cnf.Networks {
		builder = builder.Network(network)
	}

	builder = builder.
		Volume(alloyData, "/var/lib/alloy/data"). // Persistent storage
		Volume("/var/run/docker.sock", "/var/run/docker.sock", "ro").
		// Docker socket (read-only for service discovery)
		ExtraHosts("host.docker.internal:host-gateway").                 // Access host services (e.g., node_exporter)
		MountData([]byte(alloyConfig), "/etc/alloy/config.alloy", "ro"). // Config (read-only)
		Env("ENVIRONMENT", cnf.Environment).
		Env("INSTANCE", cnf.Instance).
		Env("LOG_LEVEL", cnf.LogLevel).
		Env("PROMETHEUS_ENDPOINT", cnf.PrometheusEndpoint).
		Env("PROMETHEUS_USERNAME", cnf.PrometheusUsername).
		Env("PROMETHEUS_PASSWORD", cnf.PrometheusPassword).
		Env("PROMETHEUS_BEARER_TOKEN", cnf.PrometheusBearerToken). // Bearer token for authenticated targets
		Restart("unless-stopped").
		User("473").
		// Should be $(stat -c '%g' /var/run/docker.sock) - also, ignore the warning and see this clusterfuck:
		// https://stackoverflow.com/questions/64361427/docker-unable-to-find-group-docker-using-group-add-docker
		GroupAdd("900").
		ReadOnly().
		CapDrop("ALL").
		SecurityOpt("no-new-privileges").
		Memory("512m").
		MemoryReservation("256m").
		CPUShares(256).
		CPUs("0.25").
		PIDsLimit(maxPIDs)

	// Conditionally configure admin API
	if cnf.LogLevel == "debug" {
		// Enable admin API on all interfaces and expose port
		builder = builder.
			Command(
				"run",
				"/etc/alloy/config.alloy",
				"--storage.path=/var/lib/alloy/data",
				"--server.http.listen-addr=0.0.0.0:12345",
			).
			Port("12345:12345")
	} else {
		// Enable admin API on localhost only (for health checks, not externally accessible)
		builder = builder.
			Command("run", "/etc/alloy/config.alloy", "--storage.path=/var/lib/alloy/data", "--server.http.listen-addr=127.0.0.1:12345")
	}

	builder = builder.HealthCheck(sdk.HTTPCheck("/", 12345).
		WithTimeout(10 * time.Second).
		WithInterval(30 * time.Second).
		WithRetries(3))

	return builder.Build()
}
