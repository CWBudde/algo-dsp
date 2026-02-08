(() => {
  const FREQ_MIN = 20;
  const FREQ_MAX = 20000;
  const GAIN_MIN = -18;
  const GAIN_MAX = 18;
  const SPECTRUM_RANGE_DB = 144;
  const SPECTRUM_OFFSET_DB = 96;
  const SPECTRUM_TOP_DBFS = SPECTRUM_RANGE_DB - SPECTRUM_OFFSET_DB;
  const SPECTRUM_FLOOR_DBFS = -SPECTRUM_OFFSET_DB;

  function clamp(v, min, max) {
    return Math.min(max, Math.max(min, v));
  }

  function cssVar(name, fallback) {
    const value = getComputedStyle(document.documentElement).getPropertyValue(name).trim();
    return value || fallback;
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

  function lowpassCoeffs(freq, q, sampleRate) {
    const w0 = (2 * Math.PI * freq) / sampleRate;
    const alpha = Math.sin(w0) / (2 * q);
    const cosw0 = Math.cos(w0);
    return {
      b0: (1 - cosw0) / 2,
      b1: 1 - cosw0,
      b2: (1 - cosw0) / 2,
      a0: 1 + alpha,
      a1: -2 * cosw0,
      a2: 1 - alpha,
    };
  }

  function highpassCoeffs(freq, q, sampleRate) {
    const w0 = (2 * Math.PI * freq) / sampleRate;
    const alpha = Math.sin(w0) / (2 * q);
    const cosw0 = Math.cos(w0);
    return {
      b0: (1 + cosw0) / 2,
      b1: -(1 + cosw0),
      b2: (1 + cosw0) / 2,
      a0: 1 + alpha,
      a1: -2 * cosw0,
      a2: 1 - alpha,
    };
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

  function lowShelfCoeffs(freq, gainDB, q, sampleRate) {
    const a = Math.pow(10, gainDB / 40);
    const w0 = (2 * Math.PI * freq) / sampleRate;
    const cosw0 = Math.cos(w0);
    const sinw0 = Math.sin(w0);
    const alpha = sinw0 / (2 * q);
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

  function highShelfCoeffs(freq, gainDB, q, sampleRate) {
    const a = Math.pow(10, gainDB / 40);
    const w0 = (2 * Math.PI * freq) / sampleRate;
    const cosw0 = Math.cos(w0);
    const sinw0 = Math.sin(w0);
    const alpha = sinw0 / (2 * q);
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

  class EQCanvas {
    constructor(canvas, options = {}) {
      this.canvas = canvas;
      this.ctx = canvas.getContext("2d");
      this.onChange = options.onChange || (() => {});
      this.onHover = options.onHover || (() => {});
      this.getSampleRate = options.getSampleRate || (() => 48000);
      this.getResponseDB = options.getResponseDB || null;
      this.getSpectrumDB = options.getSpectrumDB || null;
      this.params = {
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
        ...(options.initialParams || {}),
      };
      this.nodes = [];
      this.activeNode = null;
      this.hoverNode = null;
      this.cssWidth = 0;
      this.cssHeight = 0;

      this.resize();
      this.bindEvents();
      this.draw();
    }

    setParams(partial, opts = {}) {
      const emit = opts.emit !== false;
      Object.assign(this.params, partial);
      this.constrainOrder();
      if (emit) this.onChange({ ...this.params });
      this.draw();
    }

    constrainOrder() {
      this.params.hpFreq = clamp(this.params.hpFreq, FREQ_MIN, FREQ_MAX);
      this.params.lowFreq = clamp(this.params.lowFreq, FREQ_MIN, FREQ_MAX);
      this.params.midFreq = clamp(this.params.midFreq, FREQ_MIN, FREQ_MAX);
      this.params.highFreq = clamp(this.params.highFreq, FREQ_MIN, FREQ_MAX);
      this.params.lpFreq = clamp(this.params.lpFreq, FREQ_MIN, FREQ_MAX);

      this.params.lowGain = clamp(this.params.lowGain, GAIN_MIN, GAIN_MAX);
      this.params.hpGain = clamp(this.params.hpGain, GAIN_MIN, GAIN_MAX);
      this.params.midGain = clamp(this.params.midGain, GAIN_MIN, GAIN_MAX);
      this.params.highGain = clamp(this.params.highGain, GAIN_MIN, GAIN_MAX);
      this.params.lpGain = clamp(this.params.lpGain, GAIN_MIN, GAIN_MAX);
      this.params.hpQ = clamp(this.params.hpQ, 0.2, 8);
      this.params.lowQ = clamp(this.params.lowQ, 0.2, 8);
      this.params.midQ = clamp(this.params.midQ, 0.2, 8);
      this.params.highQ = clamp(this.params.highQ, 0.2, 8);
      this.params.lpQ = clamp(this.params.lpQ, 0.2, 8);
      this.params.master = clamp(this.params.master, 0, 1);
    }

    bounds() {
      const left = 64;
      const right = this.cssWidth - 60;
      const top = 50;
      const bottom = top + 300;
      return { left, right, top, bottom };
    }

    freqToX(freq) {
      const b = this.bounds();
      const minL = Math.log10(FREQ_MIN);
      const maxL = Math.log10(FREQ_MAX);
      const t = (Math.log10(freq) - minL) / (maxL - minL);
      return b.left + t * (b.right - b.left);
    }

    xToFreq(x) {
      const b = this.bounds();
      const t = clamp((x - b.left) / (b.right - b.left), 0, 1);
      const minL = Math.log10(FREQ_MIN);
      const maxL = Math.log10(FREQ_MAX);
      return Math.pow(10, minL + t * (maxL - minL));
    }

    gainToY(gain) {
      const b = this.bounds();
      const t = (gain - GAIN_MIN) / (GAIN_MAX - GAIN_MIN);
      return b.bottom - t * (b.bottom - b.top);
    }

    yToGain(y) {
      const b = this.bounds();
      const t = clamp((b.bottom - y) / (b.bottom - b.top), 0, 1);
      return GAIN_MIN + t * (GAIN_MAX - GAIN_MIN);
    }

    filterMagnitude(key, freq) {
      const p = this.params;
      const sampleRate = this.getSampleRate();
      if (key === "hp") {
        const hpMag = biquadMagnitudeAt(freq, sampleRate, highpassCoeffs(p.hpFreq, p.hpQ, sampleRate));
        return hpMag * Math.pow(10, p.hpGain / 20);
      }
      if (key === "low") return biquadMagnitudeAt(freq, sampleRate, lowShelfCoeffs(p.lowFreq, p.lowGain, p.lowQ, sampleRate));
      if (key === "mid") return biquadMagnitudeAt(freq, sampleRate, peakingCoeffs(p.midFreq, p.midGain, p.midQ, sampleRate));
      if (key === "high") return biquadMagnitudeAt(freq, sampleRate, highShelfCoeffs(p.highFreq, p.highGain, p.highQ, sampleRate));
      const lpMag = biquadMagnitudeAt(freq, sampleRate, lowpassCoeffs(p.lpFreq, p.lpQ, sampleRate));
      return lpMag * Math.pow(10, p.lpGain / 20);
    }

    eqMagnitude(freq) {
      return (
        this.filterMagnitude("hp", freq) *
        this.filterMagnitude("low", freq) *
        this.filterMagnitude("mid", freq) *
        this.filterMagnitude("high", freq) *
        this.filterMagnitude("lp", freq) *
        this.params.master
      );
    }

    computeResponseDB(freqs) {
      if (this.getResponseDB) {
        const response = this.getResponseDB(Float32Array.from(freqs));
        if (response && typeof response.length === "number" && response.length === freqs.length) {
          return response;
        }
      }
      return freqs.map((freq) => 20 * Math.log10(Math.max(1e-6, this.eqMagnitude(freq))));
    }

    computeSingleFilterDB(key, freqs) {
      return freqs.map((freq) => 20 * Math.log10(Math.max(1e-6, this.filterMagnitude(key, freq))));
    }

    computeSpectrumDB(freqs) {
      if (!this.getSpectrumDB) return null;
      const spectrum = this.getSpectrumDB(Float32Array.from(freqs));
      if (!spectrum || typeof spectrum.length !== "number" || spectrum.length !== freqs.length) return null;
      return spectrum;
    }

    resize() {
      const dpr = window.devicePixelRatio || 1;
      const rect = this.canvas.getBoundingClientRect();
      this.cssWidth = Math.max(300, Math.floor(rect.width));
      this.cssHeight = Math.max(400, Math.floor(rect.height));
      this.canvas.style.width = `${this.cssWidth}px`;
      this.canvas.style.height = `${this.cssHeight}px`;
      this.canvas.width = Math.max(300, Math.floor(this.cssWidth * dpr));
      this.canvas.height = Math.max(400, Math.floor(this.cssHeight * dpr));
      this.ctx.setTransform(1, 0, 0, 1, 0, 0);
      this.ctx.scale(dpr, dpr);
      this.draw();
    }

    drawGrid(ctx, b, w, h) {
      const gridMinor = cssVar("--canvas-grid-minor", "#ece1d2");
      const gridMajor = cssVar("--canvas-grid-major", "#d4c6b2");
      const axis = cssVar("--canvas-axis", "#9b8f7a");
      const label = cssVar("--canvas-label", "#6a5f4f");
      const crisp = (v) => Math.round(v) + 0.5;
      const drawV = (x, y1, y2) => {
        const cx = crisp(x);
        ctx.beginPath();
        ctx.moveTo(cx, crisp(y1));
        ctx.lineTo(cx, crisp(y2));
        ctx.stroke();
      };
      const drawH = (x1, x2, y) => {
        const cy = crisp(y);
        ctx.beginPath();
        ctx.moveTo(crisp(x1), cy);
        ctx.lineTo(crisp(x2), cy);
        ctx.stroke();
      };
      const logSpan = Math.log10(FREQ_MAX) - Math.log10(FREQ_MIN);
      const xAt = (f) => b.left + ((Math.log10(f) - Math.log10(FREQ_MIN)) / logSpan) * (b.right - b.left);

      const majors = [100, 1000, 10000];
      const minors = [];
      [100, 1000, 10000].forEach((base) => {
        for (let m = 2; m <= 9; m += 1) {
          const f = base * m;
          if (f >= FREQ_MIN && f <= FREQ_MAX && !majors.includes(f)) minors.push(f);
        }
      });

      ctx.lineWidth = 1;
      ctx.strokeStyle = gridMinor;
      minors.forEach((f) => drawV(xAt(f), b.top, b.bottom));

      ctx.strokeStyle = gridMajor;
      majors.forEach((f) => drawV(xAt(f), b.top, b.bottom));

      [-18, -12, -6, 0, 6, 12, 18].forEach((g) => {
        const y = b.bottom - ((g - GAIN_MIN) / (GAIN_MAX - GAIN_MIN)) * (b.bottom - b.top);
        ctx.strokeStyle = g === 0 ? gridMajor : gridMinor;
        drawH(b.left, b.right, y);
      });

      ctx.strokeStyle = axis;
      drawV(b.left, b.top, b.bottom);
      drawH(b.left, b.right, b.bottom);

      ctx.fillStyle = label;
      ctx.font = "11px IBM Plex Sans, sans-serif";
      ctx.textAlign = "center";
      [100, 1000, 10000].forEach((f) => {
        const x = xAt(f);
        const label = f >= 1000 ? `${f / 1000}k` : String(f);
        ctx.fillText(label, x, b.bottom + 18);
      });

      ctx.textAlign = "right";
      [-18, -12, -6, 0, 6, 12, 18].forEach((g) => {
        const y = b.bottom - ((g - GAIN_MIN) / (GAIN_MAX - GAIN_MIN)) * (b.bottom - b.top);
        const label = g > 0 ? `+${g}` : `${g}`;
        ctx.fillText(label, b.left - 6, y + 4);
      });

      ctx.textAlign = "left";
      [0, 24, 48, 72, 96, 120, 144].forEach((s) => {
        const y = b.bottom - (s / SPECTRUM_RANGE_DB) * (b.bottom - b.top);
        const dbfs = s - SPECTRUM_OFFSET_DB;
        const label = dbfs > 0 ? `+${dbfs}` : `${dbfs}`;
        ctx.fillText(label, b.right + 8, y + 4);
      });

      ctx.font = "12px IBM Plex Sans, sans-serif";
      ctx.textAlign = "right";
      ctx.fillText("Hz", b.right, b.bottom + 34);
      ctx.save();
      ctx.translate(18, b.top + (b.bottom - b.top) / 2);
      ctx.rotate(-Math.PI / 2);
      ctx.textAlign = "center";
      ctx.fillText("Gain [dB]", -10, 0);
      ctx.restore();
      ctx.save();
      ctx.translate(b.right + 42, b.top + (b.bottom - b.top) / 2);
      ctx.rotate(Math.PI / 2);
      ctx.textAlign = "center";
      ctx.fillText("Level [dbFS]", 0, 0);
      ctx.restore();
      ctx.textAlign = "left";
    }

    drawCurve(ctx, b, responseDB, color, width) {
      const n = responseDB.length;
      ctx.save();
      ctx.beginPath();
      ctx.rect(b.left, b.top, b.right - b.left, b.bottom - b.top);
      ctx.clip();
      ctx.strokeStyle = color;
      ctx.lineWidth = width;
      ctx.beginPath();
      for (let i = 0; i < n; i += 1) {
        const t = i / (n - 1);
        const db = responseDB[i];
        const x = b.left + t * (b.right - b.left);
        const y = b.bottom - ((clamp(db, GAIN_MIN, GAIN_MAX) - GAIN_MIN) / (GAIN_MAX - GAIN_MIN)) * (b.bottom - b.top);
        if (i === 0) ctx.moveTo(x, y);
        else ctx.lineTo(x, y);
      }
      ctx.stroke();
      ctx.restore();
    }

    drawSpectrumCurve(ctx, b, spectrumDB, color, width) {
      const n = spectrumDB.length;
      ctx.save();
      ctx.beginPath();
      ctx.rect(b.left, b.top, b.right - b.left, b.bottom - b.top);
      ctx.clip();
      ctx.strokeStyle = color;
      ctx.lineWidth = width;
      ctx.beginPath();
      for (let i = 0; i < n; i += 1) {
        const t = i / (n - 1);
        const dbFS = clamp(spectrumDB[i], SPECTRUM_FLOOR_DBFS, SPECTRUM_TOP_DBFS);
        const spectrumDBScaled = dbFS + SPECTRUM_OFFSET_DB;
        const x = b.left + t * (b.right - b.left);
        const y = b.bottom - (spectrumDBScaled / SPECTRUM_RANGE_DB) * (b.bottom - b.top);
        if (i === 0) ctx.moveTo(x, y);
        else ctx.lineTo(x, y);
      }
      ctx.stroke();
      ctx.restore();
    }

    nodeDescriptors() {
      const p = this.params;
      return [
        { key: "hp", label: "Highpass", x: this.freqToX(p.hpFreq), y: this.gainToY(p.hpGain), color: cssVar("--canvas-node-hp", "#8a4f1f") },
        { key: "low", label: "Low Shelf", x: this.freqToX(p.lowFreq), y: this.gainToY(p.lowGain), color: cssVar("--canvas-node-low", "#c24d2c") },
        { key: "mid", label: "Peak", x: this.freqToX(p.midFreq), y: this.gainToY(p.midGain), color: cssVar("--canvas-node-mid", "#225d7d") },
        { key: "high", label: "High Shelf", x: this.freqToX(p.highFreq), y: this.gainToY(p.highGain), color: cssVar("--canvas-node-high", "#3b7d44") },
        { key: "lp", label: "Lowpass", x: this.freqToX(p.lpFreq), y: this.gainToY(p.lpGain), color: cssVar("--canvas-node-lp", "#6a4aa5") },
      ];
    }

    hoverInfoForKey(key) {
      const p = this.params;
      if (key === "hp") return { key, label: "Highpass", freq: p.hpFreq, gain: p.hpGain, q: p.hpQ };
      if (key === "low") return { key, label: "Low Shelf", freq: p.lowFreq, gain: p.lowGain, q: p.lowQ };
      if (key === "mid") return { key, label: "Peak", freq: p.midFreq, gain: p.midGain, q: p.midQ };
      if (key === "high") return { key, label: "High Shelf", freq: p.highFreq, gain: p.highGain, q: p.highQ };
      if (key === "lp") return { key, label: "Lowpass", freq: p.lpFreq, gain: p.lpGain, q: p.lpQ };
      return null;
    }

    qFieldForKey(key) {
      if (key === "hp") return "hpQ";
      if (key === "low") return "lowQ";
      if (key === "mid") return "midQ";
      if (key === "high") return "highQ";
      if (key === "lp") return "lpQ";
      return null;
    }

    draw() {
      const w = this.cssWidth;
      const h = this.cssHeight;
      const ctx = this.ctx;
      const b = this.bounds();

      ctx.clearRect(0, 0, w, h);
      ctx.fillStyle = cssVar("--canvas-bg", "#fff");
      ctx.fillRect(0, 0, w, h);

      this.drawGrid(ctx, b, w, h);

      const samples = Math.max(200, Math.floor(w));
      const freqs = new Array(samples);
      for (let i = 0; i < samples; i += 1) {
        const t = i / (samples - 1);
        freqs[i] = Math.pow(10, Math.log10(FREQ_MIN) + t * (Math.log10(FREQ_MAX) - Math.log10(FREQ_MIN)));
      }

      const focusKey = this.activeNode || this.hoverNode;
      if (focusKey) {
        const singleDB = this.computeSingleFilterDB(focusKey, freqs);
        const focusColor = {
          hp: cssVar("--canvas-focus-hp", "138,79,31"),
          low: cssVar("--canvas-focus-low", "194,77,44"),
          mid: cssVar("--canvas-focus-mid", "34,93,125"),
          high: cssVar("--canvas-focus-high", "59,125,68"),
          lp: cssVar("--canvas-focus-lp", "106,74,165"),
        }[focusKey];
        const color = this.activeNode ? `rgba(${focusColor}, 0.72)` : `rgba(${focusColor}, 0.28)`;
        this.drawCurve(ctx, b, singleDB, color, this.activeNode ? 2.5 : 2);
      }

      const spectrumDB = this.computeSpectrumDB(freqs);
      if (spectrumDB) {
        this.drawSpectrumCurve(ctx, b, spectrumDB, cssVar("--canvas-spectrum", "rgba(194,77,44,0.62)"), 1.25);
      }

      const responseDB = this.computeResponseDB(freqs);
      this.drawCurve(ctx, b, responseDB, cssVar("--canvas-response", "#225d7d"), 2.4);

      this.nodes = this.nodeDescriptors();
      this.nodes.forEach((n) => {
        ctx.fillStyle = n.color;
        ctx.beginPath();
        ctx.arc(n.x, n.y, 6.5, 0, Math.PI * 2);
        ctx.fill();
        ctx.lineWidth = 2;
        ctx.strokeStyle = cssVar("--canvas-node-stroke", "#fff");
        ctx.stroke();
      });
    }

    nodeAt(x, y) {
      let best = null;
      let bestDist = Infinity;
      this.nodes.forEach((n) => {
        const d = Math.hypot(n.x - x, n.y - y);
        if (d < bestDist) {
          bestDist = d;
          best = n;
        }
      });
      return bestDist <= 16 ? best : null;
    }

    canvasPoint(ev) {
      const r = this.canvas.getBoundingClientRect();
      return { x: ev.clientX - r.left, y: ev.clientY - r.top };
    }

    dragNode(node, x, y) {
      const gain = clamp(this.yToGain(y), GAIN_MIN, GAIN_MAX);
      const freq = clamp(this.xToFreq(x), FREQ_MIN, FREQ_MAX);

      if (node.key === "hp") {
        this.params.hpFreq = freq;
        this.params.hpGain = gain;
      } else if (node.key === "low") {
        this.params.lowFreq = freq;
        this.params.lowGain = gain;
      } else if (node.key === "mid") {
        this.params.midFreq = freq;
        this.params.midGain = gain;
      } else if (node.key === "high") {
        this.params.highFreq = freq;
        this.params.highGain = gain;
      } else {
        this.params.lpFreq = freq;
        this.params.lpGain = gain;
      }

      this.onHover(this.hoverInfoForKey(node.key));
      this.onChange({ ...this.params });
      this.draw();
    }

    bindEvents() {
      this.canvas.addEventListener("pointerdown", (ev) => {
        const p = this.canvasPoint(ev);
        const node = this.nodeAt(p.x, p.y);
        if (!node) return;
        this.activeNode = node.key;
        this.hoverNode = node.key;
        this.onHover(this.hoverInfoForKey(node.key));
        this.canvas.setPointerCapture(ev.pointerId);
        this.draw();
      });

      this.canvas.addEventListener("pointermove", (ev) => {
        const p = this.canvasPoint(ev);
        if (this.activeNode) {
          const node = this.nodes.find((n) => n.key === this.activeNode);
          if (node) this.dragNode(node, p.x, p.y);
          return;
        }

        const hover = this.nodeAt(p.x, p.y);
        const newKey = hover ? hover.key : null;
        if (newKey !== this.hoverNode) {
          this.hoverNode = newKey;
          this.onHover(this.hoverInfoForKey(newKey));
          this.draw();
        }
        this.canvas.style.cursor = hover ? "grab" : "crosshair";
      });

      const release = () => {
        this.activeNode = null;
        this.canvas.style.cursor = this.hoverNode ? "grab" : "crosshair";
        this.draw();
      };

      this.canvas.addEventListener("pointerup", release);
      this.canvas.addEventListener("pointercancel", release);
      this.canvas.addEventListener("pointerleave", () => {
        this.activeNode = null;
        if (this.hoverNode !== null) {
          this.hoverNode = null;
          this.onHover(null);
          this.draw();
        }
        this.canvas.style.cursor = "crosshair";
      });

      this.canvas.addEventListener(
        "wheel",
        (ev) => {
          const key = this.activeNode || this.hoverNode;
          if (!key) return;

          const field = this.qFieldForKey(key);
          if (!field) return;

          ev.preventDefault();
          const factor = ev.deltaY < 0 ? 1.08 : 1 / 1.08;
          this.params[field] = clamp(this.params[field] * factor, 0.2, 8);
          this.onHover(this.hoverInfoForKey(key));
          this.onChange({ ...this.params });
          this.draw();
        },
        { passive: false },
      );

      window.addEventListener("resize", () => this.resize());
    }
  }

  window.EQCanvas = EQCanvas;
})();
