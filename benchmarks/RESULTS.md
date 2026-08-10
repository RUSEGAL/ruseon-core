# Benchmark Results

This document serves as the single source of truth for RUSEON Core performance and capacity metrics. 
The numbers here reflect automated tests executed via `go test -bench` in a synthetic, isolated environment.

**Hardware tested on:**
- OS: Windows 11 / Linux
- CPU: AMD Ryzen 5 5600X 6-Core Processor
- RAM: 32 GB DDR4
- Disk: NVMe SSD (for I/O tests) or in-memory `io.Discard` for synthetic benchmarks.

---

## 1. Zero-Copy HLS Pipeline

The core Zero-Copy pipeline distributes video streams without redundant memory allocations.

### `GetPlaylist` (M3U8 Manifest Generation)
- **Time per request**: ~333 ns
- **Throughput**: ~3,000,000 req/sec per core
- **Test Command**: `go test -bench=BenchmarkGetPlaylist ./internal/hls`

### `GetSegment` (Retrieving a 1 MB TS Segment)
- **Time per request**: ~87.4 µs
- **Throughput**: ~11.4 GB/s per core
- **Test Command**: `go test -bench=BenchmarkGetSegment ./internal/hls`

> **Note on Network Limits**: While a single CPU core can dispatch segments at ~11.4 GB/s, real-world deployment will strictly bottleneck on your network interface (e.g., 10 Gbps / 1.25 GB/s or 100 Gbps NIC).

---

## 2. Recorder (fMP4 Archiver)

The recorder packages Raw H.264 NAL units into fragmented MP4 (fMP4) and flushes them to disk.

### `Write GOP` (1-second video, 25 frames)
- **Time per GOP**: ~1.58 ms
- **Estimated Core Capacity**: ~630 concurrent streams per core
- **Test Command**: `go test -bench=BenchmarkRecorder_FMP4_Write_GOP ./internal/recorder`

> **Note on I/O Limits**: This is the *theoretical CPU capacity* under synthetic workloads. Production deployments are almost entirely bottlenecked by disk I/O IOPS and write speeds.
