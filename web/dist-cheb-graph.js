// Chebyshev waveshaper waveform visualization.
// Draws a sine-wave input (grey) and the Chebyshev-shaped output (accent colour)
// on a canvas without any DSP backend call — the polynomial is evaluated directly.

(function () {
  "use strict";

  const CYCLES = 2;    // number of sine cycles shown
  const SAMPLES = 512; // horizontal resolution
  const PAD = 16;      // pixel padding on all sides

  // Mirror the Go chebyshevShape logic in JS.
  function evalChebyshev(x, order, weights, gain, invert) {
    x = Math.max(-1, Math.min(1, x));

    let hasWeights = false;
    for (let k = 0; k < order; k++) {
      if (weights[k] !== 0) { hasWeights = true; break; }
    }

    // T_0=1, T_1=x, T_n = 2x·T_{n-1} − T_{n-2}
    let t0 = 1.0, t1 = x, tn = x;
    let wsum = hasWeights ? weights[0] * x : 0.0;

    for (let n = 2; n <= order; n++) {
      tn = 2 * x * t1 - t0;
      if (hasWeights) wsum += weights[n - 1] * tn;
      t0 = t1;
      t1 = tn;
    }

    let out = (hasWeights ? wsum : tn) * gain;
    if (invert) out = -out;
    return Math.max(-1, Math.min(1, out));
  }

  class DistChebGraph {
    constructor(canvas) {
      this.canvas = canvas;
      this.ctx = canvas.getContext("2d");
      this._params = null;
    }

    draw(params) {
      if (params) this._params = params;
      if (!this._params) return;

      const { order, gain, invert, drive, weights } = this._params;
      const canvas = this.canvas;
      const ctx = this.ctx;
      const w = canvas.width;
      const h = canvas.height;

      // ---- theme colours ------------------------------------------------
      const style = getComputedStyle(document.documentElement);
      const bg      = style.getPropertyValue("--canvas-bg").trim() || "#fff";
      const grid    = style.getPropertyValue("--line").trim()      || "#d9ccb6";
      const accent  = style.getPropertyValue("--accent").trim()    || "#c24d2c";
      const ink     = style.getPropertyValue("--ink").trim()       || "#1d1b18";

      // ---- clear --------------------------------------------------------
      ctx.clearRect(0, 0, w, h);
      ctx.fillStyle = bg;
      ctx.fillRect(0, 0, w, h);

      const drawW = w - 2 * PAD;
      const drawH = h - 2 * PAD;
      const midY  = PAD + drawH / 2;

      // ---- grid (±1, ±0.5, 0) ------------------------------------------
      ctx.strokeStyle = grid;
      ctx.lineWidth = 1;
      ctx.setLineDash([2, 3]);
      for (const level of [1, 0.5, 0, -0.5, -1]) {
        const y = midY - level * (drawH / 2);
        ctx.beginPath();
        ctx.moveTo(PAD, y);
        ctx.lineTo(w - PAD, y);
        ctx.stroke();
      }
      ctx.setLineDash([]);

      // ---- input sine (grey) -------------------------------------------
      ctx.strokeStyle = grid;
      ctx.globalAlpha = 0.7;
      ctx.lineWidth = 1;
      ctx.beginPath();
      for (let i = 0; i <= SAMPLES; i++) {
        const t   = (i / SAMPLES) * CYCLES * 2 * Math.PI;
        const amp = Math.sin(t);
        const cx  = PAD + (i / SAMPLES) * drawW;
        const cy  = midY - amp * (drawH / 2);
        i === 0 ? ctx.moveTo(cx, cy) : ctx.lineTo(cx, cy);
      }
      ctx.stroke();
      ctx.globalAlpha = 1.0;

      // ---- shaped output (accent) --------------------------------------
      ctx.strokeStyle = accent;
      ctx.lineWidth = 2;
      ctx.beginPath();
      for (let i = 0; i <= SAMPLES; i++) {
        const t   = (i / SAMPLES) * CYCLES * 2 * Math.PI;
        const raw = Math.max(-1, Math.min(1, Math.sin(t) * drive));
        const out = evalChebyshev(raw, order, weights, gain, invert);
        const cx  = PAD + (i / SAMPLES) * drawW;
        const cy  = midY - out * (drawH / 2);
        i === 0 ? ctx.moveTo(cx, cy) : ctx.lineTo(cx, cy);
      }
      ctx.stroke();

      // ---- labels -------------------------------------------------------
      ctx.fillStyle = ink;
      ctx.font = "9px sans-serif";
      ctx.textAlign = "right";
      ctx.fillText("+1", PAD - 3, PAD + 4);
      ctx.fillText("-1", PAD - 3, h - PAD + 4);
      ctx.textAlign = "left";
      ctx.fillText(`drive ×${drive.toFixed(1)}`, PAD + 2, h - 4);
      ctx.textAlign = "right";
      ctx.fillText(`T${order}`, w - PAD - 2, h - 4);
    }
  }

  window.DistChebGraph = DistChebGraph;
})();
