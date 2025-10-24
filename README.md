# Hadron - Declarative Docker Deployment Tool

Hadron is a Go-based deployment tool that brings remote Docker hosts to a desired state through SSH-executed docker commands.

## Architecture

Hadron consists of two components:

### 1. The Hadron Tool (Binary)

A CLI tool built with `urfave/cli` and `zerolog` that:
- Executes `go run` on the deployment plan
- Establishes SSH connections to remote Docker hosts as defined in the plan
- Maintains one SSH connection per unique host, reused for all operations on that host
- Manages connection lifecycle and error handling
- Enables parallel execution across multiple hosts

**SSH Connection Pooling**: Each unique host in the plan gets one persistent SSH connection, shared by all docker commands targeting that host. Multiple hosts can be deployed to in parallel.

### 2. The Hadron SDK (Library)

A Go library for writing deployment plans that provides:
- Host definitions with SSH connection parameters
- Resource primitives: `Network`, `Volume`, `Container`
- Declarative resource definitions with fluent API
- Automatic dependency resolution and ordering
- Built-in health checking and rollback capabilities
- SSH command execution abstraction

Plans are Go programs that import the Hadron SDK and declare desired infrastructure state, including the hosts to deploy to.

## How It Works

```
┌─────────────────┐
│  hadron deploy  │  (CLI command with -p plan.go)
└────────┬────────┘
         │
         ▼
┌─────────────────────────────────┐
│  Execute: go run plan.go        │
└────────┬────────────────────────┘
         │
         ▼
┌─────────────────────────────────┐
│  Plan defines hosts and builds  │
│  resource graph using SDK       │
└────────┬────────────────────────┘
         │
         ▼
┌─────────────────────────────────┐
│  Establish SSH connections      │
│  (one per unique host)          │
└────────┬────────────────────────┘
         │
         ▼
┌─────────────────────────────────┐
│  Hadron resolves dependencies   │
│  Networks → Volumes → Containers │
│  (per host, respecting DependsOn)│
└────────┬────────────────────────┘
         │
         ▼
┌─────────────────────────────────┐
│  Execute docker commands over   │
│  pooled SSH connections         │
│  (parallel across hosts)        │
└────────┬────────────────────────┘
         │
         ▼
┌─────────────────────────────────┐
│  Health checks & rollback       │
│  (if enabled)                   │
└────────┬────────────────────────┘
         │
         ▼
┌─────────────────────────────────┐
│  Close all SSH connections      │
│  Report results (zerolog)       │
└─────────────────────────────────┘
```

## Smart Features

### 1. Idempotent Deployments
Resources are tagged with a label containing the SHA256 hash of their complete configuration (image digest, env vars, volumes, etc.). Hadron compares the desired state hash with the deployed resource's label and skips redeployment if unchanged.

**Example**: Deploying Vector with digest `sha256:abc123` creates label:
```
hadron.config.sha=def789...  # SHA of all container parameters
```
Next deploy with same config? Hadron sees matching SHA, skips recreation.

### 2. Resource Hierarchy & Dependency Resolution
Hadron understands Docker resource dependencies and enforces correct order:

**Creation order**: Networks → Volumes → Containers (with dependency sorting)
**Destruction order**: Containers → Volumes → Networks

Containers can declare dependencies on other containers (e.g., agent depends on aggregator), ensuring proper startup sequence.

### 3. Zero-Downtime Updates with Network Aliases
For container updates, Hadron uses Docker network aliases to enable zero-downtime deployments:

**Scenario**: Updating `vector-aggregator` from v1 to v2
1. Old container runs: `--name vector-aggregator-v1 --network-alias vector-aggregator`
2. Start new container: `--name vector-aggregator-v2 --network-alias vector-aggregator`
3. Both containers respond to DNS queries for `vector-aggregator`
4. Health check new container
5. Remove old container

Dependent services experience no DNS resolution failure or connection interruption.

### 4. Health Checks & Automatic Rollback
Containers can define health checks (HTTP endpoint, TCP port, or custom command). After starting a new container version:
- Poll health endpoint (e.g., `http://container:8686/health`)
- Wait up to configured timeout (default: 60s)
- **If healthy**: Remove old container version
- **If unhealthy**: Stop new container, keep old container running, report failure

Prevents broken deployments from going live.

## Plan Structure

Plans are Go programs using the Hadron SDK:

### Single Host Example

```go
package main

import (
    "github.com/your-org/hadron/sdk"
    "github.com/joho/godotenv"
    "os"
)

func main() {
    // Load environment
    _ = sdk.LoadEnv(".env")

    ctx := context.Background()

    // Create plan
    plan := sdk.NewPlan("black-observability")

    // Define host
    blackHost := plan.Host(sdk.GetEnv("BLACK_HOST")).
        User(sdk.GetEnv("BLACK_USER")).
        Build()

    // Define network
    blackNet := plan.Network("black").
        Host(blackHost).
        Driver("bridge").
        Build()

    // Define volume
    vectorData := plan.Volume("vector-data").
        Host(blackHost).
        Build()

    // Define container with health check and zero-downtime
    vectorAgg := plan.Container("vector-aggregator").
        Host(blackHost).
        Image(fmt.Sprintf("ghcr.io/%s/vector@%s",
            os.Getenv("GITHUB_ORG"),
            os.Getenv("VECTOR_GHCR_DIGEST"))).
        Network(blackNet).
        NetworkAlias("vector-aggregator").  // DNS name
        Volume(vectorData, "/var/lib/vector").
        EnvFile(".env").
        HealthCheck(sdk.HTTPCheck("/health", 8686)).
        Timeout(60).  // Health check timeout
        Build()

    // Agent depends on aggregator
    plan.Container("vector-agent").
        Host(blackHost).
        Image(fmt.Sprintf("ghcr.io/%s/vector@%s",
            os.Getenv("GITHUB_ORG"),
            os.Getenv("VECTOR_GHCR_DIGEST"))).
        Network(blackNet).
        DependsOn(vectorAgg).  // Start after aggregator healthy
        Volume("/var/run/docker.sock", "/var/run/docker.sock", "ro").
        EnvFile(".env").
        Build()

    // Caddy reverse proxy
    plan.Container("caddy").
        Host(blackHost).
        Image(fmt.Sprintf("ghcr.io/%s/caddy@%s",
            os.Getenv("GITHUB_ORG"),
            os.Getenv("CADDY_GHCR_DIGEST"))).
        Network(blackNet).
        Port("80:80").
        Port("443:443").
        EnvFile(".env").
        Volume("./config/caddy/Caddyfile", "/etc/caddy/Caddyfile", "ro").
        Build()

    // Execute plan
    if err := plan.Execute(ctx); err != nil {
        log.Fatal().Err(err).Msg("Deployment failed")
    }
}
```

### Multi-Host Example

```go
package main

import (
    "github.com/your-org/hadron/sdk"
    "github.com/joho/godotenv"
)

func main() {
    _ = sdk.LoadEnv(".env")

    ctx := context.Background()

    plan := sdk.NewPlan("multi-region-deployment")

    // Define multiple hosts
    dbHost := plan.Host("db.example.com").
        User("deploy").
        Build()

    web1 := plan.Host("web1.example.com").
        User("deploy").
        Build()

    web2 := plan.Host("web2.example.com").
        User("deploy").
        Build()

    // Database on dedicated host
    network := plan.Network("app").Host(dbHost).Build()

    db := plan.Container("postgres").
        Host(dbHost).
        Network(network).
        Image("postgres:16@sha256:abc123...").
        EnvFile(".env").
        Build()

    // App servers on multiple hosts (deployed in parallel)
    plan.Container("app").
        Host(web1).
        Image("ghcr.io/org/app@sha256:def456...").
        EnvFile(".env").
        Build()

    plan.Container("app").
        Host(web2).
        Image("ghcr.io/org/app@sha256:def456...").
        EnvFile(".env").
        Build()

    if err := plan.Execute(ctx); err != nil {
        log.Fatal().Err(err).Msg("Deployment failed")
    }
}
```

## Reusable Stacks

Hadron provides pre-built infrastructure stacks in the `stacks/` directory:

- **`logger`**: Vector log collection and forwarding to Loki
- **`proxy`**: Caddy reverse proxy with automatic HTTPS

These can be imported and used in your plans:

```go
import (
    "github.com/the-agent-c-ai/hadron/sdk"
    "github.com/the-agent-c-ai/hadron/stacks/logger"
    "github.com/the-agent-c-ai/hadron/stacks/proxy"
)

// Deploy logging stack
logger.Logger(plan, host, &logger.Config{
    Image:        "ghcr.io/org/vector@sha256:...",
    LogLevel:     "info",
    Environment:  "production",
    LokiEndpoint: "https://loki.example.com",
    LokiUsername: "user",
    LokiPassword: "pass",
})

// Deploy reverse proxy
proxy.Proxy(plan, appContainer, network, host, &proxy.Config{
    Image:    "ghcr.io/org/caddy@sha256:...",
    LogLevel: "info",
    Static:   "./static",
    Email:    "admin@example.com",
    Domain:   "app.example.com",
})
```

## Configuration

Hadron reads from `.env` for secrets and configuration:
- Host connection details (endpoint, user)
- GHCR credentials
- Image digests
- Loki endpoints
- Webhook secrets

Plans use `os.Getenv()` to read host configuration and `EnvFile(".env")` to inject environment variables into containers.

### SSH Host Key Verification

Hadron supports two methods for SSH host key verification:

**1. Known Hosts (Default)**
```go
// Uses ~/.ssh/known_hosts for verification
host := plan.Host("user@example.com").Build()
```

**2. Fingerprint-Based (For Automation)**
```go
// For CI/CD and automated deployments where ~/.ssh/known_hosts is not practical
// Get fingerprint: ssh-keyscan -t ed25519 hostname | ssh-keygen -lf -
host := plan.Host("user@example.com").
    Fingerprint("SHA256:nThbg6kXUpJWGl7E1IGOCspRomTxdCARLviKw6E5SY8").
    Build()
```

Fingerprint verification is recommended for:
- CI/CD pipelines with ephemeral build agents
- Terraform-style infrastructure-as-code deployments
- Any scenario where the fingerprint can be securely stored in configuration

## CLI Usage

```bash
# Deploy a plan (hosts defined in plan)
hadron deploy -p deploy/plan.go

# Dry run (show what would change without executing)
hadron deploy --dry-run -p deploy/plan.go

# Destroy all resources in plan
hadron destroy -p deploy/plan.go

# Verbose logging (see all docker commands)
hadron deploy -p deploy/plan.go --log-level debug
```

## Benefits

- **Simple**: SSH + Docker CLI. No agents, no daemons.
- **Efficient**: One SSH connection per host, reused for all operations.
- **Scalable**: Multi-host deployments with parallel execution.
- **Declarative**: Hosts and infrastructure fully defined in code.
- **Idempotent**: Only redeploy what changed (SHA-based detection).
- **Safe**: Health checks prevent broken deploys.
- **Fast**: Parallel resource creation across hosts where dependencies allow.
- **Type-safe**: Plans are Go programs, catch errors at compile time.
- **Transparent**: See exact docker commands being executed (zerolog debug mode).
- **Portable**: Same plan works across dev/staging/prod by changing .env.

