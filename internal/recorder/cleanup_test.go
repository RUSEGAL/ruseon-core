package recorder

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/RUSEGAL/ruseon-core/v2/pkg/config"
	"github.com/RUSEGAL/ruseon-core/v2/pkg/storage"
	"github.com/RUSEGAL/ruseon-core/v2/pkg/registry"
	"github.com/RUSEGAL/ruseon-core/v2/pkg/storage/localfs"
)

func init() {
	registry.RegisterBlobStore(localfs.NewLocalFS(""))
}

func TestCleanupOldFiles(t *testing.T) {
	tempDir := t.TempDir()
	dbDir := filepath.Join(tempDir, "db")
	recordDir := filepath.Join(tempDir, "recordings")
	
	store, _ := storage.NewStorage(dbDir)
	defer store.Close()

	// Configure retention: global = 2 days, cam1 = 1 day, cam2 = 0 (forever)
	cfg := &config.Config{}
	cfg.Server.RecordRetentionDays = 2
	
	_ = store.SaveCamera(&config.CameraConfig{ID: "cam1", RetentionDays: 1})
	_ = store.SaveCamera(&config.CameraConfig{ID: "cam2", RetentionDays: 0}) // should fallback to global = 2
	// actually wait, the code says:
	// if cam.RetentionDays > 0 { camRetention[cam.ID] = cam.RetentionDays } else { camRetention[cam.ID] = globalRetention }
	// So cam2 will use global 2 days.
	
	_ = os.MkdirAll(filepath.Join(recordDir, "cam1"), 0755)
	_ = os.MkdirAll(filepath.Join(recordDir, "cam2"), 0755)

	// Create files
	cam1Old := filepath.Join(recordDir, "cam1", "old.mp4") // 2 days old (cam1 retention is 1 day -> should delete)
	cam1New := filepath.Join(recordDir, "cam1", "new.mp4") // now
	cam2Old := filepath.Join(recordDir, "cam2", "old.mp4") // 1.5 days old (cam2 retention is 2 days -> should keep)
	cam2VeryOld := filepath.Join(recordDir, "cam2", "vold.mp4") // 3 days old (cam2 retention is 2 days -> should delete)

	f1, _ := os.Create(cam1Old); f1.Close()
	f2, _ := os.Create(cam1New); f2.Close()
	f3, _ := os.Create(cam2Old); f3.Close()
	f4, _ := os.Create(cam2VeryOld); f4.Close()

	now := time.Now()
	os.Chtimes(cam1Old, now.Add(-48*time.Hour), now.Add(-48*time.Hour))
	os.Chtimes(cam2Old, now.Add(-36*time.Hour), now.Add(-36*time.Hour))
	os.Chtimes(cam2VeryOld, now.Add(-72*time.Hour), now.Add(-72*time.Hour))

	cleanupOldFiles(recordDir, cfg, store)

	// Verify
	if _, err := os.Stat(cam1Old); !os.IsNotExist(err) {
		t.Errorf("cam1Old should have been deleted")
	}
	if _, err := os.Stat(cam1New); os.IsNotExist(err) {
		t.Errorf("cam1New should have been kept")
	}
	if _, err := os.Stat(cam2Old); os.IsNotExist(err) {
		t.Errorf("cam2Old should have been kept")
	}
	if _, err := os.Stat(cam2VeryOld); !os.IsNotExist(err) {
		t.Errorf("cam2VeryOld should have been deleted")
	}
}

func TestStartCleanupTask(t *testing.T) {
	tempDir := t.TempDir()
	dbDir := filepath.Join(tempDir, "db")
	store, _ := storage.NewStorage(dbDir)
	defer store.Close()
	
	ctx, cancel := context.WithCancel(context.Background())
	cfg := &config.Config{}
	StartCleanupTask(ctx, tempDir, cfg, store)
	
	cancel()
	time.Sleep(10 * time.Millisecond)
}
