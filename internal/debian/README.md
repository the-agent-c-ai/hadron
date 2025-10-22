# Debian Package Management

This internal package provides secure, idempotent Debian package management operations via SSH for Hadron deployments.

## Purpose

Manages Debian packages on remote hosts with:
- **Idempotent operations**: EnsureInstalled/EnsureRemoved check state before acting
- **Security by default**: Non-interactive installations with DEBIAN_FRONTEND=noninteractive
- **Minimal installations**: Uses `--no-install-recommends` to reduce attack surface
- **Custom installers**: Special handling for complex packages (e.g., Docker CE)
- **Automatic security updates**: Unattended-upgrades configuration

## Public API

### Package Management
- `EnsureInstalled(client, packageName)` - Install package if not already installed
- `EnsureRemoved(client, packageName)` - Remove package if currently installed

### Unattended Upgrades (Security Updates)
- `EnsureAutoUpdatesEnabled(client)` - Ensure automatic security updates are installed and configured (recommended)

## Custom Installers

Special installation procedures are implemented for packages requiring non-standard setup:

### Docker CE
`installDocker(client)` follows the official Docker installation procedure:
1. Install prerequisites (ca-certificates, curl)
2. Add Docker's official GPG key to `/etc/apt/keyrings/docker.asc`
3. Configure Docker apt repository for current Debian codename
4. Install docker-ce, docker-ce-cli, containerd.io, docker-buildx-plugin, docker-compose-plugin

Automatically invoked when calling `EnsureInstalled(client, "docker-ce")`.

## Security Practices

- **Non-interactive mode**: All apt operations use `DEBIAN_FRONTEND=noninteractive` to prevent hanging prompts
- **Minimal installs**: `--no-install-recommends` reduces installed package count
- **Quiet output**: `-qq` flag minimizes command output (use stderr for diagnostics)
- **GPG verification**: Docker GPG key is downloaded over HTTPS and has proper permissions (a+r)
- **Error wrapping**: All errors wrapped with sentinel errors for programmatic handling

## Implementation Notes

- Internal functions (`isInstalled`, `install`, `remove`) are unexported - use public API
- `dpkg -l` exit codes used for logic (0 = installed, non-zero = not installed)
- `apt-get update` always run before installations to ensure fresh package lists
- `apt-get autoremove` run after removals to clean up orphaned dependencies

---

# Code Quality & Testing Audit

**Audit Date:** 2025-10-21
**Code Quality:** 8/10
**Test Coverage:** 38.9%

## Strengths

- **Clean architecture**: Clear separation of public API vs internal implementation
- **Idempotent design**: All operations check state before acting
- **Extensible pattern**: Custom installer registry for complex packages (Docker)
- **Comprehensive error handling**: Sentinel errors with wrapped context
- **Good documentation**: Package README and inline comments
- **Zero lint issues**: Passes go vet, gofmt, golangci-lint
- **Integration tests**: Real container-based validation via `testutil.StartDebianSSHContainer`

## Test Coverage Gaps (HIGH priority)

### ZERO Docker Installation Coverage (156 lines untested)
- No tests for `installDocker` or its 4 sub-functions (installDockerPrerequisites, addDockerGPGKey, setupDockerRepository, installDockerPackages)
- Complex multi-step process with 9 distinct error types
- Impact: Production deployments could silently fail at any step

**Recommended Tests:**
- `TestEnsureInstalled_DockerCE_FullInstallation` - Verify custom installer routing
- `TestInstallDocker_GPGKeyDownloadFailure` - Error path coverage
- `TestInstallDocker_RepositorySetupFailure` - Error path coverage

### Missing Test Scenarios
- Custom installer routing (verify `docker-ce` triggers `installDocker` not `installWithApt`)
- Error paths (apt-get update failures, autoremove failures)
- Edge cases (package names with dashes/dots/underscores, network failures, disk exhaustion)

### Coverage Targets
- Overall: 85%+ (currently 38.9%)
- Critical paths (Docker, auto-updates): 95%+
- Error paths: 80%+

## Code Quality Improvements (LOW priority)

### Magic Strings (package.go)
Repeated flags as string literals reduce maintainability:
- `-qq`, `--no-install-recommends`, `DEBIAN_FRONTEND=noninteractive`
- Recommendation: Extract to package constants

### Error Message Verbosity (all files)
Raw stderr included in errors exposes system details (paths, versions, network config):
```go
return fmt.Errorf("%w: %s", ErrDockerGPGDownload, stderr)
```
- Consideration: Sanitize if errors leave operator's control (e.g., exposed via API)
- Not a security issue in Hadron's threat model (operator writes Go plans, has SSH sudo access)

## Required Actions

1. **Add Docker installation tests** - Full installation flow, GPG key handling, error paths (currently 0% coverage)
2. **Increase test coverage to 85%+** - Focus on critical paths and error scenarios
