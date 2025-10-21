package sdk_test

import (
	"testing"

	"github.com/rs/zerolog"

	"github.com/the-agent-c-ai/hadron/sdk"
)

func TestNetworkBuilder(t *testing.T) {
	t.Parallel()

	plan := sdk.NewPlan("test").WithLogger(zerolog.Nop())

	host := plan.Host("testuser@192.168.1.1").
		Build()

	network := plan.Network("test-network").
		Host(host).
		Driver("overlay").
		Build()

	// Black box test: verify network public API
	if network.Name() != "test-network" {
		t.Errorf("expected network name 'test-network', got '%s'", network.Name())
	}

	if network.Driver() != "overlay" {
		t.Errorf("expected driver 'overlay', got '%s'", network.Driver())
	}

	if network.Host() != host {
		t.Error("expected host to match")
	}
}

func TestNetworkDefaultDriver(t *testing.T) {
	t.Parallel()

	plan := sdk.NewPlan("test").WithLogger(zerolog.Nop())

	host := plan.Host("testuser@192.168.1.1").
		Build()

	network := plan.Network("test-network").
		Host(host).
		Build()

	if network.Driver() != "bridge" {
		t.Errorf("expected default driver 'bridge', got '%s'", network.Driver())
	}
}

func TestNetworkConfigHash(t *testing.T) {
	t.Parallel()

	plan := sdk.NewPlan("test").WithLogger(zerolog.Nop())

	host := plan.Host("testuser@192.168.1.1").
		Build()

	network := plan.Network("test-network").
		Host(host).
		Driver("bridge").
		Build()

	hash := network.ConfigHash()

	if hash == "" {
		t.Error("expected non-empty config hash")
	}

	if len(hash) != sha256HexLength {
		t.Errorf("expected %d-character SHA256 hash, got %d characters", sha256HexLength, len(hash))
	}
}
