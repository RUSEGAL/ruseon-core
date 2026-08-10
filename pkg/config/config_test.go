package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAndSaveConfig(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yml")

	// Create a dummy config file
	initialYaml := `
server:
  port: 8080
auth:
  secret: "test_secret"
`
	err := os.WriteFile(configPath, []byte(initialYaml), 0644)
	if err != nil {
		t.Fatalf("failed to write dummy config: %v", err)
	}

	// Load the config
	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if cfg.Server.Port != 8080 {
		t.Errorf("expected port 8080, got %d", cfg.Server.Port)
	}
	if cfg.Auth.Secret == "" {
		t.Errorf("Load() should generate JWT secret if missing")
	}

	// Modify and save
	cfg.Server.Port = 9090
	err = cfg.Save(configPath)
	if err != nil {
		t.Fatalf("Save() failed: %v", err)
	}

	// Load again
	cfg2, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() after save failed: %v", err)
	}

	if cfg2.Server.Port != 9090 {
		t.Errorf("expected port 9090, got %d", cfg2.Server.Port)
	}
	if cfg2.Auth.Secret != cfg.Auth.Secret {
		t.Errorf("expected JWT secret to be preserved, got %s vs %s", cfg2.Auth.Secret, cfg.Auth.Secret)
	}
}
