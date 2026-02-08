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
  outputNode: null,
  isRunning: false,
  currentStep: 0,
  nextNoteTime: 0,
  scheduler: null,
  steps: [],
  eqUI: null,
  hoverInfo: null,
  eqParams: {
    hpFreq: 40,
    hpGain: 0,
    hpQ: 0.707,
    lowFreq: 120,
    lowGain: 0,
    lowQ: 0.707,
    midFreq: 1000,
    midGain: 0,
    midQ: 1.2,
    highFreq: 5000,
    highGain: 0,
    highQ: 0.707,
    lpFreq: 12000,
    lpGain: 0,
    lpQ: 0.707,
    master: 1,
  },
  effectsParams: {
    chorusEnabled: false,
    chorusMix: 0.18,
    chorusDepth: 0.003,
    chorusSpeedHz: 0.35,
    chorusStages: 3,
    reverbEnabled: false,
    reverbWet: 0.22,
    reverbDry: 1.0,
    reverbRoomSize: 0.72,
    reverbDamp: 0.45,
    reverbGain: 0.015,
  },
  analyzerParams: {
    fftSize: 2048,
    overlap: 0.75,
    window: "blackmanharris",
    smoothing: 0.65,
  },
  dsp: {
    ready: false,
    api: null,
    go: null,
    sampleRate: 0,
  },
  waveform: "sine",
  eqDrawLoopHandle: null,
  eqLastDrawTimeMS: 0,
};

const el = {
  runToggle: document.getElementById("run-toggle"),
  waveform: document.getElementById("waveform"),
  tempo: document.getElementById("tempo"),
  tempoValue: document.getElementById("tempo-value"),
  decay: document.getElementById("decay"),
  decayValue: document.getElementById("decay-value"),
  shuffle: document.getElementById("shuffle"),
  shuffleValue: document.getElementById("shuffle-value"),
  steps: document.getElementById("steps"),
  eqCanvas: document.getElementById("eq-canvas"),
  eqReadout: document.getElementById("eq-readout"),
  chorusEnabled: document.getElementById("chorus-enabled"),
  chorusMix: document.getElementById("chorus-mix"),
  chorusMixValue: document.getElementById("chorus-mix-value"),
  chorusDepth: document.getElementById("chorus-depth"),
  chorusDepthValue: document.getElementById("chorus-depth-value"),
  chorusSpeed: document.getElementById("chorus-speed"),
  chorusSpeedValue: document.getElementById("chorus-speed-value"),
  chorusStages: document.getElementById("chorus-stages"),
  chorusStagesValue: document.getElementById("chorus-stages-value"),
  reverbEnabled: document.getElementById("reverb-enabled"),
  reverbWet: document.getElementById("reverb-wet"),
  reverbWetValue: document.getElementById("reverb-wet-value"),
  reverbDry: document.getElementById("reverb-dry"),
  reverbDryValue: document.getElementById("reverb-dry-value"),
  reverbRoom: document.getElementById("reverb-room"),
  reverbRoomValue: document.getElementById("reverb-room-value"),
  reverbDamp: document.getElementById("reverb-damp"),
  reverbDampValue: document.getElementById("reverb-damp-value"),
  analyzerFFT: document.getElementById("analyzer-fft"),
  analyzerOverlap: document.getElementById("analyzer-overlap"),
  analyzerOverlapValue: document.getElementById("analyzer-overlap-value"),
  analyzerWindow: document.getElementById("analyzer-window"),
  analyzerSmoothing: document.getElementById("analyzer-smoothing"),
  analyzerSmoothingValue: document.getElementById("analyzer-smoothing-value"),
  theme: document.getElementById("theme"),
};

const THEME_STORAGE_KEY = "algo-dsp-theme";

function resolveTheme(theme, mql) {
  return theme === "system" ? (mql.matches ? "dark" : "light") : theme;
}

function applyTheme(theme, mql) {
  const selected = theme === "light" || theme === "dark" || theme === "system" ? theme : "system";
  const resolved = resolveTheme(selected, mql);
  const root = document.documentElement;
  root.dataset.theme = selected;
  root.dataset.resolvedTheme = resolved;
}

function initTheme() {
  if (!el.theme) return;
  const mql = window.matchMedia("(prefers-color-scheme: dark)");
  let stored = null;
  try {
    stored = localStorage.getItem(THEME_STORAGE_KEY);
  } catch {
    stored = null;
  }
  const selected = stored === "light" || stored === "dark" || stored === "system" ? stored : "system";
  el.theme.value = selected;
  applyTheme(selected, mql);

  el.theme.addEventListener("change", () => {
    const next = el.theme.value;
    applyTheme(next, mql);
    try {
      localStorage.setItem(THEME_STORAGE_KEY, next);
    } catch {
      // Ignore storage failures (private mode / disabled storage).
    }
    state.eqUI?.draw();
  });

  mql.addEventListener("change", () => {
    if (el.theme.value !== "system") return;
    applyTheme("system", mql);
    state.eqUI?.draw();
  });
}

function buildStepUI() {
  for (let i = 0; i < STEP_COUNT; i += 1) {
    const step = document.createElement("div");
    step.className = "step";

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

    const stateStep = { enabled, noteSelect, node: step };
    state.steps.push(stateStep);

    enabled.addEventListener("change", syncStepsToDSP);
    noteSelect.addEventListener("change", syncStepsToDSP);
  }
}

async function ensureDSP(sampleRate) {
  if (state.dsp.ready) {
    if (Math.abs(state.dsp.sampleRate - sampleRate) > 1) {
      const initErr = state.dsp.api.init(sampleRate);
      if (typeof initErr === "string" && initErr.length > 0) throw new Error(initErr);
      state.dsp.sampleRate = sampleRate;
      syncTransportToDSP();
      syncWaveformToDSP();
      syncStepsToDSP();
      syncEQToDSP();
      syncEffectsToDSP();
      syncSpectrumToDSP();
      state.eqUI?.draw();
    }
    return;
  }
  if (typeof Go === "undefined") throw new Error("wasm_exec.js missing. Build wasm assets first.");

  const go = new Go();
  let result;
  try {
    result = await WebAssembly.instantiateStreaming(fetch("algo_dsp_demo.wasm"), go.importObject);
  } catch {
    const response = await fetch("algo_dsp_demo.wasm");
    const bytes = await response.arrayBuffer();
    result = await WebAssembly.instantiate(bytes, go.importObject);
  }

  go.run(result.instance);

  const api = window.AlgoDSPDemo;
  if (!api) throw new Error("AlgoDSPDemo API not found after wasm init");

  const initErr = api.init(sampleRate);
  if (typeof initErr === "string" && initErr.length > 0) throw new Error(initErr);

  state.dsp.ready = true;
  state.dsp.api = api;
  state.dsp.go = go;
  state.dsp.sampleRate = sampleRate;

  syncTransportToDSP();
  syncWaveformToDSP();
  syncStepsToDSP();
  syncEQToDSP();
  syncEffectsToDSP();
  syncSpectrumToDSP();
}

async function setupAudio() {
  if (state.audioCtx) return;

  const ctx = new AudioContext();
  await ensureDSP(ctx.sampleRate);

  const node = ctx.createScriptProcessor(1024, 0, 1);
  node.onaudioprocess = (event) => {
    const out = event.outputBuffer.getChannelData(0);
    if (!state.dsp.ready || !state.dsp.api) {
      out.fill(0);
      return;
    }

    const chunk = state.dsp.api.render(out.length);
    out.set(chunk);
  };

  node.connect(ctx.destination);

  state.audioCtx = ctx;
  state.outputNode = node;
  state.eqUI?.draw();
}

function updateEQText() {
  const h = state.hoverInfo;
  if (!h) {
    el.eqReadout.textContent = "Hover a node for details. Mouse wheel adjusts that node Q.";
    return;
  }

  if (h.key === "hp" || h.key === "lp") {
    el.eqReadout.textContent = `${h.label}: ${Math.round(h.freq)} Hz, ${h.gain.toFixed(1)} dB, Q ${h.q.toFixed(2)}`;
    return;
  }

  el.eqReadout.textContent = `${h.label}: ${Math.round(h.freq)} Hz, ${h.gain.toFixed(1)} dB, Q ${h.q.toFixed(2)}`;
}

function stepDurationSeconds(stepIndex) {
  const base = 60 / Number(el.tempo.value) / 4;
  const ratio = shuffleRatio(Number(el.shuffle.value));
  if (ratio <= 0) return base;
  return stepIndex % 2 === 0 ? base * (1 + ratio) : base * (1 - ratio);
}

function shuffleRatio(shuffleValue) {
  const shuffle = Math.max(0, Math.min(1, shuffleValue));
  // Map 0..1 control to 0..1/3 timing ratio with a gentle curve.
  return (1 / 3) * Math.pow(shuffle, 1.6);
}

function schedule() {
  const lookahead = 0.1;
  while (state.nextNoteTime < state.audioCtx.currentTime + lookahead) {
    const stepIndex = state.currentStep;
    highlightStep(stepIndex);
    state.nextNoteTime += stepDurationSeconds(stepIndex);
    state.currentStep = (stepIndex + 1) % STEP_COUNT;
  }
}

function highlightStep(index) {
  state.steps.forEach((s, i) => {
    s.node.classList.toggle("current", i === index);
  });
}

function syncTransportToDSP() {
  if (!state.dsp.ready || !state.dsp.api) return;
  state.dsp.api.setTransport(Number(el.tempo.value), Number(el.decay.value), Number(el.shuffle.value));
}

function syncWaveformToDSP() {
  if (!state.dsp.ready || !state.dsp.api) return;
  const waveform = String(el.waveform.value || "sine");
  state.waveform = waveform;
  state.dsp.api.setWaveform(waveform);
}

function syncStepsToDSP() {
  if (!state.dsp.ready || !state.dsp.api) return;
  const steps = state.steps.map((step) => ({
    enabled: step.enabled.checked,
    freq: Number(step.noteSelect.value),
  }));
  state.dsp.api.setSteps(steps);
}

function syncEQToDSP() {
  if (!state.dsp.ready || !state.dsp.api) return;
  const err = state.dsp.api.setEQ(state.eqParams);
  if (typeof err === "string" && err.length > 0) console.error("setEQ failed", err);
}

function syncEffectsToDSP() {
  if (!state.dsp.ready || !state.dsp.api) return;
  const err = state.dsp.api.setEffects(state.effectsParams);
  if (typeof err === "string" && err.length > 0) console.error("setEffects failed", err);
}

function syncSpectrumToDSP() {
  if (!state.dsp.ready || !state.dsp.api) return;
  const err = state.dsp.api.setSpectrum(state.analyzerParams);
  if (typeof err === "string" && err.length > 0) console.error("setSpectrum failed", err);
}

function readSpectrumFromUI() {
  state.analyzerParams = {
    fftSize: Number(el.analyzerFFT.value),
    overlap: Number(el.analyzerOverlap.value) / 100,
    window: String(el.analyzerWindow.value),
    smoothing: Number(el.analyzerSmoothing.value),
  };
}

function updateSpectrumText() {
  const overlapPct = Math.round(Number(el.analyzerOverlap.value));
  const hopPct = Math.max(1, 100 - overlapPct);
  el.analyzerOverlapValue.textContent = `${overlapPct}% overlap (${hopPct}% hop)`;
  el.analyzerSmoothingValue.textContent = Number(el.analyzerSmoothing.value).toFixed(2);
}

function readEffectsFromUI() {
  state.effectsParams = {
    chorusEnabled: el.chorusEnabled.checked,
    chorusMix: Number(el.chorusMix.value),
    chorusDepth: Number(el.chorusDepth.value),
    chorusSpeedHz: Number(el.chorusSpeed.value),
    chorusStages: Number(el.chorusStages.value),
    reverbEnabled: el.reverbEnabled.checked,
    reverbWet: Number(el.reverbWet.value),
    reverbDry: Number(el.reverbDry.value),
    reverbRoomSize: Number(el.reverbRoom.value),
    reverbDamp: Number(el.reverbDamp.value),
    reverbGain: state.effectsParams.reverbGain,
  };
}

function updateEffectsText() {
  el.chorusMixValue.textContent = `${Math.round(Number(el.chorusMix.value) * 100)}%`;
  el.chorusDepthValue.textContent = `${(Number(el.chorusDepth.value) * 1000).toFixed(1)} ms`;
  el.chorusSpeedValue.textContent = `${Number(el.chorusSpeed.value).toFixed(2)} Hz`;
  el.chorusStagesValue.textContent = `${Number(el.chorusStages.value)}`;
  el.reverbWetValue.textContent = `${Math.round(Number(el.reverbWet.value) * 100)}%`;
  el.reverbDryValue.textContent = Number(el.reverbDry.value).toFixed(2);
  el.reverbRoomValue.textContent = Number(el.reverbRoom.value).toFixed(2);
  el.reverbDampValue.textContent = Number(el.reverbDamp.value).toFixed(2);
}

function startSequencer() {
  if (!state.audioCtx) return;
  if (state.audioCtx.state === "suspended") state.audioCtx.resume();
  if (state.isRunning) return;

  state.isRunning = true;
  state.currentStep = 0;
  state.nextNoteTime = state.audioCtx.currentTime + 0.05;
  state.scheduler = setInterval(schedule, 25);
  if (state.dsp.ready && state.dsp.api) state.dsp.api.setRunning(true);
  const sr = el.runToggle.querySelector(".sr-only");
  if (sr) sr.textContent = "Stop";
  el.runToggle.setAttribute("aria-label", "Stop");
  el.runToggle.classList.add("active");
}

function stopSequencer() {
  if (!state.isRunning) return;
  clearInterval(state.scheduler);
  state.scheduler = null;
  state.isRunning = false;
  if (state.dsp.ready && state.dsp.api) state.dsp.api.setRunning(false);
  const sr = el.runToggle.querySelector(".sr-only");
  if (sr) sr.textContent = "Play";
  el.runToggle.setAttribute("aria-label", "Play");
  el.runToggle.classList.remove("active");
  highlightStep(-1);
}

function initEQCanvas() {
  state.eqUI = new window.EQCanvas(el.eqCanvas, {
    initialParams: state.eqParams,
    onChange: (params) => {
      state.eqParams = { ...params };
      syncEQToDSP();
      updateEQText();
    },
    onHover: (info) => {
      state.hoverInfo = info;
      updateEQText();
    },
    getSampleRate: () => state.audioCtx?.sampleRate ?? 48000,
    getResponseDB: (freqs) => {
      if (!state.dsp.ready || !state.dsp.api) return null;
      return state.dsp.api.responseCurve(freqs);
    },
    getSpectrumDB: (freqs) => {
      if (!state.dsp.ready || !state.dsp.api) return null;
      return state.dsp.api.spectrumCurve(freqs);
    },
  });
}

function startEQDrawLoop() {
  if (state.eqDrawLoopHandle !== null) return;
  const targetFrameMS = 1000 / 24;

  const tick = (now) => {
    if (state.eqUI && now-state.eqLastDrawTimeMS >= targetFrameMS) {
      state.eqUI.draw();
      state.eqLastDrawTimeMS = now;
    }
    state.eqDrawLoopHandle = requestAnimationFrame(tick);
  };

  state.eqDrawLoopHandle = requestAnimationFrame(tick);
}

function bindEvents() {
  el.runToggle.addEventListener("click", async () => {
    if (!state.audioCtx) {
      try {
        await setupAudio();
      } catch (err) {
        console.error(err);
        return;
      }
    }
    if (state.isRunning) stopSequencer();
    else startSequencer();
  });

  [el.tempo, el.decay, el.shuffle].forEach((control) => {
    control.addEventListener("input", () => {
      el.tempoValue.textContent = `${Number(el.tempo.value)} BPM`;
      el.decayValue.textContent = `${Number(el.decay.value).toFixed(2)} s`;
      el.shuffleValue.textContent = `${Math.round(Number(el.shuffle.value) * 100)}%`;
      syncTransportToDSP();
    });
  });

  el.waveform.addEventListener("change", () => {
    syncWaveformToDSP();
  });

  [
    el.chorusEnabled,
    el.chorusMix,
    el.chorusDepth,
    el.chorusSpeed,
    el.chorusStages,
    el.reverbEnabled,
    el.reverbWet,
    el.reverbDry,
    el.reverbRoom,
    el.reverbDamp,
  ].forEach((control) => {
    const eventName = control.type === "checkbox" ? "change" : "input";
    control.addEventListener(eventName, () => {
      readEffectsFromUI();
      updateEffectsText();
      syncEffectsToDSP();
    });
  });

  [el.analyzerFFT, el.analyzerWindow].forEach((control) => {
    control.addEventListener("change", () => {
      readSpectrumFromUI();
      updateSpectrumText();
      syncSpectrumToDSP();
    });
  });

  [el.analyzerOverlap, el.analyzerSmoothing].forEach((control) => {
    control.addEventListener("input", () => {
      readSpectrumFromUI();
      updateSpectrumText();
      syncSpectrumToDSP();
    });
  });

  el.tempoValue.textContent = `${Number(el.tempo.value)} BPM`;
  el.decayValue.textContent = `${Number(el.decay.value).toFixed(2)} s`;
  el.shuffleValue.textContent = `${Math.round(Number(el.shuffle.value) * 100)}%`;
  el.waveform.value = state.waveform;
  updateEffectsText();
  readEffectsFromUI();
  el.analyzerFFT.value = String(state.analyzerParams.fftSize);
  el.analyzerOverlap.value = String(Math.round(state.analyzerParams.overlap * 100));
  el.analyzerWindow.value = state.analyzerParams.window;
  el.analyzerSmoothing.value = String(state.analyzerParams.smoothing);
  readSpectrumFromUI();
  updateSpectrumText();
  updateEQText();
}

buildStepUI();
initEQCanvas();
startEQDrawLoop();
bindEvents();
initTheme();
ensureDSP(48000)
  .then(() => state.eqUI?.draw())
  .catch((err) => console.error(err));
