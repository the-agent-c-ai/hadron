package sdk_test

import (
	"strings"
	"testing"
	"time"

	"github.com/the-agent-c-ai/hadron/sdk"
)

const (
	testHTTPPort                     = 8080
	testMySQLPort                    = 3306
	errMsgExpectedHealthCheckCreated = "expected health check to be created"
)

func TestHTTPCheck(t *testing.T) {
	t.Parallel()

	check := sdk.HTTPCheck("/health", testHTTPPort)

	// Black box test: verify health check can be created and has expected string representation
	if check == nil {
		t.Fatal(errMsgExpectedHealthCheckCreated)
	}

	str := check.String()
	if !strings.Contains(str, "HTTP") || !strings.Contains(str, "8080") || !strings.Contains(str, "/health") {
		t.Errorf("expected health check string to contain HTTP, port, and path, got: %s", str)
	}
}

func TestTCPCheck(t *testing.T) {
	t.Parallel()

	check := sdk.TCPCheck(testMySQLPort)

	if check == nil {
		t.Fatal(errMsgExpectedHealthCheckCreated)
	}

	str := check.String()
	if !strings.Contains(str, "TCP") || !strings.Contains(str, "3306") {
		t.Errorf("expected health check string to contain TCP and port, got: %s", str)
	}
}

func TestCommandCheck(t *testing.T) {
	t.Parallel()

	check := sdk.CommandCheck("pg_isready")

	if check == nil {
		t.Fatal(errMsgExpectedHealthCheckCreated)
	}

	str := check.String()
	if !strings.Contains(str, "Command") || !strings.Contains(str, "pg_isready") {
		t.Errorf("expected health check string to contain Command and command, got: %s", str)
	}
}

func TestHealthCheckWithTimeout(t *testing.T) {
	t.Parallel()

	// Black box test: verify health check builder methods work
	check := sdk.HTTPCheck("/health", testHTTPPort).
		WithTimeout(60 * time.Second).
		WithInterval(10 * time.Second).
		WithRetries(3)

	if check == nil {
		t.Fatal("expected health check with custom settings to be created")
	}

	// Verify it still produces a valid string representation
	str := check.String()
	if str == "" {
		t.Error("expected non-empty string representation")
	}
}
