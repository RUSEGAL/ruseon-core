<p align="center">
  <img src="web/public/favicon.svg" width="150" alt="RUSEON Logo" />
</p>

<h1 align="center">RUSEON Core</h1>

<p align="center">
  <strong>High-Performance Video Data Infrastructure & AI Media Pipeline</strong><br>
  <em>RTSP → HLS / WebRTC / fMP4 Archive without transcoding</em>
</p>

<p align="center">
  <a href="https://github.com/RUSEGAL/ruseon-core/actions/workflows/test.yml"><img src="https://img.shields.io/github/actions/workflow/status/RUSEGAL/ruseon-core/test.yml?branch=main&style=flat-square" alt="CI Status"></a>
  <a href="https://goreportcard.com/report/github.com/RUSEGAL/ruseon-core"><img src="https://goreportcard.com/badge/github.com/RUSEGAL/ruseon-core?style=flat-square" alt="Go Report Card"></a>
  <a href="https://github.com/RUSEGAL/ruseon-core/releases/latest"><img src="https://img.shields.io/github/v/release/RUSEGAL/ruseon-core?style=flat-square" alt="Latest Release"></a>
  <a href="bench-results/bench-baseline.md"><img src="https://img.shields.io/badge/Benchmark-600%20Cams%20%7C%2018k%20FPS%20%7C%200%20Drops-brightgreen?style=flat-square" alt="Stress Benchmark"></a>
  <img src="https://img.shields.io/badge/Coverage%20Gate-14%20Packages%20Validated-blue?style=flat-square" alt="Coverage Gate">
  <img src="https://img.shields.io/badge/License-MIT-blue.svg?style=flat-square" alt="License">
</p>

<p align="center">
  <em>Read this in other languages: <a href="README.md">English</a>, <a href="README.ru.md">Русский</a>.</em>
</p>

<hr>

**RUSEON Core** is a high-performance video data infrastructure platform tailored for IP cameras and Edge-to-Cloud workloads. Operating entirely on the principle of **transmuxing** rather than transcoding, RUSEON Core repackages video streams (H.264/H.265) into formats like HLS or WebRTC directly in RAM. 

This **zero-allocation approach** ensures minimal CPU usage, sub-millisecond playlist generation, and massive horizontal scalability, making it the ideal bridge between CCTV hardware and AI/Cloud applications.

---

## 🚀 Key Features

- **Zero-Allocation Transmuxing**: RTSP to HLS and WebRTC without transcoding, directly in RAM for extreme throughput.
- **Sub-Millisecond Master Playlists**: Non-blocking HLS master playlist delivery (< 0.5 ms latency) with singleflight caching.
- **Smart Crash-Resilient Archiver**: fMP4 recording with Linux kernel I/O optimizations (`FADV_DONTNEED`, sliding window `sync_file_range`) protecting Page Cache from eviction during heavy writes.
- **AI Metadata Pipeline**: Direct gRPC ingestion of AI insights (bounding boxes, telemetry) delivered synchronously via WebRTC DataChannel and HLS WebVTT with zero CPU burn.
- **WebRTC / WHEP**: Sub-second latency live streaming powered by Pion WebRTC.
- **Durable Embedded State Store**: BadgerDB state storage with synchronous disk durability (`SyncWrites = true`), atomic migrations, and crash recovery semantics.
- **Truthful Health & Readiness Probes**: Strict `/livez` and `/readyz` probes reporting actual real-time status of storage, database, and stream manager.
- **Event-Driven & IoT Integration**: Webhooks with circuit breaker and MQTT publisher with a lock-free buffer and bounded connection timeouts.
- **Modern Edge Dashboard**: React 19 UI with TypeScript, JWT authentication, live telemetry, and interactive fMP4 archive timeline.

[**Read more about Features**](https://docs.ruseon.tech/guide/features)

---

## 🏗 Architecture & Data Flow

RUSEON uses a single-copy Lock-Free Ring Buffer architecture. Video frames ingested via RTSP are stored once in memory and referenced concurrently by all outbound consumers (HLS, WebRTC, fMP4 recorder, AI pipeline) without redundant allocations.

```mermaid
flowchart TD
    CAMERA["IP Camera<br/>RTSP (H.264 / H.265)"] -->|TCP / UDP| CORE["RUSEON Stream Engine"]

    CORE --> RING["Lock-Free RingBuffer"]

    RING --> HLS["HLS Muxer<br/>fMP4 & TS"]
    RING --> WEBRTC["WebRTC / WHEP<br/>Pion Engine"]
    RING --> RECORDER["fMP4 Recorder<br/>Direct I/O"]
    RING --> AI["AI Pipeline<br/>gRPC Receiver"]

    HLS --> BROWSER["Browsers & Mobile Players"]
    WEBRTC --> CLIENTS["Ultra-Low Latency Viewers"]
    RECORDER --> ARCHIVE["Disk Archive & BlobStore"]
    AI --> META["Metadata Bus<br/>WebVTT / DataChannel / MQTT"]
```

[**Explore System Design**](https://docs.ruseon.tech/architecture/system)

---

## 🧪 Testing, Reliability & Production Verification

RUSEON Core enforces a comprehensive, multi-layer verification strategy in CI on every pull request:

```
+-----------------------------------------------------------------------------+
|                            RUSEON TEST SUITE                                |
+-----------------------------------------------------------------------------+
|  1. Go Native Fuzzing        | Lock-Free RingBuffer boundary & race fuzzing |
|  2. Testcontainers E2E Suite | MediaMTX 1.19.2 + FFmpeg 8 (RTSP -> HLS .ts) |
|  3. k6 Media Path Load Tests | Authenticated HLS, WHEP, Archive, Probes     |
|  4. React UI Testing Suite   | 36 Vitest + Testing Library component tests  |
|  5. Coverage Quality Gates   | Strict thresholds enforced on 14 packages    |
|  6. Static & Security Audit  | golangci-lint (0 issues) + gosec SAST        |
+-----------------------------------------------------------------------------+
```

### 1. Go Native Fuzz Testing (`internal/buffer`)
- Continuous fuzzing of the Lock-Free RingBuffer (`fuzz_test.go`) validating ring wrapping, concurrent head/tail racing, overflow behavior, and zero-allocation invariants under extreme random inputs.

### 2. Full-Pipeline Testcontainers E2E Suite (`tests/e2e`)
- Runs in an isolated Docker bridge network using pinned images:
  - **MediaMTX `1.19.2`** (RTSP server)
  - **FFmpeg `8-alpine`** (live H.264 test pattern publisher at 10 fps, GOP = 10)
- Validates the complete real-world media path:
  `FFmpeg Stream -> MediaMTX -> RUSEON RTSP Ingest -> RingBuffer -> /livez & /readyz -> HLS Master Playlist (index.m3u8) -> Video Playlist (stream.m3u8) -> Download & Binary Validation of MPEG-TS segment (0x47 sync byte)`.

### 3. Authenticated Media Path Performance Testing (`tests/performance`)
- Synthetic multi-scenario load testing with **k6** (`k6_load.js`):
  - **Live HLS Stream**: playlist negotiation and chunk delivery.
  - **WebRTC WHEP**: SDP offer/answer handshake and ICE connectivity.
  - **Timeshift Archive**: fMP4 seek and playback under load.
  - **Control Plane**: authenticated camera management API.
  - **Probes**: `/livez`, `/readyz`, and Prometheus `/metrics`.

### 4. Frontend UI Testing (`web/`)
- Native **Vitest** + **React Testing Library** + **jsdom** test suite (36 tests across 7 test files):
  - 24h archive timeline projection math, zoom level scaling, and segment hit-testing (`timeline-math.test.ts`).
  - Stream reconnection coordinator with exponential backoff & jitter (`reconnect-coordinator.test.ts`).
  - Browser streaming capabilities detection (`capabilities.test.ts`).
  - Player protocol state machine & failover hierarchy (`orchestrator.test.ts`).
  - Component tests for Login form submission, 401 handling, and Language switching.
  - 100% clean OxLint linter check.

### 5. Automated CI Code Coverage Gates (`scripts/check_coverage.go`)
- Mandatory threshold validation on each pull request across all critical packages:

| Package | Minimum Gate | Actual Measured Coverage | Status |
| :--- | :---: | :---: | :---: |
| `pkg/registry` | $\ge 90.0\%$ | **100.0%** | **PASS** |
| `pkg/logger` | $\ge 90.0\%$ | **97.0%** | **PASS** |
| `pkg/eventbus` | $\ge 90.0\%$ | **94.5%** | **PASS** |
| `pkg/storage/localfs` | $\ge 85.0\%$ | **90.9%** | **PASS** |
| `pkg/auth` | $\ge 80.0\%$ | **90.2%** | **PASS** |
| `internal/archive` | $\ge 80.0\%$ | **87.8%** | **PASS** |
| `internal/mqtt` | $\ge 80.0\%$ | **84.1%** | **PASS** |
| `pkg/storage` | $\ge 75.0\%$ | **81.9%** | **PASS** |
| `internal/buffer` | $\ge 70.0\%$ | **74.0%** | **PASS** |
| `internal/backup` | $\ge 70.0\%$ | **72.9%** | **PASS** |
| `pkg/config` | $\ge 70.0\%$ | **72.0%** | **PASS** |
| `internal/stream` | $\ge 65.0\%$ | **70.4%** | **PASS** |
| `internal/grpc` | $\ge 65.0\%$ | **70.3%** | **PASS** |
| `internal/recorder` | $\ge 65.0\%$ | **69.7%** | **PASS** |
| **Total Engine Core** | $\ge 50.0\%$ | **62.3%** | **PASS** |

---

## 📊 Performance & Stress Benchmarks

Under standardized full-stack load testing (**600 simultaneous 30 FPS cameras**, **1,800 concurrent HLS viewers**, **240 WebRTC WHEP clients**, and **gRPC AI streaming** on a 12-core CPU):

| Component | Metric | Measured Value | Latency / Notes |
| :--- | :--- | :--- | :--- |
| **Ingest Throughput** | **18,139 FPS (1,339.9 Mbps)** | 600 cameras @ 30 FPS | `0` dropped frames |
| **HLS fMP4 Delivery** | **500.2 MB/s** (1,098,000 segments) | 1,800 active viewers | `p50: 50.1 ms` / `p95: 277.1 ms` |
| **REST API Engine** | **375 RPS** (675k requests, 0 errors) | Full JWT auth verification | `p50: 0.63 ms` / `p95: 18.9 ms` |
| **WebRTC WHEP** | **51,793 RTP packets** | 240 live peer connections | Non-Trickle handshake: `568 ms` |
| **gRPC AI Stream** | **18,136 FPS / 1,900 RPS** | Frame extraction & metadata | Stream latency: `32.4 ms` |
| **EventBus Webhooks** | **582,476 delivered events** | 100% delivery rate | `0` dropped (0.0% loss) |
| **Memory Footprint** | **471 MB RSS** (140 MB Heap Alloc) | 2,448 active goroutines | Max GC pause: `24.98 ms` |

👉 **[Read the Full Benchmark Report (bench-results/bench-baseline.md)](bench-results/bench-baseline.md)**

---

## 🛠 Local Verification & Testing Guide

You can reproduce all quality and test checks locally:

```bash
# 1. Run full Go backend test suite with race detector and coverage
go test -v -race -timeout=15m -coverprofile=coverage.out ./...

# 2. Run Lock-Free RingBuffer Fuzz tests
go test -fuzz=FuzzRingBuffer -fuzztime=30s ./internal/buffer

# 3. Run E2E Media Pipeline tests with Testcontainers (requires Docker)
go test -v -run=TestE2E ./tests/e2e/...

# 4. Verify Code Coverage Thresholds
go run scripts/check_coverage.go coverage.out

# 5. Run Backend Linters & Security Scanners
golangci-lint run ./...
gosec -exclude-dir=pkg/grpc/pb ./...

# 6. Run Frontend Tests & Linter
cd web
npm run lint
npm test
npm run build
```

---

## 🏎 Quick Start

### 1. Launch via Docker
Deploy RUSEON Core with a single command. On first run, the system automatically generates an initial admin password and prints it securely to the console.

```bash
docker run -p 8080:8080 -v data:/app/data -v recordings:/app/recordings ghcr.io/rusegal/ruseon-core:latest
```

### 2. Access the Dashboard
Open your browser at [http://localhost:8080](http://localhost:8080). Log in with username `admin` and the generated password.

### 3. Add a Camera via API
Dynamically manage streams without server restarts:
```bash
curl -X POST http://localhost:8080/api/cameras \
  -H 'Authorization: Bearer YOUR_JWT_TOKEN' \
  -H 'Content-Type: application/json' \
  -d '{"id":"cam1","url":"rtsp://user:pass@192.168.1.100:554/stream","record":true}'
```

[**Read the full Quick Start Guide**](https://docs.ruseon.tech/guide/quick-start)

---

## 📖 Documentation

The official documentation is the single source of truth for RUSEON Core:

- **[Getting Started](https://docs.ruseon.tech/guide/introduction)**
- **[Configuration Reference](https://docs.ruseon.tech/reference/configuration)**
- **[Architecture & System Design](https://docs.ruseon.tech/architecture/overview)**
- **[Streaming & WebRTC](https://docs.ruseon.tech/streaming/overview)**
- **[Archive & Smart I/O](https://docs.ruseon.tech/archive/overview)**
- **[API Reference](https://docs.ruseon.tech/api/overview)**
- **[Deployment Guide](https://docs.ruseon.tech/deployment/overview)**
- **[Troubleshooting](https://docs.ruseon.tech/troubleshooting/overview)**

---

## ⚠️ Known Limitations

RUSEON Core is purpose-built for efficient video routing, storage, and AI piping. It operates entirely on transmuxing, which means **it does not perform lossy server-side video transcoding**. If your cameras output formats that certain browsers do not natively support (such as H.265 on legacy client environments), playback will utilize the client-side WebCodecs / canvas player fallback or require external stream conditioning.

[**See all Known Limitations**](https://docs.ruseon.tech/reference/known-limitations)

---

## 💎 Choose Your RUSEON Edition

RUSEON is built on an Open Core model. Start for free with the Community Edition, and upgrade to Pro or Enterprise when your video infrastructure scales and requires advanced B2B features like SSO/OIDC, Cloud Archiving (S3), and High Availability Clustering.

> **Ready to scale?** Contact our Team: [rusegal.dev@yahoo.com](mailto:rusegal.dev@yahoo.com)

---

## 📄 License

RUSEON Core (Community Edition) is distributed under the [MIT License](LICENSE).
