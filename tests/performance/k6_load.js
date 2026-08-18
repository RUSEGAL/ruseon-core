import http from 'k6/http';
import { check, sleep } from 'k6';
import { Trend, Rate } from 'k6/metrics';

// Custom metric trends for fine-grained SLA tracking
const probeDuration = new Trend('probe_duration', true);
const apiDuration = new Trend('api_duration', true);
const hlsDuration = new Trend('hls_duration', true);
const whepDuration = new Trend('whep_duration', true);
const customErrorRate = new Rate('custom_error_rate');

export const options = {
  stages: [
    { duration: '10s', target: 30 }, // Ramp-up to 30 concurrent viewers/workers
    { duration: '20s', target: 30 }, // Sustain steady load
    { duration: '10s', target: 0 },  // Graceful ramp-down
  ],
  thresholds: {
    http_req_duration: ['p(95)<300'], // 95% of total requests below 300ms
    http_req_failed: ['rate<0.01'],   // Global error rate below 1%
    probe_duration: ['p(95)<100'],    // Liveness & readiness probes below 100ms
    hls_duration: ['p(95)<250'],      // HLS playlists below 250ms
    whep_duration: ['p(95)<300'],     // WebRTC WHEP handshakes below 300ms
  },
};

// Setup stage: initialize environment parameters and session configuration
export function setup() {
  const baseURL = __ENV.BASE_URL || 'http://localhost:8080';
  const cameraID = __ENV.CAMERA_ID || 'cam-01';
  const jwtToken = __ENV.JWT_TOKEN || '';
  const streamToken = __ENV.STREAM_TOKEN || '';

  return {
    baseURL,
    cameraID,
    jwtToken,
    streamToken,
  };
}

export default function (data) {
  const baseURL = data.baseURL;
  const cameraID = data.cameraID;
  const tokenQuery = data.streamToken ? `?token=${data.streamToken}` : '';
  const authHeaders = data.jwtToken
    ? { headers: { Authorization: `Bearer ${data.jwtToken}` } }
    : {};

  // ── 1. Infrastructure & Health Probes ──────────────────────────────────────
  const resLive = http.get(`${baseURL}/livez`);
  probeDuration.add(resLive.timings.duration);
  const liveOK = check(resLive, {
    'liveness status is 200': (r) => r.status === 200,
  });
  if (!liveOK) customErrorRate.add(1);

  const resReady = http.get(`${baseURL}/readyz`);
  probeDuration.add(resReady.timings.duration);
  const readyOK = check(resReady, {
    'readiness status is 200': (r) => r.status === 200,
  });
  if (!readyOK) customErrorRate.add(1);

  const resMetrics = http.get(`${baseURL}/metrics`);
  probeDuration.add(resMetrics.timings.duration);
  const metricsOK = check(resMetrics, {
    'metrics status is 200': (r) => r.status === 200,
  });
  if (!metricsOK) customErrorRate.add(1);

  // ── 2. Authenticated Control-Plane API ─────────────────────────────────────
  const resCams = http.get(`${baseURL}/api/cameras`, authHeaders);
  apiDuration.add(resCams.timings.duration);
  check(resCams, {
    'cameras API returns 200 or 401': (r) => r.status === 200 || r.status === 401,
  });

  // ── 3. Authenticated Live HLS Media Delivery ───────────────────────────────
  const resHLS = http.get(`${baseURL}/stream/hls/${cameraID}/index.m3u8${tokenQuery}`, authHeaders);
  hlsDuration.add(resHLS.timings.duration);
  check(resHLS, {
    'HLS index playlist returns valid code': (r) => r.status === 200 || r.status === 404,
  });

  const resHLSStream = http.get(`${baseURL}/stream/hls/${cameraID}/stream.m3u8${tokenQuery}`, authHeaders);
  hlsDuration.add(resHLSStream.timings.duration);
  check(resHLSStream, {
    'HLS video playlist returns valid code': (r) => r.status === 200 || r.status === 404,
  });

  // ── 4. WebRTC (WHEP) Signaling Handshake ───────────────────────────────────
  const mockSDP = 'v=0\r\no=- 0 0 IN IP4 127.0.0.1\r\ns=-\r\nt=0 0\r\nm=video 9 UDP/TLS/RTP/SAVPF 96\r\na=rtpmap:96 H264/90000\r\na=sendrecv\r\n';
  const whepHeaders = {
    headers: Object.assign(
      { 'Content-Type': 'application/sdp' },
      data.jwtToken ? { Authorization: `Bearer ${data.jwtToken}` } : {}
    ),
  };

  const resWHEP = http.post(`${baseURL}/stream/webrtc/whep/${cameraID}${tokenQuery}`, mockSDP, whepHeaders);
  whepDuration.add(resWHEP.timings.duration);
  check(resWHEP, {
    'WHEP handshake returns valid code': (r) =>
      r.status === 201 || r.status === 200 || r.status === 404 || r.status === 400,
  });

  // ── 5. Timeshift & Archive HLS Delivery ────────────────────────────────────
  const now = Math.floor(Date.now() / 1000);
  const resArchive = http.get(
    `${baseURL}/hls/${cameraID}/archive.m3u8?start=${now - 3600}&end=${now}${tokenQuery ? '&' + tokenQuery.substring(1) : ''}`,
    authHeaders
  );
  hlsDuration.add(resArchive.timings.duration);
  check(resArchive, {
    'Archive playlist returns valid code': (r) =>
      r.status === 200 || r.status === 404 || r.status === 400,
  });

  sleep(1);
}
