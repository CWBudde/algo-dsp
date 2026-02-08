const NOTE_FREQS = [
  ["A2", 110],
  ["C3", 130.81],
  ["E3", 164.81],
  ["G3", 196],
  ["A3", 220],
  ["C4", 261.63],
  ["E4", 329.63],
  ["G4", 392],
  ["A4", 440],
  ["C5", 523.25],
];

const STEP_COUNT = 16;

const state = {
  audioCtx: null,
  isRunning: false,
  currentStep: 0,
  nextNoteTime: 0,
  scheduler: null,
  steps: [],
  eq: null,
  chart: null,
  chartFreqs: null,
};

const el = {
  runToggle: document.getElementById("run-toggle"),
  tempo: document.getElementById("tempo"),
  tempoValue: document.getElementById("tempo-value"),
  decay: document.getElementById("decay"),
  decayValue: document.getElementById("decay-value"),
  steps: document.getElementById("steps"),
  lowGain: document.getElementById("low-gain"),
  lowGainValue: document.getElementById("low-gain-value"),
  midGain: document.getElementById("mid-gain"),
  midGainValue: document.getElementById("mid-gain-value"),
  midQ: document.getElementById("mid-q"),
  midQValue: document.getElementById("mid-q-value"),
  highGain: document.getElementById("high-gain"),
  highGainValue: document.getElementById("high-gain-value"),
  master: document.getElementById("master"),
  masterValue: document.getElementById("master-value"),
};

function buildStepUI() {
  for (let i = 0; i < STEP_COUNT; i += 1) {
    const step = document.createElement("div");
    step.className = "step";
    step.dataset.index = String(i);

    const head = document.createElement("div");
    head.className = "step-head";
    head.innerHTML = `<strong>${i + 1}</strong>`;

    const enabled = document.createElement("input");
    enabled.type = "checkbox";
    enabled.checked = i % 4 === 0;
    head.appendChild(enabled);

    const noteSelect = document.createElement("select");
    NOTE_FREQS.forEach(([label, freq], idx) => {
      const opt = document.createElement("option");
      opt.value = String(freq);
      opt.textContent = label;
      if (idx === (i % 8) + 1) opt.selected = true;
      noteSelect.appendChild(opt);
    });

    step.appendChild(head);
    step.appendChild(noteSelect);
    el.steps.appendChild(step);

    state.steps.push({
      enabled,
      noteSelect,
      node: step,
    });
  }
}

function setupAudio() {
  if (state.audioCtx) return;
  const ctx = new AudioContext();

  const low = ctx.createBiquadFilter();
  low.type = "lowshelf";
  low.frequency.value = 100;

  const mid = ctx.createBiquadFilter();
  mid.type = "peaking";
  mid.frequency.value = 1000;

  const high = ctx.createBiquadFilter();
  high.type = "highshelf";
  high.frequency.value = 6000;

  const master = ctx.createGain();
  master.gain.value = Number(el.master.value);

  low.connect(mid);
  mid.connect(high);
  high.connect(master);
  master.connect(ctx.destination);

  state.audioCtx = ctx;
  state.eq = { low, mid, high, master };
  applyEQControls();
  updateChart();
}

function applyEQControls() {
  const lowGain = Number(el.lowGain.value);
  const midGain = Number(el.midGain.value);
  const midQ = Number(el.midQ.value);
  const highGain = Number(el.highGain.value);
  const master = Number(el.master.value);

  if (state.eq) {
    state.eq.low.gain.value = lowGain;
    state.eq.mid.gain.value = midGain;
    state.eq.mid.Q.value = midQ;
    state.eq.high.gain.value = highGain;
    state.eq.master.gain.value = master;
  }

  el.lowGainValue.textContent = `${lowGain.toFixed(1)} dB`;
  el.midGainValue.textContent = `${midGain.toFixed(1)} dB`;
  el.midQValue.textContent = midQ.toFixed(1);
  el.highGainValue.textContent = `${highGain.toFixed(1)} dB`;
  el.masterValue.textContent = master.toFixed(2);
}

function playStep(stepIndex, when) {
  if (!state.audioCtx || !state.eq) return;
  const step = state.steps[stepIndex];
  if (!step.enabled.checked) return;

  const freq = Number(step.noteSelect.value);
  const decay = Number(el.decay.value);
  const amp = 0.22;

  const osc = state.audioCtx.createOscillator();
  osc.type = "sine";
  osc.frequency.value = freq;

  const env = state.audioCtx.createGain();
  env.gain.setValueAtTime(0.0001, when);
  env.gain.exponentialRampToValueAtTime(amp, when + 0.005);
  env.gain.exponentialRampToValueAtTime(0.0001, when + decay);

  osc.connect(env);
  env.connect(state.eq.low);

  osc.start(when);
  osc.stop(when + decay + 0.02);
}

function stepDurationSeconds() {
  const bpm = Number(el.tempo.value);
  return 60 / bpm / 4;
}

function schedule() {
  const lookahead = 0.1;
  while (state.nextNoteTime < state.audioCtx.currentTime + lookahead) {
    playStep(state.currentStep, state.nextNoteTime);
    highlightStep(state.currentStep);
    state.nextNoteTime += stepDurationSeconds();
    state.currentStep = (state.currentStep + 1) % STEP_COUNT;
  }
}

function highlightStep(index) {
  state.steps.forEach((s, i) => {
    s.node.classList.toggle("current", i === index);
  });
}

function startSequencer() {
  if (!state.audioCtx) return;
  if (state.audioCtx.state === "suspended") state.audioCtx.resume();
  if (state.isRunning) return;

  state.isRunning = true;
  state.currentStep = 0;
  state.nextNoteTime = state.audioCtx.currentTime + 0.05;
  state.scheduler = setInterval(schedule, 25);
  el.runToggle.textContent = "Stop";
  el.runToggle.classList.add("active");
}

function stopSequencer() {
  if (!state.isRunning) return;
  clearInterval(state.scheduler);
  state.scheduler = null;
  state.isRunning = false;
  el.runToggle.textContent = "Play";
  el.runToggle.classList.remove("active");
  highlightStep(-1);
}

function initChart() {
  if (state.chart) return;
  const ctx = document.getElementById("eq-chart");
  state.chartFreqs = chartFrequencies();
  const labels = Array.from(state.chartFreqs, (hz) => {
    if (hz >= 1000) return `${(hz / 1000).toFixed(1)}k`;
    return `${Math.round(hz)}`;
  });

  state.chart = new Chart(ctx, {
    type: "line",
    data: {
      labels,
      datasets: [
        {
          label: "Magnitude (dB)",
          data: labels.map(() => 0),
          borderColor: "#225d7d",
          borderWidth: 2,
          pointRadius: 0,
          tension: 0.18,
        },
      ],
    },
    options: {
      responsive: true,
      animation: false,
      plugins: { legend: { display: false } },
      scales: {
        x: {
          ticks: { maxTicksLimit: 8 },
        },
        y: {
          min: -24,
          max: 24,
          title: { display: true, text: "dB" },
        },
      },
    },
  });
}

function chartFrequencies() {
  const n = 128;
  const min = 20;
  const max = 20000;
  const out = new Float32Array(n);
  const ratio = Math.pow(max / min, 1 / (n - 1));
  out[0] = min;
  for (let i = 1; i < n; i += 1) out[i] = out[i - 1] * ratio;
  return out;
}

function biquadMagnitudeAt(freq, sampleRate, c) {
  const omega = (2 * Math.PI * freq) / sampleRate;
  const cos1 = Math.cos(omega);
  const sin1 = Math.sin(omega);
  const cos2 = Math.cos(2 * omega);
  const sin2 = Math.sin(2 * omega);

  const numRe = c.b0 + c.b1 * cos1 + c.b2 * cos2;
  const numIm = -(c.b1 * sin1 + c.b2 * sin2);
  const denRe = c.a0 + c.a1 * cos1 + c.a2 * cos2;
  const denIm = -(c.a1 * sin1 + c.a2 * sin2);

  const numPow = numRe * numRe + numIm * numIm;
  const denPow = denRe * denRe + denIm * denIm;
  return Math.sqrt(Math.max(1e-20, numPow / Math.max(1e-20, denPow)));
}

function peakingCoeffs(freq, gainDB, q, sampleRate) {
  const a = Math.pow(10, gainDB / 40);
  const w0 = (2 * Math.PI * freq) / sampleRate;
  const alpha = Math.sin(w0) / (2 * q);
  const cosw0 = Math.cos(w0);
  return {
    b0: 1 + alpha * a,
    b1: -2 * cosw0,
    b2: 1 - alpha * a,
    a0: 1 + alpha / a,
    a1: -2 * cosw0,
    a2: 1 - alpha / a,
  };
}

function lowShelfCoeffs(freq, gainDB, sampleRate) {
  const a = Math.pow(10, gainDB / 40);
  const w0 = (2 * Math.PI * freq) / sampleRate;
  const cosw0 = Math.cos(w0);
  const sinw0 = Math.sin(w0);
  const s = 1;
  const alpha = (sinw0 / 2) * Math.sqrt((a + 1 / a) * (1 / s - 1) + 2);
  const twoSqrtAAlpha = 2 * Math.sqrt(a) * alpha;
  return {
    b0: a * ((a + 1) - (a - 1) * cosw0 + twoSqrtAAlpha),
    b1: 2 * a * ((a - 1) - (a + 1) * cosw0),
    b2: a * ((a + 1) - (a - 1) * cosw0 - twoSqrtAAlpha),
    a0: (a + 1) + (a - 1) * cosw0 + twoSqrtAAlpha,
    a1: -2 * ((a - 1) + (a + 1) * cosw0),
    a2: (a + 1) + (a - 1) * cosw0 - twoSqrtAAlpha,
  };
}

function highShelfCoeffs(freq, gainDB, sampleRate) {
  const a = Math.pow(10, gainDB / 40);
  const w0 = (2 * Math.PI * freq) / sampleRate;
  const cosw0 = Math.cos(w0);
  const sinw0 = Math.sin(w0);
  const s = 1;
  const alpha = (sinw0 / 2) * Math.sqrt((a + 1 / a) * (1 / s - 1) + 2);
  const twoSqrtAAlpha = 2 * Math.sqrt(a) * alpha;
  return {
    b0: a * ((a + 1) + (a - 1) * cosw0 + twoSqrtAAlpha),
    b1: -2 * a * ((a - 1) + (a + 1) * cosw0),
    b2: a * ((a + 1) + (a - 1) * cosw0 - twoSqrtAAlpha),
    a0: (a + 1) - (a - 1) * cosw0 + twoSqrtAAlpha,
    a1: 2 * ((a - 1) - (a + 1) * cosw0),
    a2: (a + 1) - (a - 1) * cosw0 - twoSqrtAAlpha,
  };
}

function updateChart() {
  if (!state.chart) return;

  const freqs = state.chartFreqs ?? chartFrequencies();
  let db;

  if (state.eq) {
    const p = new Float32Array(freqs.length);
    const lowMag = new Float32Array(freqs.length);
    const midMag = new Float32Array(freqs.length);
    const highMag = new Float32Array(freqs.length);

    state.eq.low.getFrequencyResponse(freqs, lowMag, p);
    state.eq.mid.getFrequencyResponse(freqs, midMag, p);
    state.eq.high.getFrequencyResponse(freqs, highMag, p);

    db = Array.from(freqs, (_, i) => {
      const mag = lowMag[i] * midMag[i] * highMag[i];
      return 20 * Math.log10(Math.max(1e-6, mag));
    });
  } else {
    const sampleRate = 48000;
    const lowCoeffs = lowShelfCoeffs(100, Number(el.lowGain.value), sampleRate);
    const midCoeffs = peakingCoeffs(1000, Number(el.midGain.value), Number(el.midQ.value), sampleRate);
    const highCoeffs = highShelfCoeffs(6000, Number(el.highGain.value), sampleRate);
    const master = Number(el.master.value);

    db = Array.from(freqs, (freq) => {
      const mag =
        biquadMagnitudeAt(freq, sampleRate, lowCoeffs) *
        biquadMagnitudeAt(freq, sampleRate, midCoeffs) *
        biquadMagnitudeAt(freq, sampleRate, highCoeffs) *
        master;
      return 20 * Math.log10(Math.max(1e-6, mag));
    });
  }

  state.chart.data.datasets[0].data = db;
  state.chart.update();
}

function bindEvents() {
  el.runToggle.addEventListener("click", () => {
    if (!state.audioCtx) setupAudio();
    if (state.isRunning) {
      stopSequencer();
    } else {
      startSequencer();
    }
  });

  [el.tempo, el.decay].forEach((control) => {
    control.addEventListener("input", () => {
      el.tempoValue.textContent = `${Number(el.tempo.value)} BPM`;
      el.decayValue.textContent = `${Number(el.decay.value).toFixed(2)} s`;
    });
  });

  [el.lowGain, el.midGain, el.midQ, el.highGain, el.master].forEach((control) => {
    control.addEventListener("input", () => {
      applyEQControls();
      updateChart();
    });
  });

  el.tempoValue.textContent = `${Number(el.tempo.value)} BPM`;
  el.decayValue.textContent = `${Number(el.decay.value).toFixed(2)} s`;
}

buildStepUI();
initChart();
applyEQControls();
updateChart();
bindEvents();
