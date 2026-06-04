import http from 'k6/http';
import { check } from 'k6';
import { BASE, registerUser, resolveSaleId } from '../lib.js';
import { envInt } from '../options.js';
import { handleSummaryWithThresholds } from '../thresholds.js';

const vus = envInt('K6_VUS_MAX', 50);

export const options = {
  vus,
  iterations: vus,
};

const saleId = resolveSaleId();
let blocked = 0;
let total = 0;

export default function () {
  const user = registerUser(`scalper_${__VU}@load.test`);

  const transfer = http.post(`${BASE}/v1/tickets/transfer`, '{}', {
    headers: { 'Content-Type': 'application/json' },
  });
  total++;
  if (transfer.status === 403) blocked++;

  const checkout = http.post(
    `${BASE}/v1/sales/${saleId}/checkout`,
    JSON.stringify({ hold_id: 'x', idempotency_key: 's' }),
    {
      headers: {
        Authorization: `Bearer ${user.token}`,
        'Content-Type': 'application/json',
        'X-Forwarded-For': '203.0.113.50',
      },
    }
  );
  total++;
  if (checkout.status === 403 || checkout.status === 429) blocked++;

  check(transfer, { 'transfer blocked': (r) => r.status === 403 });
}

export function handleSummary(data) {
  const rate = blocked / Math.max(total, 1);
  return handleSummaryWithThresholds(data, {
    sale_id: saleId,
    blocked_rate: rate,
  });
}
