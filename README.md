# Gritprof Media Server

Gritprof Media Server is a high-performance, modern RTSP to HLS transmuxing server built with Go and React. It provides real-time streaming, dynamic camera management, and an advanced statistics dashboard.

## Features

- **Blazing Fast Transmuxing**: Uses Go with `gortsplib` and `mediacommon` to pull RTSP streams and transmux them into in-memory HLS (HTTP Live Streaming) on the fly, with zero disk I/O overhead.
- **Dynamic Camera Management**: Add, edit, or delete IP cameras directly from the web interface without restarting the server.
- **Advanced Dashboard**: Built with React (Vite, TypeScript), featuring a stunning modern Glassmorphism UI.
- **Live Statistics**: Real-time metrics for network traffic, memory heap allocations, goroutines, frames processed, and currently active HLS viewers.
- **Standalone Delivery**: The Go server statically serves the compiled React frontend, meaning you get a single unified application out of the box.

## Architecture

- **Backend**: Go 1.23+ with Gin web framework.
- **Frontend**: React 19, TypeScript, Vite, Lucide Icons, and standard CSS (Flex/Grid).
- **Video Protocol**: RTSP ingestion -> H.264 video tracking -> TS segments over HLS (`.m3u8`).

## Requirements

- [Go](https://go.dev/) 1.23 or newer
- [Node.js](https://nodejs.org/) 18+ (for building the frontend)
- Any RTSP camera (or simulated stream) supporting H.264 video.

## Quick Start

### 1. Build the Frontend

First, you need to compile the React dashboard so the Go server can serve it.

```bash
cd web
npm install
npm run build
cd ..
```

### 2. Run the Backend

```bash
go mod tidy
go run ./cmd/server
```

### 3. Access the Dashboard

Open your browser and navigate to: [http://localhost:8080](http://localhost:8080)

*Default Login credentials (can be configured in `config.yaml`):*
- **Username**: admin
- **Password**: admin

## Configuration

When the server runs for the first time, it generates a `config.yaml` file in the root directory. You can edit this file to change the server port, authentication details, or modify the initial list of cameras.

Example `config.yaml`:
```yaml
server:
  port: 8080
auth:
  username: admin
  password: mysecretpassword
cameras:
  - id: CAM01
    url: rtsp://user:pass@192.168.1.100:554/stream1
    record: true
    retention_days: 7
```

## Contributing
Feel free to open issues or submit pull requests. See `docs/ROADMAP.MD` for upcoming features and planned improvements.
