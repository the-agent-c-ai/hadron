package sdk_test

import (
	"testing"

	"github.com/rs/zerolog"

	"github.com/the-agent-c-ai/hadron/sdk"
)

const (
	sha256HexLength            = 64 // SHA256 hash as hex string is 64 characters
	errMsgExpectedNonEmptyHash = "expected non-empty config hash"
)

func TestContainerBuilder(t *testing.T) {
	t.Parallel()

	plan := sdk.NewPlan("test").WithLogger(zerolog.Nop())

	host := plan.Host("testuser@192.168.1.1").
		Build()

	network := plan.Network("test-network").
		Host(host).
		Build()

	container := plan.Container("test-container").
		Host(host).
		Image("nginx:latest").
		Network(network).
		NetworkAlias("nginx").
		Port("80:80").
		Env("FOO", "bar").
		ReadOnly().
		CapDrop("ALL").
		CapAdd("NET_BIND_SERVICE").
		SecurityOpt("no-new-privileges").
		Build()

	// Black box test: verify container public API
	if container.Name() != "test-container" {
		t.Errorf("expected container name 'test-container', got '%s'", container.Name())
	}

	if container.Image() != "nginx:latest" {
		t.Errorf("expected image 'nginx:latest', got '%s'", container.Image())
	}

	if container.NetworkAlias() != "nginx" {
		t.Errorf("expected network alias 'nginx', got '%s'", container.NetworkAlias())
	}

	if container.Host() != host {
		t.Error("expected host to match")
	}

	// Verify config hash is generated
	if container.ConfigHash() == "" {
		t.Error(errMsgExpectedNonEmptyHash)
	}
}

func TestContainerVolumeMounts(t *testing.T) {
	t.Parallel()

	plan := sdk.NewPlan("test").WithLogger(zerolog.Nop())

	host := plan.Host("testuser@192.168.1.1").
		Build()

	volume := plan.Volume("data").
		Host(host).
		Build()

	// Black box test: verify volume mounts can be added without error
	container := plan.Container("test").
		Host(host).
		Image("nginx:latest").
		Volume(volume, "/data").
		Volume("/host/path", "/container/path", "ro").
		Build()

	if container == nil {
		t.Fatal("expected container to be created")
	}

	// Config hash should reflect volumes
	hash1 := container.ConfigHash()
	if hash1 == "" {
		t.Error(errMsgExpectedNonEmptyHash)
	}
}

func TestContainerConfigHash(t *testing.T) {
	t.Parallel()

	plan := sdk.NewPlan("test").WithLogger(zerolog.Nop())

	host := plan.Host("testuser@192.168.1.1").
		Build()

	container := plan.Container("test").
		Host(host).
		Image("nginx:latest").
		Port("80:80").
		Build()

	hash := container.ConfigHash()

	if hash == "" {
		t.Error(errMsgExpectedNonEmptyHash)
	}

	if len(hash) != sha256HexLength {
		t.Errorf("expected %d-character SHA256 hash, got %d characters", sha256HexLength, len(hash))
	}

	// Build identical container - should have same hash
	container2 := plan.Container("test").
		Host(host).
		Image("nginx:latest").
		Port("80:80").
		Build()

	if container.ConfigHash() != container2.ConfigHash() {
		t.Error("expected identical containers to have same config hash")
	}

	// Build different container - should have different hash
	container3 := plan.Container("test").
		Host(host).
		Image("nginx:alpine").
		Port("80:80").
		Build()

	if container.ConfigHash() == container3.ConfigHash() {
		t.Error("expected different containers to have different config hash")
	}
}
