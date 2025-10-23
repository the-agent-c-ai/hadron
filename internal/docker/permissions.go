package docker

import "os"

// File permission constants for Docker operations.
const (
	// PermSecretFile is the permission for secret files (owner read/write only).
	// Used for sensitive data like credentials, keys, and configuration secrets.
	PermSecretFile os.FileMode = 0o600

	// PermPublicFile is the permission for public files (owner read/write, others read).
	// Used for non-sensitive data that containers need to read.
	PermPublicFile os.FileMode = 0o644

	// PermSecretDir is the permission for secret directories (owner read/write/execute only).
	// Used for directories containing sensitive data.
	PermSecretDir os.FileMode = 0o700
)
