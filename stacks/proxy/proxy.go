// Package proxy provides Caddy reverse proxy infrastructure with automatic HTTPS.
package proxy

import (
	_ "embed"
	"time"

	"github.com/the-agent-c-ai/hadron/sdk"
)

//go:embed Caddyfile
var caddyfile string

// Config contains configuration for the Caddy reverse proxy.
type Config struct {
	Image         string
	LogLevel      string
	Static        string
	Email         string
	Domain        string
	ReversePort   string
	ReverseHealth string
}

// Proxy deploys a Caddy reverse proxy container with automatic HTTPS.
// It creates volumes for TLS certificates and runtime config, and configures
// Caddy to reverse proxy to the specified dependency container.
func Proxy(plan *sdk.Plan, depends *sdk.Container, network *sdk.Network, host *sdk.Host, cnf *Config) {
	// Caddy data (TLS certificates from Let's Encrypt)
	caddyData := plan.Volume("caddy-data").
		Host(host).
		Build()

	// Caddy config (runtime configuration)
	caddyConfig := plan.Volume("caddy-config").
		Host(host).
		Build()

	plan.Container("caddy").
		Host(host).
		Image(cnf.Image).
		Network(network).
		Volume(caddyData, "/data").                                 // Writable: TLS certificates
		Volume(caddyConfig, "/config").                             // Writable: runtime config
		MountData([]byte(caddyfile), "/etc/caddy/Caddyfile", "ro"). // Config (read-only)
		Mount(cnf.Static, "/etc/caddy/static", "ro").               // Favicon and images (read-only)
		Env("LOG_LEVEL", cnf.LogLevel).
		Env("DOMAIN", cnf.Domain).
		Env("EMAIL", cnf.Email).
		Env("REVERSE", depends.NetworkAlias()).
		Env("REVERSE_PORT", "3002").
		Env("REVERSE_HEALTH", "/ping").
		Restart("unless-stopped").
		ReadOnly().
		CapDrop("ALL").
		CapAdd("NET_BIND_SERVICE"). // Required to bind to ports 80/443
		SecurityOpt("no-new-privileges").
		DependsOn(depends). // Wait for SCIM to be healthy
		Port("80:80").      // HTTP (redirects to HTTPS)
		Port("443:443").    // HTTPS
		Memory("256m").
		MemoryReservation("128m").
		CPUShares(512).
		HealthCheck(sdk.TCPCheck(443).
			WithTimeout(30 * time.Second).
			WithInterval(30 * time.Second).
			WithRetries(3)).
		Build()
}
