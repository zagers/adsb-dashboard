const history = { messages: [], aircraft: [] };
const MAX_HISTORY = 60;

function connect() {
  const es = new EventSource('/events');
  const status = document.getElementById('status');

  es.onopen = () => {
    status.textContent = 'live';
    status.className = 'status-indicator online';
  };

  es.onerror = () => {
    status.textContent = 'disconnected';
    status.className = 'status-indicator offline';
  };

  es.onmessage = (e) => {
    try {
      const data = JSON.parse(e.data);
      updateDashboard(data);
    } catch (err) {
      console.error('parse error:', err);
    }
  };
}

function updateDashboard(data) {
  const s = data.stats;

  document.getElementById('mps-value').textContent = (s.latest.valid || 0).toLocaleString();
  document.getElementById('mps-total').textContent = `total: ${(s.latest.total || 0).toLocaleString()}`;

  document.getElementById('ac-now').textContent = s.aircraft.now || 0;
  document.getElementById('ac-max').textContent = `peak: ${s.aircraft.max || 0}`;

  const strong = s.strong_ratio || (s.latest.total > 0 ? s.latest.strong / s.latest.total : 0);
  const weak = s.weak_ratio || (s.latest.total > 0 ? s.latest.weak / s.latest.total : 0);
  const err = s.error_ratio || (s.latest.total > 0 ? s.latest.error / s.latest.total : 0);
  renderSignalBar(strong, weak, err);

  if (data.system) {
    document.getElementById('sys-cpu').textContent = `CPU: ${data.system.cpu_percent.toFixed(1)}%`;
    document.getElementById('sys-mem').textContent = `MEM: ${data.system.mem_percent.toFixed(1)}%`;
  }

  const now = s.now ? new Date(s.now * 1000) : new Date();
  const age = Math.floor((Date.now() - now.getTime()) / 1000);
  document.getElementById('health-status').textContent = age < 120 ? 'healthy' : 'stale';
  document.getElementById('health-age').textContent = `last update: ${age}s ago`;

  history.messages.push(s.latest.valid || 0);
  history.aircraft.push(s.aircraft.now || 0);
  if (history.messages.length > MAX_HISTORY) history.messages.shift();
  if (history.aircraft.length > MAX_HISTORY) history.aircraft.shift();

  drawSparkline('msg-sparkline', history.messages, '#2ea043');
  drawSparkline('ac-sparkline', history.aircraft, '#58a6ff');
}

function renderSignalBar(strong, weak, err) {
  const bar = document.getElementById('signal-bar');
  bar.innerHTML = `
    <div class="bar-segment strong" style="width:${(strong * 100).toFixed(1)}%"></div>
    <div class="bar-segment weak" style="width:${(weak * 100).toFixed(1)}%"></div>
    <div class="bar-segment error" style="width:${(err * 100).toFixed(1)}%"></div>
  `;
  document.getElementById('signal-labels').textContent =
    `${(strong * 100).toFixed(0)}% strong · ${(weak * 100).toFixed(0)}% weak · ${(err * 100).toFixed(0)}% error`;
}

function drawSparkline(canvasId, values, color) {
  const canvas = document.getElementById(canvasId);
  const ctx = canvas.getContext('2d');
  const w = canvas.width;
  const h = canvas.height;
  ctx.clearRect(0, 0, w, h);

  if (values.length < 2) return;

  const max = Math.max(...values, 1);
  const step = w / (values.length - 1);

  ctx.beginPath();
  ctx.strokeStyle = color;
  ctx.lineWidth = 2;
  ctx.moveTo(0, h - (values[0] / max) * (h - 4) - 2);

  for (let i = 1; i < values.length; i++) {
    const x = i * step;
    const y = h - (values[i] / max) * (h - 4) - 2;
    ctx.lineTo(x, y);
  }
  ctx.stroke();
}

connect();
