package debian_test

import (
	"strings"
	"testing"

	"github.com/the-agent-c-ai/hadron/internal/debian"
	"github.com/the-agent-c-ai/hadron/internal/testutil"
)

func TestEnsureAutoUpdatesEnabled(t *testing.T) { //nolint:paralleltest // Integration tests use shared container
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	container := testutil.StartDebianSSHContainer(t)
	client := container.Client()

	t.Run("enables auto-updates when not configured", func(t *testing.T) { //nolint:paralleltest // Subtests share
		// Ensure auto-updates are enabled
		err := debian.EnsureAutoUpdatesEnabled(client)
		if err != nil {
			t.Fatalf("expected EnsureAutoUpdatesEnabled to succeed, got error: %v", err)
		}

		// Verify unattended-upgrades package is installed
		_, _, err = client.Execute("dpkg -l unattended-upgrades | grep ^ii")
		if err != nil {
			t.Error("expected unattended-upgrades to be installed")
		}

		// Verify configuration file exists
		_, _, err = client.Execute("test -f /etc/apt/apt.conf.d/20auto-upgrades")
		if err != nil {
			t.Error("expected /etc/apt/apt.conf.d/20auto-upgrades to exist")
		}

		// Verify configuration contains proper settings
		stdout, _, err := client.Execute("cat /etc/apt/apt.conf.d/20auto-upgrades")
		if err != nil {
			t.Fatalf("failed to read auto-upgrades config: %v", err)
		}

		if !strings.Contains(stdout, "APT::Periodic::Update-Package-Lists") {
			t.Error("expected config to contain Update-Package-Lists setting")
		}

		if !strings.Contains(stdout, "APT::Periodic::Unattended-Upgrade") {
			t.Error("expected config to contain Unattended-Upgrade setting")
		}
	})

	t.Run("is idempotent when already configured", func(t *testing.T) { //nolint:paralleltest // Subtests share
		// Enable first time
		err := debian.EnsureAutoUpdatesEnabled(client)
		if err != nil {
			t.Fatalf("first enable failed: %v", err)
		}

		// Enable again - should be idempotent
		err = debian.EnsureAutoUpdatesEnabled(client)
		if err != nil {
			t.Fatalf("expected EnsureAutoUpdatesEnabled to be idempotent, got error: %v", err)
		}
	})
}
