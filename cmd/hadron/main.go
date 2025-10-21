// Package main is hadron binary entrypoing
package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v2"
)

const (
	flagNamePlan     = "plan"
	goCommandRunVerb = "run"
)

var errPlanFileNotFound = errors.New("plan file not found")

func main() {
	// Configure zerolog
	zerolog.TimeFieldFormat = time.RFC3339
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	app := &cli.App{
		Name:  "hadron",
		Usage: "Declarative Docker deployment tool",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "log-level",
				Aliases: []string{"l"},
				Value:   "info",
				Usage:   "Log level (debug, info, warn, error)",
			},
		},
		Before: func(c *cli.Context) error {
			// Set log level
			level, err := zerolog.ParseLevel(c.String("log-level"))
			if err != nil {
				return fmt.Errorf("invalid log level: %w", err)
			}
			zerolog.SetGlobalLevel(level)

			return nil
		},
		Commands: []*cli.Command{
			{
				Name:  "deploy",
				Usage: "Deploy a plan to remote hosts",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     flagNamePlan,
						Aliases:  []string{"p"},
						Required: true,
						Usage:    "Path to the deployment plan (Go file)",
					},
					&cli.BoolFlag{
						Name:  "dry-run",
						Usage: "Show what would be deployed without executing",
					},
				},
				Action: deploy,
			},
			{
				Name:  "destroy",
				Usage: "Destroy resources defined in a plan",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     flagNamePlan,
						Aliases:  []string{"p"},
						Required: true,
						Usage:    "Path to the deployment plan (Go file)",
					},
				},
				Action: destroy,
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal().Err(err).Msg("Command failed")
	}
}

func deploy(c *cli.Context) error {
	planPath := c.String(flagNamePlan)
	dryRun := c.Bool("dry-run")

	// Verify plan file exists
	if _, err := os.Stat(planPath); err != nil {
		return fmt.Errorf("%w: %s", errPlanFileNotFound, planPath)
	}

	// Get absolute path
	absPath, err := filepath.Abs(planPath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	log.Info().Str("plan", absPath).Bool("dry-run", dryRun).Msg("Deploying plan")

	// Build go run command args
	// Check if shared.go exists in the same directory
	goRunArgs := []string{goCommandRunVerb, absPath}
	sharedPath := filepath.Join(filepath.Dir(absPath), "shared.go")

	if _, err := os.Stat(sharedPath); err == nil {
		goRunArgs = []string{goCommandRunVerb, absPath, sharedPath}
	}

	// Execute go run on the plan
	//nolint:gosec
	cmd := exec.Command("go", goRunArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), fmt.Sprintf("HADRON_DRY_RUN=%t", dryRun))
	cmd.Dir = filepath.Dir(absPath)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to execute plan: %w", err)
	}

	return nil
}

func destroy(c *cli.Context) error {
	planPath := c.String(flagNamePlan)

	// Verify plan file exists
	if _, err := os.Stat(planPath); err != nil {
		return fmt.Errorf("%w: %s", errPlanFileNotFound, planPath)
	}

	// Get absolute path
	absPath, err := filepath.Abs(planPath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	log.Info().Str("plan", absPath).Msg("Destroying resources")

	// Build go run command args
	// Check if shared.go exists in the same directory
	goRunArgs := []string{goCommandRunVerb, absPath}
	sharedPath := filepath.Join(filepath.Dir(absPath), "shared.go")

	if _, err := os.Stat(sharedPath); err == nil {
		goRunArgs = []string{goCommandRunVerb, absPath, sharedPath}
	}

	// Execute go run on the plan with destroy mode
	//nolint:gosec
	cmd := exec.Command("go", goRunArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), "HADRON_DESTROY=true")
	cmd.Dir = filepath.Dir(absPath)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to execute plan: %w", err)
	}

	return nil
}
