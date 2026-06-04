import { check, sleep } from 'k6';
import { registerUser, joinQueue, resolveSaleId } from '../lib.js';
import { iterationOptions } from '../options.js';
import { handleSummaryWithThresholds } from '../thresholds.js';

export const options = {
  ...iterationOptions(),
  thresholds: {
    checks: ['rate>0.99'],
  },
};

const saleId = resolveSaleId();
const positions = [];

export default function () {
  const user = registerUser(`fair_${__VU}_${Date.now()}@load.test`);
  if (!user) return;

  const res = joinQueue(user.token, saleId);
  const ok = check(res, {
    joined: (r) => r.status === 200 || r.status === 201,
  });
  if (ok) {
    positions.push({
      vu: __VU,
      position: res.json('position'),
      ts: Date.now(),
    });
  }
  sleep(0.01);
}

export function teardown() {
  positions.sort((a, b) => a.ts - b.ts);
  let inversions = 0;
  for (let i = 1; i < positions.length; i++) {
    if (positions[i].position < positions[i - 1].position) {
      inversions++;
    }
  }
  console.log(`queue_fairness inversions=${inversions} samples=${positions.length}`);
}

export function handleSummary(data) {
  positions.sort((a, b) => a.ts - b.ts);
  let inversions = 0;
  for (let i = 1; i < positions.length; i++) {
    if (positions[i].position < positions[i - 1].position) {
      inversions++;
    }
  }
  return handleSummaryWithThresholds(data, { inversions, sale_id: saleId });
}
