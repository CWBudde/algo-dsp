(() => {
  const FREQ_MIN = 20;
  const FREQ_MAX = 20000;
  const GAIN_MIN = -18;
  const GAIN_MAX = 18;
  const SPECTRUM_RANGE_DB = 144;
  const SPECTRUM_OFFSET_DB = 120;
  const SPECTRUM_TOP_DBFS = SPECTRUM_RANGE_DB - SPECTRUM_OFFSET_DB;
  const SPECTRUM_FLOOR_DBFS = -SPECTRUM_OFFSET_DB;
  const NODE_TYPE_OPTIONS = {
    hp: ["highpass", "lowpass", "bandpass", "notch", "allpass", "peak", "lowshelf", "highshelf"],
    low: ["highpass", "lowpass", "bandpass", "notch", "allpass", "peak", "lowshelf", "highshelf"],
    mid: ["highpass", "lowpass", "bandpass", "notch", "allpass", "peak", "lowshelf", "highshelf"],
    high: ["highpass", "lowpass", "bandpass", "notch", "allpass", "peak", "lowshelf", "highshelf"],
    lp: ["highpass", "lowpass", "bandpass", "notch", "allpass", "peak", "lowshelf", "highshelf"],
  };
  const TYPE_LABELS = {
    highpass: "Highpass",
    lowpass: "Lowpass",
    bandpass: "Band EQ",
    notch: "Notch",
    allpass: "Allpass",
    peak: "Peak",
    highshelf: "High Shelf",
    lowshelf: "Low Shelf",
  };
  const FAMILY_OPTIONS = ["rbj", "butterworth", "chebyshev1", "chebyshev2", "elliptic"];
  const FAMILY_LABELS = {
    rbj: "RBJ",
    butterworth: "Butterworth",
    chebyshev1: "Chebyshev 1",
    chebyshev2: "Chebyshev 2",
    elliptic: "Elliptic",
  };
  const ORDER_MIN = 1;
  const ORDER_MAX = 12;

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

  function bandpassCoeffs(freq, q, sampleRate) {
    const w0 = (2 * Math.PI * freq) / sampleRate;
    const alpha = Math.sin(w0) / (2 * q);
    const cosw0 = Math.cos(w0);
    const sinw0 = Math.sin(w0);
    return {
      b0: sinw0 / 2,
      b1: 0,
      b2: -sinw0 / 2,
      a0: 1 + alpha,
      a1: -2 * cosw0,
      a2: 1 - alpha,
    };
  }

  function notchCoeffs(freq, q, sampleRate) {
    const w0 = (2 * Math.PI * freq) / sampleRate;
    const alpha = Math.sin(w0) / (2 * q);
    const cosw0 = Math.cos(w0);
    return {
      b0: 1,
      b1: -2 * cosw0,
      b2: 1,
      a0: 1 + alpha,
      a1: -2 * cosw0,
      a2: 1 - alpha,
    };
  }

  function allpassCoeffs(freq, q, sampleRate) {
    const w0 = (2 * Math.PI * freq) / sampleRate;
    const alpha = Math.sin(w0) / (2 * q);
    const cosw0 = Math.cos(w0);
    return {
      b0: 1 - alpha,
      b1: -2 * cosw0,
      b2: 1 + alpha,
      a0: 1 + alpha,
      a1: -2 * cosw0,
      a2: 1 - alpha,
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
      this.getNodeResponseDB = options.getNodeResponseDB || null;
      this.getSpectrumDB = options.getSpectrumDB || null;
      this.params = {
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
        ...(options.initialParams || {}),
      };
      this.nodes = [];
      this.activeNode = null;
      this.hoverNode = null;
      this.contextMenu = this.createContextMenu();
      this.menuNodeKey = null;
      this.cssWidth = 0;
      this.cssHeight = 0;

      this.constrainOrder();
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
      this.params.hpType = this.normalizeTypeForKey("hp", this.params.hpType);
      this.params.lowType = this.normalizeTypeForKey("low", this.params.lowType);
      this.params.midType = this.normalizeTypeForKey("mid", this.params.midType);
      this.params.highType = this.normalizeTypeForKey("high", this.params.highType);
      this.params.lpType = this.normalizeTypeForKey("lp", this.params.lpType);
      this.params.hpFamily = this.normalizeFamilyForKeyType("hp", this.params.hpType, this.params.hpFamily);
      this.params.lowFamily = this.normalizeFamilyForKeyType("low", this.params.lowType, this.params.lowFamily);
      this.params.midFamily = this.normalizeFamilyForKeyType("mid", this.params.midType, this.params.midFamily);
      this.params.highFamily = this.normalizeFamilyForKeyType("high", this.params.highType, this.params.highFamily);
      this.params.lpFamily = this.normalizeFamilyForKeyType("lp", this.params.lpType, this.params.lpFamily);
      this.params.hpOrder = this.normalizeOrderForKeyTypeFamily("hp", this.params.hpType, this.params.hpFamily, this.params.hpOrder);
      this.params.lowOrder = this.normalizeOrderForKeyTypeFamily("low", this.params.lowType, this.params.lowFamily, this.params.lowOrder);
      this.params.midOrder = this.normalizeOrderForKeyTypeFamily("mid", this.params.midType, this.params.midFamily, this.params.midOrder);
      this.params.highOrder = this.normalizeOrderForKeyTypeFamily("high", this.params.highType, this.params.highFamily, this.params.highOrder);
      this.params.lpOrder = this.normalizeOrderForKeyTypeFamily("lp", this.params.lpType, this.params.lpFamily, this.params.lpOrder);
      this.params.master = clamp(this.params.master, 0, 1);
    }

    typeFieldForKey(key) {
      if (key === "hp") return "hpType";
      if (key === "low") return "lowType";
      if (key === "mid") return "midType";
      if (key === "high") return "highType";
      if (key === "lp") return "lpType";
      return null;
    }

    familyFieldForKey(key) {
      if (key === "hp") return "hpFamily";
      if (key === "low") return "lowFamily";
      if (key === "mid") return "midFamily";
      if (key === "high") return "highFamily";
      if (key === "lp") return "lpFamily";
      return null;
    }

    orderFieldForKey(key) {
      if (key === "hp") return "hpOrder";
      if (key === "low") return "lowOrder";
      if (key === "mid") return "midOrder";
      if (key === "high") return "highOrder";
      if (key === "lp") return "lpOrder";
      return null;
    }

    normalizeTypeForKey(key, value) {
      const options = NODE_TYPE_OPTIONS[key] || [];
      if (options.includes(value)) return value;
      return options[0] || "peak";
    }

    typeForKey(key) {
      const field = this.typeFieldForKey(key);
      if (!field) return "peak";
      return this.normalizeTypeForKey(key, this.params[field]);
    }

    normalizeFamily(value) {
      if (FAMILY_OPTIONS.includes(value)) return value;
      return "rbj";
    }

    supportsFamilyForType(type, family) {
      if (family === "rbj") return true;
      if (family === "elliptic") return type === "bandpass";
      if (family === "butterworth" || family === "chebyshev1" || family === "chebyshev2") {
        return type === "highpass" || type === "lowpass" || type === "bandpass" || type === "lowshelf" || type === "highshelf";
      }
      return false;
    }

    supportsOrderForTypeFamily(type, family) {
      if (family === "rbj") return false;
      if (family === "elliptic") return type === "bandpass";
      if (family === "butterworth" || family === "chebyshev1" || family === "chebyshev2") {
        return type === "highpass" || type === "lowpass" || type === "bandpass" || type === "lowshelf" || type === "highshelf";
      }
      return false;
    }

    normalizeOrderForKeyTypeFamily(key, type, family, order) {
      if (!this.supportsOrderForTypeFamily(type, family)) return 1;
      let v = Number(order);
      if (!Number.isFinite(v) || v <= 0) v = 4;
      v = Math.round(clamp(v, ORDER_MIN, ORDER_MAX));
      if (type === "bandpass") {
        if (v < 4) v = 4;
        if (v % 2 !== 0) v += 1;
      }
      return v;
    }

    normalizeFamilyForKeyType(key, type, family) {
      const normalized = this.normalizeFamily(family);
      if (this.supportsFamilyForType(type, normalized)) return normalized;
      return "rbj";
    }

    familyForKey(key) {
      const familyField = this.familyFieldForKey(key);
      if (!familyField) return "rbj";
      return this.normalizeFamilyForKeyType(key, this.typeForKey(key), this.params[familyField]);
    }

    orderForKey(key) {
      const orderField = this.orderFieldForKey(key);
      if (!orderField) return 1;
      return this.normalizeOrderForKeyTypeFamily(key, this.typeForKey(key), this.familyForKey(key), this.params[orderField]);
    }

    familyLabel(family) {
      return FAMILY_LABELS[family] || family;
    }

    typeLabel(type) {
      return TYPE_LABELS[type] || type;
    }

    typeUsesGainInCoeffs(family, type) {
      if (type === "peak" || type === "lowshelf" || type === "highshelf") return true;
      return type === "bandpass" && family !== "rbj";
    }

    labelForKey(key) {
      return this.typeLabel(this.typeForKey(key));
    }

    filterCoeffs(type, freq, gainDB, q, sampleRate) {
      if (type === "highpass") return highpassCoeffs(freq, q, sampleRate);
      if (type === "lowpass") return lowpassCoeffs(freq, q, sampleRate);
      if (type === "bandpass") return bandpassCoeffs(freq, q, sampleRate);
      if (type === "notch") return notchCoeffs(freq, q, sampleRate);
      if (type === "allpass") return allpassCoeffs(freq, q, sampleRate);
      if (type === "highshelf") return highShelfCoeffs(freq, gainDB, q, sampleRate);
      if (type === "lowshelf") return lowShelfCoeffs(freq, gainDB, q, sampleRate);
      return peakingCoeffs(freq, gainDB, q, sampleRate);
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
      const type = this.typeForKey(key);
      const family = this.familyForKey(key);
      const typeHasEmbeddedGain = this.typeUsesGainInCoeffs(family, type);
      if (key === "hp") {
        const hpMag = biquadMagnitudeAt(freq, sampleRate, this.filterCoeffs(type, p.hpFreq, 0, p.hpQ, sampleRate));
        return hpMag * Math.pow(10, p.hpGain / 20);
      }
      if (key === "low") {
        const lowMag = biquadMagnitudeAt(
          freq,
          sampleRate,
          this.filterCoeffs(type, p.lowFreq, typeHasEmbeddedGain ? p.lowGain : 0, p.lowQ, sampleRate),
        );
        return lowMag * (typeHasEmbeddedGain ? 1 : Math.pow(10, p.lowGain / 20));
      }
      if (key === "mid") {
        const midMag = biquadMagnitudeAt(
          freq,
          sampleRate,
          this.filterCoeffs(type, p.midFreq, typeHasEmbeddedGain ? p.midGain : 0, p.midQ, sampleRate),
        );
        return midMag * (typeHasEmbeddedGain ? 1 : Math.pow(10, p.midGain / 20));
      }
      if (key === "high") {
        const highMag = biquadMagnitudeAt(
          freq,
          sampleRate,
          this.filterCoeffs(type, p.highFreq, typeHasEmbeddedGain ? p.highGain : 0, p.highQ, sampleRate),
        );
        return highMag * (typeHasEmbeddedGain ? 1 : Math.pow(10, p.highGain / 20));
      }
      const lpMag = biquadMagnitudeAt(freq, sampleRate, this.filterCoeffs(type, p.lpFreq, 0, p.lpQ, sampleRate));
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
      if (this.getNodeResponseDB) {
        const response = this.getNodeResponseDB(key, Float32Array.from(freqs));
        if (response && typeof response.length === "number" && response.length === freqs.length) {
          return response;
        }
      }
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
        { key: "hp", label: this.labelForKey("hp"), x: this.freqToX(p.hpFreq), y: this.gainToY(p.hpGain), color: cssVar("--canvas-node-hp", "#8a4f1f") },
        { key: "low", label: this.labelForKey("low"), x: this.freqToX(p.lowFreq), y: this.gainToY(p.lowGain), color: cssVar("--canvas-node-low", "#c24d2c") },
        { key: "mid", label: this.labelForKey("mid"), x: this.freqToX(p.midFreq), y: this.gainToY(p.midGain), color: cssVar("--canvas-node-mid", "#225d7d") },
        { key: "high", label: this.labelForKey("high"), x: this.freqToX(p.highFreq), y: this.gainToY(p.highGain), color: cssVar("--canvas-node-high", "#3b7d44") },
        { key: "lp", label: this.labelForKey("lp"), x: this.freqToX(p.lpFreq), y: this.gainToY(p.lpGain), color: cssVar("--canvas-node-lp", "#6a4aa5") },
      ];
    }

    hoverInfoForKey(key) {
      const p = this.params;
      if (key === "hp") return { key, label: this.labelForKey("hp"), type: this.typeForKey("hp"), family: this.familyForKey("hp"), order: this.orderForKey("hp"), freq: p.hpFreq, gain: p.hpGain, q: p.hpQ };
      if (key === "low") return { key, label: this.labelForKey("low"), type: this.typeForKey("low"), family: this.familyForKey("low"), order: this.orderForKey("low"), freq: p.lowFreq, gain: p.lowGain, q: p.lowQ };
      if (key === "mid") return { key, label: this.labelForKey("mid"), type: this.typeForKey("mid"), family: this.familyForKey("mid"), order: this.orderForKey("mid"), freq: p.midFreq, gain: p.midGain, q: p.midQ };
      if (key === "high") return { key, label: this.labelForKey("high"), type: this.typeForKey("high"), family: this.familyForKey("high"), order: this.orderForKey("high"), freq: p.highFreq, gain: p.highGain, q: p.highQ };
      if (key === "lp") return { key, label: this.labelForKey("lp"), type: this.typeForKey("lp"), family: this.familyForKey("lp"), order: this.orderForKey("lp"), freq: p.lpFreq, gain: p.lpGain, q: p.lpQ };
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

    createContextMenu() {
      const menu = document.createElement("div");
      menu.className = "eq-context-menu";
      menu.hidden = true;
      document.body.appendChild(menu);
      return menu;
    }

    renderContextMenu(key) {
      const menu = this.contextMenu;
      const selected = this.typeForKey(key);
      const selectedFamily = this.familyForKey(key);
      const selectedOrder = this.orderForKey(key);
      menu.innerHTML = "";
      const title = document.createElement("div");
      title.className = "eq-context-menu-title";
      title.textContent = "Filter";
      menu.appendChild(title);

      const grid = document.createElement("div");
      grid.className = "eq-context-menu-grid";
      const typeCol = document.createElement("div");
      typeCol.className = "eq-context-menu-col";
      const divider = document.createElement("div");
      divider.className = "eq-context-menu-divider";
      const familyCol = document.createElement("div");
      familyCol.className = "eq-context-menu-col";
      grid.appendChild(typeCol);
      grid.appendChild(divider);
      grid.appendChild(familyCol);
      menu.appendChild(grid);

      const typeHeader = document.createElement("div");
      typeHeader.className = "eq-context-menu-section";
      typeHeader.textContent = "Type";
      typeCol.appendChild(typeHeader);
      for (const type of NODE_TYPE_OPTIONS[key] || []) {
        const button = document.createElement("button");
        button.type = "button";
        button.className = "eq-context-menu-item";
        if (type === selected) button.classList.add("is-active");
        button.textContent = this.typeLabel(type);
        button.addEventListener("click", () => {
          const field = this.typeFieldForKey(key);
          const familyField = this.familyFieldForKey(key);
          if (!field) return;
          this.params[field] = type;
          if (familyField) {
            this.params[familyField] = this.normalizeFamilyForKeyType(key, type, this.params[familyField]);
          }
          this.constrainOrder();
          this.hideContextMenu();
          this.onHover(this.hoverInfoForKey(key));
          this.onChange({ ...this.params });
          this.draw();
        });
        typeCol.appendChild(button);
      }

      const familyHeader = document.createElement("div");
      familyHeader.className = "eq-context-menu-section";
      familyHeader.textContent = "Design";
      familyCol.appendChild(familyHeader);
      const currentType = this.typeForKey(key);
      for (const family of FAMILY_OPTIONS) {
        if (!this.supportsFamilyForType(currentType, family)) continue;
        const button = document.createElement("button");
        button.type = "button";
        button.className = "eq-context-menu-item";
        if (family === selectedFamily) button.classList.add("is-active");
        button.textContent = this.familyLabel(family);
        button.addEventListener("click", () => {
          const field = this.familyFieldForKey(key);
          if (!field) return;
          this.params[field] = family;
          this.constrainOrder();
          this.hideContextMenu();
          this.onHover(this.hoverInfoForKey(key));
          this.onChange({ ...this.params });
          this.draw();
        });
        familyCol.appendChild(button);
      }

      if (this.supportsOrderForTypeFamily(currentType, selectedFamily)) {
        const orderWrap = document.createElement("div");
        orderWrap.className = "eq-context-order";
        const orderLabel = document.createElement("div");
        orderLabel.className = "eq-context-menu-section";
        orderLabel.textContent = "Order";
        orderWrap.appendChild(orderLabel);

        const select = document.createElement("select");
        select.className = "eq-context-order-select";
        const min = currentType === "bandpass" ? 4 : ORDER_MIN;
        for (let v = min; v <= ORDER_MAX; v += 1) {
          if (currentType === "bandpass" && v % 2 !== 0) continue;
          const option = document.createElement("option");
          option.value = String(v);
          option.textContent = String(v);
          if (v === selectedOrder) option.selected = true;
          select.appendChild(option);
        }
        select.addEventListener("change", () => {
          const orderField = this.orderFieldForKey(key);
          if (!orderField) return;
          this.params[orderField] = Number(select.value);
          this.constrainOrder();
          this.hideContextMenu();
          this.onHover(this.hoverInfoForKey(key));
          this.onChange({ ...this.params });
          this.draw();
        });
        orderWrap.appendChild(select);
        familyCol.appendChild(orderWrap);
      }
    }

    showContextMenu(key, clientX, clientY) {
      this.menuNodeKey = key;
      this.renderContextMenu(key);
      const menu = this.contextMenu;
      menu.hidden = false;
      menu.style.left = "0px";
      menu.style.top = "0px";
      const pad = 8;
      const rect = menu.getBoundingClientRect();
      const maxLeft = Math.max(pad, window.innerWidth - rect.width - pad);
      const maxTop = Math.max(pad, window.innerHeight - rect.height - pad);
      menu.style.left = `${clamp(clientX, pad, maxLeft)}px`;
      menu.style.top = `${clamp(clientY, pad, maxTop)}px`;
    }

    hideContextMenu() {
      this.menuNodeKey = null;
      this.contextMenu.hidden = true;
      this.contextMenu.style.left = "0px";
      this.contextMenu.style.top = "0px";
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
        this.hideContextMenu();
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

      this.canvas.addEventListener("contextmenu", (ev) => {
        const p = this.canvasPoint(ev);
        const node = this.nodeAt(p.x, p.y);
        if (!node) {
          this.hideContextMenu();
          return;
        }
        ev.preventDefault();
        this.activeNode = null;
        this.hoverNode = node.key;
        this.onHover(this.hoverInfoForKey(node.key));
        this.showContextMenu(node.key, ev.clientX, ev.clientY);
        this.draw();
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

      window.addEventListener("pointerdown", (ev) => {
        if (this.contextMenu.hidden) return;
        if (this.contextMenu.contains(ev.target)) return;
        this.hideContextMenu();
      });
      window.addEventListener("keydown", (ev) => {
        if (ev.key !== "Escape") return;
        this.hideContextMenu();
      });
      window.addEventListener("scroll", () => this.hideContextMenu(), true);

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

      window.addEventListener("resize", () => {
        this.hideContextMenu();
        this.resize();
      });
    }
  }

  window.EQCanvas = EQCanvas;
})();
