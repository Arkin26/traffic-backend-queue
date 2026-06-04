// SLO evaluation aligned with scripts/traffic/config.yaml and docs/capacity.md

const DEFAULTS = {
  launch_spike: { error_rate_max: 0.01, join_p99_ms: 500 },
  queue_fairness: { inversions_max: 0 },
  refresh_storm: { position_changes_max: 0 },
};

export function scenarioName() {
  return __ENV.K6_SCENARIO || __ENV.SCENARIO || 'unknown';
}

function metricValue(data, name, field) {
  const m = data.metrics[name];
  if (!m || !m.values) return null;
  return m.values[field] ?? m.values[`${field}`] ?? null;
}

export function evaluateSummary(data, extra = {}) {
  const scn = scenarioName();
  const slo = { ...DEFAULTS[scn], ...extra };
  const failures = [];

  const httpFailed = metricValue(data, 'http_req_failed', 'rate');
  if (httpFailed !== null && slo.error_rate_max !== undefined) {
    if (httpFailed > slo.error_rate_max) {
      failures.push(`http error rate ${(httpFailed * 100).toFixed(2)}% > ${slo.error_rate_max * 100}%`);
    }
  }

  const joinP99 = metricValue(data, 'http_req_duration{name:join}', 'p(99)');
  if (joinP99 === null) {
    const allP99 = metricValue(data, 'http_req_duration', 'p(99)');
    if (allP99 !== null && slo.join_p99_ms !== undefined && allP99 > slo.join_p99_ms) {
      failures.push(`join p99 ${allP99.toFixed(0)}ms > ${slo.join_p99_ms}ms`);
    }
  } else if (slo.join_p99_ms !== undefined && joinP99 > slo.join_p99_ms) {
    failures.push(`join p99 ${joinP99.toFixed(0)}ms > ${slo.join_p99_ms}ms`);
  }

  if (extra.inversions !== undefined && slo.inversions_max !== undefined) {
    if (extra.inversions > slo.inversions_max) {
      failures.push(`queue inversions ${extra.inversions} > ${slo.inversions_max}`);
    }
  }

  if (extra.fairness_pass === false) {
    failures.push(extra.fairness_reason || 'fairness check failed');
  }

  const checksRate = metricValue(data, 'checks', 'rate');
  const thresholdFailures = data.root_group?.checks || [];
  let checksFailed = 0;
  if (data.metrics.checks && data.metrics.checks.values) {
    const passes = data.metrics.checks.values.passes || 0;
    const fails = data.metrics.checks.values.fails || 0;
    checksFailed = fails;
    if (fails > 0 && checksRate !== null && checksRate < 0.99) {
      failures.push(`checks failed: ${fails} failure(s)`);
    }
  }

  const passed = failures.length === 0;
  const p99 = metricValue(data, 'http_req_duration', 'p(99)');
  const p95 = metricValue(data, 'http_req_duration', 'p(95)');
  const p50 = metricValue(data, 'http_req_duration', 'med');
  const avg = metricValue(data, 'http_req_duration', 'avg');

  return {
    scenario: scn,
    passed,
    failures,
    checks_failed: checksFailed,
    metrics: {
      http_req_failed_rate: httpFailed,
      http_req_duration_p50_ms: p50,
      http_req_duration_p95_ms: p95,
      http_req_duration_p99_ms: p99,
      http_req_duration_avg_ms: avg,
      http_reqs: data.metrics.http_reqs?.values?.count ?? 0,
      vus_max: data.metrics.vus_max?.values?.max ?? 0,
      checks_rate: checksRate,
    },
    extra,
  };
}

export function summaryExportPath() {
  const runId = __ENV.K6_RUN_ID || __ENV.RUN_ID || '';
  if (!runId) return null;
  const dir = __ENV.K6_RESULTS_DIR || '/loadtest/results';
  return `${dir}/${runId}.json`;
}

export function handleSummaryWithThresholds(data, extra = {}) {
  const report = evaluateSummary(data, extra);
  const runId = __ENV.K6_RUN_ID || '';
  const saleId = __ENV.SALE_ID || '';
  report.run_id = runId;
  report.sale_id = saleId;
  report.finished_at = new Date().toISOString();

  console.log(
    JSON.stringify({
      event: 'loadtest_complete',
      run_id: runId,
      scenario: report.scenario,
      passed: report.passed,
      failures: report.failures,
    })
  );

  const out = {};
  const path = summaryExportPath();
  if (path) {
    out[path] = JSON.stringify(report, null, 2);
  }
  return out;
}
