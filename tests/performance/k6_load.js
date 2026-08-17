import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
  stages: [
    { duration: '10s', target: 50 }, // Ramp-up to 50 users over 10s
    { duration: '30s', target: 50 }, // Stay at 50 users for 30s
    { duration: '10s', target: 0 },  // Ramp-down to 0 users
  ],
  thresholds: {
    http_req_duration: ['p(95)<200'], // 95% of requests must complete below 200ms
    http_req_failed: ['rate<0.01'],   // Error rate should be less than 1%
  },
};

export default function () {
  // We hit the registered liveness and metrics endpoints to ensure the core is responsive
  // In a real scenario, this would be the HLS playlist endpoint for an active stream
  
  const res1 = http.get('http://localhost:8080/livez');
  check(res1, {
    'liveness status is 200': (r) => r.status === 200,
  });

  const res2 = http.get('http://localhost:8080/metrics');
  check(res2, {
    'metrics status is 200': (r) => r.status === 200,
  });

  sleep(1);
}
