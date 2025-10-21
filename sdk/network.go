package sdk

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// Network represents a Docker network.
type Network struct {
	name   string
	host   *Host
	driver string
	plan   *Plan
}

// NetworkBuilder builds a Network with a fluent API.
type NetworkBuilder struct {
	plan   *Plan
	name   string
	host   *Host
	driver string
}

// Host sets the host where this network will be created.
func (nb *NetworkBuilder) Host(host *Host) *NetworkBuilder {
	nb.host = host

	return nb
}

// Driver sets the network driver (default: bridge).
func (nb *NetworkBuilder) Driver(driver string) *NetworkBuilder {
	nb.driver = driver

	return nb
}

// Build creates the Network and registers it with the plan.
func (nb *NetworkBuilder) Build() *Network {
	if nb.host == nil {
		nb.plan.logger.Fatal().Str("network", nb.name).Msg("network must be assigned to a host")
	}

	network := &Network{
		name:   nb.name,
		host:   nb.host,
		driver: nb.driver,
		plan:   nb.plan,
	}

	nb.plan.networks = append(nb.plan.networks, network)

	return network
}

// Name returns the network name.
func (n *Network) Name() string {
	return n.name
}

// Host returns the host where this network is deployed.
func (n *Network) Host() *Host {
	return n.host
}

// Driver returns the network driver.
func (n *Network) Driver() string {
	return n.driver
}

// ConfigHash returns a SHA256 hash of the network configuration.
// Used for idempotent deployments.
func (n *Network) ConfigHash() string {
	config := fmt.Sprintf("%s|%s|%s", n.name, n.driver, n.host.String())
	hash := sha256.Sum256([]byte(config))

	return hex.EncodeToString(hash[:])
}
