# Firewall Management via UFW

This internal package provides UFW (Uncomplicated Firewall) management operations via SSH for Hadron deployments.

## Purpose

Manages Linux firewall rules on remote hosts using UFW:
- **Installation check**: Verify UFW is installed
- **Rule management**: Add, remove, and query firewall rules
- **Default policies**: Configure default incoming/outgoing traffic policies
- **Status management**: Enable/disable firewall, check current state

## Public API

### Status Operations
- `IsInstalled(client)` - Check if UFW is installed on remote host
- `IsEnabled(client)` - Check if UFW is currently active
- `Enable(client)` - Enable UFW (non-interactive with --force flag)

### Configuration Operations
- `GetDefaults(client)` - Retrieve current default incoming/outgoing policies
- `SetDefaults(client, incoming, outgoing)` - Set default policies ("allow", "deny", "reject")
- `GetRules(client)` - Retrieve all active firewall rules

### Rule Operations
- `AddRule(client, rule)` - Add firewall rule (ALLOW or LIMIT for rate limiting)
- `RemoveRule(client, port, protocol)` - Remove firewall rule by port/protocol

### Installation
- `Install(client)` - Install UFW via apt-get (Debian/Ubuntu only)

### Utility Functions
- `RulesEqual(r1, r2)` - Compare rules for equivalence (ignoring comments)
- `FindRule(rules, port, protocol)` - Find rule in slice by port/protocol

## Rule Structure

```go
type Rule struct {
    Port      int    // Port number (1-65535)
    Protocol  string // "tcp" or "udp"
    Comment   string // Optional comment for rule
    RateLimit bool   // If true, uses LIMIT instead of ALLOW (connection rate limiting)
}
```

## Config Structure

```go
type Config struct {
    DefaultIncoming string // "deny", "allow", or "reject"
    DefaultOutgoing string // "deny", "allow", or "reject"
    Rules           []Rule // Firewall rules to apply
}
```

## Usage Notes

- All operations require sudo privileges on remote host
- `Enable()` uses `--force` flag to avoid interactive prompts
- UFW must be installed before other operations (use `Install()` or manual installation)
- Rule comments support alphanumeric characters, spaces, dashes, underscores
- GetRules() only parses simple port-based rules (no IP restrictions, IPv6, port ranges, or OUT direction)

## Security Practices

- **Non-interactive execution**: `--force` flag prevents hanging on confirmation prompts
- **Rate limiting support**: LIMIT action provides automatic rate limiting (e.g., SSH brute force protection)
- **Structured parsing**: Regex-based parsing of UFW output for reliable rule extraction
- **Error context**: All errors include stderr output for troubleshooting

## Implementation Notes

- Regex patterns compile on each function call (see audit for optimization opportunity)
- Only supports Debian/Ubuntu for installation (apt-get hardcoded)
- RemoveRule always uses "delete allow" (cannot remove LIMIT rules - see audit)
- FindRule returns pointer to slice element (not loop variable)

