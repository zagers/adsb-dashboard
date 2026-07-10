const MSG_MAX_HISTORY = 300;
const AC_MAX_HISTORY = 900;
const STORAGE_KEY = 'adsb-history-v2';

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

try {
  const raw = localStorage.getItem(STORAGE_KEY);
  if (raw) {
    const saved = JSON.parse(raw);
    if (saved.gainMarkers) gainChangeMarkers = saved.gainMarkers;
  }
} catch (_) {}

function updateStatus(online) {
  const el = document.getElementById('status');
  if (online) {
    el.textContent = 'live';
    el.className = 'status-indicator online';
  } else {
    el.textContent = 'disconnected';
    el.className = 'status-indicator offline';
  }
}

let pageVersion = null;

function handleStatsMessage(e) {
  try {
    const data = JSON.parse(e.data);
    if (pageVersion !== null && data.version !== pageVersion) {
      location.reload();
      return;
    }
    pageVersion = data.version;
    updateStats(data);
    pushMsgHistory(data);
    saveHistory();
    drawMsgSparkline();
    drawAcSparkline();
  } catch (err) {
    console.error('stats parse error:', err);
  }
}

function handleAircraftMessage(e) {
  try {
    const data = JSON.parse(e.data);
    data.positioning_ratio = data.tracks_heard > 0
      ? (data.positions_count / data.tracks_heard) * 100
      : 0;
    renderAircraft(data);
    pushAcHistory(data);
    saveHistory();
    drawAcSparkline();
  } catch (err) {
    console.error('aircraft parse error:', err);
  }
}

function connect() {
  const statsES = new EventSource('/events');
  statsES.onopen = () => updateStatus(true);
  statsES.onerror = () => updateStatus(false);
  statsES.onmessage = handleStatsMessage;

  const acES = new EventSource('/events/aircraft');
  acES.onmessage = handleAircraftMessage;
}

function pushMsgHistory(data) {
  if (data.gain_changed) {
    gainChangeMarkers.push(history.messagesTotal.length);
  }

  history.messagesTotal.push(data.messages_per_sec);
  history.messagesBad.push(data.bad_messages_per_sec || 0);

  while (history.messagesTotal.length > MSG_MAX_HISTORY) {
    history.messagesTotal.shift();
    history.messagesBad.shift();
    gainChangeMarkers = gainChangeMarkers.map(m => m - 1).filter(m => m >= 0);
  }
}

function pushAcHistory(data) {
  history.aircraftHeard.push(data.tracks_heard || 0);
  history.aircraftPositioned.push(data.positions_count || 0);

  while (history.aircraftHeard.length > AC_MAX_HISTORY) {
    history.aircraftHeard.shift();
    history.aircraftPositioned.shift();
  }
}

function updateStats(data) {
  document.getElementById('mps-value').textContent = data.messages_per_sec.toFixed(1);
  document.getElementById('mps-total').textContent = `total: ${data.total_messages.toLocaleString()}`;

  renderSignalAndGain(data);
  renderErrorRate(data);

  if (data.system) {
    renderSystemHealth(data.system);
  }


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
  document.getElementById('err-rate').textContent = `${data.error_rate.toFixed(1)}% bad`;
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

function drawMsgSparkline() {
  drawSparkline('msg-sparkline',
    [history.messagesTotal, history.messagesBad],
    ['#2ea043', '#8b0000'],
    gainChangeMarkers);
}

function drawAcSparkline() {
  drawSparkline('ac-sparkline',
    [history.aircraftHeard, history.aircraftPositioned],
    ['#58a6ff', '#6b7b8d'],
    []);
}

function drawSparkline(canvasId, datasets, colors, markers) {
  const canvas = document.getElementById(canvasId);
  if (!canvas) return;
  const ctx = canvas.getContext('2d');
  const dpr = window.devicePixelRatio || 1;
  const cssW = parseInt(canvas.getAttribute('width')) || canvas.width;
  const cssH = parseInt(canvas.getAttribute('height')) || canvas.height;
  if (canvas.width !== cssW * dpr) {
    canvas.width = cssW * dpr;
    canvas.height = cssH * dpr;
  }
  ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
  ctx.clearRect(0, 0, cssW, cssH);

  const primary = datasets[0];
  if (primary.length === 0) return;

  const max = Math.max(...primary, 1);
  const step = cssW / (primary.length - 1 || 1);

  function drawLine(data, color, lineWidth) {
    ctx.strokeStyle = color;
    ctx.lineWidth = lineWidth;
    ctx.beginPath();
    ctx.moveTo(0, cssH - (data[0] / max) * (cssH - 4) - 2);
    for (let i = 1; i < data.length; i++) {
      const x = i * step;
      const y = cssH - (data[i] / max) * (cssH - 4) - 2;
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
        ctx.lineTo(x, cssH);
        ctx.stroke();
      }
    }
    ctx.setLineDash([]);
  }
}

connect();
