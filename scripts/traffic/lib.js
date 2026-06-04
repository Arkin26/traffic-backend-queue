import http from 'k6/http';
import { check, sleep } from 'k6';

export const BASE = __ENV.BASE_URL || 'http://localhost:8080';
export const ADMIN_KEY = __ENV.ADMIN_KEY || 'dev-admin-key';

export function registerUser(email) {
  const res = http.post(
    `${BASE}/v1/auth/register`,
    JSON.stringify({ email, password: 'testpass123' }),
    { headers: { 'Content-Type': 'application/json' }, tags: { name: 'register' } }
  );
  if (res.status === 201 || res.status === 200) {
    const body = res.json();
    return { token: body.token, userId: body.user_id };
  }
  if (res.status === 409) {
    const login = http.post(
      `${BASE}/v1/auth/login`,
      JSON.stringify({ email, password: 'testpass123' }),
      { headers: { 'Content-Type': 'application/json' }, tags: { name: 'login' } }
    );
    const body = login.json();
    return { token: body.token, userId: body.user_id };
  }
  return null;
}

export function joinQueue(token, saleId) {
  return http.post(`${BASE}/v1/sales/${saleId}/waiting-room/join`, null, {
    headers: { Authorization: `Bearer ${token}` },
    tags: { name: 'join' },
  });
}

export function queueStatus(token, saleId) {
  return http.get(`${BASE}/v1/sales/${saleId}/waiting-room/status`, {
    headers: { Authorization: `Bearer ${token}` },
    tags: { name: 'status' },
  });
}

export function authHeaders(token) {
  return { Authorization: `Bearer ${token}` };
}

export function checkJoinOk(res) {
  return check(res, {
    'join ok': (r) => r.status === 200 || r.status === 201,
    'has position': (r) => {
      try {
        return r.json('position') > 0;
      } catch {
        return false;
      }
    },
  });
}

function envInt(name, fallback) {
  const v = __ENV[name];
  if (v === undefined || v === '') return fallback;
  const n = parseInt(v, 10);
  return Number.isFinite(n) ? n : fallback;
}

export function resolveSaleId() {
  if (__ENV.SALE_ID) return __ENV.SALE_ID;
  return adminCreateSale();
}

export function adminCreateSale() {
  const totalSeats = envInt('K6_TOTAL_SEATS', 1000);
  const admitPerMin = envInt('K6_ADMIT_PER_MIN', 12000);
  const waitingRoomCap = envInt('K6_WAITING_ROOM_CAP', 200000);

  const ev = http.post(
    `${BASE}/v1/admin/events`,
    JSON.stringify({
      name: 'Load Test Concert',
      venue: 'Arena',
      starts_at: '2026-12-01T20:00:00Z',
    }),
    {
      headers: { 'X-Admin-Key': ADMIN_KEY, 'Content-Type': 'application/json' },
    }
  );
  const eventId = ev.json('event_id');
  const now = new Date();
  const opens = new Date(now.getTime() - 60000).toISOString();
  const ends = new Date(now.getTime() + 86400000).toISOString();
  const sale = http.post(
    `${BASE}/v1/admin/sales`,
    JSON.stringify({
      event_id: eventId,
      opens_at: opens,
      ends_at: ends,
      total_seats: totalSeats,
      admit_per_minute: admitPerMin,
      waiting_room_cap: waitingRoomCap,
    }),
    {
      headers: { 'X-Admin-Key': ADMIN_KEY, 'Content-Type': 'application/json' },
    }
  );
  return sale.json('sale_id');
}
