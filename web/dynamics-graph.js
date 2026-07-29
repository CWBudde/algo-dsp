(() => {
  class DynamicsGraph {
    constructor(canvas, options = {}) {
      this.canvas = canvas;
      this.ctx = canvas.getContext("2d");
      this.getCurve = options.getCurve || null;
      this.minDB = -60;
      this.maxDB = 12;
      this.range = this.maxDB - this.minDB;
      this.pad = 20;

      // CSS pixel size, fixed by the width/height attributes in the markup.
      this.cssWidth = canvas.width;
      this.cssHeight = canvas.height;
      canvas.style.width = `${this.cssWidth}px`;
      canvas.style.height = `${this.cssHeight}px`;
      this._dpr = 0;

      this.draw();
    }

    /**
     * Match the backing store to the device pixel ratio.
     *
     * Without this the canvas renders at 1x and is upscaled by the compositor,
     * which is visibly soft on any HiDPI display -- and these plots are mostly
     * thin lines and 9px labels, where it shows most.
     */
    _applyDPR() {
      const dpr = window.devicePixelRatio || 1;
      if (dpr === this._dpr) return;

      this._dpr = dpr;
      this.canvas.width = Math.round(this.cssWidth * dpr);
      this.canvas.height = Math.round(this.cssHeight * dpr);
    }

    draw() {
      this._applyDPR();

      const ctx = this.ctx;
      const w = this.cssWidth;
      const h = this.cssHeight;
      ctx.setTransform(this._dpr, 0, 0, this._dpr, 0, 0);
      const pad = this.pad;
      const minDB = this.minDB;
      const maxDB = this.maxDB;
      const range = this.range;

      ctx.clearRect(0, 0, w, h);

      // Colors from CSS vars or defaults
      const root = document.documentElement;
      const style = getComputedStyle(root);
      const gridColor = style.getPropertyValue("--line").trim() || "#d9ccb6";
      const accentColor =
        style.getPropertyValue("--accent").trim() || "#c24d2c";
      const textColor = style.getPropertyValue("--ink").trim() || "#1d1b18";

      // Grid
      ctx.strokeStyle = gridColor;
      ctx.lineWidth = 1;
      ctx.setLineDash([2, 2]);
      for (let db = minDB; db <= maxDB; db += 12) {
        const x = pad + ((db - minDB) / range) * (w - 2 * pad);
        const y = pad + (1 - (db - minDB) / range) * (h - 2 * pad);

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
      if (this.getCurve) {
        const points = 100;
        const inputs = new Float32Array(points + 1);
        for (let i = 0; i <= points; i++) {
          inputs[i] = minDB + (i / points) * range;
        }

        const outputs = this.getCurve(inputs);

        if (outputs && outputs.length === inputs.length) {
          ctx.strokeStyle = accentColor;
          ctx.lineWidth = 2;
          ctx.beginPath();
          for (let i = 0; i <= points; i++) {
            const inDB = inputs[i];
            const outDB = outputs[i];

            const x = pad + ((inDB - minDB) / range) * (w - 2 * pad);
            const y = h - (pad + ((outDB - minDB) / range) * (h - 2 * pad));

            if (i === 0) ctx.moveTo(x, y);
            else ctx.lineTo(x, y);
          }
          ctx.stroke();
        }
      }

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
  }

  window.DynamicsGraph = DynamicsGraph;
})();
