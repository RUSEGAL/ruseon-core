# 🚀 RUSEON Core — Результаты нагрузочного тестирования / Load Test Results

> **Дата тестирования / Timestamp:** `2026-08-20 07:38:06` | **Длительность / Duration:** `60.1 сек / 60.1 s` | **CPU Cores:** `12`

---

## 🇷🇺 Отчет на русском языке

> ⚠️ **In-process benchmark:** REST API и HLS latency измеряются через `httptest.Server` loopback (сетевой RTT не включён). WebRTC WHEP использует реальный стек Pion P2P.

### ⚙️ Конфигурация нагрузки

| Параметр | Значение | Описание |
| :--- | :--- | :--- |
| **Синтетические камеры** | `100` | Входящие потоки 30 FPS (~2.5 Mbps каждый) |
| **HLS fMP4 Зрители** | `150` | Клиенты, выкачивающие манифесты и видеосегменты |
| **WebRTC WHEP Клиенты** | `30` | Клиенты с SDP-хендшейком и приемом RTP видео |
| **REST API Воркеры** | `150` | Параллельные запросы к роутеру Gin (с JWT авторизацией) |
| **gRPC AI Воркеры** | `20` | Двунаправленный gRPC стриминг метаданных и кадров |
| **Реконнекты RTSP** | `5/сек` | Симуляция обрывов и переподключений камер |
| **Реальный диск (MP4)** | `false` | Запись видеоархива через `pkg/storage/localfs` |

### 📊 Ключевые показатели производительности

| Компонент | Метрика | Значение | Латентность p50 / p95 / p99 |
| :--- | :--- | :--- | :--- |
| **Ingest (RTSP/NALU)** | Суммарный FPS | **3030 FPS** (225.0 Mbps) | — (Drops: `0`, Reconn: `329`) |
| **REST API** | Throughput / RPS | **7315 RPS** (OK: `439641`, Err: `0`) | `0.37 ms` / `1.81 ms` / `5.47 ms` |
| **HLS fMP4 Delivery** | Отдано сегментов | **2278 seg** (31.1 MB/s) | `1.21 ms` / `5.48 ms` / `45.55 ms` |
| **WebRTC WHEP** | RTP Пакетов | **430015 pkts** (7.9 MB/s) | Handshake: `6.90 ms` / `259.59 ms` |
| **gRPC Stream & AI** | Кадров / Метаданных | **3018 FPS** / **1985 RPS** | Stream: `33.00 ms` (Err: `0`) |
| **EventBus Webhooks** | Published / Delivered / Dropped | **23943** / **23943** / **0** (0.0%) | — |

### 🖥️ Потребление системных ресурсов

| Ресурс | Значение |
| :--- | :--- |
| **Активные горутины** | `570` |
| **Heap Alloc / In-Use** | `52 MB` / `65 MB` |
| **System Memory (Sys)** | `181 MB` |
| **RSS (Реальная память процесса)** | `140 MB` |
| **CPU System (avg / peak)** | `43.5%` / `48.6%` |
| **Process CPU Usage** | `234.2%` |
| **Network RX / TX (OS)** | `1392.7 Mbps` / `32.9 Mbps` |
| **GC Cycles / Pause Total** | `763 циклов` / `220.92 ms` (Max pause: `2.81 ms`) |

### 💡 Инженерные примечания и продакшен-рекомендации

1. **WebRTC WHEP Handshake (~140–180 ms):**
   * В бенчмарке замеряется полный Non-Trickle хендшейк со сбором сетевых интерфейсов ОС. В веб-браузерах с поддержкой Trickle ICE отклик видео наступает мгновенно (<50 ms).
   * Все соединения мультиплексируются через единый порт `8555/UDP` (UDP Muxer), исключая необходимость проброса сотен портов в Firewall.
2. **Дисковая подсистема (30 MB/s на 100 камер):**
   * Для 100+ камер с непрерывной MP4 записью рекомендуется использовать NVMe/SATA SSD или RAID-массивы для предотвращения деградации IOPS при параллельной очистке старого архива (`CleanupTask`).
3. **Защита ядра от медленных клиентов (Slow Consumers):**
   * Изолированные кольцевые буферы (`RingBuffer`) сбрасывают устаревшие кадры только для отстающих клиентов, предотвращая накопление очереди в оперативной памяти и не затрагивая других зрителей.
4. **AI & gRPC Интеграция:**
   * Канал передачи кадров в нейросети выдерживает 3 000+ FPS. При высокой вычислительной нагрузке на GPU рекомендуется настраивать частоту инференса на уровне 5–10 FPS на камеру.
5. **Безопасность и WebRTC:**
   * Для воспроизведения WebRTC в современных браузерах (Chrome, Safari, Firefox) требуется развертывание с HTTPS/TLS сертификатом (Secure Context).

---

## 🇬🇧 English Report

> ⚠️ **In-process benchmark:** REST API and HLS latency are measured via `httptest.Server` loopback (network RTT not included). WebRTC WHEP uses real Pion P2P stack.

### ⚙️ Load Configuration

| Parameter | Value | Description |
| :--- | :--- | :--- |
| **Synthetic Cameras** | `100` | Ingest streams @ 30 FPS (~2.5 Mbps each) |
| **HLS fMP4 Viewers** | `150` | Clients continuously fetching playlists and video segments |
| **WebRTC WHEP Clients** | `30` | Clients with SDP handshake and RTP video reception |
| **REST API Workers** | `150` | Concurrent requests to Gin router (with JWT auth) |
| **gRPC AI Workers** | `20` | Bidirectional gRPC streaming for frames and metadata |
| **RTSP Reconnect Rate** | `5/sec` | Simulation of camera drops and reconnections |
| **Real Disk (MP4)** | `false` | Video archive recording via `pkg/storage/localfs` |

### 📊 Key Performance Metrics

| Component | Metric | Value | Latency p50 / p95 / p99 |
| :--- | :--- | :--- | :--- |
| **Ingest (RTSP/NALU)** | Total FPS | **3030 FPS** (225.0 Mbps) | — (Drops: `0`, Reconn: `329`) |
| **REST API** | Throughput / RPS | **7315 RPS** (OK: `439641`, Err: `0`) | `0.37 ms` / `1.81 ms` / `5.47 ms` |
| **HLS fMP4 Delivery** | Delivered Segments | **2278 seg** (31.1 MB/s) | `1.21 ms` / `5.48 ms` / `45.55 ms` |
| **WebRTC WHEP** | RTP Packets | **430015 pkts** (7.9 MB/s) | Handshake: `6.90 ms` / `259.59 ms` |
| **gRPC Stream & AI** | Frames / Metadata | **3018 FPS** / **1985 RPS** | Stream: `33.00 ms` (Err: `0`) |
| **EventBus Webhooks** | Published / Delivered / Dropped | **23943** / **23943** / **0** (0.0%) | — |

### 🖥️ System Resource Consumption

| Resource | Value |
| :--- | :--- |
| **Active Goroutines** | `570` |
| **Heap Alloc / In-Use** | `52 MB` / `65 MB` |
| **System Memory (Sys)** | `181 MB` |
| **Process RSS Memory** | `140 MB` |
| **System CPU (avg / peak)** | `43.5%` / `48.6%` |
| **Process CPU Usage** | `234.2%` |
| **Network RX / TX (OS)** | `1392.7 Mbps` / `32.9 Mbps` |
| **GC Cycles / Pause Total** | `763 cycles` / `220.92 ms` (Max pause: `2.81 ms`) |

### 💡 Engineering Notes & Production Recommendations

1. **WebRTC WHEP Handshake (~140–180 ms):**
   * Measures complete Non-Trickle ICE handshake including OS network interface gathering. In browsers supporting Trickle ICE, video starts even faster (<50 ms).
   * All peer connections are multiplexed over a single `8555/UDP` port (UDP Muxer), eliminating the need to expose large port ranges in Firewalls.
2. **Storage Subsystem (30 MB/s per 100 cameras):**
   * For 100+ cameras with continuous MP4 recording, NVMe/SATA SSDs or RAID arrays are recommended to prevent IOPS degradation during background archive retention cleanup (`CleanupTask`).
3. **Core Protection Against Slow Consumers:**
   * Isolated per-stream ring buffers (`RingBuffer`) drop outdated frames only for lagging clients, preventing memory queue buildup and zero-copy stability for other viewers.
4. **AI & gRPC Integration:**
   * The frame extraction channel easily sustains 3,000+ FPS. For compute-heavy GPU inference, frame subsampling (5–10 FPS per stream) is advised.
5. **Security & WebRTC:**
   * Modern web browsers (Chrome, Safari, Firefox) require HTTPS/TLS (Secure Context) to initiate WebRTC sessions.

---
*Report automatically generated by `cmd/loadtest`.*
