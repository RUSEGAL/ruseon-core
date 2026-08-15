# 🚀 RUSEON Core — Результаты нагрузочного тестирования / Load Test Results

> **Дата тестирования / Timestamp:** `2026-08-14 12:43:34` | **Длительность / Duration:** `60.0 сек / 60.0 s` | **CPU Cores:** `12`

---

## 🇷🇺 Отчет на русском языке

### ⚙️ Конфигурация нагрузки

| Параметр | Значение | Описание |
| :--- | :--- | :--- |
| **Синтетические камеры** | `100` | Входящие потоки 30 FPS (~2.5 Mbps каждый) |
| **HLS fMP4 Зрители** | `150` | Клиенты, выкачивающие манифесты и видеосегменты |
| **WebRTC WHEP Клиенты** | `30` | Клиенты с SDP-хендшейком и приемом RTP видео |
| **REST API Воркеры** | `150` | Параллельные запросы к роутеру Gin (с JWT авторизацией) |
| **gRPC AI Воркеры** | `20` | Двунаправленный gRPC стриминг метаданных и кадров |
| **Реальный диск (MP4)** | `true` | Запись видеоархива через `pkg/storage/localfs` |

### 📊 Ключевые показатели производительности

| Компонент | Метрика | Значение | Латентность p50 / p95 / p99 |
| :--- | :--- | :--- | :--- |
| **Ingest (RTSP/NALU)** | Суммарный FPS | **3027 FPS** (252.3 Mbps) | — (Drops: `0`) |
| **REST API** | Throughput / RPS | **6960 RPS** (OK: `417738`, Err: `0`) | `0.00 ms` / `7.54 ms` / `27.22 ms` |
| **HLS fMP4 Delivery** | Отдано сегментов | **2995 seg** (46.0 MB/s) | `34.31 ms` / `77.13 ms` / `91.86 ms` |
| **WebRTC WHEP** | RTP Пакетов | **488101 pkts** (9.0 MB/s) | Handshake: `271.87 ms` / `305.76 ms` |
| **gRPC Stream & AI** | Кадров / Метаданных | **3027 FPS** / **198 RPS** | Stream: `32.98 ms` (Err: `0`) |
| **EventBus Webhooks** | Доставлено событий | **23459 events** (391/sec) | — |
| **Disk Storage (MP4)** | Скорость записи | **29.80 MB/s** (100 файлов) | — |

### 🖥️ Потребление системных ресурсов

| Ресурс | Значение |
| :--- | :--- |
| **Активные горутины** | `1427` |
| **Heap Alloc / In-Use** | `1089 MB` / `1137 MB` |
| **System Memory (Sys)** | `1598 MB` |
| **GC Cycles / Pause Total** | `126 циклов` / `23.51 ms` (Max pause: `0.00 ms`) |

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

### ⚙️ Load Configuration

| Parameter | Value | Description |
| :--- | :--- | :--- |
| **Synthetic Cameras** | `100` | Ingest streams @ 30 FPS (~2.5 Mbps each) |
| **HLS fMP4 Viewers** | `150` | Clients continuously fetching playlists and video segments |
| **WebRTC WHEP Clients** | `30` | Clients with SDP handshake and RTP video reception |
| **REST API Workers** | `150` | Concurrent requests to Gin router (with JWT auth) |
| **gRPC AI Workers** | `20` | Bidirectional gRPC streaming for frames and metadata |
| **Real Disk (MP4)** | `true` | Video archive recording via `pkg/storage/localfs` |

### 📊 Key Performance Metrics

| Component | Metric | Value | Latency p50 / p95 / p99 |
| :--- | :--- | :--- | :--- |
| **Ingest (RTSP/NALU)** | Total FPS | **3027 FPS** (252.3 Mbps) | — (Drops: `0`) |
| **REST API** | Throughput / RPS | **6960 RPS** (OK: `417738`, Err: `0`) | `0.00 ms` / `7.54 ms` / `27.22 ms` |
| **HLS fMP4 Delivery** | Delivered Segments | **2995 seg** (46.0 MB/s) | `34.31 ms` / `77.13 ms` / `91.86 ms` |
| **WebRTC WHEP** | RTP Packets | **488101 pkts** (9.0 MB/s) | Handshake: `271.87 ms` / `305.76 ms` |
| **gRPC Stream & AI** | Frames / Metadata | **3027 FPS** / **198 RPS** | Stream: `32.98 ms` (Err: `0`) |
| **EventBus Webhooks** | Delivered Events | **23459 events** (391/sec) | — |
| **Disk Storage (MP4)** | Write Rate | **29.80 MB/s** (100 files) | — |

### 🖥️ System Resource Consumption

| Resource | Value |
| :--- | :--- |
| **Active Goroutines** | `1427` |
| **Heap Alloc / In-Use** | `1089 MB` / `1137 MB` |
| **System Memory (Sys)** | `1598 MB` |
| **GC Cycles / Pause Total** | `126 cycles` / `23.51 ms` (Max pause: `0.00 ms`) |

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
*Отчет сгенерирован автоматически встроенным инструментом `cmd/loadtest` / Report automatically generated by `cmd/loadtest`.*
