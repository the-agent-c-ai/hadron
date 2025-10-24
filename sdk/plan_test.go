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

func TestPlanHostWithFingerprint(t *testing.T) {
	t.Parallel()

	plan := sdk.NewPlan("test").WithLogger(zerolog.Nop())

	expectedFingerprint := "SHA256:nThbg6kXUpJWGl7E1IGOCspRomTxdCARLviKw6E5SY8"
	host := plan.Host("test-host").
		Fingerprint(expectedFingerprint).
		Build()

	// Black box test: verify fingerprint is stored and retrievable
	if host.SSHFingerprint() != expectedFingerprint {
		t.Errorf("expected fingerprint '%s', got '%s'", expectedFingerprint, host.SSHFingerprint())
	}

	// Verify fingerprint doesn't affect other host properties
	if host.Endpoint() != "test-host" {
		t.Errorf("expected endpoint 'test-host', got '%s'", host.Endpoint())
	}
}

func TestPlanHostWithoutFingerprint(t *testing.T) {
	t.Parallel()

	plan := sdk.NewPlan("test").WithLogger(zerolog.Nop())

	host := plan.Host("test-host").
		Build()

	// Black box test: verify empty fingerprint when not set
	if host.SSHFingerprint() != "" {
		t.Errorf("expected empty fingerprint, got '%s'", host.SSHFingerprint())
	}
}
