import http from 'k6/http';
import { check } from 'k6';
import { BASE, registerUser, joinQueue, resolveSaleId } from '../lib.js';
import { constantLoadOptions } from '../options.js';
import { handleSummaryWithThresholds } from '../thresholds.js';

export const options = {
  ...constantLoadOptions(),
  thresholds: {
    'checks{check:bot checkout blocked}': ['rate>0.9'],
  },
};

const saleId = resolveSaleId();

export default function () {
  const user = registerUser(`bot_${__VU}_${__ITER}@load.test`);
  joinQueue(user.token, saleId);

  for (let i = 0; i < 10; i++) {
    const res = http.post(
      `${BASE}/v1/sales/${saleId}/checkout`,
      JSON.stringify({ hold_id: 'fake', idempotency_key: `bot-${__VU}-${i}` }),
      {
        headers: {
          Authorization: `Bearer ${user.token}`,
          'Content-Type': 'application/json',
          'X-Forwarded-For': '10.0.0.1',
        },
      }
    );
    check(res, {
      'bot checkout blocked': (r) =>
        r.status === 403 || r.status === 429 || r.status === 400,
    });
  }
}

export function handleSummary(data) {
  return handleSummaryWithThresholds(data, { sale_id: saleId });
}
