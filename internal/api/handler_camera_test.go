package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/RUSEGAL/ruseon-core/v2/pkg/config"
)

func TestHandler_CameraLifecycleInvariants(t *testing.T) {
	router, handler, store := setupTestRouter(t)
	defer store.Close()

	router.GET("/api/cameras", handler.GetCameras)
	router.POST("/api/cameras", handler.AddCamera)
	router.PUT("/api/cameras/:id", handler.EditCamera)
	router.DELETE("/api/cameras/:id", handler.DeleteCamera)

	// 1. Попытка добавить камеру с пустым ID -> 400 Bad Request
	emptyIDCam := []byte(`{"id":"","url":"rtsp://127.0.0.1:8554/cam1","disabled":true}`)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/cameras", bytes.NewBuffer(emptyIDCam))
	router.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty camera ID, got %d", w.Code)
	}

	// 2. Успешное добавление камеры -> 200 OK
	validCam := []byte(`{"id":"cam1","url":"rtsp://127.0.0.1:8554/cam1","disabled":false,"record":false,"lazyHLS":true}`)
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/cameras", bytes.NewBuffer(validCam))
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for valid camera, got %d: %s", w.Code, w.Body.String())
	}

	// Проверяем что стрим зарегистрирован в рантайме
	if !handler.manager.HasStream("cam1") {
		t.Fatalf("expected stream manager to have cam1 running")
	}

	// 3. Попытка добавить дубликат ID -> 409 Conflict
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/cameras", bytes.NewBuffer(validCam))
	router.ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409 Conflict when adding duplicate camera, got %d", w.Code)
	}

	// 4. Редактирование несуществующей камеры -> 404 Not Found
	nonExistentEdit := []byte(`{"url":"rtsp://127.0.0.1:8554/cam_none","disabled":true}`)
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("PUT", "/api/cameras/cam_none", bytes.NewBuffer(nonExistentEdit))
	router.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for non-existent camera edit, got %d", w.Code)
	}

	// 5. Отключение камеры через Edit (disabled=true) -> 200 OK
	disableEdit := []byte(`{"url":"rtsp://127.0.0.1:8554/cam1","disabled":true,"disableReason":"technical"}`)
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("PUT", "/api/cameras/cam1", bytes.NewBuffer(disableEdit))
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for disable edit, got %d", w.Code)
	}

	// Проверяем что поток остановлен в менеджере
	if handler.manager.HasStream("cam1") {
		t.Fatalf("expected cam1 stream to be stopped when disabled")
	}

	// Проверяем статус в БД
	storedCam, err := store.GetCamera("cam1")
	if err != nil || storedCam == nil {
		t.Fatalf("expected cam1 to exist in store: %v", err)
	}
	if !storedCam.Disabled || storedCam.DisableReason != "technical" {
		t.Fatalf("expected stored cam1 to be disabled with reason 'technical'")
	}

	// 6. Повторное включение камеры (disabled=false) -> 200 OK
	enableEdit := []byte(`{"url":"rtsp://127.0.0.1:8554/cam1_new","disabled":false,"lazyHLS":true}`)
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("PUT", "/api/cameras/cam1", bytes.NewBuffer(enableEdit))
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for enable edit, got %d", w.Code)
	}

	if !handler.manager.HasStream("cam1") {
		t.Fatalf("expected cam1 stream to be restarted in manager after enabling")
	}
	st, _ := handler.manager.GetStream("cam1")
	if st.URL != "rtsp://127.0.0.1:8554/cam1_new" {
		t.Fatalf("expected stream URL to be updated to cam1_new, got %s", st.URL)
	}

	// 7. Удаление несуществующей камеры -> 404 Not Found
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("DELETE", "/api/cameras/cam_none", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for non-existent camera delete, got %d", w.Code)
	}

	// 8. Успешное удаление существующей камеры -> 200 OK
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("DELETE", "/api/cameras/cam1", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for camera delete, got %d", w.Code)
	}

	// Проверяем что поток удален из менеджера и БД
	if handler.manager.HasStream("cam1") {
		t.Fatalf("expected cam1 stream to be removed from manager after deletion")
	}
	if _, err := store.GetCamera("cam1"); err == nil {
		t.Fatalf("expected cam1 to be deleted from store")
	}
}

func TestHandler_GetCameras_ReflectsDynamicState(t *testing.T) {
	router, handler, store := setupTestRouter(t)
	defer store.Close()

	router.GET("/api/cameras", handler.GetCameras)
	router.POST("/api/cameras", handler.AddCamera)

	// Добавляем 2 камеры: одну активную, одну отключенную
	cam1 := &config.CameraConfig{ID: "cam_active", URL: "rtsp://test/active", Disabled: false, LazyHLS: true}
	cam2 := &config.CameraConfig{ID: "cam_disabled", URL: "rtsp://test/disabled", Disabled: true}

	body1, _ := json.Marshal(cam1)
	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest("POST", "/api/cameras", bytes.NewBuffer(body1))
	router.ServeHTTP(w1, req1)

	body2, _ := json.Marshal(cam2)
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("POST", "/api/cameras", bytes.NewBuffer(body2))
	router.ServeHTTP(w2, req2)

	// Получаем список камер через GET
	wGet := httptest.NewRecorder()
	reqGet, _ := http.NewRequest("GET", "/api/cameras", nil)
	router.ServeHTTP(wGet, reqGet)

	if wGet.Code != http.StatusOK {
		t.Fatalf("expected 200 for GetCameras, got %d", wGet.Code)
	}

	var cams []CameraInfo
	if err := json.Unmarshal(wGet.Body.Bytes(), &cams); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(cams) != 2 {
		t.Fatalf("expected 2 cameras, got %d", len(cams))
	}

	var foundActive, foundDisabled bool
	for _, c := range cams {
		if c.ID == "cam_active" {
			foundActive = true
			if c.Disabled {
				t.Errorf("cam_active should not be marked disabled")
			}
		}
		if c.ID == "cam_disabled" {
			foundDisabled = true
			if !c.Disabled {
				t.Errorf("cam_disabled should be marked disabled")
			}
		}
	}

	if !foundActive || !foundDisabled {
		t.Errorf("expected to find both active and disabled cameras in list")
	}
}
