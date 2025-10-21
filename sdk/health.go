package sdk

import (
	"fmt"
	"time"
)

const (
	defaultHealthCheckTimeout  = 30 * time.Second
	defaultHealthCheckInterval = 5 * time.Second
	defaultHealthCheckRetries  = 5
)

// HealthCheck represents a container health check configuration.
type HealthCheck struct {
	checkType HealthCheckType
	path      string   // for HTTP checks
	port      int      // for HTTP and TCP checks
	command   []string // for command checks
	timeout   time.Duration
	interval  time.Duration
	retries   int
}

// HealthCheckType defines the type of health check.
type HealthCheckType string

const (
	// HealthCheckHTTP performs an HTTP GET request.
	HealthCheckHTTP HealthCheckType = "http"
	// HealthCheckTCP performs a TCP connection check.
	HealthCheckTCP HealthCheckType = "tcp"
	// HealthCheckCommand executes a command inside the container.
	HealthCheckCommand HealthCheckType = "command"
)

// HTTPCheck creates an HTTP health check.
func HTTPCheck(path string, port int) *HealthCheck {
	return &HealthCheck{
		checkType: HealthCheckHTTP,
		path:      path,
		port:      port,
		timeout:   defaultHealthCheckTimeout,
		interval:  defaultHealthCheckInterval,
		retries:   defaultHealthCheckRetries,
	}
}

// TCPCheck creates a TCP health check.
func TCPCheck(port int) *HealthCheck {
	return &HealthCheck{
		checkType: HealthCheckTCP,
		port:      port,
		timeout:   defaultHealthCheckTimeout,
		interval:  defaultHealthCheckInterval,
		retries:   defaultHealthCheckRetries,
	}
}

// CommandCheck creates a command-based health check.
// Accepts command and optional arguments: CommandCheck("curl", "-f", "http://localhost/health").
func CommandCheck(command string, args ...string) *HealthCheck {
	cmd := append([]string{command}, args...)

	return &HealthCheck{
		checkType: HealthCheckCommand,
		command:   cmd,
		timeout:   defaultHealthCheckTimeout,
		interval:  defaultHealthCheckInterval,
		retries:   defaultHealthCheckRetries,
	}
}

// WithTimeout sets the total timeout for the health check.
func (hc *HealthCheck) WithTimeout(timeout time.Duration) *HealthCheck {
	hc.timeout = timeout

	return hc
}

// WithInterval sets the interval between health check attempts.
func (hc *HealthCheck) WithInterval(interval time.Duration) *HealthCheck {
	hc.interval = interval

	return hc
}

// WithRetries sets the number of retries before marking as unhealthy.
func (hc *HealthCheck) WithRetries(retries int) *HealthCheck {
	hc.retries = retries

	return hc
}

// String returns a string representation of the health check.
func (hc *HealthCheck) String() string {
	switch hc.checkType {
	case HealthCheckHTTP:
		return fmt.Sprintf("HTTP %s:%d%s", "localhost", hc.port, hc.path)
	case HealthCheckTCP:
		return fmt.Sprintf("TCP %s:%d", "localhost", hc.port)
	case HealthCheckCommand:
		return fmt.Sprintf("Command: %v", hc.command)
	default:
		return "unknown"
	}
}
