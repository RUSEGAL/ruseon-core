package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

// TestE2ERaceConditions имитирует высокую конкурентную нагрузку на API и ядро сервера.
// Если запустить этот тест с флагом -race, он со 100% вероятностью выявит гонки данных,
// так как сотни горутин одновременно читают и пишут в базу данных и менеджер потоков.
func TestE2ERaceConditions(t *testing.T) {
	router, handler, store := setupTestRouter(t)
	defer store.Close()

	router.GET("/api/cameras", handler.GetCameras)
	router.POST("/api/cameras", handler.AddCamera)
	router.DELETE("/api/cameras/:id", handler.DeleteCamera)
	router.GET("/api/stats", handler.GetServerStats)
	router.GET("/hls/:id/index.m3u8", handler.GetHLSPlaylist)

	var wg sync.WaitGroup
	numWorkers := 50
	iterations := 100

	// 1. Воркер: Постоянно добавляет и удаляет камеры (Пишущая нагрузка)
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			camJSON := []byte(`{"id":"race_cam","url":"rtsp://test","record":true,"disabled":true}`)
			w1 := httptest.NewRecorder()
			req1, _ := http.NewRequest("POST", "/api/cameras", bytes.NewBuffer(camJSON))
			router.ServeHTTP(w1, req1)

			w2 := httptest.NewRecorder()
			req2, _ := http.NewRequest("DELETE", "/api/cameras/race_cam", nil)
			router.ServeHTTP(w2, req2)
		}
	}()

	// 2. Десятки воркеров: Постоянно запрашивают список камер (Читающая нагрузка)
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				w := httptest.NewRecorder()
				req, _ := http.NewRequest("GET", "/api/cameras", nil)
				router.ServeHTTP(w, req)
			}
		}()
	}

	// 3. Десятки воркеров: Постоянно дергают статистику (Читающая + вычислительная нагрузка)
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				w := httptest.NewRecorder()
				req, _ := http.NewRequest("GET", "/api/stats", nil)
				router.ServeHTTP(w, req)
			}
		}()
	}

	// 4. Десятки воркеров: Имитируют зрителей HLS
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				w := httptest.NewRecorder()
				req, _ := http.NewRequest("GET", "/hls/race_cam/index.m3u8", nil)
				router.ServeHTTP(w, req)
			}
		}()
	}

	// Ждем завершения всего хаоса
	wg.Wait()
}
