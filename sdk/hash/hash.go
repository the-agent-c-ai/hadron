// Package hash provides file and directory hashing utilities.
package hash

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// File computes the SHA256 hash of a file.
func File(path string) (string, error) {
	//nolint:gosec // Path is from user config, not user input
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrFileOpen, err)
	}

	defer func() { _ = file.Close() }()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("%w: %w", ErrFileRead, err)
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// Path computes the SHA256 hash of a file or directory.
// For files: hash the content.
// For directories: hash the entire tree (paths + contents).
func Path(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrFileStat, err)
	}

	// Single file
	if !info.IsDir() {
		return File(path)
	}

	// Directory - hash entire tree
	return Directory(path)
}

// Directory recursively hashes a directory tree.
func Directory(dirPath string) (string, error) {
	hash := sha256.New()

	//nolint:gosec // Path is from user config, not user input
	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Get relative path for consistent hashing
		relPath, err := filepath.Rel(dirPath, path)
		if err != nil {
			return fmt.Errorf("%w: %w", ErrPathRelative, err)
		}

		// Write path to hash (for directory structure)
		if _, err := hash.Write([]byte(relPath)); err != nil {
			return fmt.Errorf("failed to write path to hash: %w", err)
		}

		// If it's a file, hash its content
		if !info.IsDir() {
			//nolint:gosec // Path is from user config, not user input
			file, err := os.Open(path)
			if err != nil {
				return fmt.Errorf("%w: %w", ErrFileOpen, err)
			}

			defer func() { _ = file.Close() }()

			if _, err := io.Copy(hash, file); err != nil {
				return fmt.Errorf("%w: %w", ErrFileRead, err)
			}
		}

		return nil
	})
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrDirectoryWalk, err)
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}
