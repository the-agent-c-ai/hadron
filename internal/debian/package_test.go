package debian_test

import (
	"errors"
	"testing"

	"github.com/the-agent-c-ai/hadron/internal/debian"
	"github.com/the-agent-c-ai/hadron/internal/testutil"
)

func TestEnsureInstalled(t *testing.T) { //nolint:paralleltest // Integration tests use shared container
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	container := testutil.StartDebianSSHContainer(t)
	client := container.Client()

	t.Run("installs package when not present", func(t *testing.T) { //nolint:paralleltest // Subtests share container
		// Install curl (small package for fast test)
		err := debian.EnsureInstalled(client, "curl")
		if err != nil {
			t.Fatalf("expected EnsureInstalled to succeed, got error: %v", err)
		}

		// Verify curl is actually installed by running it
		stdout, _, err := client.Execute("curl --version")
		if err != nil {
			t.Fatalf("expected curl to be installed and runnable: %v", err)
		}

		if stdout == "" {
			t.Error("expected curl --version to return output")
		}
	})

	t.Run("is idempotent when package already installed", func(t *testing.T) { //nolint:paralleltest // Subtests share
		// Install curl first time
		err := debian.EnsureInstalled(client, "curl")
		if err != nil {
			t.Fatalf("first install failed: %v", err)
		}

		// Install again - should be idempotent
		err = debian.EnsureInstalled(client, "curl")
		if err != nil {
			t.Fatalf("expected EnsureInstalled to be idempotent, got error: %v", err)
		}
	})

	t.Run( //nolint:paralleltest // Subtests share container
		"returns error for non-existent package",
		func(t *testing.T) {
			err := debian.EnsureInstalled(client, "this-package-does-not-exist-12345")
			if err == nil {
				t.Fatal("expected error for non-existent package")
			}

			if !errors.Is(err, debian.ErrPackageInstallFailed) {
				t.Errorf("expected ErrPackageInstallFailed, got: %v", err)
			}
		},
	)
}

func TestEnsureRemoved(t *testing.T) { //nolint:paralleltest // Integration tests use shared container
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	container := testutil.StartDebianSSHContainer(t)
	client := container.Client()

	t.Run("removes installed package", func(t *testing.T) { //nolint:paralleltest // Subtests share container
		// First install curl
		err := debian.EnsureInstalled(client, "curl")
		if err != nil {
			t.Fatalf("failed to install curl: %v", err)
		}

		// Now remove it
		err = debian.EnsureRemoved(client, "curl")
		if err != nil {
			t.Fatalf("expected EnsureRemoved to succeed, got error: %v", err)
		}

		// Verify curl is actually removed
		_, _, err = client.Execute("curl --version")
		if err == nil {
			t.Error("expected curl to be removed, but it's still runnable")
		}
	})

	t.Run( //nolint:paralleltest // Subtests share container
		"is idempotent when package not installed",
		func(t *testing.T) {
			// Remove package that's not installed
			err := debian.EnsureRemoved(client, "wget")
			if err != nil {
				t.Fatalf("expected EnsureRemoved to be idempotent, got error: %v", err)
			}
		},
	)
}
