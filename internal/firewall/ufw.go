// Package firewall provides firewall management via ufw.
package firewall

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/the-agent-c-ai/hadron/internal/debian"
	"github.com/the-agent-c-ai/hadron/sdk/ssh"
)

// Rule represents a single firewall rule.
type Rule struct {
	Port      int
	Protocol  string // "tcp" or "udp"
	Comment   string
	RateLimit bool
}

// Config represents the complete firewall configuration.
type Config struct {
	DefaultIncoming string // "deny" or "allow"
	DefaultOutgoing string // "deny" or "allow"
	Rules           []Rule
}

// IsInstalled checks if ufw is installed on the remote host.
func IsInstalled(client ssh.Connection) (bool, error) {
	_, _, err := client.Execute("which ufw")
	if err != nil {
		// which returns non-zero if not found
		return false, nil //nolint:nilerr // exit code used for logic, not error indication
	}

	return true, nil
}

// IsEnabled checks if ufw is currently enabled.
func IsEnabled(client ssh.Connection) (bool, error) {
	stdout, _, err := client.Execute("sudo ufw status")
	if err != nil {
		return false, fmt.Errorf("failed to check ufw status: %w", err)
	}

	return strings.Contains(stdout, "Status: active"), nil
}

// GetRules retrieves the current firewall rules.
func GetRules(client ssh.Connection) ([]Rule, error) {
	stdout, _, err := client.Execute("sudo ufw status numbered")
	if err != nil {
		return nil, fmt.Errorf("failed to get ufw rules: %w", err)
	}

	var rules []Rule

	lines := strings.Split(stdout, "\n")

	// Example line: [ 1] 22/tcp                     ALLOW IN    Anywhere                   # SSH
	// Example with LIMIT: [ 2] 22/tcp                     LIMIT IN    Anywhere
	const (
		portMatchIndex     = 1
		protocolMatchIndex = 2
		actionMatchIndex   = 3
	)

	ruleRegex := regexp.MustCompile(`\[\s*\d+\]\s+(\d+)/(tcp|udp)\s+(ALLOW|LIMIT)\s+IN`)
	commentRegex := regexp.MustCompile(`#\s*(.+)$`)

	for _, line := range lines {
		if matches := ruleRegex.FindStringSubmatch(line); matches != nil {
			port, _ := strconv.Atoi(matches[portMatchIndex])
			protocol := matches[protocolMatchIndex]
			action := matches[actionMatchIndex]

			rule := Rule{
				Port:      port,
				Protocol:  protocol,
				RateLimit: action == "LIMIT",
			}

			// Extract comment if present
			if commentMatches := commentRegex.FindStringSubmatch(line); commentMatches != nil {
				rule.Comment = strings.TrimSpace(commentMatches[1])
			}

			rules = append(rules, rule)
		}
	}

	return rules, nil
}

// GetDefaults retrieves the current default policies.
func GetDefaults(client ssh.Connection) (incoming, outgoing string, err error) {
	stdout, _, err := client.Execute("sudo ufw status verbose")
	if err != nil {
		return "", "", fmt.Errorf("failed to get ufw defaults: %w", err)
	}

	// Example output:
	// Default: deny (incoming), allow (outgoing), disabled (routed)
	const (
		incomingMatchIndex = 1
		outgoingMatchIndex = 2
	)

	defaultRegex := regexp.MustCompile(`Default:\s+(\w+)\s+\(incoming\),\s+(\w+)\s+\(outgoing\)`)
	if matches := defaultRegex.FindStringSubmatch(stdout); matches != nil {
		return matches[incomingMatchIndex], matches[outgoingMatchIndex], nil
	}

	return "", "", ErrParseDefaults
}

// Install installs ufw on the remote host using debian package manager.
func Install(client ssh.Connection) error {
	if err := debian.EnsureInstalled(client, "ufw"); err != nil {
		return fmt.Errorf("failed to install ufw: %w", err)
	}

	return nil
}

// SetDefaults sets the default policies for incoming and outgoing traffic.
func SetDefaults(client ssh.Connection, incoming, outgoing string) error {
	// Set default incoming
	cmd := fmt.Sprintf("sudo ufw default %s incoming", incoming)
	if _, stderr, err := client.Execute(cmd); err != nil {
		return fmt.Errorf("failed to set default incoming: %w (stderr: %s)", err, stderr)
	}

	// Set default outgoing
	cmd = fmt.Sprintf("sudo ufw default %s outgoing", outgoing)
	if _, stderr, err := client.Execute(cmd); err != nil {
		return fmt.Errorf("failed to set default outgoing: %w (stderr: %s)", err, stderr)
	}

	return nil
}

// AddRule adds a firewall rule.
func AddRule(client ssh.Connection, rule Rule) error {
	var cmd string
	if rule.RateLimit {
		cmd = fmt.Sprintf("sudo ufw limit %d/%s", rule.Port, rule.Protocol)
	} else {
		cmd = fmt.Sprintf("sudo ufw allow %d/%s", rule.Port, rule.Protocol)
	}

	if rule.Comment != "" {
		cmd += fmt.Sprintf(" comment '%s'", rule.Comment)
	}

	_, stderr, err := client.Execute(cmd)
	if err != nil {
		return fmt.Errorf("failed to add rule %d/%s: %w (stderr: %s)", rule.Port, rule.Protocol, err, stderr)
	}

	return nil
}

// RemoveRule removes a firewall rule by port and protocol.
func RemoveRule(client ssh.Connection, port int, protocol string) error {
	cmd := fmt.Sprintf("sudo ufw delete allow %d/%s", port, protocol)

	_, stderr, err := client.Execute(cmd)
	if err != nil {
		return fmt.Errorf("failed to remove rule %d/%s: %w (stderr: %s)", port, protocol, err, stderr)
	}

	return nil
}

// Enable enables the firewall (non-interactively).
func Enable(client ssh.Connection) error {
	// Use --force to avoid interactive prompt
	cmd := "sudo ufw --force enable"

	_, stderr, err := client.Execute(cmd)
	if err != nil {
		return fmt.Errorf("failed to enable ufw: %w (stderr: %s)", err, stderr)
	}

	return nil
}

// RulesEqual checks if two rules are equivalent (ignoring comment differences).
func RulesEqual(r1, r2 Rule) bool {
	return r1.Port == r2.Port &&
		r1.Protocol == r2.Protocol &&
		r1.RateLimit == r2.RateLimit
}

// FindRule finds a rule in a slice by port and protocol.
func FindRule(rules []Rule, port int, protocol string) *Rule {
	for _, rule := range rules {
		if rule.Port == port && rule.Protocol == protocol {
			return &rule
		}
	}

	return nil
}
