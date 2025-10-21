package docker

import "errors"

var (
	// ErrFileSystemWalk indicates failure while walking directory tree.
	ErrFileSystemWalk = errors.New("failed to walk directory tree")

	// ErrPathRelative indicates failure to compute relative path.
	ErrPathRelative = errors.New("failed to compute relative path")
)
