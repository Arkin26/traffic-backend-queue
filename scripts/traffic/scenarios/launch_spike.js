import { check } from 'k6';
import {
  registerUser,
  joinQueue,
  queueStatus,
  resolveSaleId,
} from '../lib.js';
import { rampingScenario } from '../options.js';
import { handleSummaryWithThresholds } from '../thresholds.js';

export const options = {
  ...rampingScenario('spike'),
  thresholds: {
    http_req_failed: ['rate<0.01'],
    'http_req_duration{name:join}': ['p(99)<500'],
  },
};

const saleId = resolveSaleId();

export default function () {
  const email = `spike_${__VU}_${__ITER}@load.test`;
  const user = registerUser(email);
  if (!user) return;

  const join = joinQueue(user.token, saleId);
  check(join, {
    'join not 5xx': (r) => r.status < 500,
    'join accepted or limited': (r) =>
      r.status === 200 || r.status === 201 || r.status === 429 || r.status === 503,
  });

  queueStatus(user.token, saleId);
}

export function handleSummary(data) {
  return handleSummaryWithThresholds(data, { sale_id: saleId });
}
