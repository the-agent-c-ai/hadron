package sdk

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// Volume represents a Docker volume.
type Volume struct {
	name   string
	host   *Host
	driver string
	plan   *Plan
}

// VolumeBuilder builds a Volume with a fluent API.
type VolumeBuilder struct {
	plan   *Plan
	name   string
	host   *Host
	driver string
}

// Host sets the host where this volume will be created.
func (vb *VolumeBuilder) Host(host *Host) *VolumeBuilder {
	vb.host = host

	return vb
}

// Driver sets the volume driver (default: local).
func (vb *VolumeBuilder) Driver(driver string) *VolumeBuilder {
	vb.driver = driver

	return vb
}

// Build creates the Volume and registers it with the plan.
func (vb *VolumeBuilder) Build() *Volume {
	if vb.host == nil {
		vb.plan.logger.Fatal().Str("volume", vb.name).Msg("volume must be assigned to a host")
	}

	if vb.driver == "" {
		vb.driver = "local"
	}

	volume := &Volume{
		name:   vb.name,
		host:   vb.host,
		driver: vb.driver,
		plan:   vb.plan,
	}

	vb.plan.volumes = append(vb.plan.volumes, volume)

	return volume
}

// Name returns the volume name.
func (v *Volume) Name() string {
	return v.name
}

// Host returns the host where this volume is deployed.
func (v *Volume) Host() *Host {
	return v.host
}

// Driver returns the volume driver.
func (v *Volume) Driver() string {
	return v.driver
}

// ConfigHash returns a SHA256 hash of the volume configuration.
// Used for idempotent deployments.
func (v *Volume) ConfigHash() string {
	config := fmt.Sprintf("%s|%s|%s", v.name, v.driver, v.host.String())
	hash := sha256.Sum256([]byte(config))

	return hex.EncodeToString(hash[:])
}
