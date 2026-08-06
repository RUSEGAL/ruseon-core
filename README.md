<p align="center">
  <img src="web/public/favicon.svg" width="150" alt="RUSEON Logo" />
</p>

<h1 align="center">RUSEON Core</h1>

<p align="center">
  <strong>Edge Video Infrastructure & AI Data Pipeline</strong><br>
  <em>Cloud-Native, High-Performance Video Data Platform</em>
</p>

<p align="center">
  <a href="https://github.com/RUSEGAL/ruseon-core/actions/workflows/ci.yml"><img src="https://img.shields.io/github/actions/workflow/status/RUSEGAL/ruseon-core/ci.yml?branch=main&style=flat-square" alt="CI Status"></a>
  <a href="https://goreportcard.com/report/github.com/RUSEGAL/ruseon-core"><img src="https://goreportcard.com/badge/github.com/RUSEGAL/ruseon-core?style=flat-square" alt="Go Report Card"></a>
  <a href="https://github.com/RUSEGAL/ruseon-core/releases/latest"><img src="https://img.shields.io/github/v/release/RUSEGAL/ruseon-core?style=flat-square" alt="Latest Release"></a>

  <img src="https://img.shields.io/badge/License-MIT-blue.svg?style=flat-square" alt="License">
</p>

<p align="center">
  <em>Read this in other languages: <a href="README.md">English</a>, <a href="README.ru.md">Русский</a>.</em>
</p>

<hr>

<p align="center">
  <em>Most open-source video servers solve streaming.<br>RUSEON solves the entire video data lifecycle.<br>Ingest. Route. Record. Analyze. Export.<br>One engine.</em>
</p>

**RUSEON Core** is an open-source Edge Video Infrastructure Platform engineered in Go. Designed for cloud-native and edge environments, it provides zero-copy RTSP-to-HLS transmuxing, fMP4 archiving, and an API foundation for AI video analytics.

Rather than trying to do everything (like AI inference or GPU transcoding) inside a single monolith, RUSEON focuses purely on moving and storing video bytes efficiently. It is designed to act as the central routing and storage engine, while heavy analytical tasks (like YOLO or Face Recognition) can run as separate downstream modules (e.g., `ruseon-yolo`, `ruseon-lpr`).

## 🚀 Key Features

* ⚡ **Zero-Copy Transmuxing**: Ultra-low latency bridging from RTSP to HLS directly in RAM. Bypasses intermediate transcoding for maximum efficiency.
* 🛡 **Cloud-Native Architecture**: Built-in Thundering Herd protection and strict OOM management to safely handle thousands of concurrent streams.
* 📦 **High-Performance Archiving (fMP4)**: Continuous, gapless recording into fragmented MP4. Optimized for edge storage and rapid cloud synchronization.
* ⏪ **Advanced Timeshift Pipeline**: Real-time HLS playback of historical data, with seamless export capabilities for AI training datasets.
* 🗄 **Embedded NoSQL Engine**: Powered by BadgerDB for sub-millisecond configuration states and metrics, delivering high IOPS without external dependencies.
* 🎨 **Modern Observability UI**: Includes a React 19 (TypeScript) Edge Dashboard with JWT auth, real-time SSE telemetry, and rich timeline visualization.

---

## ⚔️ Comparison & Philosophy

RUSEON Core has a different philosophy compared to other popular tools in the video ecosystem. Rather than directly competing, it serves a completely different architectural purpose:

- **MediaMTX**: A fantastic **Media Router**. It focuses on broad protocol translation (RTSP, WebRTC, RTMP, SRT). RUSEON, on the other hand, focuses on the **Video Data Lifecycle** (archiving, timeshift, AI dataset export).
- **FFmpeg**: The ultimate **Media Toolkit**. It is exceptional for processing video, but requires complex scripting to run as a reliable, API-driven daemon.
- **Flussonic**: A comprehensive **IPTV Platform** tailored for telecom and broadcasting networks.
- **RUSEON Core**: An **Edge Video Infrastructure**. Built specifically to ingest CCTV streams, persist them to disk, and efficiently deliver them to human operators and AI pipelines.

---

## 🏗 Architecture & AI Data Pipeline

RUSEON Core acts as the critical bridge between edge hardware and your AI / Cloud workloads.

```mermaid
graph LR
  subgraph Edge [Edge Devices / Cameras]
    Cam1[RTSP Stream]
    Cam2[RTSP Stream]
  end

  subgraph Engine [RUSEON Core]
    Demux[Zero-Copy Demuxer]
    Pool[Memory Pool]
    HLS[Edge HLS Muxer]
    Rec[fMP4 Storage Engine]
    DB[(BadgerDB)]
  end

  subgraph Cloud [Cloud & AI Infrastructure]
    Browser[Observability Dashboard]
    Player[Analytics Node]
    AI[AI / ML Pipeline]
  end

  Cam1 & Cam2 -->|H.264/H.265| Demux
  Demux --> Pool
  Pool --> HLS
  Pool --> Rec
  DB -.->|State & Config| Demux
  HLS -->|Live Feed| Player
  Rec -->|Dataset Export| AI
  Browser <-->|REST & SSE Telemetry| Demux
```

---

## 📊 Performance

RUSEON Core is designed for maximum efficiency. At its core lies a **Zero-Copy** router that ensures CPU and memory are spent only on useful work. The architecture completely avoids the Garbage Collector (GC) in the hot paths of video frame transmission.

**🖥 Test Environment:**
* **CPU:** AMD Ryzen 5 5600X (All benchmarks below were run on a **single** core)
* **OS / Arch:** Windows / amd64
* **Runtime:** Go 1.23+

### 1. Frame Broadcasting (Zero-Copy RingBuffer)
The core receives a video frame (H.264/H.265) and instantly broadcasts it to subscribers (HLS Muxer, Recorder, and AI Agents) without copying data in memory.

| Operation | Time (ns/op) | Memory Allocations | Description |
| :--- | :--- | :--- | :--- |
| `Write` | **13.9 ns** | **0 B/op** (0 allocs) | Writing a frame to the buffer |
| `Broadcast (100 subs)` | **10.8 µs** | **0 B/op** (0 allocs) | Broadcasting 1 frame to 100 subscribers simultaneously |

> **Bottom line:** The engine can dispatch tens of thousands of frames per second on a single CPU core, **allocating absolutely zero new memory on the heap** (0 allocs/op).

### 2. Edge HLS Delivery (Muxer)
Even under the *Thundering Herd* problem (when thousands of users simultaneously connect to a live stream), RUSEON serves M3U8 playlists and TS segments directly from the memory cache.

| Operation | Time | Throughput | Description |
| :--- | :--- | :--- | :--- |
| `GetPlaylist` | **~1.1 µs** | ~1,000,000 req/sec | Generating an M3U8 manifest |
| `GetSegment (1 MB)` | **~113 µs** | **~8.7 GB/s** | Retrieving a TS segment (1 MB) from the pool |

> **Bottom line:** Your server won't crash during massive traffic spikes. A single CPU core can handle segment delivery at speeds of nearly 9 Gigabytes per second.

### 3. High-Speed Archive (fMP4 Recorder)
Packaging video streams into fragmented MP4 (fMP4) format for writing to persistent storage.

| Operation | Time | Description |
| :--- | :--- | :--- |
| `Write GOP (1 sec video)` | **~1.58 ms** | Packaging 25 frames (1 sec. of video) and flushing to disk |

> **Bottom line:** A single CPU core can continuously archive **~630 concurrent video streams**. The performance of RUSEON Core is strictly bottlenecked by the I/O throughput of your disks and network, not the CPU!

### 4. End-to-End HLS Load Testing (Thundering Herd) 🌩️
We performed an End-to-End load test on Live HLS delivery using Grafana's `k6`. The goal was to simulate 1000 simultaneous viewers tuning into a single live broadcast (camera) to validate the Muxer's Zero-Copy caching architecture.

**Test Conditions:** 1000 concurrent Virtual Users (`k6`), continuously downloading the `index.m3u8` playlist and new binary `.ts` segments over 70 seconds.

| Metric | Result | Description |
| :--- | :--- | :--- |
| **Throughput** | **1.1 GB/s (8.8 Gbps)** | Served 81 Gigabytes of video data in ~70 seconds |
| **Success Rate (HTTP 200)** | **100%** (60,822 requests) | Zero dropped connections (0% fail rate) |
| **Latency (avg)** | **3.13 ms** | Average time to serve a video segment to a viewer |
| **Latency p(95)** | **6.13 ms** | 95% of all viewers received segments in under 6 milliseconds |

> **Bottom line:** The architecture is designed to mitigate the thundering herd problem and demonstrated highly stable behavior under our benchmark scenarios. RUSEON Core effortlessly saturated local 10G interfaces while keeping response latencies under 6 milliseconds for thousands of concurrent TCP connections.

![k6 HLS Stress Test Results](assets/k6-test.png)

### 5. Ingest Resource Consumption (100 RTSP Streams) 🎥
We also tested the engine's ability to simultaneously receive and process 100 RTSP streams (H.264/HEVC).

Despite handling hundreds of megabytes of incoming traffic per second, thanks to Zero-Copy RTP parsing and no transcoding, the server consumes **just over 250 MB of RAM and ~1% of a standard desktop CPU**!

![Dashboard with 100 cameras](assets/100-cameras.png)
![Advanced memory and GC stats](assets/advanced-stats.png)
![CPU and RAM consumption in Task Manager](assets/ram-cpu.png)

The low Garbage Collector overhead (only 68 collections) proves the efficiency of the `sync.Pool` byte buffers and the RingBuffer architecture.

---

## 🏎 Quick Start

### Prerequisites
- [Docker](https://www.docker.com/) (Recommended for rapid deployment)
- [Go](https://go.dev/) 1.23+ (For source builds)

### Deploy via Docker (GHCR) 🐳

The fastest way to deploy RUSEON Core is using our official multi-arch Docker image:

```bash
docker run -d \
  -p 8080:8080 \
  -v ruseon-data:/data \
  --name ruseon-core \
  ghcr.io/RUSEGAL/ruseon-core:latest
```

The Enterprise Edge Dashboard will be available at `http://localhost:8080`.

### Build from Source

For developers and contributors:

```bash
# 1. Clone the repository
git clone https://github.com/RUSEGAL/ruseon-core.git
cd ruseon-core

# 2. Build the Edge Dashboard (React)
cd web && npm install && npm run build && cd ..

# 3. Start the Core Engine
go mod tidy
go run ./cmd/server
```
*(Default Credentials: admin / admin)*

---

## ⚙️ Configuration

RUSEON Core requires a `config.yaml` file to run. By default, the engine will look for it in the current directory.

You can copy the provided [`config.example.yaml`](config.example.yaml) to get started:

```bash
cp config.example.yaml config.yaml
```

**Example configuration:**
```yaml
server:
  port: 8080
  record_retention_days: 7

auth:
  username: "admin"
  password: "password123"

cameras:
  - id: "cam-01"
    url: "rtsp://admin:admin@192.168.1.100/stream"
    record: true
```

---

## 💎 Choose Your RUSEON Edition

RUSEON is built on an Open Core model. Start for free with the Community Edition, and upgrade to Pro or Enterprise when your video infrastructure scales and requires advanced B2B features.

| Feature / Capability | 🟢 **Core (Community)** | 🔵 **Pro** | 🟣 **Enterprise** |
| :--- | :---: | :---: | :---: |
| **Zero-Copy Routing (RTSP/HLS)** | ✅ Yes | ✅ Yes | ✅ Yes |
| **React Edge Dashboard** | ✅ Yes | ✅ Yes | ✅ Yes |
| **fMP4 Archiving (Local)** | ✅ Yes | ✅ Yes | ✅ Yes |
| **Max Cameras per Node** | Unlimited (Hardware limit) | Unlimited | Unlimited |
| **Advanced IAM & RBAC** | ❌ Basic Auth | ✅ Role-based Access | ✅ Role-based Access |
| **SSO (Active Directory, OIDC, SAML)** | ❌ No | ❌ No | ✅ **Yes** |
| **Infinite Cloud Archiving (S3 / Minio)**| ❌ No | ❌ No | ✅ **Yes** |
| **Clustering & High Availability** | ❌ Single Node | ⚠️ Basic Sync | 🚧 **Planned (Roadmap)** |
| **Hardware / GPU Transcoding** | ❌ No | 🚧 Planned | 🚧 **Planned (Roadmap)** |
| **Support SLA** | 🌐 Community (GitHub) | 📧 Email Support | 🚀 **24/7 Dedicated SLA** |
| **License / Pricing** | **Free (MIT)** | **Pay per Camera** | **Custom Enterprise** |

> **Ready to scale?** [Contact our Sales Team](mailto:rusegal.dev@yahoo.com) to request a trial key for **RUSEON Enterprise** and unlock SSO, S3 storage, and Clustering.

---
## 🤝 Contributing

We believe in the power of open-source and welcome contributions from the community. 
Whether it's a bug report, new feature, or documentation improvement, please see our [Contributing Guidelines](CONTRIBUTING.md) to get started.

Please ensure your commits follow the [Conventional Commits](https://www.conventionalcommits.org/) specification.

## 📄 License

RUSEON Core (Community Edition) is distributed under the [MIT License](LICENSE). 
