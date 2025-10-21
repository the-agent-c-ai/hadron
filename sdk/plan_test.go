package sdk_test

import (
	"testing"

	"github.com/rs/zerolog"

	"github.com/the-agent-c-ai/hadron/sdk"
)

func TestNewPlan(t *testing.T) {
	t.Parallel()

	plan := sdk.NewPlan("test-plan")

	// Black box test: verify plan can be created and used
	if plan == nil {
		t.Fatal("expected plan to be created")
	}

	// Verify plan can have logger set
	plan.WithLogger(zerolog.Nop())
}

func TestPlanWithLogger(t *testing.T) {
	t.Parallel()

	logger := zerolog.Nop()
	plan := sdk.NewPlan("test").WithLogger(logger)

	// Black box test: verify logger can be set without error
	if plan == nil {
		t.Fatal("expected plan with logger to be created")
	}
}

func TestPlanHostBuilder(t *testing.T) {
	t.Parallel()

	plan := sdk.NewPlan("test").WithLogger(zerolog.Nop())

	host := plan.Host("test-host").
		Build()

	// Black box test: verify host public API
	if host.Endpoint() != "test-host" {
		t.Errorf("expected endpoint 'test-host', got '%s'", host.Endpoint())
	}

	// Verify string representation works
	if host.String() == "" {
		t.Error("expected non-empty string representation")
	}
}
