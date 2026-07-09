const MAX_HISTORY = 300;
const STORAGE_KEY = 'adsb-history';

function loadHistory() {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (raw) {
      const saved = JSON.parse(raw);
      if (saved.messagesTotal && saved.messagesTotal.length > 0) {
        return {
          messagesTotal: saved.messagesTotal,
          messagesBad: saved.messagesBad || [],
          aircraftHeard: saved.aircraftHeard || [],
          aircraftPositioned: saved.aircraftPositioned || []
        };
      }
    }
  } catch (_) {}
  return { messagesTotal: [], messagesBad: [], aircraftHeard: [], aircraftPositioned: [] };
}

function saveHistory() {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify({
      messagesTotal: history.messagesTotal,
      messagesBad: history.messagesBad,
      aircraftHeard: history.aircraftHeard,
      aircraftPositioned: history.aircraftPositioned,
      gainMarkers: gainChangeMarkers
    }));
  } catch (_) {}
}

const history = loadHistory();
let gainChangeMarkers = [];

// restore markers from storage
try {
  const raw = localStorage.getItem(STORAGE_KEY);
  if (raw) {
    const saved = JSON.parse(raw);
    if (saved.gainMarkers) gainChangeMarkers = saved.gainMarkers;
  }
} catch (_) {}

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
  document.getElementById('mps-value').textContent = data.messages_per_sec.toFixed(1);
  document.getElementById('mps-total').textContent = `total: ${data.total_messages.toLocaleString()}`;

  renderAircraft(data);
  renderSignalAndGain(data);
  renderErrorRate(data);

  if (data.system) {
    renderSystemHealth(data.system);
  }

  const s = data.stats;
  const lastMin = s ? s.last1min : null;
  const age = lastMin ? Math.floor((Date.now() / 1000) - lastMin.end) : 0;
  const healthEl = document.getElementById('health-status');
  if (healthEl) {
    healthEl.textContent = age < 120 ? 'healthy' : 'stale';
  }
  const healthAge = document.getElementById('health-age');
  if (healthAge) {
    healthAge.textContent = `last update: ${age}s ago`;
  }

  if (data.gain_changed) {
    gainChangeMarkers.push(history.messagesTotal.length);
  }

  history.messagesTotal.push(data.messages_per_sec);
  history.messagesBad.push(data.bad_messages_per_sec || 0);
  history.aircraftHeard.push(data.tracks_heard || 0);
  history.aircraftPositioned.push(data.positions_count || 0);

  if (history.messagesTotal.length > MAX_HISTORY) {
    history.messagesTotal.shift();
    history.messagesBad.shift();
    history.aircraftHeard.shift();
    history.aircraftPositioned.shift();
    gainChangeMarkers = gainChangeMarkers.map(m => m - 1).filter(m => m >= 0);
  }

  saveHistory();

  drawSparkline('msg-sparkline',
    [history.messagesTotal, history.messagesBad],
    ['#2ea043', '#8b0000'],
    gainChangeMarkers);

  drawSparkline('ac-sparkline',
    [history.aircraftHeard, history.aircraftPositioned],
    ['#58a6ff', '#6b7b8d'],
    gainChangeMarkers);
}

function renderAircraft(data) {
  const heard = data.tracks_heard || 0;
  const pos = data.positions_count || 0;
  const ratio = data.positioning_ratio || 0;

  document.getElementById('ac-heard').innerHTML = `${heard} <span class="unit">heard</span>`;
  document.getElementById('ac-positioned').textContent = `${pos} positioned (${ratio.toFixed(0)}%)`;

  const ratioEl = document.getElementById('ac-pos-ratio');
  if (ratioEl) {
    ratioEl.textContent = ratio.toFixed(0);
    ratioEl.className = ratio >= 75 ? 'ratio-high' : ratio >= 50 ? 'ratio-mid' : 'ratio-low';
  }
}

function renderSignalAndGain(data) {
  const bar = document.getElementById('signal-bar');
  const pct = (data.strong_signal_ratio * 100).toFixed(1);
  bar.innerHTML = `
    <div class="bar-segment strong" style="width:${pct}%"></div>
    <div class="bar-segment weak" style="width:${(100 - pct).toFixed(1)}%"></div>
  `;

  const noise = data.noise_floor;
  const noiseClass = noise < -30 ? 'noise-green' : noise > -20 ? 'noise-red' : 'noise-yellow';

  document.getElementById('signal-labels').innerHTML =
    `<span>SNR: ${data.snr.toFixed(1)} dB</span>` +
    `<span>Gain: ${(data.gain_db || 0).toFixed(1)} dB</span>` +
    `<span class="${noiseClass}">Noise: ${noise.toFixed(1)} dB</span>`;
}

function renderErrorRate(data) {
  const pct = (data.error_rate * 100).toFixed(1);
  document.getElementById('err-rate').textContent = `${pct}% bad`;
  document.getElementById('err-bad-mps').textContent = `${(data.bad_messages_per_sec || 0).toFixed(1)} bad/s`;
  document.getElementById('err-strong').textContent = `${data.strong_signals_count || 0} strong pings`;
}

function renderSystemHealth(sys) {
  document.getElementById('sys-cpu').textContent = `CPU: ${sys.cpu_percent.toFixed(1)}%`;
  document.getElementById('sys-mem').textContent = `MEM: ${sys.mem_percent.toFixed(1)}%`;
  const throttledEl = document.getElementById('sys-throttled');
  const status = sys.throttled || 'Unknown';
  throttledEl.textContent = `Throttled: ${status}`;
  throttledEl.className = status === 'OK' ? 'throttled-ok' : 'throttled-warn';
}

function drawSparkline(canvasId, datasets, colors, markers) {
  const canvas = document.getElementById(canvasId);
  if (!canvas) return;
  const ctx = canvas.getContext('2d');
  const w = canvas.width;
  const h = canvas.height;
  ctx.clearRect(0, 0, w, h);

  const primary = datasets[0];
  if (primary.length === 0) return;

  const max = Math.max(...primary, 1);
  const step = w / (primary.length - 1 || 1);

  function drawLine(data, color, lineWidth) {
    ctx.strokeStyle = color;
    ctx.lineWidth = lineWidth;
    ctx.beginPath();
    ctx.moveTo(0, h - (data[0] / max) * (h - 4) - 2);
    for (let i = 1; i < data.length; i++) {
      const x = i * step;
      const y = h - (data[i] / max) * (h - 4) - 2;
      ctx.lineTo(x, y);
    }
    ctx.stroke();
  }

  drawLine(primary, colors[0], 2);

  if (datasets.length > 1 && datasets[1].length > 0) {
    drawLine(datasets[1], colors[1], 1.5);
  }

  if (markers && markers.length > 0) {
    ctx.strokeStyle = '#f0883e';
    ctx.lineWidth = 1;
    ctx.setLineDash([3, 3]);
    for (const idx of markers) {
      if (idx >= 0 && idx < primary.length) {
        const x = idx * step;
        ctx.beginPath();
        ctx.moveTo(x, 0);
        ctx.lineTo(x, h);
        ctx.stroke();
      }
    }
    ctx.setLineDash([]);
  }
}

connect();
