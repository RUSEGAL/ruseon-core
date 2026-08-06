# RUSEON Core Benchmarks 📊

This directory contains the tools and scripts used to benchmark the performance and load capacity of RUSEON Core. Reproducibility is a core tenet of this project.

## Methodology

We separate benchmarking into two distinct areas:
1. **Ingest & Routing (RTSP/fMP4)**: Testing the system's ability to receive, parse, and write massive amounts of video data.
2. **Delivery (HLS Thundering Herd)**: Testing the system's ability to serve a single stream to thousands of concurrent viewers.

**⚠️ IMPORTANT WARNING**: Do not run massive Ingest and Delivery tests on the same local machine simultaneously. Running 100 `ffmpeg` processes alongside 1000 `k6` virtual users will cause severe CPU and Disk thrashing, invalidating the test results.

## Requirements
- Python 3.8+ (for provisioning scripts)
- [Grafana k6](https://k6.io/) (for load testing)
- `ffmpeg` (for generating RTSP streams)

---

## Scenario A: HLS Delivery (Thundering Herd)

This scenario tests how many concurrent users can watch a single live stream (e.g. testing the Edge HLS Muxer's Zero-Copy caching).

1. **Prepare a keyframed video file**
   Ensure your test video has a rigid 1-second GOP (Group of Pictures). This ensures the Muxer cuts segments predictably:
   ```bash
   ffmpeg -i sample.mp4 -c:v libx264 -preset fast -g 30 -keyint_min 30 -sc_threshold 0 -c:a aac -b:a 128k sample_keyframed.mp4
   ```

2. **Start a single stream**
   Push the stream to RUSEON Core (or its upstream MediaMTX router):
   ```bash
   python benchmarks/rtsp_flood.py --video sample_keyframed.mp4 --streams 1
   ```

3. **Run the k6 Load Test**
   Wait 3-5 seconds for the first HLS segments to be generated, then unleash the herd:
   ```bash
   k6 run benchmarks/hls_herd.js
   ```

---

## Scenario B: Mass Ingest (RTSP Flood)

This scenario tests RUSEON Core's ability to process hundreds of incoming RTSP streams and archive them (if enabled).

1. **Provision 100 Cameras via API**
   This script quickly registers 100 cameras into RUSEON Core's configuration:
   ```bash
   python benchmarks/setup_cameras.py --streams 100
   ```

2. **Flood the Ingest**
   This script spawns 100 `ffmpeg` subprocesses pushing to the router.
   *(Note: This requires a powerful CPU and fast NVMe storage if reading from disk).*
   ```bash
   python benchmarks/rtsp_flood.py --video sample_keyframed.mp4 --streams 100
   ```
   
Monitor the RUSEON Core Dashboard to observe RAM (Heap in Use) and CPU load. You should see high Network I/O with extremely low Garbage Collection overhead.
