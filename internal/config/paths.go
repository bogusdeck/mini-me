package config

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	DirName   = "mini-me"
	DBName    = "mini-me.db"
	EnvDBPath = "MINI_ME_DB_PATH"
)

// DefaultConfigDir returns the base configuration directory for mini-me.
// It uses os.UserConfigDir()/mini-me unless overridden.
func DefaultConfigDir() (string, error) {
	userConfig, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("failed to locate user config dir: %w", err)
	}
	return filepath.Join(userConfig, DirName), nil
}

// DBPath returns the path to mini-me.db.
// If MINI_ME_DB_PATH environment variable is set, it takes precedence.
// Otherwise, it returns os.UserConfigDir()/mini-me/mini-me.db.
func DBPath() (string, error) {
	if envPath := os.Getenv(EnvDBPath); envPath != "" {
		return envPath, nil
	}
	dir, err := DefaultConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, DBName), nil
}

// EnsureDir creates the directory if it does not exist with 0700 permissions.
func EnsureDir(dir string) error {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}
	return nil
}
