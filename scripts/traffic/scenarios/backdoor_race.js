import http from 'k6/http';
import { check } from 'k6';
import { BASE, ADMIN_KEY, registerUser, resolveSaleId } from '../lib.js';
import { constantLoadOptions } from '../options.js';
import { handleSummaryWithThresholds } from '../thresholds.js';

export const options = {
  ...constantLoadOptions(),
  thresholds: {
    'checks{check:early join blocked}': ['rate>0.95'],
  },
};

const futureSale = (() => {
  const ev = http.post(
    `${BASE}/v1/admin/events`,
    JSON.stringify({ name: 'Future', venue: 'X', starts_at: '2027-01-01T20:00:00Z' }),
    { headers: { 'X-Admin-Key': ADMIN_KEY, 'Content-Type': 'application/json' } }
  );
  const opens = new Date(Date.now() + 3600000).toISOString();
  const ends = new Date(Date.now() + 7200000).toISOString();
  const sale = http.post(
    `${BASE}/v1/admin/sales`,
    JSON.stringify({
      event_id: ev.json('event_id'),
      opens_at: opens,
      ends_at: ends,
      total_seats: 100,
    }),
    { headers: { 'X-Admin-Key': ADMIN_KEY, 'Content-Type': 'application/json' } }
  );
  return sale.json('sale_id');
})();

const openSaleId = resolveSaleId();

export default function () {
  const user = registerUser(`backdoor_${__VU}@load.test`);
  if (!user) return;

  const early = http.post(`${BASE}/v1/sales/${futureSale}/waiting-room/join`, null, {
    headers: { Authorization: `Bearer ${user.token}` },
  });
  check(early, { 'early join blocked': (r) => r.status === 403 });

  const hold = http.post(
    `${BASE}/v1/sales/${openSaleId}/holds`,
    JSON.stringify({ seat_ids: [], idempotency_key: 'x' }),
    {
      headers: {
        Authorization: `Bearer ${user.token}`,
        'Content-Type': 'application/json',
      },
    }
  );
  check(hold, { 'hold without admission blocked': (r) => r.status === 403 });

  const checkout = http.post(
    `${BASE}/v1/sales/${openSaleId}/checkout`,
    JSON.stringify({ hold_id: 'fake', idempotency_key: 'x' }),
    {
      headers: {
        Authorization: `Bearer ${user.token}`,
        'Content-Type': 'application/json',
      },
    }
  );
  check(checkout, { 'checkout without admission blocked': (r) => r.status === 403 });
}

export function handleSummary(data) {
  return handleSummaryWithThresholds(data, { sale_id: openSaleId });
}
