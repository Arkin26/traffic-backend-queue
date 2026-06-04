import { sleep } from 'k6';
import { registerUser, joinQueue, resolveSaleId } from '../lib.js';
import { envInt } from '../options.js';
import { handleSummaryWithThresholds } from '../thresholds.js';

const batchSize = envInt('K6_VUS_MAX', 150);
const batchBStart = __ENV.K6_LATE_START || '20s';

export const options = {
  scenarios: {
    batchA: {
      executor: 'shared-iterations',
      vus: batchSize,
      iterations: batchSize,
      exec: 'batchA',
    },
    batchB: {
      executor: 'shared-iterations',
      vus: batchSize,
      iterations: batchSize,
      startTime: batchBStart,
      exec: 'batchB',
    },
  },
};

const saleId = resolveSaleId();
const batchAPos = [];
const batchBPos = [];

export function batchA() {
  const u = registerUser(`a_${__ITER}_${__VU}@load.test`);
  const p = joinQueue(u.token, saleId).json('position');
  batchAPos.push(p);
  sleep(0.05);
}

export function batchB() {
  const u = registerUser(`b_${__ITER}_${__VU}@load.test`);
  const p = joinQueue(u.token, saleId).json('position');
  batchBPos.push(p);
}

export function handleSummary(data) {
  const maxA = batchAPos.length ? Math.max(...batchAPos) : 0;
  const minB = batchBPos.length ? Math.min(...batchBPos) : Infinity;
  const pass = maxA < minB;
  return handleSummaryWithThresholds(data, {
    sale_id: saleId,
    max_a: maxA,
    min_b: minB,
    fairness_pass: pass,
    fairness_reason: pass
      ? null
      : `batch B leapfrogged: maxA=${maxA} minB=${minB}`,
  });
}
