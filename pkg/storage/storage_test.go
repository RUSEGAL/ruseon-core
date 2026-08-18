package storage

import (
	"context"
	"os"
	"testing"

	"github.com/RUSEGAL/ruseon-core/internal/models"
	"github.com/RUSEGAL/ruseon-core/pkg/config"
)

func TestStorage_CameraCRUD(t *testing.T) {
	tempDir := t.TempDir()
	store, err := NewStorage(tempDir)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer store.Close()

	cam := &config.CameraConfig{
		ID:     "cam1",
		URL:    "rtsp://test",
		Record: true,
	}

	// 1. Create (Save)
	if err := store.SaveCamera(cam); err != nil {
		t.Fatalf("failed to save camera: %v", err)
	}

	// 2. Read (Get)
	fetchedCam, err := store.GetCamera("cam1")
	if err != nil {
		t.Fatalf("failed to get camera: %v", err)
	}
	if fetchedCam.ID != "cam1" || fetchedCam.URL != "rtsp://test" || fetchedCam.Record != true {
		t.Errorf("fetched camera mismatch: %+v", fetchedCam)
	}

	// 3. Update (UpdateCameraTx)
	err = store.UpdateCameraTx("cam1", func(c *config.CameraConfig) bool {
		c.URL = "rtsp://updated"
		c.Disabled = true
		return true // trigger save
	})
	if err != nil {
		t.Fatalf("failed to update camera: %v", err)
	}

	updatedCam, err := store.GetCamera("cam1")
	if err != nil {
		t.Fatalf("failed to get updated camera: %v", err)
	}
	if updatedCam.URL != "rtsp://updated" || !updatedCam.Disabled {
		t.Errorf("camera not updated correctly: %+v", updatedCam)
	}

	// 4. List
	cams, err := store.ListCameras()
	if err != nil {
		t.Fatalf("failed to list cameras: %v", err)
	}
	if len(cams) != 1 {
		t.Errorf("expected 1 camera, got %d", len(cams))
	}
	if cams[0].ID != "cam1" {
		t.Errorf("expected cam1 in list, got %s", cams[0].ID)
	}

	// 5. Delete
	if err := store.DeleteCamera("cam1"); err != nil {
		t.Fatalf("failed to delete camera: %v", err)
	}

	// Verify deletion
	_, err = store.GetCamera("cam1")
	if err == nil {
		t.Errorf("expected error when getting deleted camera, got nil")
	}
	camsAfter, _ := store.ListCameras()
	if len(camsAfter) != 0 {
		t.Errorf("expected 0 cameras after deletion, got %d", len(camsAfter))
	}
}

func TestStorage_TagCRUD(t *testing.T) {
	tempDir := t.TempDir()
	store, err := NewStorage(tempDir)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer store.Close()

	tag := &config.TagConfig{
		ID:    "tag1",
		Name:  "Test Tag",
		Color: "#FF0000",
	}

	// 1. Create (Save)
	if err := store.SaveTag(tag); err != nil {
		t.Fatalf("failed to save tag: %v", err)
	}

	// 2. Read (Get)
	fetchedTag, err := store.GetTag("tag1")
	if err != nil {
		t.Fatalf("failed to get tag: %v", err)
	}
	if fetchedTag.Name != "Test Tag" || fetchedTag.Color != "#FF0000" {
		t.Errorf("fetched tag mismatch: %+v", fetchedTag)
	}

	// 3. List
	tags, err := store.ListTags()
	if err != nil {
		t.Fatalf("failed to list tags: %v", err)
	}
	if len(tags) != 1 {
		t.Errorf("expected 1 tag, got %d", len(tags))
	}

	// 4. Delete
	if err := store.DeleteTag("tag1"); err != nil {
		t.Fatalf("failed to delete tag: %v", err)
	}

	// Verify deletion
	_, err = store.GetTag("tag1")
	if err == nil {
		t.Errorf("expected error when getting deleted tag, got nil")
	}
}

func TestStorage_ExportImportJSON(t *testing.T) {
	tempDir := t.TempDir()
	store, err := NewStorage(tempDir)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	cam := &config.CameraConfig{ID: "cam1", URL: "rtsp://test"}
	_ = store.SaveCamera(cam)
	tag := &config.TagConfig{ID: "tag1", Name: "Test"}
	_ = store.SaveTag(tag)

	// Export
	jsonBytes, err := store.ExportJSON()
	if err != nil {
		t.Fatalf("failed to export: %v", err)
	}

	// Ensure exported data has some size
	if len(jsonBytes) < 10 {
		t.Fatalf("exported json is too small")
	}

	// Close old store, open a new empty one
	store.Close()

	store2Dir := t.TempDir()
	store2, _ := NewStorage(store2Dir)
	defer store2.Close()

	// Ensure empty
	cams, _ := store2.ListCameras()
	if len(cams) != 0 {
		t.Fatalf("expected empty store")
	}

	// Import
	if err := store2.ImportJSON(jsonBytes); err != nil {
		t.Fatalf("failed to import: %v", err)
	}

	// Verify
	cams, _ = store2.ListCameras()
	if len(cams) != 1 || cams[0].ID != "cam1" {
		t.Errorf("expected cam1 to be imported, got %v", cams)
	}

	tags, _ := store2.ListTags()
	if len(tags) != 1 || tags[0].ID != "tag1" {
		t.Errorf("expected tag1 to be imported, got %v", tags)
	}
}

func TestStorage_BackupBadger(t *testing.T) {
	tempDir := t.TempDir()
	store, _ := NewStorage(tempDir)
	defer store.Close()

	_ = store.SaveCamera(&config.CameraConfig{ID: "cam1"})

	backupPath := tempDir + "/backup.bak"
	f, err := os.Create(backupPath)
	if err != nil {
		t.Fatalf("failed to create backup file: %v", err)
	}
	defer f.Close()

	if err := store.BackupBadger(f); err != nil {
		t.Fatalf("failed to backup: %v", err)
	}

	info, err := os.Stat(backupPath)
	if err != nil {
		t.Fatalf("backup file not found: %v", err)
	}
	if info.Size() == 0 {
		t.Fatalf("backup file is empty")
	}
}

func TestStorage_FolderCRUD(t *testing.T) {
	tempDir := t.TempDir()
	store, err := NewStorage(tempDir)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer store.Close()

	folder := &config.FolderConfig{
		ID:   "folder1",
		Name: "Test Folder",
	}

	if err := store.SaveFolder(folder); err != nil {
		t.Fatalf("failed to save folder: %v", err)
	}

	fetchedFolder, err := store.GetFolder("folder1")
	if err != nil {
		t.Fatalf("failed to get folder: %v", err)
	}
	if fetchedFolder.Name != "Test Folder" {
		t.Errorf("fetched folder mismatch: %+v", fetchedFolder)
	}

	folders, err := store.ListFolders()
	if err != nil {
		t.Fatalf("failed to list folders: %v", err)
	}
	if len(folders) != 1 {
		t.Errorf("expected 1 folder, got %d", len(folders))
	}

	if err := store.DeleteFolder("folder1"); err != nil {
		t.Fatalf("failed to delete folder: %v", err)
	}

	_, err = store.GetFolder("folder1")
	if err == nil {
		t.Errorf("expected error when getting deleted folder, got nil")
	}
}

func TestStorage_MigrateFromConfig(t *testing.T) {
	tempDir := t.TempDir()
	store, _ := NewStorage(tempDir)
	defer store.Close()

	cfg := &config.Config{}
	cfg.Cameras = []config.CameraConfig{
		{ID: "cam_migrate_1", URL: "rtsp://migrate1"},
		{ID: "cam_migrate_2", URL: "rtsp://migrate2"},
	}
	cfg.GlobalTags = []config.TagConfig{
		{ID: "tag_migrate_1", Name: "Tag1"},
	}

	if err := store.MigrateFromConfig(cfg); err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	cams, _ := store.ListCameras()
	if len(cams) != 2 {
		t.Fatalf("expected 2 cameras after migration, got %d", len(cams))
	}

	tags, _ := store.ListTags()
	if len(tags) != 1 {
		t.Fatalf("expected 1 tag after migration, got %d", len(tags))
	}

	// Ensure subsequent migration doesn't run again if db is populated
	cfg.Cameras = []config.CameraConfig{
		{ID: "cam_migrate_3"},
	}
	if err := store.MigrateFromConfig(cfg); err != nil {
		t.Fatalf("migration failed: %v", err)
	}
	cams, _ = store.ListCameras()
	if len(cams) != 2 {
		t.Fatalf("expected 2 cameras since migration should be skipped, got %d", len(cams))
	}
}

func TestStorage_UserCRUD(t *testing.T) {
	tempDir := t.TempDir()
	store, err := NewStorage(tempDir)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer store.Close()

	user := &models.User{
		Username:     "testuser",
		PasswordHash: "testhash",
		Role:         models.RoleOperator,
	}

	if err := store.SaveUser(user); err != nil {
		t.Fatalf("failed to save user: %v", err)
	}

	fetchedUser, err := store.GetUser("testuser")
	if err != nil {
		t.Fatalf("failed to get user: %v", err)
	}
	if fetchedUser.Username != "testuser" || fetchedUser.Role != models.RoleOperator {
		t.Errorf("fetched user mismatch: %+v", fetchedUser)
	}

	_, err = store.GetUser("nonexistent")
	if err == nil {
		t.Errorf("expected error for nonexistent user, got nil")
	} else if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestStorage_Sync(t *testing.T) {
	tempDir := t.TempDir()
	store, err := NewStorage(tempDir)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	_ = store.SaveCamera(&config.CameraConfig{ID: "cam_sync_test", URL: "rtsp://sync"})

	if err := store.Sync(); err != nil {
		t.Fatalf("expected Sync to succeed, got %v", err)
	}

	store.Close()

	if err := store.Sync(); err == nil {
		t.Errorf("expected Sync on closed storage to fail, got nil")
	}
}

func TestStorage_Durability_SuddenTermination_Reopen(t *testing.T) {
	tempDir := t.TempDir()

	// Phase 1: Initialize, write configurations and users, sync to disk
	store1, err := NewStorage(tempDir)
	if err != nil {
		t.Fatalf("failed to open storage 1: %v", err)
	}

	cam := &config.CameraConfig{
		ID:            "cam_durable_1",
		URL:           "rtsp://durable.local/live",
		Record:        true,
		RetentionDays: 30,
	}
	if err := store1.SaveCamera(cam); err != nil {
		t.Fatalf("failed to save camera: %v", err)
	}

	tag := &config.TagConfig{
		ID:    "tag_durable_1",
		Name:  "Security HQ",
		Color: "#00FF00",
	}
	if err := store1.SaveTag(tag); err != nil {
		t.Fatalf("failed to save tag: %v", err)
	}

	user := &models.User{
		Username:     "admin_durable",
		PasswordHash: "secure_hash_123",
		Role:         models.RoleAdmin,
	}
	if err := store1.SaveUser(user); err != nil {
		t.Fatalf("failed to save user: %v", err)
	}

	if err := store1.Sync(); err != nil {
		t.Fatalf("failed to sync storage: %v", err)
	}

	// Simulate unexpected termination / restart by closing store1 handle
	if err := store1.Close(); err != nil {
		t.Fatalf("failed to close store1: %v", err)
	}

	// Phase 2: Open completely new Storage instance on the same directory
	store2, err := NewStorage(tempDir)
	if err != nil {
		t.Fatalf("failed to reopen storage 2 after simulated restart: %v", err)
	}
	defer store2.Close()

	// Verify Camera durability
	fetchedCam, err := store2.GetCamera("cam_durable_1")
	if err != nil {
		t.Fatalf("camera lost after reopen: %v", err)
	}
	if fetchedCam.URL != "rtsp://durable.local/live" || fetchedCam.RetentionDays != 30 {
		t.Errorf("camera data corrupted after reopen: %+v", fetchedCam)
	}

	// Verify Tag durability
	fetchedTag, err := store2.GetTag("tag_durable_1")
	if err != nil {
		t.Fatalf("tag lost after reopen: %v", err)
	}
	if fetchedTag.Name != "Security HQ" {
		t.Errorf("tag data corrupted after reopen: %+v", fetchedTag)
	}

	// Verify User durability
	fetchedUser, err := store2.GetUser("admin_durable")
	if err != nil {
		t.Fatalf("user lost after reopen: %v", err)
	}
	if fetchedUser.Username != "admin_durable" || fetchedUser.Role != models.RoleAdmin {
		t.Errorf("user data corrupted after reopen: %+v", fetchedUser)
	}
}

func TestStorage_MigrateFromConfig_NilAndIdempotent(t *testing.T) {
	tempDir := t.TempDir()
	store, err := NewStorage(tempDir)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer store.Close()

	// Nil config should succeed safely without panic
	if err := store.MigrateFromConfig(nil); err != nil {
		t.Errorf("expected nil config migration to succeed, got %v", err)
	}

	// Migration with multiple cameras and tags
	cfg := &config.Config{
		Cameras: []config.CameraConfig{
			{ID: "c1", URL: "rtsp://c1"},
			{ID: "c2", URL: "rtsp://c2"},
		},
		GlobalTags: []config.TagConfig{
			{ID: "t1", Name: "Tag1"},
		},
	}

	if err := store.MigrateFromConfig(cfg); err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	cams, err := store.ListCameras()
	if err != nil || len(cams) != 2 {
		t.Fatalf("expected 2 cameras, got %d (err: %v)", len(cams), err)
	}

	tags, err := store.ListTags()
	if err != nil || len(tags) != 1 {
		t.Fatalf("expected 1 tag, got %d (err: %v)", len(tags), err)
	}

	// Re-running migration must be a no-op
	if err := store.MigrateFromConfig(cfg); err != nil {
		t.Fatalf("subsequent migration failed: %v", err)
	}
	camsAfter, _ := store.ListCameras()
	if len(camsAfter) != 2 {
		t.Fatalf("expected camera count to remain 2, got %d", len(camsAfter))
	}
}

func TestStorage_UserCRUD_And_Ping(t *testing.T) {
	tempDir := t.TempDir()
	store, err := NewStorage(tempDir)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer store.Close()

	// 1. Ping
	if err := store.Ping(context.Background()); err != nil {
		t.Errorf("expected ping to succeed, got %v", err)
	}

	// 2. HasUsers initial
	hasUsers, err := store.HasUsers()
	if err != nil || hasUsers {
		t.Fatalf("expected hasUsers to be false, got %v (err: %v)", hasUsers, err)
	}

	// 3. Save Users
	u1 := &models.User{Username: "user1", PasswordHash: "hash1", Role: models.RoleAdmin}
	u2 := &models.User{Username: "user2", PasswordHash: "hash2", Role: models.RoleViewer}
	if err := store.SaveUser(u1); err != nil {
		t.Fatalf("failed to save u1: %v", err)
	}
	if err := store.SaveUser(u2); err != nil {
		t.Fatalf("failed to save u2: %v", err)
	}

	// 4. HasUsers now true
	hasUsers, err = store.HasUsers()
	if err != nil || !hasUsers {
		t.Fatalf("expected hasUsers to be true, got %v", hasUsers)
	}

	// 5. List Users
	users, err := store.ListUsers()
	if err != nil || len(users) != 2 {
		t.Fatalf("expected 2 users, got %d (err: %v)", len(users), err)
	}

	// 6. Delete Users
	if err := store.DeleteUser("user1"); err != nil {
		t.Fatalf("failed to delete user1: %v", err)
	}
	if err := store.DeleteUser("user2"); err != nil {
		t.Fatalf("failed to delete user2: %v", err)
	}

	usersAfter, err := store.ListUsers()
	if err != nil || len(usersAfter) != 0 {
		t.Fatalf("expected 0 users after deletion, got %d", len(usersAfter))
	}
}
