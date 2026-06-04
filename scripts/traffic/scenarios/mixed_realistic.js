import { sleep } from 'k6';
import { registerUser, joinQueue, queueStatus, resolveSaleId } from '../lib.js';
import { envInt, envStr } from '../options.js';
import { handleSummaryWithThresholds } from '../thresholds.js';

const humanVus = envInt('K6_VUS_MAX', 80);
const botVus = envInt('K6_BOT_VUS', 10);
const duration = envStr('K6_STEADY', '60s');

export const options = {
  scenarios: {
    humans: {
      executor: 'ramping-vus',
      startVUs: 10,
      stages: [
        { duration: '30s', target: humanVus },
        { duration: '30s', target: 0 },
      ],
      exec: 'human',
    },
    bots: {
      executor: 'constant-vus',
      vus: botVus,
      duration,
      exec: 'bot',
    },
  },
};

const saleId = resolveSaleId();

export function human() {
  const u = registerUser(`human_${__VU}@load.test`);
  joinQueue(u.token, saleId);
  sleep(Math.random() * 2 + 0.5);
  queueStatus(u.token, saleId);
  sleep(1);
}

export function bot() {
  const u = registerUser(`botmix_${__VU}@load.test`);
  for (let i = 0; i < 5; i++) {
    joinQueue(u.token, saleId);
  }
  sleep(0.1);
}

export function handleSummary(data) {
  return handleSummaryWithThresholds(data, { sale_id: saleId });
}
