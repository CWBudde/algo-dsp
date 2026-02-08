(() => {
  const FREQ_MIN = 20;
  const FREQ_MAX = 20000;
  const GAIN_MIN = -18;
  const GAIN_MAX = 18;

  function clamp(v, min, max) {
    return Math.min(max, Math.max(min, v));
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
    const alpha = (sinw0 / 2) * Math.sqrt(2);
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
    const alpha = (sinw0 / 2) * Math.sqrt(2);
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
      this.getSampleRate = options.getSampleRate || (() => 48000);
      this.getResponseDB = options.getResponseDB || null;
      this.params = {
        lowFreq: 100,
        lowGain: 0,
        midFreq: 1000,
        midGain: 0,
        highFreq: 6000,
        highGain: 0,
        midQ: 1.2,
        master: 0.75,
        ...(options.initialParams || {}),
      };
      this.nodes = [];
      this.activeNode = null;
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
      this.draw();
      if (emit) this.onChange({ ...this.params });
    }

    getParams() {
      return { ...this.params };
    }

    constrainOrder() {
      this.params.lowFreq = clamp(this.params.lowFreq, FREQ_MIN, FREQ_MAX);
      this.params.midFreq = clamp(this.params.midFreq, FREQ_MIN, FREQ_MAX);
      this.params.highFreq = clamp(this.params.highFreq, FREQ_MIN, FREQ_MAX);

      this.params.midFreq = clamp(this.params.midFreq, this.params.lowFreq * 1.25, FREQ_MAX);
      this.params.highFreq = clamp(this.params.highFreq, this.params.midFreq * 1.25, FREQ_MAX);
      this.params.lowFreq = clamp(this.params.lowFreq, FREQ_MIN, this.params.midFreq / 1.25);

      this.params.lowGain = clamp(this.params.lowGain, GAIN_MIN, GAIN_MAX);
      this.params.midGain = clamp(this.params.midGain, GAIN_MIN, GAIN_MAX);
      this.params.highGain = clamp(this.params.highGain, GAIN_MIN, GAIN_MAX);
      this.params.midQ = clamp(this.params.midQ, 0.2, 8);
      this.params.master = clamp(this.params.master, 0, 1);
    }

    bounds() {
      const left = 64;
      const right = this.cssWidth - 24;
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

    eqMagnitude(freq) {
      const p = this.params;
      const sampleRate = this.getSampleRate();
      const low = lowShelfCoeffs(p.lowFreq, p.lowGain, sampleRate);
      const mid = peakingCoeffs(p.midFreq, p.midGain, p.midQ, sampleRate);
      const high = highShelfCoeffs(p.highFreq, p.highGain, sampleRate);
      return (
        biquadMagnitudeAt(freq, sampleRate, low) *
        biquadMagnitudeAt(freq, sampleRate, mid) *
        biquadMagnitudeAt(freq, sampleRate, high)
      );
    }

    computeResponseDB(freqs) {
      if (this.getResponseDB) {
        const response = this.getResponseDB(Float32Array.from(freqs));
        if (response && typeof response.length === "number" && response.length === freqs.length) {
          return response;
        }
      }
      return freqs.map((freq) => 20 * Math.log10(Math.max(1e-6, this.eqMagnitude(freq) * this.params.master)));
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
      ctx.strokeStyle = "#ece1d2";
      minors.forEach((f) => {
        drawV(xAt(f), b.top, b.bottom);
      });

      ctx.strokeStyle = "#d4c6b2";
      majors.forEach((f) => {
        drawV(xAt(f), b.top, b.bottom);
      });

      [-18, -12, -6, 0, 6, 12, 18].forEach((g) => {
        const y = b.bottom - ((g - GAIN_MIN) / (GAIN_MAX - GAIN_MIN)) * (b.bottom - b.top);
        ctx.strokeStyle = g === 0 ? "#d4c6b2" : "#ece1d2";
        drawH(b.left, b.right, y);
      });

      ctx.strokeStyle = "#9b8f7a";
      drawV(b.left, b.top, b.bottom);
      drawH(b.left, b.right, b.bottom);

      ctx.fillStyle = "#6a5f4f";
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

      ctx.font = "12px IBM Plex Sans, sans-serif";
      ctx.textAlign = "right";
      ctx.fillText("Hz", b.right, b.bottom + 34);
      ctx.save();
      ctx.translate(18, b.top + (b.bottom - b.top) / 2);
      ctx.rotate(-Math.PI / 2);
      ctx.textAlign = "center";
      ctx.fillText("dB", -10, 0);
      ctx.restore();
      ctx.textAlign = "left";
    }

    draw() {
      const w = this.cssWidth;
      const h = this.cssHeight;
      const ctx = this.ctx;
      const b = this.bounds();

      ctx.clearRect(0, 0, w, h);
      ctx.fillStyle = "#fff";
      ctx.fillRect(0, 0, w, h);

      this.drawGrid(ctx, b, w, h);

      const samples = Math.max(200, Math.floor(w));
      const freqs = new Array(samples);
      for (let i = 0; i < samples; i += 1) {
        const t = i / (samples - 1);
        freqs[i] = Math.pow(10, Math.log10(FREQ_MIN) + t * (Math.log10(FREQ_MAX) - Math.log10(FREQ_MIN)));
      }
      const responseDB = this.computeResponseDB(freqs);

      ctx.strokeStyle = "#225d7d";
      ctx.lineWidth = 2;
      ctx.beginPath();
      for (let i = 0; i < samples; i += 1) {
        const t = i / (samples - 1);
        const db = responseDB[i];
        const x = b.left + t * (b.right - b.left);
        const y = b.bottom - ((clamp(db, GAIN_MIN - 6, GAIN_MAX + 6) - GAIN_MIN) / (GAIN_MAX - GAIN_MIN)) * (b.bottom - b.top);
        if (i === 0) ctx.moveTo(x, y);
        else ctx.lineTo(x, y);
      }
      ctx.stroke();

      const p = this.params;
      this.nodes = [
        { key: "low", x: this.freqToX(p.lowFreq), y: this.gainToY(p.lowGain), color: "#c24d2c" },
        { key: "mid", x: this.freqToX(p.midFreq), y: this.gainToY(p.midGain), color: "#225d7d" },
        { key: "high", x: this.freqToX(p.highFreq), y: this.gainToY(p.highGain), color: "#3b7d44" },
      ];

      this.nodes.forEach((n) => {
        ctx.fillStyle = n.color;
        ctx.beginPath();
        ctx.arc(n.x, n.y, 6.5, 0, Math.PI * 2);
        ctx.fill();
        ctx.lineWidth = 2;
        ctx.strokeStyle = "#fff";
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

      if (node.key === "low") {
        this.params.lowFreq = clamp(this.xToFreq(x), FREQ_MIN, this.params.midFreq / 1.25);
        this.params.lowGain = gain;
      } else if (node.key === "mid") {
        this.params.midFreq = clamp(this.xToFreq(x), this.params.lowFreq * 1.25, this.params.highFreq / 1.25);
        this.params.midGain = gain;
      } else {
        this.params.highFreq = clamp(this.xToFreq(x), this.params.midFreq * 1.25, FREQ_MAX);
        this.params.highGain = gain;
      }

      this.draw();
      this.onChange({ ...this.params });
    }

    bindEvents() {
      this.canvas.addEventListener("pointerdown", (ev) => {
        const p = this.canvasPoint(ev);
        const node = this.nodeAt(p.x, p.y);
        if (!node) return;
        this.activeNode = node.key;
        this.canvas.setPointerCapture(ev.pointerId);
      });

      this.canvas.addEventListener("pointermove", (ev) => {
        const p = this.canvasPoint(ev);
        if (this.activeNode) {
          const node = this.nodes.find((n) => n.key === this.activeNode);
          if (node) this.dragNode(node, p.x, p.y);
          return;
        }
        const hover = this.nodeAt(p.x, p.y);
        this.canvas.style.cursor = hover ? "grab" : "crosshair";
      });

      const release = () => {
        this.activeNode = null;
        this.canvas.style.cursor = "crosshair";
      };

      this.canvas.addEventListener("pointerup", release);
      this.canvas.addEventListener("pointercancel", release);
      this.canvas.addEventListener("pointerleave", () => {
        if (!this.activeNode) this.canvas.style.cursor = "crosshair";
      });

      window.addEventListener("resize", () => this.resize());
    }
  }

  window.EQCanvas = EQCanvas;
})();
