import http from 'k6/http';
import { check } from 'k6';

export const options = {
  // Virtual Users
  vus: __ENV.VUS || 1000, // Adjust
  duration: '10s',        // duration

  thresholds: {
    http_req_failed: ['rate<0.1'], // Allow up to 10% errors
  },
};

export default function () {
  const url = __ENV.BASE_URL || 'http://localhost:8001/transactions?select=id,user_id'
  const res = http.get(url, {
    headers: {
      'Authorization': `Bearer ${__ENV.TOKEN}`,
    }
  });
  check(res, { 'status was 200': (r) => r.status == 200 });
}
