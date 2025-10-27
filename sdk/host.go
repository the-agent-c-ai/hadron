package sdk

// RegistryCredential represents credentials for a Docker registry.
type RegistryCredential struct {
	Registry string
	Username string
	Password string
}

// FirewallRule represents a single firewall rule.
type FirewallRule struct {
	Port      int
	Protocol  string // "tcp" or "udp"
	Comment   string
	RateLimit bool
}

// FirewallConfig represents firewall configuration for a host.
type FirewallConfig struct {
	Enabled         bool
	DefaultIncoming string // "deny" or "allow"
	DefaultOutgoing string // "deny" or "allow"
	Rules           []FirewallRule
}

// Host represents a remote Docker host accessible via SSH.
type Host struct {
	endpoint       string
	packages       []string
	removePackages []string
	registries     []RegistryCredential
	firewallConfig *FirewallConfig
	hardenDocker   bool
	hardenOS       bool
	hardenSSH      bool
	sshFingerprint string
	sshKeyContent  string
	plan           *Plan
}

// HostBuilder builds a Host with a fluent API.
type HostBuilder struct {
	plan           *Plan
	endpoint       string
	packages       []string
	removePackages []string
	registries     []RegistryCredential
	firewallConfig *FirewallConfig
	hardenDocker   bool
	hardenOS       bool
	hardenSSH      bool
	sshFingerprint string
	sshKeyContent  string
}

// FirewallBuilder builds firewall configuration with a fluent API.
type FirewallBuilder struct {
	host   *HostBuilder
	config *FirewallConfig
}

// Package adds a Debian package to be installed on this host.
func (hb *HostBuilder) Package(name string) *HostBuilder {
	hb.packages = append(hb.packages, name)

	return hb
}

// RemovePackage adds a Debian package to be removed from this host.
func (hb *HostBuilder) RemovePackage(name string) *HostBuilder {
	hb.removePackages = append(hb.removePackages, name)

	return hb
}

// Registry adds Docker registry credentials for this host.
func (hb *HostBuilder) Registry(registry, username, password string) *HostBuilder {
	hb.registries = append(hb.registries, RegistryCredential{
		Registry: registry,
		Username: username,
		Password: password,
	})

	return hb
}

// HardenDocker enables Docker daemon security hardening.
// Applies recommended security settings from deploy-security.md:
// - live-restore: true (containers survive daemon restarts)
// - userland-proxy: false (better performance, uses iptables)
// - no-new-privileges: true (prevents privilege escalation)
// - icc: false (containers can't talk unless explicitly networked)
// - log-driver limits (prevents disk exhaustion).
func (hb *HostBuilder) HardenDocker() *HostBuilder {
	hb.hardenDocker = true

	return hb
}

// HardenOS enables OS-level security hardening via sysctl.
// Applies balanced kernel parameter tuning that:
// - Enables SYN flood protection (tcp_syncookies)
// - Disables ICMP redirects (prevents MITM)
// - Restricts kernel information disclosure (dmesg, kptr)
// - Enables process isolation (ptrace_scope)
// - Protects against symlink/hardlink attacks
// - Optimizes network stack for security
//
// Configuration is Docker-compatible and doesn't break normal operations.
// Settings are written to /etc/sysctl.d/99-hadron-security.conf.
func (hb *HostBuilder) HardenOS() *HostBuilder {
	hb.hardenOS = true

	return hb
}

// HardenSSH enables SSH daemon security hardening.
// Applies hardened SSH configuration:
// - Ed25519 host keys only (disables RSA/ECDSA/DSA)
// - Public key authentication only (no passwords)
// - Modern cryptography (Curve25519, ChaCha20, AES-GCM)
// - Disables X11/agent/TCP forwarding
// - Connection timeouts and rate limiting
// - Verbose logging for auditing
//
// Configuration is written to /etc/ssh/sshd_config (original backed up).
// SSH daemon is reloaded (not restarted) to preserve current connections.
func (hb *HostBuilder) HardenSSH() *HostBuilder {
	hb.hardenSSH = true

	return hb
}

// Fingerprint sets the expected SSH host key fingerprint for verification.
// When set, Hadron will verify the host key matches this fingerprint instead of using ~/.ssh/known_hosts.
// This is useful for automated deployments where pre-populating known_hosts is not practical.
//
// Supported formats:
// - SHA256: "SHA256:abc123..." (recommended, obtained via: ssh-keyscan -t ed25519 host | ssh-keygen -lf -)
// - MD5: "MD5:ab:cd:ef:..." (legacy, not recommended)
//
// Example:
//
//	host := plan.Host("user@example.com").
//	    Fingerprint("SHA256:nThbg6kXUpJWGl7E1IGOCspRomTxdCARLviKw6E5SY8").
//	    Build()
func (hb *HostBuilder) Fingerprint(fingerprint string) *HostBuilder {
	hb.sshFingerprint = fingerprint

	return hb
}

// SSHKey sets the SSH private key content for authentication.
// When set, Hadron will use this key instead of SSH agent for authentication.
// The key should be in OpenSSH format (PEM).
//
// Note: Only unencrypted keys are supported. For passphrase-protected keys, use SSH agent instead.
//
// Example:
//
//	keyContent := "-----BEGIN OPENSSH PRIVATE KEY-----\n...\n-----END OPENSSH PRIVATE KEY-----"
//	host := plan.Host("user@example.com").
//	    SSHKey(keyContent).
//	    Build()
func (hb *HostBuilder) SSHKey(keyContent string) *HostBuilder {
	hb.sshKeyContent = keyContent

	return hb
}

const (
	// Standard service ports.
	portSSH   = 22
	portHTTP  = 80
	portHTTPS = 443

	// Protocol constants.
	protocolTCP = "tcp"
	protocolUDP = "udp"
)

// Firewall starts firewall configuration with defaults.
// Default: deny incoming, allow outgoing, SSH (22) allowed with rate limiting.
func (hb *HostBuilder) Firewall() *FirewallBuilder {
	config := &FirewallConfig{
		Enabled:         true,
		DefaultIncoming: "deny",
		DefaultOutgoing: "allow",
		Rules: []FirewallRule{
			{Port: portSSH, Protocol: protocolTCP, Comment: "SSH", RateLimit: true},
			{Port: portHTTP, Protocol: protocolTCP, Comment: "HTTP"},
			{Port: portHTTPS, Protocol: protocolTCP, Comment: "HTTPS"},
		},
	}

	return &FirewallBuilder{
		host:   hb,
		config: config,
	}
}

// Allow adds a firewall rule to allow a port.
func (fb *FirewallBuilder) Allow(port int, protocol string) *FirewallRuleBuilder {
	return &FirewallRuleBuilder{
		firewall: fb,
		rule: FirewallRule{
			Port:     port,
			Protocol: protocol,
		},
	}
}

// DefaultIncoming sets the default policy for incoming traffic.
func (fb *FirewallBuilder) DefaultIncoming(policy string) *FirewallBuilder {
	fb.config.DefaultIncoming = policy

	return fb
}

// DefaultOutgoing sets the default policy for outgoing traffic.
func (fb *FirewallBuilder) DefaultOutgoing(policy string) *FirewallBuilder {
	fb.config.DefaultOutgoing = policy

	return fb
}

// ClearDefaultRules removes the default SSH/HTTP/HTTPS rules.
// Useful if you want full control over rules.
func (fb *FirewallBuilder) ClearDefaultRules() *FirewallBuilder {
	fb.config.Rules = []FirewallRule{}

	return fb
}

// Done finalizes firewall configuration and returns to host builder.
func (fb *FirewallBuilder) Done() *HostBuilder {
	// Ensure SSH is always allowed (safety check)
	hasSSH := false

	for _, rule := range fb.config.Rules {
		if rule.Port == portSSH && rule.Protocol == protocolTCP {
			hasSSH = true

			break
		}
	}

	if !hasSSH {
		fb.host.plan.logger.Warn().Msg("SSH (port 22) not in firewall rules, adding with rate limiting for safety")
		fb.config.Rules = append([]FirewallRule{
			{Port: portSSH, Protocol: protocolTCP, Comment: "SSH (auto-added)", RateLimit: true},
		}, fb.config.Rules...)
	}

	fb.host.firewallConfig = fb.config

	return fb.host
}

// FirewallRuleBuilder builds a single firewall rule.
type FirewallRuleBuilder struct {
	firewall *FirewallBuilder
	rule     FirewallRule
}

// Comment sets a comment for the rule.
func (frb *FirewallRuleBuilder) Comment(comment string) *FirewallRuleBuilder {
	frb.rule.Comment = comment

	return frb
}

// RateLimit enables rate limiting for this rule (useful for SSH).
func (frb *FirewallRuleBuilder) RateLimit() *FirewallRuleBuilder {
	frb.rule.RateLimit = true

	return frb
}

// Done finalizes this rule and returns to firewall builder.
func (frb *FirewallRuleBuilder) Done() *FirewallBuilder {
	frb.firewall.config.Rules = append(frb.firewall.config.Rules, frb.rule)

	return frb.firewall
}

// Build creates the Host and registers it with the plan.
func (hb *HostBuilder) Build() *Host {
	if hb.endpoint == "" {
		hb.plan.logger.Fatal().Msg("host endpoint is required")
	}

	host := &Host{
		endpoint:       hb.endpoint,
		packages:       hb.packages,
		removePackages: hb.removePackages,
		registries:     hb.registries,
		firewallConfig: hb.firewallConfig,
		hardenDocker:   hb.hardenDocker,
		hardenOS:       hb.hardenOS,
		hardenSSH:      hb.hardenSSH,
		sshFingerprint: hb.sshFingerprint,
		sshKeyContent:  hb.sshKeyContent,
		plan:           hb.plan,
	}

	hb.plan.hosts = append(hb.plan.hosts, host)

	return host
}

// Endpoint returns the SSH endpoint (IP, hostname, or SSH config alias).
func (h *Host) Endpoint() string {
	return h.endpoint
}

// SSHFingerprint returns the configured SSH host key fingerprint, or empty string if not set.
func (h *Host) SSHFingerprint() string {
	return h.sshFingerprint
}

// SSHKeyContent returns the configured SSH private key content, or empty string if not set.
func (h *Host) SSHKeyContent() string {
	return h.sshKeyContent
}

// String returns a string representation of the host.
func (h *Host) String() string {
	return h.endpoint
}
