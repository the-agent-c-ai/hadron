package hash

import "errors"

var (
	// ErrFileOpen indicates failure to open file for hashing.
	ErrFileOpen = errors.New("failed to open file")

	// ErrFileRead indicates failure to read file content.
	ErrFileRead = errors.New("failed to read file content")

	// ErrFileStat indicates failure to stat file or directory.
	ErrFileStat = errors.New("failed to stat path")

	// ErrPathRelative indicates failure to compute relative path.
	ErrPathRelative = errors.New("failed to compute relative path")

	// ErrDirectoryWalk indicates failure while walking directory tree.
	ErrDirectoryWalk = errors.New("failed to walk directory tree")
)
