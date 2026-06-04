import { check } from 'k6';
import { registerUser, joinQueue, queueStatus, resolveSaleId } from '../lib.js';
import { envInt } from '../options.js';
import { handleSummaryWithThresholds } from '../thresholds.js';

const vus = envInt('K6_VUS_MAX', 20);

export const options = {
  vus,
  iterations: vus,
};

const saleId = resolveSaleId();

export default function () {
  const user = registerUser(`refresh_${__VU}@load.test`);
  const join = joinQueue(user.token, saleId);
  const basePos = join.json('position');

  for (let i = 0; i < 10; i++) {
    joinQueue(user.token, saleId);
  }

  for (let i = 0; i < 50; i++) {
    const st = queueStatus(user.token, saleId);
    check(st, {
      'position stable': (r) => r.json('position') === basePos,
    });
  }
}

export function handleSummary(data) {
  return handleSummaryWithThresholds(data, { sale_id: saleId });
}
