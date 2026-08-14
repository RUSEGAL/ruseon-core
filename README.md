<p align="center">
  <img src="web/public/favicon.svg" width="150" alt="RUSEON Logo" />
</p>

<h1 align="center">RUSEON Core</h1>

<p align="center">
  <strong>High-Performance Video Data Infrastructure</strong><br>
  <em>RTSP → HLS/WebRTC/Archive without transcoding</em>
</p>

<p align="center">
  <a href="https://github.com/RUSEGAL/ruseon-core/actions/workflows/test.yml"><img src="https://img.shields.io/github/actions/workflow/status/RUSEGAL/ruseon-core/test.yml?branch=main&style=flat-square" alt="CI Status"></a>
  <a href="https://goreportcard.com/report/github.com/RUSEGAL/ruseon-core"><img src="https://goreportcard.com/badge/github.com/RUSEGAL/ruseon-core?style=flat-square" alt="Go Report Card"></a>
  <a href="https://github.com/RUSEGAL/ruseon-core/releases/latest"><img src="https://img.shields.io/github/v/release/RUSEGAL/ruseon-core?style=flat-square" alt="Latest Release"></a>
  <a href="bench_max.md"><img src="https://img.shields.io/badge/Stress%20Benchmark-100%20Cams%20%7C%207k%20RPS%20%7C%200%20Drops-brightgreen?style=flat-square" alt="Stress Benchmark"></a>
  <img src="https://img.shields.io/badge/License-MIT-blue.svg?style=flat-square" alt="License">
</p>

<p align="center">
  <em>Read this in other languages: <a href="README.md">English</a>, <a href="README.ru.md">Русский</a>.</em>
</p>

<hr>

**RUSEON Core** is a high-performance video data infrastructure platform tailored for IP cameras. Operating entirely on the principle of **transmuxing** rather than transcoding, RUSEON Core repackages video data (H.264/H.265) into formats like HLS or WebRTC directly in RAM. 

This **zero-allocation approach** ensures minimal CPU usage and massive scalability, making it the ideal edge bridge between CCTV hardware and your AI or Cloud workloads.

## 🚀 Key Features

- **Zero-Allocation Transmuxing**: RTSP to HLS and WebRTC without transcoding, directly in RAM for extreme performance.
- **Smart Archive**: Crash-resilient fMP4 recording with advanced Linux kernel I/O optimizations (`sync_file_range`, `FADV_DONTNEED`).
- **AI Metadata Pipeline**: Direct gRPC ingestion of AI insights (like bounding boxes), delivered via WebRTC DataChannel and HLS WebVTT with zero CPU burn.
- **WebRTC/WHEP**: Sub-second latency live streaming powered by Pion WebRTC.
- **Event-Driven & IoT**: Webhooks with circuit breaker and MQTT with a lock-free buffer for reliable integration.
- **Production-Ready**: Prometheus metrics, structured JSON logging, chaos-tested, and built-in `pprof`.

[**Read more about Features**](https://docs.ruseon.tech/guide/features)

## 🏗 Architecture & Data Flow

RUSEON uses a highly efficient Ring Buffer architecture. Video frames ingested via RTSP are stored once and referenced by all outbound streams (HLS, WebRTC) and the recording module, avoiding redundant memory copying.

```mermaid
flowchart TD
    CAMERA["IP Camera<br/>RTSP"] -->|TCP / UDP| CORE["RUSEON Core"]

    CORE --> RING["Ring Buffer"]

    RING --> HLS["HLS Muxer"]
    RING --> WEBRTC["WebRTC / WHEP"]
    RING --> RECORDER["fMP4 Recorder"]
    RING --> AI["AI Pipeline"]

    HLS --> BROWSER["Browsers / Players"]
    WEBRTC --> CLIENTS["Low-Latency Clients"]
    RECORDER --> ARCHIVE["Disk Archive"]
    AI --> API["gRPC / Webhooks"]
```

[**Explore System Design**](https://docs.ruseon.tech/architecture/system)

## 🏎 Quick Start

### 1. Launch via Docker
The fastest way to deploy RUSEON Core is using our official Docker image. On the first run, the system will generate an admin password and print it to the console.

```bash
docker run -p 8080:8080 -v data:/app/data -v recordings:/app/recordings ghcr.io/rusegal/ruseon-core:latest
```
*(Copy the generated admin password from the terminal logs!)*

### 2. Access the Dashboard
Open your browser and navigate to [http://localhost:8080](http://localhost:8080). Log in with username `admin` and the generated password.

### 3. Add a Camera via API
You can easily manage streams dynamically via the REST API without restarting the server:
```bash
curl -X POST http://localhost:8080/api/cameras \
  -H 'Authorization: Bearer YOUR_JWT_TOKEN' \
  -d '{"id":"cam1","url":"rtsp://user:pass@192.168.1.100:554/stream","record":true}'
```

[**Read the full Quick Start Guide**](https://docs.ruseon.tech/guide/quick-start)

## 📊 Performance & Stress Benchmarks

RUSEON Core is engineered for high throughput, sub-second latency, and zero-allocation memory stability. 

Under our standardized full-stack stress test (100 simultaneous 30 FPS cameras, 350 concurrent viewers/workers, and real MP4 disk recording on a 12-core CPU):

| Metric | Result | Notes |
| :--- | :--- | :--- |
| **Ingest Throughput** | **3,028 FPS (252.3 Mbps)** | 100 cameras, `0` dropped frames |
| **REST API RPS** | **6,897 RPS** (414k requests) | `p50: 0.0 ms` / `p95: 3.0 ms` / `p99: 39.5 ms` |
| **HLS fMP4 Delivery** | **46.1 MB/s** (3,000 segments) | `p50: 44.9 ms` per 2s video chunk |
| **WebRTC WHEP** | **488k RTP packets (9.0 MB/s)** | `0` handshake errors |
| **Disk Storage (MP4)** | **29.8 MB/s** | 100 files recorded simultaneously |

👉 **[Read the Full Bilingual Benchmark Report (bench_max.md)](bench_max.md)**

```bash
# Reproduce the benchmark locally:
go run ./cmd/loadtest -cameras=100 -api-workers=150 -hls-viewers=150 -webrtc-viewers=30 -duration=60 -real-disk=true
```

[**View Online Documentation Benchmarks**](https://docs.ruseon.tech/performance/benchmarks)

## 📖 Documentation

The official VitePress documentation is the single source of truth for RUSEON Core:

- **[Getting Started](https://docs.ruseon.tech/guide/introduction)**
- **[Configuration](https://docs.ruseon.tech/reference/configuration)**
- **[Architecture](https://docs.ruseon.tech/architecture/overview)**
- **[Streaming](https://docs.ruseon.tech/streaming/overview)**
- **[Archive & Recording](https://docs.ruseon.tech/archive/overview)**
- **[API Reference](https://docs.ruseon.tech/api/overview)**
- **[Deployment](https://docs.ruseon.tech/deployment/overview)**
- **[Troubleshooting](https://docs.ruseon.tech/troubleshooting/overview)**

## ⚙️ Deployment & Configuration

RUSEON Core supports Docker, Docker Compose, and bare-metal binary deployments. Configuration can be managed via `config.yaml` for startup parameters, while cameras and streams are managed dynamically via the embedded BadgerDB and exposed via REST API.

[**Read Deployment Guide**](https://docs.ruseon.tech/deployment/overview)

## ⚠️ Known Limitations

RUSEON Core is purpose-built for efficient video routing and archiving. It operates entirely on transmuxing, which means **it does not transcode video**. If your cameras output formats that browsers do not natively support (e.g., H.265 in some environments), you may encounter playback issues unless handled client-side or transcoded externally.

[**See all Known Limitations**](https://docs.ruseon.tech/reference/known-limitations)

## 💎 Choose Your RUSEON Edition

RUSEON is built on an Open Core model. Start for free with the Community Edition, and upgrade to Pro or Enterprise when your video infrastructure scales and requires advanced B2B features like SSO, Scalable Cloud Archiving (S3), and High Availability Clustering.

> **Ready to scale?** Contact our Sales Team: [rusegal.dev@yahoo.com](mailto:rusegal.dev@yahoo.com)

## 📄 License

RUSEON Core (Community Edition) is distributed under the [MIT License](LICENSE).
