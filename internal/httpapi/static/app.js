let saleId = null;
let activeRunId = null;
let pollRunTimer = null;
const queueHistory = [];
const MAX_SPARK = 60;

let demoLimits = { max_vus: 500, demo_key_required: false };
let demoKey = sessionStorage.getItem('traffic_demo_key') || '';

const PRESETS = {
  launch: {
    scenario: 'launch_spike',
    vusMax: 150,
    rampUp: '10s',
    steady: '30s',
    rampDown: '10s',
    admitPerMin: 6000,
    waitingRoomCap: 100000,
    totalSeats: 500,
  },
  refresh: {
    scenario: 'refresh_storm',
    vusMax: 20,
    rampUp: '5s',
    steady: '20s',
    rampDown: '5s',
    admitPerMin: 6000,
    waitingRoomCap: 50000,
    totalSeats: 500,
  },
  fairness: {
    scenario: 'queue_fairness',
    vusMax: 200,
    rampUp: '5s',
    steady: '15s',
    rampDown: '5s',
    admitPerMin: 3000,
    waitingRoomCap: 100000,
    totalSeats: 500,
  },
  bots: {
    scenario: 'bot_checkout',
    vusMax: 100,
    rampUp: '5s',
    steady: '20s',
    rampDown: '5s',
    admitPerMin: 6000,
    waitingRoomCap: 100000,
    totalSeats: 500,
  },
  mixed: {
    scenario: 'mixed_realistic',
    vusMax: 80,
    rampUp: '30s',
    steady: '60s',
    rampDown: '30s',
    admitPerMin: 6000,
    waitingRoomCap: 100000,
    totalSeats: 1000,
  },
};

function clampVUs(n) {
  const max = demoLimits.max_vus || 500;
  return Math.min(Math.max(n, 10), max);
}

async function api(path, opts = {}) {
  const headers = { 'Content-Type': 'application/json', ...(opts.headers || {}) };
  if (token && !headers.Authorization) {
    headers.Authorization = `Bearer ${token}`;
  }
  if (path.startsWith('/v1/demo') && demoKey) {
    headers['X-Demo-Key'] = demoKey;
  }
  const res = await fetch(path, { ...opts, headers });
  const text = await res.text();
  let body = {};
  try {
    body = JSON.parse(text);
  } catch {
    body = { raw: text };
  }
  return { ok: res.ok, status: res.status, body };
}

function readConfig() {
  return {
    scenario: document.getElementById('scenario').value,
    vus_max: parseInt(document.getElementById('vusMax').value, 10),
    ramp_up: document.getElementById('rampUp').value,
    steady: document.getElementById('steady').value,
    ramp_down: document.getElementById('rampDown').value,
    admit_per_minute: parseInt(document.getElementById('admitPerMin').value, 10),
    waiting_room_cap: parseInt(document.getElementById('waitingRoomCap').value, 10),
    total_seats: parseInt(document.getElementById('totalSeats').value, 10),
    sale_id: saleId || undefined,
  };
}

async function loadDemoLimits() {
  const res = await api('/v1/demo/config');
  if (!res.ok) return;
  demoLimits = res.body;
  const slider = document.getElementById('vusMax');
  const max = demoLimits.max_vus || 500;
  slider.max = max;
  slider.value = clampVUs(parseInt(slider.value, 10));
  document.getElementById('vusVal').textContent = slider.value;
  const keyRow = document.getElementById('demoKeyRow');
  if (keyRow) {
    keyRow.classList.toggle('hidden', !demoLimits.demo_key_required);
  }
  const hint = document.getElementById('deployHint');
  if (hint && demoLimits.max_concurrent === 1) {
    hint.textContent =
      `Shared lab: max ${max} VUs, one k6 run at a time (queue up to ${demoLimits.max_queue || 3}).`;
  }
}

function applyPreset(name) {
  const p = PRESETS[name];
  if (!p) return;
  document.getElementById('scenario').value = p.scenario;
  const vus = clampVUs(p.vusMax);
  document.getElementById('vusMax').value = vus;
  document.getElementById('vusVal').textContent = vus;
  document.getElementById('rampUp').value = p.rampUp;
  document.getElementById('steady').value = p.steady;
  document.getElementById('rampDown').value = p.rampDown;
  document.getElementById('admitPerMin').value = p.admitPerMin;
  document.getElementById('waitingRoomCap').value = p.waitingRoomCap;
  document.getElementById('totalSeats').value = p.totalSeats;
}

function setRunBadge(status, text) {
  const el = document.getElementById('runBadge');
  el.textContent = text || status;
  el.className = `run-badge ${status}`;
}

async function loadScenarios() {
  const res = await api('/v1/demo/loadtests');
  const sel = document.getElementById('scenario');
  const list = res.body.scenarios || [
    'launch_spike', 'queue_fairness', 'refresh_storm', 'backdoor_race',
    'late_surge', 'reconnect_churn', 'bot_checkout', 'scalper_network',
    'second_sale_regression', 'mixed_realistic',
  ];
  sel.innerHTML = '';
  list.forEach((s) => {
    const o = document.createElement('option');
    o.value = s;
    o.textContent = s.replace(/_/g, ' ');
    sel.appendChild(o);
  });
  renderHistory(res.body.runs || []);
}

function renderHistory(runs) {
  const ul = document.getElementById('runHistory');
  ul.innerHTML = '';
  runs.slice(0, 8).forEach((r) => {
    const li = document.createElement('li');
    const tag = r.passed ? 'PASS' : (r.status === 'running' ? '…' : 'FAIL');
    li.textContent = `${r.scenario || '?'} — ${tag}`;
    li.title = r.run_id;
    li.onclick = () => showRunResult(r);
    ul.appendChild(li);
  });
}

function showRunResult(r) {
  const card = document.getElementById('resultCard');
  card.classList.remove('hidden');
  const verdict = document.getElementById('resultVerdict');
  if (r.status === 'queued' || r.status === 'running') {
    verdict.textContent = `Running: ${r.scenario || ''}`;
    verdict.className = 'verdict running';
    return;
  }
  verdict.textContent = r.passed ? 'PASS' : 'FAIL';
  verdict.className = `verdict ${r.passed ? 'pass' : 'fail'}`;

  const fl = document.getElementById('resultFailures');
  fl.innerHTML = '';
  (r.failures || []).forEach((f) => {
    const li = document.createElement('li');
    li.textContent = f;
    fl.appendChild(li);
  });

  const m = r.metrics || {};
  document.getElementById('resP99').textContent =
    m.http_req_duration_p99_ms != null ? `${Math.round(m.http_req_duration_p99_ms)} ms` : '—';
  const err = m.http_req_failed_rate;
  document.getElementById('resErr').textContent =
    err != null ? `${(err * 100).toFixed(2)}%` : '—';
  document.getElementById('resReqs').textContent = m.http_reqs ?? '—';
}

async function resetSale() {
  const cfg = readConfig();
  const res = await api('/v1/demo/reset', {
    method: 'POST',
    body: JSON.stringify({
      admit_per_minute: cfg.admit_per_minute,
      waiting_room_cap: cfg.waiting_room_cap,
      total_seats: cfg.total_seats,
    }),
  });
  if (res.ok) {
    saleId = res.body.sale_id;
    document.getElementById('saleId').value = saleId;
    document.getElementById('runMeta').textContent = `Fresh sale: ${saleId.slice(0, 8)}…`;
  }
}

async function startLoadTest() {
  const cfg = readConfig();
  setRunBadge('queued', 'Queued');
  document.getElementById('runMeta').textContent = `Starting ${cfg.scenario}…`;
  document.getElementById('resultCard').classList.add('hidden');

  const res = await api('/v1/demo/loadtest', {
    method: 'POST',
    body: JSON.stringify(cfg),
  });
  if (!res.ok) {
    setRunBadge('fail', 'Error');
    const msg = res.body.message || res.body.error || 'Start failed';
    document.getElementById('runMeta').textContent =
      res.status === 429 ? `${msg} — wait for the current run to finish` : msg;
    return;
  }
  activeRunId = res.body.run_id;
  saleId = res.body.sale_id || saleId;
  document.getElementById('saleId').value = saleId || '';
  setRunBadge('running', 'Running');
  document.getElementById('runMeta').textContent =
    `Run ${activeRunId.slice(0, 8)}… — ${cfg.scenario} @ ${cfg.vus_max} VUs`;

  if (pollRunTimer) clearInterval(pollRunTimer);
  pollRunTimer = setInterval(pollRun, 1000);
  pollRun();
}

function finishRun(r) {
  clearInterval(pollRunTimer);
  pollRunTimer = null;
  document.getElementById('resultCard').classList.remove('hidden');
  showRunResult(r);
  if (r.status === 'error') {
    setRunBadge('fail', 'Error');
    document.getElementById('runMeta').textContent = r.error || 'Load test failed to run';
    return;
  }
  setRunBadge(r.passed ? 'pass' : 'fail', r.passed ? 'Passed' : 'Failed');
  document.getElementById('runMeta').textContent =
    `${r.scenario || 'Test'} finished — ${r.passed ? 'PASS' : 'FAIL'}`;
}

async function pollRun() {
  if (!activeRunId) return;
  const res = await api(`/v1/demo/loadtest/${activeRunId}`);
  if (!res.ok) return;

  const r = res.body;
  if (r.status === 'queued') {
    setRunBadge('queued', 'Queued');
    document.getElementById('runMeta').textContent =
      'Waiting for load test runner… (ensure loadtest-runner container is up)';
    return;
  }
  if (r.status === 'running') {
    setRunBadge('running', 'Running');
    document.getElementById('runMeta').textContent =
      `k6 running ${r.scenario || ''} — watch live metrics →`;
    document.getElementById('resultCard').classList.add('hidden');
    return;
  }

  finishRun(r);

  const list = await api('/v1/demo/loadtests');
  if (list.ok) renderHistory(list.body.runs || []);
}

function drawSparkline() {
  const canvas = document.getElementById('sparkline');
  if (!canvas) return;
  const ctx = canvas.getContext('2d');
  const w = canvas.width;
  const h = canvas.height;
  ctx.fillStyle = '#0a0a0a';
  ctx.fillRect(0, 0, w, h);
  if (queueHistory.length < 2) return;
  const max = Math.max(...queueHistory, 1);
  ctx.strokeStyle = '#d32f2f';
  ctx.lineWidth = 2;
  ctx.beginPath();
  queueHistory.forEach((v, i) => {
    const x = (i / (MAX_SPARK - 1)) * w;
    const y = h - (v / max) * (h - 4) - 2;
    if (i === 0) ctx.moveTo(x, y);
    else ctx.lineTo(x, y);
  });
  ctx.stroke();
}

async function pollDashboard() {
  const q = saleId ? `?sale_id=${saleId}` : '';
  const res = await api(`/v1/demo/dashboard${q}`);
  if (!res.ok) return;
  const d = res.body;

  const h = document.getElementById('health');
  h.textContent = d.health;
  h.className = `health ${d.health}`;

  document.getElementById('mQueue').textContent = d.queue_depth;
  document.getElementById('mAdmitted').textContent = d.admitted_count;
  document.getElementById('mTickets').textContent = d.tickets_sold;
  document.getElementById('mSeats').textContent = d.available_seats;
  document.getElementById('mReq').textContent = d.simulation?.requests_total ?? 0;
  document.getElementById('mBlocked').textContent = d.simulation?.blocked_total ?? 0;
  document.getElementById('mErr').textContent = `${(d.error_rate_pct || 0).toFixed(1)}%`;
  document.getElementById('mLat').textContent = `${d.last_latency_ms || 0} ms`;
  document.getElementById('mRps').textContent = (d.requests_per_sec_approx || 0).toFixed(1);

  const active = activeRunId ? `Load test running` : 'Monitoring sale';
  document.getElementById('mActive').textContent = active;

  queueHistory.push(d.queue_depth || 0);
  if (queueHistory.length > MAX_SPARK) queueHistory.shift();
  drawSparkline();
}

// Optional single-user flow
let token = null;

async function registerAndJoin() {
  const email = document.getElementById('email').value || `fan_${Date.now()}@demo.com`;
  let res = await api('/v1/auth/register', {
    method: 'POST',
    body: JSON.stringify({ email, password: 'demo1234' }),
  });
  if (!res.ok) {
    res = await api('/v1/auth/login', {
      method: 'POST',
      body: JSON.stringify({ email, password: 'demo1234' }),
    });
  }
  if (!res.ok) {
    document.getElementById('userInfo').textContent = 'Auth failed';
    return;
  }
  token = res.body.token;
  document.getElementById('userInfo').textContent = `Logged in as ${email}`;
  if (!saleId) await resetSale();
  const join = await api(`/v1/sales/${saleId}/waiting-room/join`, { method: 'POST' });
  if (join.ok) {
    document.getElementById('position').textContent = join.body.position;
    document.getElementById('queueMessage').textContent = join.body.message || '';
  }
}

document.getElementById('vusMax').oninput = (e) => {
  document.getElementById('vusVal').textContent = e.target.value;
};
document.getElementById('btnStart').onclick = startLoadTest;
document.getElementById('btnReset').onclick = resetSale;
document.getElementById('btnRegister').onclick = registerAndJoin;
document.querySelectorAll('[data-preset]').forEach((btn) => {
  btn.onclick = () => applyPreset(btn.dataset.preset);
});

document.getElementById('demoKey')?.addEventListener('change', (e) => {
  demoKey = e.target.value.trim();
  sessionStorage.setItem('traffic_demo_key', demoKey);
});

const savedKey = document.getElementById('demoKey');
if (savedKey && demoKey) savedKey.value = demoKey;

loadDemoLimits().then(() => {
  loadScenarios();
  resetSale();
});
setInterval(pollDashboard, 1000);
