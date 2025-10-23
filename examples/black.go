// Package main documents examples for Hadron SDK
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/the-agent-c-ai/hadron/sdk"
)

const (
	healthcheckTimeout  = time.Second * 60
	healthcheckInterval = time.Second * 5
	port                = 8686
)

func main() {
	// Configure logging
	sdk.ConfigureDefaultLogger()

	// Load environment variables
	_ = sdk.LoadEnv("../.env")

	// Create deployment plan
	plan := sdk.NewPlan("black-observability").
		WithLogger(log.Logger)

	// Define the host (SSH config resolves connection parameters)
	blackHost := plan.Host(sdk.GetEnv("BLACK_HOST")).
		Build()

	// Create custom Docker network
	blackNet := plan.Network("black").
		Host(blackHost).
		Driver("bridge").
		Build()

	// Create volumes
	vectorData := plan.Volume("vector-data").
		Host(blackHost).
		Build()

	vectorAgentData := plan.Volume("vector-agent-data").
		Host(blackHost).
		Build()

	caddyData := plan.Volume("caddy-data").
		Host(blackHost).
		Build()

	caddyConfig := plan.Volume("caddy-config").
		Host(blackHost).
		Build()

	caddyLogs := plan.Volume("caddy-logs").
		Host(blackHost).
		Build()

	// Vector Aggregator (main service)
	vectorAgg := plan.Container("vector-aggregator").
		Host(blackHost).
		Image(fmt.Sprintf("ghcr.io/%s/vector@%s",
			sdk.GetEnv("GITHUB_ORG"),
			sdk.GetEnv("VECTOR_GHCR_DIGEST"))).
		Network(blackNet).
		NetworkAlias("vector-aggregator").
		Volume(vectorData, "/var/lib/vector").
		Volume("../config/vector/aggregator.yaml", "/etc/vector/vector.yaml", "ro").
		EnvFile("../.env").
		Env("VECTOR_LOG", "info").
		Restart("unless-stopped").
		ReadOnly().
		CapDrop("ALL").
		SecurityOpt("no-new-privileges").
		HealthCheck(sdk.HTTPCheck("/health", port).
			WithTimeout(healthcheckTimeout).
			WithInterval(healthcheckInterval)).
		Build()

	// Vector Agent (collects Docker logs)
	plan.Container("vector-agent").
		Host(blackHost).
		Image(fmt.Sprintf("ghcr.io/%s/vector@%s",
			sdk.GetEnv("GITHUB_ORG"),
			sdk.GetEnv("VECTOR_GHCR_DIGEST"))).
		Network(blackNet).
		DependsOn(vectorAgg).
		Volume("/var/run/docker.sock", "/var/run/docker.sock", "ro").
		Volume(vectorAgentData, "/var/lib/vector").
		Volume("../config/vector/agent.yaml", "/etc/vector/vector.yaml", "ro").
		EnvFile("../.env").
		Env("VECTOR_LOG", "info").
		Restart("unless-stopped").
		ReadOnly().
		CapDrop("ALL").
		SecurityOpt("no-new-privileges").
		Build()

	// Caddy reverse proxy
	plan.Container("caddy").
		Host(blackHost).
		Image(fmt.Sprintf("ghcr.io/%s/caddy@%s",
			sdk.GetEnv("GITHUB_ORG"),
			sdk.GetEnv("CADDY_GHCR_DIGEST"))).
		Network(blackNet).
		Port("80:80").
		Port("443:443").
		Volume("../config/caddy/Caddyfile", "/etc/caddy/Caddyfile", "ro").
		Volume(caddyData, "/data").
		Volume(caddyConfig, "/config").
		Volume(caddyLogs, "/var/log/caddy").
		EnvFile("../.env").
		Restart("unless-stopped").
		CapDrop("ALL").
		CapAdd("NET_BIND_SERVICE").
		SecurityOpt("no-new-privileges").
		Build()

	// Execute plan based on environment
	ctx := context.Background()

	var err error

	switch {
	case os.Getenv("HADRON_DESTROY") == "true":
		err = plan.Destroy()
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to destroy resources")
		}
	case os.Getenv("HADRON_DRY_RUN") == "true":
		err = plan.DryRun()
		if err != nil {
			log.Fatal().Err(err).Msg("Dry run failed")
		}
	default:
		err = plan.Execute(ctx)
		if err != nil {
			log.Fatal().Err(err).Msg("Deployment failed")
		}
	}
}
