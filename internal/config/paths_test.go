package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"memex/internal/config"
)

func TestDBPathEnvOverride(t *testing.T) {
	expectedPath := "/custom/path/memex.db"
	t.Setenv(config.EnvDBPath, expectedPath)

	path, err := config.DBPath()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path != expectedPath {
		t.Errorf("expected %s, got %s", expectedPath, path)
	}
}

func TestDBPathDefault(t *testing.T) {
	t.Setenv(config.EnvDBPath, "")
	path, err := config.DBPath()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if filepath.Base(path) != config.DBName {
		t.Errorf("expected filename %s, got %s", config.DBName, filepath.Base(path))
	}
}

func TestEnsureDir(t *testing.T) {
	tmpDir := t.TempDir()
	targetDir := filepath.Join(tmpDir, "sub", "memex")

	err := config.EnsureDir(targetDir)
	if err != nil {
		t.Fatalf("failed to ensure dir: %v", err)
	}

	info, err := os.Stat(targetDir)
	if err != nil {
		t.Fatalf("stat failed: %v", err)
	}
	if !info.IsDir() {
		t.Errorf("expected %s to be directory", targetDir)
	}
}
