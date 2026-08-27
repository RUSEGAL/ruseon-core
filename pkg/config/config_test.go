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
	err := os.WriteFile(configPath, []byte(initialYaml), 0600)
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
	if cfg.Auth.Secret != "test_secret" {
		t.Errorf("expected secret test_secret, got %s", cfg.Auth.Secret)
	}
	if cfg.Server.GCPercent != 50 {
		t.Errorf("expected default GCPercent 50, got %d", cfg.Server.GCPercent)
	}
	if cfg.Server.GRPC.Port != 50051 {
		t.Errorf("expected default GRPC.Port 50051, got %d", cfg.Server.GRPC.Port)
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

func TestLoad_GenerateSecret(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "empty_secret.yml")

	initialYaml := `
server:
  port: 8080
`
	err := os.WriteFile(configPath, []byte(initialYaml), 0600)
	if err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if cfg.Auth.Secret == "" {
		t.Errorf("expected generated secret, got empty")
	}

	// Verify it was persisted to disk
	cfgReload, err := Load(configPath)
	if err != nil {
		t.Fatalf("Reload failed: %v", err)
	}
	if cfgReload.Auth.Secret != cfg.Auth.Secret {
		t.Errorf("expected persisted secret %s, got %s", cfg.Auth.Secret, cfgReload.Auth.Secret)
	}
}

func TestNewDefaultConfig(t *testing.T) {
	cfg := NewDefaultConfig()
	if cfg.Server.Port != 8080 {
		t.Errorf("expected port 8080, got %d", cfg.Server.Port)
	}
	if cfg.Server.GCPercent != 50 {
		t.Errorf("expected GCPercent 50, got %d", cfg.Server.GCPercent)
	}
	if cfg.Server.HLS.LiveSegmentsInMemory != 3 {
		t.Errorf("expected HLS segments 3, got %d", cfg.Server.HLS.LiveSegmentsInMemory)
	}
	if cfg.Server.GRPC.Port != 50051 {
		t.Errorf("expected gRPC port 50051, got %d", cfg.Server.GRPC.Port)
	}
	if len(cfg.GlobalTags) != 2 {
		t.Errorf("expected 2 default global tags, got %d", len(cfg.GlobalTags))
	}
}

func TestLoad_NonExistentFileAutoGenerate(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "auto_generated.yaml")

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("expected Load to auto-generate config without error, got: %v", err)
	}

	if cfg.Server.Port != 8080 {
		t.Errorf("expected port 8080, got %d", cfg.Server.Port)
	}
	if cfg.Auth.Secret == "" {
		t.Errorf("expected auto-generated JWT secret, got empty")
	}

	// Verify file was written to disk
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatalf("expected file to be created on disk at %s", configPath)
	}

	// Reload and verify
	reloaded, err := Load(configPath)
	if err != nil {
		t.Fatalf("failed to reload auto-generated config: %v", err)
	}
	if reloaded.Auth.Secret != cfg.Auth.Secret {
		t.Errorf("expected preserved secret %s, got %s", cfg.Auth.Secret, reloaded.Auth.Secret)
	}
}

func TestLoad_Errors(t *testing.T) {
	t.Run("invalid yaml", func(t *testing.T) {
		tempDir := t.TempDir()
		invalidPath := filepath.Join(tempDir, "invalid.yml")
		_ = os.WriteFile(invalidPath, []byte("server: [unclosed"), 0600)

		_, err := Load(invalidPath)
		if err == nil {
			t.Errorf("expected error for invalid yaml")
		}
	})
}

func TestSave_Error(t *testing.T) {
	cfg := &Config{}
	err := cfg.Save("/non_existent_dir_12345/sub/config.yml")
	if err == nil {
		t.Errorf("expected error when saving to invalid directory")
	}
}

func TestCameraConfig_Clone(t *testing.T) {
	orig := CameraConfig{
		ID:        "cam1",
		URL:       "rtsp://example.com/live",
		Record:    true,
		LazyHLS:   true,
		Transport: "tcp",
		Tags:      []string{"tag1", "tag2"},
		DisableHistory: []DisableRecord{
			{Timestamp: "2026-08-21T10:00:00Z", Action: "disable", Reason: "maintenance"},
		},
		RecordHistory: []DisableRecord{
			{Timestamp: "2026-08-21T10:00:00Z", Action: "enable", Reason: "manual"},
		},
	}

	clone := orig.Clone()

	// Verify deep copy
	if clone.ID != orig.ID || clone.URL != orig.URL {
		t.Fatalf("expected identical values in clone")
	}

	// Mutate slices in clone to verify isolation
	clone.Tags[0] = "mutated_tag"
	if orig.Tags[0] == "mutated_tag" {
		t.Errorf("Tags slice was not deeply copied")
	}

	clone.DisableHistory[0].Reason = "mutated_reason"
	if orig.DisableHistory[0].Reason == "mutated_reason" {
		t.Errorf("DisableHistory slice was not deeply copied")
	}

	clone.RecordHistory[0].Reason = "mutated_record"
	if orig.RecordHistory[0].Reason == "mutated_record" {
		t.Errorf("RecordHistory slice was not deeply copied")
	}

	// Test nil slices clone
	empty := CameraConfig{ID: "empty"}
	emptyClone := empty.Clone()
	if emptyClone.ID != "empty" || emptyClone.Tags != nil {
		t.Errorf("expected nil slices on empty clone")
	}
}

