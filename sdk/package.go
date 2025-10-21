package sdk

// Package represents a Debian package to be installed on a host.
type Package struct {
	name string
	host *Host
}

// PackageRemoval represents a Debian package to be removed from a host.
type PackageRemoval struct {
	name string
	host *Host
}

// Name returns the package name.
func (p *Package) Name() string {
	return p.name
}

// Host returns the host where this package will be installed.
func (p *Package) Host() *Host {
	return p.host
}

// Name returns the package name to be removed.
func (pr *PackageRemoval) Name() string {
	return pr.name
}

// Host returns the host where this package will be removed.
func (pr *PackageRemoval) Host() *Host {
	return pr.host
}
