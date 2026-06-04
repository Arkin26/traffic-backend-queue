import { check, sleep } from 'k6';
import { registerUser, joinQueue, queueStatus, resolveSaleId } from '../lib.js';
import { constantLoadOptions } from '../options.js';
import { handleSummaryWithThresholds } from '../thresholds.js';

export const options = constantLoadOptions();

const saleId = resolveSaleId();

export default function () {
  const user = registerUser(`churn_${__VU}@load.test`);
  const join = joinQueue(user.token, saleId);
  const pos = join.json('position');

  sleep(Math.random() * 3 + 1);

  const after = queueStatus(user.token, saleId);
  check(after, {
    'position after disconnect': (r) => r.json('position') === pos,
  });
}

export function handleSummary(data) {
  return handleSummaryWithThresholds(data, { sale_id: saleId });
}
