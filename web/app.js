const SCALES = {
  pentatonic: [0, 2, 4, 7, 9],
  pentatonicMinor: [0, 3, 5, 7, 10],
  major: [0, 2, 4, 5, 7, 9, 11],
  minor: [0, 2, 3, 5, 7, 8, 10],
  dorian: [0, 2, 3, 5, 7, 9, 10],
  phrygian: [0, 1, 3, 5, 7, 8, 10],
  lydian: [0, 2, 4, 6, 7, 9, 11],
  mixolydian: [0, 2, 4, 5, 7, 9, 10],
  blues: [0, 3, 5, 6, 7, 10],
  hijazkiar: [0, 1, 4, 5, 7, 8, 11],
  chromatic: [0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11],
};

const ROOT_NOTES = [
  "C",
  "C#",
  "D",
  "D#",
  "E",
  "F",
  "F#",
  "G",
  "G#",
  "A",
  "A#",
  "B",
];

function getNoteFreq(noteIndex) {
  // A4 (index 57) = 440Hz.
  return 440 * Math.pow(2, (noteIndex - 57) / 12);
}

function generateNotes(rootName, scaleKey) {
  const rootOffset = ROOT_NOTES.indexOf(rootName);
  const intervals = SCALES[scaleKey] || SCALES.pentatonic;
  const notes = [];

  // Generate 2 octaves starting from octave 3
  for (let octave = 3; octave <= 4; octave++) {
    for (const interval of intervals) {
      const noteIdx = octave * 12 + rootOffset + interval;
      const freq = getNoteFreq(noteIdx);
      const noteName = ROOT_NOTES[(rootOffset + interval) % 12];
      const label = `${noteName}${octave}`;
      notes.push([label, freq]);
    }
  }
  // Add one more root note at the top
  const topIdx = 5 * 12 + rootOffset;
  notes.push([`${rootName}5`, getNoteFreq(topIdx)]);

  return notes;
}

let currentNotes = generateNotes("C", "pentatonic");

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
    hpFamily: "rbj",
    hpType: "highpass",
    hpOrder: 4,
    hpFreq: 40,
    hpGain: 0,
    hpQ: 0.707,
    lowFamily: "rbj",
    lowType: "lowshelf",
    lowOrder: 4,
    lowFreq: 120,
    lowGain: 0,
    lowQ: 0.707,
    midFamily: "rbj",
    midType: "peak",
    midOrder: 4,
    midFreq: 1000,
    midGain: 0,
    midQ: 1.2,
    highFamily: "rbj",
    highType: "highshelf",
    highOrder: 4,
    highFreq: 5000,
    highGain: 0,
    highQ: 0.707,
    lpFamily: "rbj",
    lpType: "lowpass",
    lpOrder: 4,
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
    reverbWet: 0.42,
    reverbDry: 1.0,
    reverbRoomSize: 0.72,
    reverbDamp: 0.45,
    reverbGain: 0.015,
  },
  compParams: {
    enabled: false,
    thresholdDB: -20,
    ratio: 4,
    kneeDB: 6,
    attackMs: 10,
    releaseMs: 100,
    makeupGainDB: 0,
    autoMakeup: true,
  },
  limParams: {
    enabled: true,
    threshold: -0.1,
    release: 100,
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
  scale: document.getElementById("scale"),
  rootNote: document.getElementById("root-note"),
  randomizeSteps: document.getElementById("randomize-steps"),
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
  compEnabled: document.getElementById("comp-enabled"),
  compThresh: document.getElementById("comp-thresh"),
  compThreshValue: document.getElementById("comp-thresh-value"),
  compRatio: document.getElementById("comp-ratio"),
  compRatioValue: document.getElementById("comp-ratio-value"),
  compKnee: document.getElementById("comp-knee"),
  compKneeValue: document.getElementById("comp-knee-value"),
  compAttack: document.getElementById("comp-attack"),
  compAttackValue: document.getElementById("comp-attack-value"),
  compRelease: document.getElementById("comp-release"),
  compReleaseValue: document.getElementById("comp-release-value"),
  compAuto: document.getElementById("comp-auto"),
  compMakeup: document.getElementById("comp-makeup"),
  compMakeupValue: document.getElementById("comp-makeup-value"),
  limEnabled: document.getElementById("lim-enabled"),
  limThresh: document.getElementById("lim-thresh"),
  limThreshValue: document.getElementById("lim-thresh-value"),
  limRelease: document.getElementById("lim-release"),
  limReleaseValue: document.getElementById("lim-release-value"),
  compGraph: document.getElementById("comp-graph"),
  limGraph: document.getElementById("lim-graph"),
  analyzerFFT: document.getElementById("analyzer-fft"),
  analyzerOverlap: document.getElementById("analyzer-overlap"),
  analyzerOverlapValue: document.getElementById("analyzer-overlap-value"),
  analyzerWindow: document.getElementById("analyzer-window"),
  analyzerSmoothing: document.getElementById("analyzer-smoothing"),
  analyzerSmoothingValue: document.getElementById("analyzer-smoothing-value"),
  themeToggle: document.getElementById("theme-toggle"),
};

const THEME_STORAGE_KEY = "algo-dsp-theme";
const THEME_MODES = ["system", "light", "dark"];

function getThemeIconMarkup(mode, resolvedMode = mode) {
  const effectiveMode = mode === "system" ? resolvedMode : mode;
  if (effectiveMode === "light") {
    return `
      <circle cx="12" cy="12" r="5"></circle>
      <line x1="12" y1="1" x2="12" y2="3"></line>
      <line x1="12" y1="21" x2="12" y2="23"></line>
      <line x1="4.22" y1="4.22" x2="5.64" y2="5.64"></line>
      <line x1="18.36" y1="18.36" x2="19.78" y2="19.78"></line>
      <line x1="1" y1="12" x2="3" y2="12"></line>
      <line x1="21" y1="12" x2="23" y2="12"></line>
      <line x1="4.22" y1="19.78" x2="5.64" y2="18.36"></line>
      <line x1="18.36" y1="5.64" x2="19.78" y2="4.22"></line>
    `;
  }
  if (effectiveMode === "dark") {
    return `<path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z"></path>`;
  }
  return `
    <rect x="3" y="4" width="18" height="12" rx="2"></rect>
    <line x1="8" y1="20" x2="16" y2="20"></line>
    <line x1="12" y1="16" x2="12" y2="20"></line>
  `;
}

function updateThemeToggleButton(mode) {
  if (!el.themeToggle) return;
  const icon = el.themeToggle.querySelector(".theme-toggle-icon");
  const label = el.themeToggle.querySelector(".theme-toggle-label");
  const labels = { system: "Auto", light: "Light", dark: "Dark" };
  const text = labels[mode] || labels.system;
  const resolved = document.documentElement.dataset.resolvedTheme || "light";
  if (icon) icon.innerHTML = getThemeIconMarkup(mode, resolved);
  if (label) label.textContent = text;
  el.themeToggle.setAttribute("aria-label", `Theme: ${text}. Click to cycle.`);
  el.themeToggle.title = `Theme: ${text} (resolved ${resolved})`;
}

function resolveTheme(theme, mql) {
  return theme === "system" ? (mql.matches ? "dark" : "light") : theme;
}

function applyTheme(theme, mql) {
  const selected =
    theme === "light" || theme === "dark" || theme === "system"
      ? theme
      : "system";
  const resolved = resolveTheme(selected, mql);
  const root = document.documentElement;
  root.dataset.theme = selected;
  root.dataset.resolvedTheme = resolved;
}

function initTheme() {
  if (!el.themeToggle) return;
  const mql = window.matchMedia("(prefers-color-scheme: dark)");
  let stored = null;
  try {
    stored = localStorage.getItem(THEME_STORAGE_KEY);
  } catch {
    stored = null;
  }
  let currentTheme = THEME_MODES.includes(stored) ? stored : "system";
  applyTheme(currentTheme, mql);
  updateThemeToggleButton(currentTheme);

  el.themeToggle.addEventListener("click", () => {
    const currentIdx = THEME_MODES.indexOf(currentTheme);
    currentTheme = THEME_MODES[(currentIdx + 1) % THEME_MODES.length];
    applyTheme(currentTheme, mql);
    updateThemeToggleButton(currentTheme);
    try {
      localStorage.setItem(THEME_STORAGE_KEY, currentTheme);
    } catch {
      // Ignore storage failures (private mode / disabled storage).
    }
    state.eqUI?.draw();
  });

  mql.addEventListener("change", () => {
    if (currentTheme !== "system") return;
    applyTheme("system", mql);
    updateThemeToggleButton("system");
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
    currentNotes.forEach(([label, freq], idx) => {
      const opt = document.createElement("option");
      opt.value = String(freq);
      opt.textContent = label;
      if (idx === (i % currentNotes.length)) opt.selected = true;
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
      if (typeof initErr === "string" && initErr.length > 0)
        throw new Error(initErr);
      state.dsp.sampleRate = sampleRate;
      syncTransportToDSP();
      syncWaveformToDSP();
      syncStepsToDSP();
      syncEQToDSP();
      syncEffectsToDSP();
      syncCompressorToDSP();
      syncLimiterToDSP();
      syncSpectrumToDSP();
      state.eqUI?.draw();
    }
    return;
  }
  if (typeof Go === "undefined")
    throw new Error("wasm_exec.js missing. Build wasm assets first.");

  const go = new Go();
  let result;
  try {
    result = await WebAssembly.instantiateStreaming(
      fetch("algo_dsp_demo.wasm"),
      go.importObject,
    );
  } catch {
    const response = await fetch("algo_dsp_demo.wasm");
    const bytes = await response.arrayBuffer();
    result = await WebAssembly.instantiate(bytes, go.importObject);
  }

  go.run(result.instance);

  const api = window.AlgoDSPDemo;
  if (!api) throw new Error("AlgoDSPDemo API not found after wasm init");

  const initErr = api.init(sampleRate);
  if (typeof initErr === "string" && initErr.length > 0)
    throw new Error(initErr);

  state.dsp.ready = true;
  state.dsp.api = api;
  state.dsp.go = go;
  state.dsp.sampleRate = sampleRate;

  syncTransportToDSP();
  syncWaveformToDSP();
  syncStepsToDSP();
  syncEQToDSP();
  syncEffectsToDSP();
  syncCompressorToDSP();
  syncLimiterToDSP();
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
    el.eqReadout.textContent =
      "Hover a node for details. Mouse wheel adjusts Q/shape. Right-click a node to change filter type.";
    return;
  }

  const family = typeof h.family === "string" ? h.family.toUpperCase() : "RBJ";
  const orderPart = Number(h.order) > 1 ? `, Order ${Number(h.order)}` : "";
  const isChebyshev = h.family === "chebyshev1" || h.family === "chebyshev2";
  const usesRipple =
    isChebyshev &&
    (h.type === "highpass" ||
      h.type === "lowpass" ||
      h.type === "highshelf" ||
      h.type === "lowshelf");
  const shapeLabel = usesRipple ? `Ripple ${h.q.toFixed(2)} dB` : `Q ${h.q.toFixed(2)}`;
  el.eqReadout.textContent = `${h.label} [${family}${orderPart}]: ${Math.round(h.freq)} Hz, ${h.gain.toFixed(1)} dB, ${shapeLabel}`;
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
  state.dsp.api.setTransport(
    Number(el.tempo.value),
    Number(el.decay.value),
    Number(el.shuffle.value),
  );
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
  if (typeof err === "string" && err.length > 0)
    console.error("setEQ failed", err);
}

function syncEffectsToDSP() {
  if (!state.dsp.ready || !state.dsp.api) return;
  const err = state.dsp.api.setEffects(state.effectsParams);
  if (typeof err === "string" && err.length > 0)
    console.error("setEffects failed", err);
}

function syncSpectrumToDSP() {
  if (!state.dsp.ready || !state.dsp.api) return;
  const err = state.dsp.api.setSpectrum(state.analyzerParams);
  if (typeof err === "string" && err.length > 0)
    console.error("setSpectrum failed", err);
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
  el.analyzerSmoothingValue.textContent = Number(
    el.analyzerSmoothing.value,
  ).toFixed(2);
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

function syncCompressorToDSP() {
  if (!state.dsp.ready || !state.dsp.api) return;
  const err = state.dsp.api.setCompressor(state.compParams);
  if (typeof err === "string" && err.length > 0)
    console.error("setCompressor failed", err);
}

function readCompressorFromUI() {
  state.compParams = {
    enabled: el.compEnabled.checked,
    thresholdDB: Number(el.compThresh.value),
    ratio: Number(el.compRatio.value),
    kneeDB: Number(el.compKnee.value),
    attackMs: Number(el.compAttack.value),
    releaseMs: Number(el.compRelease.value),
    autoMakeup: el.compAuto.checked,
    makeupGainDB: Number(el.compMakeup.value),
  };
}

function updateCompressorText() {
  el.compThreshValue.textContent = `${Number(el.compThresh.value).toFixed(1)} dB`;
  el.compRatioValue.textContent = `${Number(el.compRatio.value).toFixed(1)}:1`;
  el.compKneeValue.textContent = `${Number(el.compKnee.value).toFixed(1)} dB`;
  el.compAttackValue.textContent = `${Number(el.compAttack.value).toFixed(1)} ms`;
  el.compReleaseValue.textContent = `${Number(el.compRelease.value).toFixed(0)} ms`;
  el.compMakeupValue.textContent = `${Number(el.compMakeup.value).toFixed(1)} dB`;

  if (el.compAuto.checked) {
    el.compMakeup.disabled = true;
    el.compMakeupValue.style.opacity = "0.5";
  } else {
    el.compMakeup.disabled = false;
    el.compMakeupValue.style.opacity = "1";
  }

  drawDynamicsCurve(el.compGraph, "compressor");
}

function syncLimiterToDSP() {
  if (!state.dsp.ready || !state.dsp.api) return;
  const err = state.dsp.api.setLimiter(state.limParams);
  if (typeof err === "string" && err.length > 0)
    console.error("setLimiter failed", err);
}

function readLimiterFromUI() {
  state.limParams = {
    enabled: el.limEnabled.checked,
    threshold: Number(el.limThresh.value),
    release: Number(el.limRelease.value),
  };
}

function updateLimiterText() {
  el.limThreshValue.textContent = `${Number(el.limThresh.value).toFixed(1)} dB`;
  el.limReleaseValue.textContent = `${Number(el.limRelease.value).toFixed(0)} ms`;

  drawDynamicsCurve(el.limGraph, "limiter");
}

function drawDynamicsCurve(canvas, type) {
  if (!canvas) return;
  const ctx = canvas.getContext("2d");
  const w = canvas.width;
  const h = canvas.height;
  const pad = 20;
  const range = 60; // -60dB to 0dB

  ctx.clearRect(0, 0, w, h);

  // Colors from CSS vars or defaults
  const gridColor = getComputedStyle(document.documentElement).getPropertyValue("--line").trim() || "#d9ccb6";
  const accentColor = getComputedStyle(document.documentElement).getPropertyValue("--accent").trim() || "#c24d2c";
  const textColor = getComputedStyle(document.documentElement).getPropertyValue("--ink").trim() || "#1d1b18";

  // Grid
  ctx.strokeStyle = gridColor;
  ctx.lineWidth = 1;
  ctx.setLineDash([2, 2]);
  for (let db = -range; db <= 0; db += 20) {
    const x = pad + ((db + range) / range) * (w - 2 * pad);
    const y = pad + (1 - (db + range) / range) * (h - 2 * pad);

    ctx.beginPath();
    ctx.moveTo(x, pad);
    ctx.lineTo(x, h - pad);
    ctx.stroke();

    ctx.beginPath();
    ctx.moveTo(pad, y);
    ctx.lineTo(w - pad, y);
    ctx.stroke();
  }
  ctx.setLineDash([]);

  // Diagonal (1:1)
  ctx.strokeStyle = gridColor;
  ctx.globalAlpha = 0.5;
  ctx.beginPath();
  ctx.moveTo(pad, h - pad);
  ctx.lineTo(w - pad, pad);
  ctx.stroke();
  ctx.globalAlpha = 1.0;

  // Transfer function
  ctx.strokeStyle = accentColor;
  ctx.lineWidth = 2;
  ctx.beginPath();

  const getOutDB = (inDB) => {
    if (type === "compressor") {
      const t = state.compParams.thresholdDB;
      const r = state.compParams.ratio;
      const k = state.compParams.kneeDB;
      let outDB = inDB;

      if (k > 0) {
        if (inDB > t - k / 2 && inDB < t + k / 2) {
          // Soft knee quadratic interpolation
          outDB = inDB + ((1 / r - 1) * Math.pow(inDB - t + k / 2, 2)) / (2 * k);
        } else if (inDB >= t + k / 2) {
          outDB = t + (inDB - t) / r;
        }
      } else {
        if (inDB > t) {
          outDB = t + (inDB - t) / r;
        }
      }

      // Makeup gain (approximate auto makeup)
      if (state.compParams.autoMakeup) {
        const makeup = -(t + (0 - t) / r) / 2; // Rough estimate
        outDB += makeup;
      } else {
        outDB += state.compParams.makeupGainDB;
      }
      return outDB;
    } else {
      // Limiter
      const t = state.limParams.threshold;
      return Math.min(inDB, t);
    }
  };

  for (let i = 0; i <= 100; i++) {
    const inDB = -range + (i / 100) * range;
    const outDB = getOutDB(inDB);

    const x = pad + ((inDB + range) / range) * (w - 2 * pad);
    const y = h - (pad + ((outDB + range) / range) * (h - 2 * pad));

    if (i === 0) ctx.moveTo(x, y);
    else ctx.lineTo(x, y);
  }
  ctx.stroke();

  // Labels
  ctx.fillStyle = textColor;
  ctx.font = "9px sans-serif";
  ctx.textAlign = "center";
  ctx.fillText("In [dB]", w / 2, h - 5);
  ctx.save();
  ctx.translate(5, h / 2);
  ctx.rotate(-Math.PI / 2);
  ctx.fillText("Out [dB]", 0, 0);
  ctx.restore();
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
    getNodeResponseDB: (key, freqs) => {
      if (!state.dsp.ready || !state.dsp.api) return null;
      return state.dsp.api.nodeResponseCurve(key, freqs);
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
    if (state.eqUI && now - state.eqLastDrawTimeMS >= targetFrameMS) {
      state.eqUI.draw();
      state.eqLastDrawTimeMS = now;
    }
    state.eqDrawLoopHandle = requestAnimationFrame(tick);
  };

  state.eqDrawLoopHandle = requestAnimationFrame(tick);
}

function updateStepOptions() {
  currentNotes = generateNotes(el.rootNote.value, el.scale.value);
  state.steps.forEach((step, i) => {
    const prevIndex = step.noteSelect.selectedIndex;
    step.noteSelect.innerHTML = "";
    currentNotes.forEach(([label, freq]) => {
      const opt = document.createElement("option");
      opt.value = String(freq);
      opt.textContent = label;
      step.noteSelect.appendChild(opt);
    });
    if (prevIndex >= 0 && prevIndex < currentNotes.length) {
      step.noteSelect.selectedIndex = prevIndex;
    } else {
      step.noteSelect.selectedIndex = i % currentNotes.length;
    }
  });
  syncStepsToDSP();
}

function randomizeSteps() {
  const intervals = SCALES[el.scale.value] || SCALES.pentatonic;
  const hasFifth = intervals.includes(7);
  
  // Find indices in currentNotes for root and fifth (using octave 3 as base)
  const rootIndex = 0; // First note in currentNotes is root octave 3
  let fifthIndex = -1;
  if (hasFifth) {
    fifthIndex = intervals.indexOf(7);
  }

  state.steps.forEach((step, i) => {
    // 1-indexed steps: 1, 5, 9, 13
    // 0-indexed: 0, 4, 8, 12
    if (i === 0 || i === 8) {
      step.enabled.checked = true;
      step.noteSelect.selectedIndex = rootIndex;
    } else if ((i === 4 || i === 12) && hasFifth) {
      step.enabled.checked = true;
      step.noteSelect.selectedIndex = fifthIndex;
    } else {
      // Randomize other steps
      step.enabled.checked = Math.random() > 0.6; // ~40% chance to be enabled
      step.noteSelect.selectedIndex = Math.floor(Math.random() * currentNotes.length);
    }
  });
  syncStepsToDSP();
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

  el.scale.addEventListener("change", updateStepOptions);
  el.rootNote.addEventListener("change", updateStepOptions);
  el.randomizeSteps.addEventListener("click", randomizeSteps);

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

  [
    el.compEnabled,
    el.compThresh,
    el.compRatio,
    el.compKnee,
    el.compAttack,
    el.compRelease,
    el.compAuto,
    el.compMakeup,
  ].forEach((control) => {
    const eventName = control.type === "checkbox" ? "change" : "input";
    control.addEventListener(eventName, () => {
      readCompressorFromUI();
      updateCompressorText();
      syncCompressorToDSP();
    });
  });

  [el.limEnabled, el.limThresh, el.limRelease].forEach((control) => {
    const eventName = control.type === "checkbox" ? "change" : "input";
    control.addEventListener(eventName, () => {
      readLimiterFromUI();
      updateLimiterText();
      syncLimiterToDSP();
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
  updateCompressorText();
  readCompressorFromUI();
  updateLimiterText();
  readLimiterFromUI();
  el.analyzerFFT.value = String(state.analyzerParams.fftSize);
  el.analyzerOverlap.value = String(
    Math.round(state.analyzerParams.overlap * 100),
  );
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
