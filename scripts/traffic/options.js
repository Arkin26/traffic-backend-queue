// Shared k6 options from environment (UI / loadtest-runner).
export function envInt(name, fallback) {
  const v = __ENV[name];
  if (v === undefined || v === '') return fallback;
  const n = parseInt(v, 10);
  return Number.isFinite(n) ? n : fallback;
}

export function envStr(name, fallback) {
  const v = __ENV[name];
  return v !== undefined && v !== '' ? v : fallback;
}

export function rampingScenario(name = 'load') {
  const maxVus = envInt('K6_VUS_MAX', 500);
  const rampUp = envStr('K6_RAMP_UP', '10s');
  const steady = envStr('K6_STEADY', '30s');
  const rampDown = envStr('K6_RAMP_DOWN', '10s');
  const quickRamp = envStr('K6_QUICK_RAMP', '5s');

  return {
    scenarios: {
      [name]: {
        executor: 'ramping-vus',
        startVUs: 0,
        stages: [
          { duration: quickRamp, target: Math.min(100, maxVus) },
          { duration: rampUp, target: maxVus },
          { duration: steady, target: maxVus },
          { duration: rampDown, target: 0 },
        ],
      },
    },
  };
}

export function constantLoadOptions() {
  const maxVus = envInt('K6_VUS_MAX', 100);
  const duration = envStr('K6_STEADY', '30s');
  return {
    vus: maxVus,
    duration,
  };
}

export function iterationOptions() {
  const maxVus = envInt('K6_VUS_MAX', 200);
  return {
    vus: maxVus,
    iterations: maxVus,
  };
}
