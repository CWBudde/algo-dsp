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
    master: 0.75,
  },
  dsp: {
    ready: false,
    api: null,
    go: null,
    sampleRate: 0,
  },
  waveform: "sine",
};

const el = {
  runToggle: document.getElementById("run-toggle"),
  waveform: document.getElementById("waveform"),
  tempo: document.getElementById("tempo"),
  tempoValue: document.getElementById("tempo-value"),
  decay: document.getElementById("decay"),
  decayValue: document.getElementById("decay-value"),
  steps: document.getElementById("steps"),
  eqCanvas: document.getElementById("eq-canvas"),
  eqReadout: document.getElementById("eq-readout"),
  master: document.getElementById("master"),
  masterValue: document.getElementById("master-value"),
};

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
  el.masterValue.textContent = state.eqParams.master.toFixed(2);

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

function stepDurationSeconds() {
  return 60 / Number(el.tempo.value) / 4;
}

function schedule() {
  const lookahead = 0.1;
  while (state.nextNoteTime < state.audioCtx.currentTime + lookahead) {
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

function syncTransportToDSP() {
  if (!state.dsp.ready || !state.dsp.api) return;
  state.dsp.api.setTransport(Number(el.tempo.value), Number(el.decay.value));
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
  });
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

  [el.tempo, el.decay].forEach((control) => {
    control.addEventListener("input", () => {
      el.tempoValue.textContent = `${Number(el.tempo.value)} BPM`;
      el.decayValue.textContent = `${Number(el.decay.value).toFixed(2)} s`;
      syncTransportToDSP();
    });
  });

  el.waveform.addEventListener("change", () => {
    syncWaveformToDSP();
  });

  el.master.addEventListener("input", () => {
    state.eqUI.setParams({ master: Number(el.master.value) });
  });

  el.tempoValue.textContent = `${Number(el.tempo.value)} BPM`;
  el.decayValue.textContent = `${Number(el.decay.value).toFixed(2)} s`;
  el.waveform.value = state.waveform;
  updateEQText();
}

buildStepUI();
initEQCanvas();
bindEvents();
ensureDSP(48000)
  .then(() => state.eqUI?.draw())
  .catch((err) => console.error(err));
