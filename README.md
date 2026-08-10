<p align="center">
  <img src="web/public/favicon.svg" width="150" alt="RUSEON Logo" />
</p>

<h1 align="center">RUSEON Core</h1>

<p align="center">
  <strong>Video Data Infrastructure & AI Pipeline</strong><br>
  <em>Cloud-Native, High-Performance Video Data Platform</em>
</p>

<p align="center">
  <a href="https://github.com/RUSEGAL/ruseon-core/actions/workflows/test.yml"><img src="https://img.shields.io/github/actions/workflow/status/RUSEGAL/ruseon-core/test.yml?branch=main&style=flat-square" alt="CI Status"></a>
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

**RUSEON Core** is an open-source Edge Video Infrastructure Platform engineered in Go. Designed for cloud-native and edge environments, it provides zero-copy RTSP-to-HLS/WebRTC transmuxing, fMP4 archiving, and an API foundation for AI video analytics.

Rather than trying to do everything (like AI inference or GPU transcoding) inside a single monolith, RUSEON focuses purely on moving and storing video bytes efficiently. It is designed to act as the central routing and storage engine, while heavy analytical tasks (like YOLO or Face Recognition) can run as separate downstream modules (e.g., `ruseon-yolo`, `ruseon-lpr`).

## 🚀 Key Features

* ⚡ **Zero-Copy Transmuxing**: Ultra-low latency bridging from RTSP to HLS and WebRTC directly in RAM. Bypasses intermediate transcoding for maximum efficiency.
* 🎥 **Ultra-Low Latency (WebRTC WHEP)**: Built-in WebRTC support for near-zero latency playback. Crucial for live PTZ camera control and real-time operator monitoring.
* 🧠 **Zero-Transcoding AI Metadata**: Direct injection of AI metadata (Bounding Boxes, labels) into video streams via **HLS WebVTT** and **WebRTC DataChannels**. Eliminates server-side video re-encoding for AI overlays, keeping AI metadata on a separate data plane. (no re-encoding required).
* 💾 **Smart I/O Archiver**: High-performance continuous fMP4 archiving utilizing advanced Linux kernel mechanisms (`FADV_DONTNEED`, sliding window `sync_file_range`). Protects the OS Page Cache from being flushed by gigabytes of video data, ensuring stable RAM and preventing I/O stalls.
* 🛡 **Cloud-Native Architecture & Security**: Built-in Thundering Herd protection, strict OOM management to handle thousands of concurrent streams, and API defenses against Path Traversal vulnerabilities (verified via CodeQL).
* 🔌 **Live Configuration API & CLI**: Add or remove cameras dynamically via REST API (Swagger included) or the `ruseon-cli` tool without restarting the daemon.
* 🌐 **IoT MQTT Gateway**: Built-in asynchronous MQTT publisher that dispatches AI metadata directly to home automation or industrial IoT buses (e.g., Home Assistant, Node-RED).
* ⏪ **Advanced Timeshift Pipeline**: Real-time HLS playback of historical data, with seamless export capabilities for AI training datasets.
* 🗄 **Embedded NoSQL Engine**: Powered by BadgerDB (with background Value Log GC) for sub-millisecond configuration states and metrics, delivering high IOPS without external dependencies.
* 🎨 **Modern Observability UI**: Includes a React 19 (TypeScript) Edge Dashboard with JWT auth, real-time SSE telemetry, and rich timeline visualization.
* 🧪 **Enterprise Reliability**: Codebase protected by automated k6 performance CI pipelines, continuous profiling (pprof), and chaos engineering tests for robust regression testing.

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
    Cam[RTSP Streams]
  end

  subgraph Engine [RUSEON Core]
    API[Live REST / SSE API]
    gRPC[gRPC AI Receiver]
    Demux[Zero-Copy Demuxer]
    Pool[RingBuffer / Pool]
    HLS[HLS Muxer + WebVTT]
    RTC[WebRTC WHEP + DataChannel]
    Rec[fMP4 Storage 'Direct I/O']
    MQTT[MQTT Publisher]
    DB[(BadgerDB Control Plane)]
  end

  subgraph Clients [Clients & Ecosystem]
    Dashboard[React Dashboard]
    CLI[ruseon-cli / Swagger]
    Player[Video Clients]
    IoT[IoT Broker / Home Assistant]
    AI[AI / CV Models]
  end

  %% Ingest Pipeline
  Cam -->|H.264 / HEVC| Demux
  Demux --> Pool
  
  %% Delivery Pipeline
  Pool --> HLS
  Pool --> RTC
  Pool --> Rec
  Pool --> MQTT
  
  %% State Management
  DB -.->|Config & State| API
  API -.-> Demux
  API <--> CLI
  
  %% AI Metadata Loop
  AI -->|Push Bounding Boxes| gRPC
  gRPC -->|Inject Metadata| Pool
  
  %% Outputs
  HLS -->|M3U8 / TS| Player
  RTC -->|Sub-second Latency| Player
  Rec -->|Dataset Export| AI
  MQTT -->|JSON Telemetry| IoT
  
  %% Observability
  Dashboard <-->|Metrics & Config| API
```

---

## 📊 Performance

RUSEON Core is designed for maximum efficiency. At its core lies a routing engine that provides **zero-copy / low-copy frame distribution with allocation minimization on the hot path**.

The architecture demonstrated highly stable behavior under our benchmark scenarios. RUSEON Core effectively utilized local 10G interfaces achieving **~8.8 Gbit/s end-to-end** throughput for HLS delivery.

For full automated benchmark results, stress tests, and chaos engineering reports, please refer to our single source of truth:
👉 **[benchmarks/RESULTS.md](benchmarks/RESULTS.md)**

---

## 🏎 Quick Start

### Prerequisites
- [Docker](https://www.docker.com/) (Recommended for rapid deployment)
- [Go](https://go.dev/) 1.26+ (For source builds)

### Deploy via Docker (GHCR) 🐳

The fastest way to deploy RUSEON Core is using our official multi-arch Docker image:

```bash
docker run -d \
  --network host \
  -v ruseon-data:/data \
  --name ruseon-core \
  ghcr.io/RUSEGAL/ruseon-core:latest
# Note: --network host is required for WebRTC UDP routing.
# Alternatively, expose -p 8080:8080 and your configured ICE UDP port.
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
*(On first startup, RUSEON generates a random administrator password and prints it once to the server console.)*

---

## ⚙️ Configuration & Live API

RUSEON Core requires a `config.yaml` file for basic startup settings. However, unlike traditional servers, **cameras and streams are managed dynamically** via the Live REST API or the integrated CLI tool.

You can copy the provided [`config.example.yaml`](config.example.yaml) to get started:

```bash
cp config.example.yaml config.yaml
```

**Base configuration (config.yaml):**
```yaml
server:
  port: 8080
  record_retention_days: 7

auth:
  username: "admin"

mqtt:
  enabled: true
  broker: "tcp://localhost:1883"
  client_id: "ruseon-core"
```

### Managing Cameras dynamically (No Restart Required)
Instead of hardcoding streams in a file, RUSEON persists its state securely in BadgerDB. Use the built-in CLI to add streams on the fly:

```bash
# Add a new camera to the cluster
./ruseon-cli stream add cam-01 rtsp://admin:admin@192.168.1.100/stream --record

# Check stream health and metrics
./ruseon-cli stream list
```
*(Interactive API documentation is always available at `http://localhost:8080/swagger/index.html`)*

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
