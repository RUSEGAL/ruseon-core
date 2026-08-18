package api

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"github.com/RUSEGAL/ruseon-core/internal/models"
	"github.com/RUSEGAL/ruseon-core/pkg/auth"
	"github.com/RUSEGAL/ruseon-core/pkg/config"
	"github.com/RUSEGAL/ruseon-core/pkg/registry"
	"github.com/RUSEGAL/ruseon-core/pkg/storage"
	"github.com/RUSEGAL/ruseon-core/pkg/storage/localfs"
	"github.com/RUSEGAL/ruseon-core/internal/stream"
)

func setupTestRouter(t *testing.T) (*gin.Engine, *Handler, registry.StateStore) {
	gin.SetMode(gin.TestMode)
	tempDir := t.TempDir()
	store, _ := storage.NewStorage(filepath.Join(tempDir, "db"))
	registry.CurrentStateStore = store
	registry.CurrentBlobStore = localfs.NewLocalFS(tempDir)
	
	cfg := &config.Config{}
	cfg.Auth.Secret = "secret"
	
	manager := stream.NewManager()
	
	handler := NewHandler(manager, cfg, store)
	
	router := gin.New()
	// (мы не используем SetupRouter здесь во всех тестах, но там, где нужно, можно использовать auth)
	
	return router, handler, store
}

func TestLivenessCheck(t *testing.T) {
	router, handler, store := setupTestRouter(t)
	defer store.Close()
	
	router.GET("/livez", handler.LivenessCheck)
	
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/livez", nil)
	router.ServeHTTP(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestReadinessCheck_Healthy(t *testing.T) {
	router, handler, store := setupTestRouter(t)
	defer store.Close()

	router.GET("/readyz", handler.ReadinessCheck)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/readyz", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	var res struct {
		Status     string            `json:"status"`
		Components map[string]string `json:"components"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to decode json: %v", err)
	}

	if res.Status != "ready" {
		t.Errorf("expected status 'ready', got '%s'", res.Status)
	}
	if res.Components["database"] != "ok" || res.Components["storage"] != "ok" || res.Components["stream_manager"] != "ok" {
		t.Errorf("expected all components ok, got: %+v", res.Components)
	}
}

func TestReadinessCheck_UnhealthyDatabase(t *testing.T) {
	router, handler, store := setupTestRouter(t)
	// Close store to make database unavailable
	store.Close()

	router.GET("/readyz", handler.ReadinessCheck)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/readyz", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 Service Unavailable, got %d: %s", w.Code, w.Body.String())
	}

	var res struct {
		Status     string            `json:"status"`
		Error      string            `json:"error"`
		Components map[string]string `json:"components"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &res)
	if res.Status != "not_ready" {
		t.Errorf("expected status 'not_ready', got '%s'", res.Status)
	}
	if res.Components["database"] == "ok" {
		t.Errorf("expected database component to report error, got ok")
	}
}

func TestReadinessCheck_UnhealthyStorage(t *testing.T) {
	router, handler, store := setupTestRouter(t)
	defer store.Close()

	oldBlobStore := registry.CurrentBlobStore
	registry.CurrentBlobStore = nil
	defer func() { registry.CurrentBlobStore = oldBlobStore }()

	router.GET("/readyz", handler.ReadinessCheck)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/readyz", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 Service Unavailable, got %d: %s", w.Code, w.Body.String())
	}
}

func TestReadinessCheck_UninitializedStreamManager(t *testing.T) {
	tempDir := t.TempDir()
	store, _ := storage.NewStorage(filepath.Join(tempDir, "db"))
	defer store.Close()
	registry.CurrentStateStore = store
	registry.CurrentBlobStore = localfs.NewLocalFS(tempDir)

	cfg := &config.Config{}
	handler := NewHandler(nil, cfg, store) // nil StreamManager

	router := gin.New()
	router.GET("/readyz", handler.ReadinessCheck)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/readyz", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 Service Unavailable, got %d: %s", w.Code, w.Body.String())
	}
}

func TestLogin(t *testing.T) {
	router, _, store := setupTestRouter(t)
	defer store.Close()
	
	// В тестах логина нам нужно использовать SetupRouter, чтобы был корректный роутинг с auth.Login
	cfg := &config.Config{}
	cfg.Auth.Secret = "secret"
	
	registry.CurrentStateStore = store
	hash, _ := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.DefaultCost)
	_ = store.SaveUser(&models.User{
		Username:     "admin",
		PasswordHash: string(hash),
		Role:         models.RoleAdmin,
	})

	authenticator := auth.NewLocalAuthenticator(cfg)
	
	router.POST("/login", authenticator.Login)
	
	// Valid login
	validBody := []byte(`{"username":"admin","password":"password"}`)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/login", bytes.NewBuffer(validBody))
	router.ServeHTTP(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for valid login, got %d", w.Code)
	}
	
	// Invalid login
	invalidBody := []byte(`{"username":"admin","password":"wrong"}`)
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("POST", "/login", bytes.NewBuffer(invalidBody))
	router.ServeHTTP(w2, req2)
	
	if w2.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for invalid login, got %d", w2.Code)
	}
}

func TestCameraCRUD(t *testing.T) {
	router, handler, store := setupTestRouter(t)
	defer store.Close()
	
	router.GET("/api/cameras", handler.GetCameras)
	router.POST("/api/cameras", handler.AddCamera)
	router.PUT("/api/cameras/:id", handler.EditCamera)
	router.DELETE("/api/cameras/:id", handler.DeleteCamera)
	
	// 1. Add Camera
	camJSON := []byte(`{"id":"cam1","url":"rtsp://test","record":true,"disabled":true}`) // disabled so it doesn't try to connect
	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest("POST", "/api/cameras", bytes.NewBuffer(camJSON))
	router.ServeHTTP(w1, req1)
	
	if w1.Code != http.StatusOK {
		t.Fatalf("expected 200 for add camera, got %d: %s", w1.Code, w1.Body.String())
	}
	
	// 2. Get Cameras
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/api/cameras", nil)
	router.ServeHTTP(w2, req2)
	
	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200 for get cameras")
	}
	var cams []map[string]interface{}
	json.Unmarshal(w2.Body.Bytes(), &cams)
	if len(cams) != 1 || cams[0]["id"] != "cam1" {
		t.Errorf("expected 1 camera 'cam1', got: %v", cams)
	}
	
	// 3. Edit Camera
	editJSON := []byte(`{"id":"cam1","url":"rtsp://updated","disabled":true}`)
	w3 := httptest.NewRecorder()
	req3, _ := http.NewRequest("PUT", "/api/cameras/cam1", bytes.NewBuffer(editJSON))
	router.ServeHTTP(w3, req3)
	
	if w3.Code != http.StatusOK {
		t.Fatalf("expected 200 for edit camera")
	}
	
	// 4. Delete Camera
	w4 := httptest.NewRecorder()
	req4, _ := http.NewRequest("DELETE", "/api/cameras/cam1", nil)
	router.ServeHTTP(w4, req4)
	
	if w4.Code != http.StatusOK {
		t.Fatalf("expected 200 for delete camera")
	}
	
	// Verify deleted
	w5 := httptest.NewRecorder()
	req5, _ := http.NewRequest("GET", "/api/cameras", nil)
	router.ServeHTTP(w5, req5)
	var camsAfter []map[string]interface{}
	json.Unmarshal(w5.Body.Bytes(), &camsAfter)
	if len(camsAfter) != 0 {
		t.Errorf("expected 0 cameras, got %d", len(camsAfter))
	}
}

func TestTagCRUD(t *testing.T) {
	router, handler, store := setupTestRouter(t)
	defer store.Close()
	
	router.GET("/api/tags", handler.GetTags)
	router.POST("/api/tags", handler.AddTag)
	router.PUT("/api/tags/:id", handler.EditTag)
	router.DELETE("/api/tags/:id", handler.DeleteTag)
	
	// 1. Add Tag
	tagJSON := []byte(`{"id":"tag1","name":"Test","color":"#000000"}`)
	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest("POST", "/api/tags", bytes.NewBuffer(tagJSON))
	router.ServeHTTP(w1, req1)
	
	if w1.Code != http.StatusOK {
		t.Fatalf("expected 200 for add tag, got %d", w1.Code)
	}
	
	// 2. Get Tags
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/api/tags", nil)
	router.ServeHTTP(w2, req2)
	
	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200 for get tags")
	}
	var tags []map[string]interface{}
	json.Unmarshal(w2.Body.Bytes(), &tags)
	if len(tags) != 1 || tags[0]["id"] != "tag1" {
		t.Errorf("expected 1 tag 'tag1', got: %v", tags)
	}
	
	// 3. Edit Tag
	editJSON := []byte(`{"name":"Updated","color":"#FFFFFF"}`)
	w3 := httptest.NewRecorder()
	req3, _ := http.NewRequest("PUT", "/api/tags/tag1", bytes.NewBuffer(editJSON))
	router.ServeHTTP(w3, req3)
	
	if w3.Code != http.StatusOK {
		t.Fatalf("expected 200 for edit tag")
	}
	
	// 4. Delete Tag
	w4 := httptest.NewRecorder()
	req4, _ := http.NewRequest("DELETE", "/api/tags/tag1", nil)
	router.ServeHTTP(w4, req4)
	
	if w4.Code != http.StatusOK {
		t.Fatalf("expected 200 for delete tag")
	}
}

func TestServerStatsAndBackup(t *testing.T) {
	router, handler, store := setupTestRouter(t)
	defer store.Close()
	
	router.GET("/api/stats", handler.GetServerStats)
	router.GET("/api/backup/export", handler.ExportBackupJSON)
	
	// 1. Stats
	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest("GET", "/api/stats", nil)
	router.ServeHTTP(w1, req1)
	
	if w1.Code != http.StatusOK {
		t.Fatalf("expected 200 for stats, got %d", w1.Code)
	}
	
	// 2. Export Backup
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/api/backup/export", nil)
	router.ServeHTTP(w2, req2)
	
	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200 for backup export, got %d", w2.Code)
	}
	if w2.Body.Len() == 0 {
		t.Fatalf("expected non-empty backup")
	}
}

func TestHLSAndArchive(t *testing.T) {
	router, handler, store := setupTestRouter(t)
	defer store.Close()
	
	router.GET("/api/cameras/:id/archive", handler.GetCameraArchive)
	router.GET("/hls/:id/index.m3u8", handler.GetHLSPlaylist)
	router.GET("/hls/:id/:segment", handler.GetHLSSegment)
	
	// Test Camera Archive for non-existent camera
	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest("GET", "/api/cameras/cam1/archive", nil)
	router.ServeHTTP(w1, req1)
	
	if w1.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for non-existent camera archive, got %d", w1.Code)
	}
	
	// Test HLS for non-existent stream
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/hls/cam1/index.m3u8", nil)
	router.ServeHTTP(w2, req2)
	
	if w2.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for non-existent stream playlist, got %d", w2.Code)
	}
	
	w3 := httptest.NewRecorder()
	req3, _ := http.NewRequest("GET", "/hls/cam1/0.ts", nil)
	router.ServeHTTP(w3, req3)
	
	if w3.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for non-existent stream segment, got %d", w3.Code)
	}
}

func TestFolderCRUD(t *testing.T) {
	router, handler, store := setupTestRouter(t)
	defer store.Close()
	
	router.GET("/api/folders", handler.GetFolders)
	router.POST("/api/folders", handler.AddFolder)
	router.DELETE("/api/folders/:id", handler.DeleteFolder)
	
	// 1. Add Folder
	folderJSON := []byte(`{"id":"folder1","name":"Main Area"}`)
	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest("POST", "/api/folders", bytes.NewBuffer(folderJSON))
	router.ServeHTTP(w1, req1)
	
	if w1.Code != http.StatusOK {
		t.Fatalf("expected 200 for add folder, got %d: %s", w1.Code, w1.Body.String())
	}
	
	// 2. Get Folders
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/api/folders", nil)
	router.ServeHTTP(w2, req2)
	
	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200 for get folders")
	}
	var folders []map[string]interface{}
	json.Unmarshal(w2.Body.Bytes(), &folders)
	if len(folders) != 1 || folders[0]["id"] != "folder1" {
		t.Errorf("expected 1 folder 'folder1', got: %v", folders)
	}
	
	// 3. Delete Folder
	w3 := httptest.NewRecorder()
	req3, _ := http.NewRequest("DELETE", "/api/folders/folder1", nil)
	router.ServeHTTP(w3, req3)
	
	if w3.Code != http.StatusOK {
		t.Fatalf("expected 200 for delete folder")
	}
}

func TestImportBackupJSON(t *testing.T) {
	router, handler, store := setupTestRouter(t)
	defer store.Close()
	
	router.POST("/api/backup/import", handler.ImportBackupJSON)
	
	backupData := []byte(`{"cameras":[{"id":"import1","url":"rtsp://import"}],"tags":[{"id":"tag1","name":"T"}]}`)
	
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("backup", "backup.json")
	part.Write(backupData)
	writer.Close()
	
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/backup/import", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	router.ServeHTTP(w, req)
	
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for import, got %d", w.Code)
	}
	
	// check DB
	cam, err := store.GetCamera("import1")
	if err != nil || cam.URL != "rtsp://import" {
		t.Fatalf("camera not imported correctly")
	}
}

func TestArchiveEndpoints(t *testing.T) {
	router, handler, store := setupTestRouter(t)
	defer store.Close()
	
	router.GET("/hls/:id/archive.m3u8", handler.GetArchiveHLSPlaylist)
	router.GET("/hls/:id/archive/:segment", handler.GetArchiveHLSSegment)
	router.GET("/api/cameras/:id/export", handler.ExportCameraArchive)
	
	// Just test that they return 400 or 404 for invalid data
	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest("GET", "/hls/cam1/archive.m3u8?start=0&end=1000", nil)
	router.ServeHTTP(w1, req1)
	if w1.Code != http.StatusNotFound && w1.Code != http.StatusBadRequest {
		t.Errorf("expected 404 or 400, got %d", w1.Code)
	}
	
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/api/cameras/cam1/export?start=0&end=1000", nil)
	router.ServeHTTP(w2, req2)
	if w2.Code != http.StatusNotFound && w2.Code != http.StatusBadRequest {
		t.Errorf("expected 404 or 400, got %d", w2.Code)
	}
}
