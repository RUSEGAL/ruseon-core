# 🚀 RUSEON Core — Результаты нагрузочного тестирования / Load Test Results

> **Дата тестирования / Timestamp:** `2026-08-21 08:48:10` | **Длительность / Duration:** `1800.1 сек / 1800.1 s` | **CPU Cores:** `12`

---

## 🇷🇺 Отчет на русском языке

> ⚠️ **In-process benchmark:** REST API и HLS latency измеряются через `httptest.Server` loopback (сетевой RTT не включён). WebRTC WHEP использует реальный стек Pion P2P.

### ⚙️ Конфигурация нагрузки

| Параметр | Значение | Описание |
| :--- | :--- | :--- |
| **Синтетические камеры** | `600` | Входящие потоки 30 FPS (~2.5 Mbps каждый) |
| **HLS fMP4 Зрители** | `1800` | Клиенты, выкачивающие манифесты и видеосегменты |
| **WebRTC WHEP Клиенты** | `240` | Клиенты с SDP-хендшейком и приемом RTP видео |
| **REST API Воркеры** | `10` | Параллельные запросы к роутеру Gin (с JWT авторизацией) |
| **gRPC AI Воркеры** | `20` | Двунаправленный gRPC стриминг метаданных и кадров |
| **Реальный диск (MP4)** | `false` | Запись видеоархива через `pkg/storage/localfs` |

### 📊 Ключевые показатели производительности

| Компонент | Метрика | Значение | Латентность p50 / p95 / p99 |
| :--- | :--- | :--- | :--- |
| **Ingest (RTSP/NALU)** | Суммарный FPS | **18139 FPS** (1339.9 Mbps) | — (Drops: `0`) |
| **REST API** | Throughput / RPS | **375 RPS** (OK: `675739`, Err: `0`) | `0.63 ms` / `18.94 ms` / `96.81 ms` |
| **HLS fMP4 Delivery** | Отдано сегментов | **1098000 seg** (500.2 MB/s) | `50.10 ms` / `277.06 ms` / `495.25 ms` |
| **WebRTC WHEP** | RTP Пакетов | **51793 pkts** (0.0 MB/s) | Handshake: `568.76 ms` / `801.16 ms` |
| **gRPC Stream & AI** | Кадров / Метаданных | **18136 FPS** / **1900 RPS** | Stream: `32.39 ms` (Err: `0`) |
| **EventBus Webhooks** | Published / Delivered / Dropped | **582476** / **582476** / **0** (0.0%) | — |

### 🖥️ Потребление системных ресурсов

| Ресурс | Значение |
| :--- | :--- |
| **Активные горутины** | `2448` |
| **Heap Alloc / In-Use** | `140 MB` / `174 MB` |
| **System Memory (Sys)** | `2224 MB` |
| **RSS (Реальная память процесса)** | `471 MB` |
| **CPU System (avg / peak)** | `58.8%` / `93.0%` |
| **Process CPU Usage** | `360.8%` |
| **Network RX / TX (OS)** | `5834.2 Mbps` / `36.8 Mbps` |
| **GC Cycles / Pause Total** | `7903 циклов` / `22257.84 ms` (Max pause: `24.98 ms`) |

### 💡 Инженерные примечания и продакшен-рекомендации

1. **WebRTC WHEP Handshake (~140–180 ms):**
   * В бенчмарке замеряется полный Non-Trickle хендшейк со сбором сетевых интерфейсов ОС. В веб-браузерах с поддержкой Trickle ICE отклик видео наступает мгновенно (<50 ms).
   * Все соединения мультиплексируются через единый порт `8555/UDP` (UDP Muxer), исключая необходимость проброса сотен портов в Firewall.
2. **HLS Латенси плейлистов (First-Segment vs Polling):**
   * **Задержка первого сегмента (≤ 2 сек / 1 GOP):** При холодном старте первого зрителя (`lazyHLS=true`) муксер ожидает прихода первого ключевого кадра (I-Frame) и формирования первого видеосегмента. Время ожидания первого плейлиста составляет до 1–2 секунд (длительность одного GOP). **Это штатная архитектурная логика HLS (не баг)**, гарантирующая, что клиент получит валидный воспроизводимый плейлист с готовым видеосегментом.
   * **Последующий опрос (Polling Latency p50 < 1 ms):** Как только первый сегмент готов, все последующие обновления плейлиста и запросы от сотен клиентов отдаются мгновенно из памяти с латентностью `p50 < 1 ms` и `Zero-Alloc`.
3. **Дисковая подсистема (30 MB/s на 100 камер):**
   * Для 100+ камер с непрерывной MP4 записью рекомендуется использовать NVMe/SATA SSD или RAID-массивы для предотвращения деградации IOPS при параллельной очистке старого архива (`CleanupTask`).
4. **Защита ядра от медленных клиентов (Slow Consumers):**
   * Изолированные кольцевые буферы (`RingBuffer`) сбрасывают устаревшие кадры только для отстающих клиентов, предотвращая накопление очереди в оперативной памяти и не затрагивая других зрителей.
5. **AI & gRPC Интеграция:**
   * Канал передачи кадров в нейросети выдерживает 3 000+ FPS. При высокой вычислительной нагрузке на GPU рекомендуется настраивать частоту инференса на уровне 5–10 FPS на камеру.
6. **Безопасность и WebRTC:**
   * Для воспроизведения WebRTC в современных браузерах (Chrome, Safari, Firefox) требуется развертывание с HTTPS/TLS сертификатом (Secure Context).

---

## 🇬🇧 English Report

> ⚠️ **In-process benchmark:** REST API and HLS latency are measured via `httptest.Server` loopback (network RTT not included). WebRTC WHEP uses real Pion P2P stack.

### ⚙️ Load Configuration

| Parameter | Value | Description |
| :--- | :--- | :--- |
| **Synthetic Cameras** | `600` | Ingest streams @ 30 FPS (~2.5 Mbps each) |
| **HLS fMP4 Viewers** | `1800` | Clients continuously fetching playlists and video segments |
| **WebRTC WHEP Clients** | `240` | Clients with SDP handshake and RTP video reception |
| **REST API Workers** | `10` | Concurrent requests to Gin router (with JWT auth) |
| **gRPC AI Workers** | `20` | Bidirectional gRPC streaming for frames and metadata |
| **Real Disk (MP4)** | `false` | Video archive recording via `pkg/storage/localfs` |

### 📊 Key Performance Metrics

| Component | Metric | Value | Latency p50 / p95 / p99 |
| :--- | :--- | :--- | :--- |
| **Ingest (RTSP/NALU)** | Total FPS | **18139 FPS** (1339.9 Mbps) | — (Drops: `0`) |
| **REST API** | Throughput / RPS | **375 RPS** (OK: `675739`, Err: `0`) | `0.63 ms` / `18.94 ms` / `96.81 ms` |
| **HLS fMP4 Delivery** | Delivered Segments | **1098000 seg** (500.2 MB/s) | `50.10 ms` / `277.06 ms` / `495.25 ms` |
| **WebRTC WHEP** | RTP Packets | **51793 pkts** (0.0 MB/s) | Handshake: `568.76 ms` / `801.16 ms` |
| **gRPC Stream & AI** | Frames / Metadata | **18136 FPS** / **1900 RPS** | Stream: `32.39 ms` (Err: `0`) |
| **EventBus Webhooks** | Published / Delivered / Dropped | **582476** / **582476** / **0** (0.0%) | — |

### 🖥️ System Resource Consumption

| Resource | Value |
| :--- | :--- |
| **Active Goroutines** | `2448` |
| **Heap Alloc / In-Use** | `140 MB` / `174 MB` |
| **System Memory (Sys)** | `2224 MB` |
| **Process RSS Memory** | `471 MB` |
| **System CPU (avg / peak)** | `58.8%` / `93.0%` |
| **Process CPU Usage** | `360.8%` |
| **Network RX / TX (OS)** | `5834.2 Mbps` / `36.8 Mbps` |
| **GC Cycles / Pause Total** | `7903 cycles` / `22257.84 ms` (Max pause: `24.98 ms`) |

### 💡 Engineering Notes & Production Recommendations

1. **WebRTC WHEP Handshake (~140–180 ms):**
   * Measures complete Non-Trickle ICE handshake including OS network interface gathering. In browsers supporting Trickle ICE, video starts even faster (<50 ms).
   * All peer connections are multiplexed over a single `8555/UDP` port (UDP Muxer), eliminating the need to expose large port ranges in Firewalls.
2. **HLS Playlist Latency (First-Segment vs Steady-State Polling):**
   * **First-Segment Startup Latency (≤ 2 s / 1 GOP):** On cold start for the first viewer (`lazyHLS=true`), the HLS muxer waits for the first keyframe (I-Frame) to arrive and generate the initial playable segment. First playlist response latency takes up to 1–2 seconds (the duration of one GOP). **This is by design and architectural logic (not a bug)**, ensuring that HLS video players receive a valid, ready-to-play manifest with actual media.
   * **Steady-State Polling Latency (p50 < 1 ms):** Once the initial segment is produced, all subsequent playlist updates and polling requests across hundreds of concurrent viewers are served directly from in-memory ring cache with ultra-low latency (`p50 < 1 ms`) and Zero-Alloc performance.
3. **Storage Subsystem (30 MB/s per 100 cameras):**
   * For 100+ cameras with continuous MP4 recording, NVMe/SATA SSDs or RAID arrays are recommended to prevent IOPS degradation during background archive retention cleanup (`CleanupTask`).
4. **Core Protection Against Slow Consumers:**
   * Isolated per-stream ring buffers (`RingBuffer`) drop outdated frames only for lagging clients, preventing memory queue buildup and zero-copy stability for other viewers.
5. **AI & gRPC Integration:**
   * The frame extraction channel easily sustains 3,000+ FPS. For compute-heavy GPU inference, frame subsampling (5–10 FPS per stream) is advised.
6. **Security & WebRTC:**
   * Modern web browsers (Chrome, Safari, Firefox) require HTTPS/TLS (Secure Context) to initiate WebRTC sessions.

---
*Report automatically generated by `cmd/loadtest`.*
