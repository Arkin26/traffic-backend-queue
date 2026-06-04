import { sleep } from 'k6';
import { registerUser, joinQueue, resolveSaleId } from '../lib.js';
import { envInt } from '../options.js';
import { handleSummaryWithThresholds } from '../thresholds.js';

const batchSize = envInt('K6_VUS_MAX', 100);
const lateDelay = __ENV.K6_LATE_START || '15s';

export const options = {
  scenarios: {
    early: {
      executor: 'per-vu-iterations',
      vus: batchSize,
      iterations: 1,
      startTime: '0s',
      exec: 'earlyJoin',
    },
    late: {
      executor: 'per-vu-iterations',
      vus: batchSize,
      iterations: 1,
      startTime: lateDelay,
      exec: 'lateJoin',
    },
  },
};

const saleId = resolveSaleId();
let maxEarly = 0;
let minLate = Infinity;

export function earlyJoin() {
  const u = registerUser(`early_${__VU}@load.test`);
  const res = joinQueue(u.token, saleId);
  const pos = res.json('position');
  if (pos > maxEarly) maxEarly = pos;
}

export function lateJoin() {
  const u = registerUser(`late_${__VU}@load.test`);
  const res = joinQueue(u.token, saleId);
  const pos = res.json('position');
  if (pos < minLate) minLate = pos;
  sleep(0.1);
}

export function handleSummary(data) {
  const pass = maxEarly < minLate;
  return handleSummaryWithThresholds(data, {
    sale_id: saleId,
    max_early_position: maxEarly,
    min_late_position: minLate,
    fairness_pass: pass,
    fairness_reason: pass
      ? null
      : `late users leapfrogged: maxEarly=${maxEarly} minLate=${minLate}`,
  });
}
