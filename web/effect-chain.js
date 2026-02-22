// Effect Chain — visual node-graph editor for the algo-dsp web demo.
// Replaces the mode-selector UI with a 2-D canvas where users can add,
// position, connect and bypass effect blocks.

(function () {
  "use strict";

  // ---- effect type registry ------------------------------------------------
  const FX_TYPES = {
    chorus:           { label: "Chorus",            hue: 15,  category: "Modulation" },
    flanger:          { label: "Flanger",           hue: 200, category: "Modulation" },
    ringmod:          { label: "Ring Mod",          hue: 320, category: "Modulation" },
    phaser:           { label: "Phaser",            hue: 140, category: "Modulation" },
    tremolo:          { label: "Tremolo",           hue: 270, category: "Modulation" },
    bitcrusher:       { label: "Bit Crusher",       hue: 24,  category: "Color" },
    distortion:       { label: "Distortion",        hue: 6,   category: "Color" },
    transformer:      { label: "Transformer Sat",   hue: 44,  category: "Color" },
    filter:           { label: "Filter",            hue: 188, category: "Filters" },
    delay:            { label: "Delay",             hue: 35,  category: "Time/Space" },
    reverb:           { label: "Reverb",            hue: 260, category: "Time/Space" },
    widener:          { label: "Stereo Widener",    hue: 286, category: "Spatial" },
    bass:             { label: "Bass Enhancer",     hue: 10,  category: "Spatial" },
    "pitch-time":     { label: "Pitch (Time)",      hue: 190, category: "Pitch" },
    "pitch-spectral": { label: "Pitch (Spectral)",  hue: 170, category: "Pitch" },
    "dyn-compressor": { label: "Compressor",        hue: 120, category: "Dynamics" },
    "dyn-limiter":    { label: "Limiter",           hue: 98,  category: "Dynamics" },
    "dyn-gate":       { label: "Gate/Expander",     hue: 82,  category: "Dynamics" },
    "dyn-expander":   { label: "Expander",          hue: 66,  category: "Dynamics" },
    "dyn-deesser":    { label: "De-Esser",          hue: 58,  category: "Dynamics" },
    "dyn-multiband":  { label: "Multiband Comp",    hue: 108, category: "Dynamics" },
    "split-freq":     { label: "Split Freq",        hue: 210, category: "Routing", utility: true },
    split:            { label: "Split",             hue: 210, category: "Routing", utility: true, hidden: true },
    sum:              { label: "Sum",               hue: 35,  category: "Routing", utility: true },
  };

  // ---- geometry constants ---------------------------------------------------
  const NODE_W   = 152;
  const NODE_H   = 52;
  const SUM_PORT_SPACING = 18;
  const SUM_PORT_PAD = 12;
  const PORT_R   = 7;
  const BYPASS_S = 14; // bypass-button square size
  const BYPASS_PAD = 6;
  const DRAG_THRESHOLD = 4;

  // ---- id generator ---------------------------------------------------------
  let _nextId = 1;
  function genId() { return "n" + (_nextId++); }

  // ---- roundRect polyfill ---------------------------------------------------
  // CanvasRenderingContext2D.roundRect() is unavailable in older browsers.
  if (!CanvasRenderingContext2D.prototype.roundRect) {
    CanvasRenderingContext2D.prototype.roundRect = function (x, y, w, h, r) {
      const radius = Math.min(r, w / 2, h / 2);
      this.moveTo(x + radius, y);
      this.lineTo(x + w - radius, y);
      this.arcTo(x + w, y, x + w, y + radius, radius);
      this.lineTo(x + w, y + h - radius);
      this.arcTo(x + w, y + h, x + w - radius, y + h, radius);
      this.lineTo(x + radius, y + h);
      this.arcTo(x, y + h, x, y + h - radius, radius);
      this.lineTo(x, y + radius);
      this.arcTo(x, y, x + radius, y, radius);
      this.closePath();
    };
  }

  // ---- helpers --------------------------------------------------------------
  function isDark() {
    return document.documentElement.dataset.resolvedTheme === "dark";
  }

  function cssVar(name) {
    return getComputedStyle(document.documentElement).getPropertyValue(name).trim();
  }

  function nodeColor(node, alpha) {
    if (node.type === "_input" || node.type === "_output") {
      const base = isDark() ? "180,200,220" : "80,90,100";
      return `rgba(${base},${alpha})`;
    }
    const def = FX_TYPES[node.type];
    const hue = def ? def.hue : 0;
    const sat = isDark() ? "45%" : "55%";
    const lit = isDark() ? "52%" : "42%";
    return `hsla(${hue},${sat},${lit},${alpha})`;
  }

  function nodeFill(node) {
    if (node.type === "_input" || node.type === "_output") {
      return isDark() ? "#283440" : "#e8edf2";
    }
    const def = FX_TYPES[node.type];
    const hue = def ? def.hue : 0;
    const sat = isDark() ? "20%" : "75%";
    const lit = isDark() ? "22%" : "94%";
    return `hsl(${hue},${sat},${lit})`;
  }

  // ---- EffectChain class ----------------------------------------------------
  class EffectChain {
    constructor(canvas, opts) {
      this.canvas = canvas;
      this.ctx    = canvas.getContext("2d");
      this.opts   = opts || {};

      // view transform (pan offset in CSS pixels)
      this.panX = 0;
      this.panY = 0;

      // data
      this.nodes       = [];
      this.connections  = []; // { from: id, to: id }
      this.selectedId   = null;

      // interaction state
      this._action      = null; // null | 'pan' | 'drag' | 'connect'
      this._startCX     = 0;
      this._startCY     = 0;
      this._dragNodeStartX = 0;
      this._dragNodeStartY = 0;
      this._moved        = false;
      this._connectFrom  = null; // node id when connecting
      this._connectEnd   = null; // {x,y} temp wire end (world coords)
      this._connectPort  = null; // 'output' | 'input'
      this._connectPortIndex = null;
      this._hoveredNode  = null;
      this._hoveredPort  = null; // { nodeId, port: 'input'|'output' }
      this._hoveredWire  = null; // { from, to }

      // context menu DOM
      this._menu = null;
      this._submenu = null;

      // add fixed Input and Output nodes
      this._addFixedNode("_input",  "Input",  60,  130);
      this._addFixedNode("_output", "Output", 560, 130);
      // default connection
      this.connections.push({ from: "_input", to: "_output" });

      this._bind();
      this.draw();
    }

    // ---- public API --------------------------------------------------------

    /** Add an effect node at world position (wx, wy). Returns node id. */
    addEffect(type, wx, wy) {
      const def = FX_TYPES[type];
      if (!def) return null;
      const id = genId();
      const instances = this.nodes.filter((n) => n.type === type).length;
      const label = instances > 0 ? `${def.label} ${instances + 1}` : def.label;
      const params = this.opts.createParams?.(type) || {};
      this.nodes.push({
        id, type, label,
        x: wx - NODE_W / 2, y: wy - NODE_H / 2,
        bypassed: false, fixed: false, params,
      });
      this._autoInsert(id);
      this._emitChange();
      this.draw();
      return id;
    }

    /** Remove an effect node. */
    removeNode(id) {
      const node = this._nodeById(id);
      if (!node || node.fixed) return;
      this._removeFromChain(id);
      this.nodes = this.nodes.filter((n) => n.id !== id);
      if (this.selectedId === id) {
        this.selectedId = null;
        this.opts.onSelect?.(null);
      }
      this._emitChange();
      this.draw();
    }

    /** Select a node (or null to deselect). */
    selectNode(id) {
      this.selectedId = id;
      this.opts.onSelect?.(id ? this._nodeById(id) : null);
      this.draw();
    }

    /** Returns selected node object or null. */
    getSelectedNode() {
      if (!this.selectedId) return null;
      return this._nodeById(this.selectedId);
    }

    /** Merge params into node and emit change. */
    updateNodeParams(nodeId, partial) {
      const node = this._nodeById(nodeId);
      if (!node || node.fixed) return false;
      node.params = { ...(node.params || {}), ...(partial || {}) };
      this._emitChange();
      this.draw();
      return true;
    }

    /** Returns a Set of effect type strings that are connected & not bypassed. */
    getEnabledEffects() {
      const enabled = new Set();
      const queue = ["_input"];
      const visited = new Set();
      while (queue.length > 0) {
        const cur = queue.shift();
        if (!cur || visited.has(cur)) continue;
        visited.add(cur);
        const node = this._nodeById(cur);
        if (
          node &&
          node.type !== "_input" &&
          node.type !== "_output" &&
          node.type !== "split" &&
          node.type !== "split-freq" &&
          node.type !== "sum" &&
          !node.bypassed
        ) {
          enabled.add(node.type);
        }
        for (const conn of this.connections) {
          if (conn.from === cur && !visited.has(conn.to)) {
            queue.push(conn.to);
          }
        }
      }
      return enabled;
    }

    /** Returns map of effect type -> bypass state for all effect nodes. */
    getNodeStates() {
      const states = {};
      for (const n of this.nodes) {
        if (n.type !== "_input" && n.type !== "_output") {
          states[n.type] = { bypassed: n.bypassed, connected: this._isInChain(n.id) };
        }
      }
      return states;
    }

    /** Serialisable state for persistence. */
    getState() {
      return {
        nodes: this.nodes.map((n) => ({
          id: n.id, type: n.type, label: n.label,
          x: n.x, y: n.y, bypassed: n.bypassed, fixed: n.fixed, params: n.params || {},
        })),
        connections: this.connections.map((c) => {
          const out = { from: c.from, to: c.to };
          if (Number.isInteger(c.fromPortIndex)) out.fromPortIndex = c.fromPortIndex;
          if (Number.isInteger(c.toPortIndex)) out.toPortIndex = c.toPortIndex;
          return out;
        }),
        panX: this.panX, panY: this.panY,
      };
    }

    /** Restore from serialised state. */
    setState(data) {
      if (!data) return;
      if (Array.isArray(data.nodes)) {
        this.nodes = data.nodes.map((n) => {
          const node = { ...n };
          if (!node.fixed && !node.params) {
            node.params = this.opts.createParams?.(node.type) || {};
          }
          return node;
        });
        // ensure _input and _output exist and are always marked fixed
        const inp = this._nodeById("_input");
        if (inp) { inp.fixed = true; inp.type = "_input"; }
        else this._addFixedNode("_input",  "Input",  60, 130);
        const out = this._nodeById("_output");
        if (out) { out.fixed = true; out.type = "_output"; }
        else this._addFixedNode("_output", "Output", 560, 130);
      }
      if (Array.isArray(data.connections)) {
        const nodeIds = new Set(this.nodes.map((n) => n.id));
        this.connections = data.connections
          .map((c) => {
            const out = { from: c.from, to: c.to };
            if (Number.isInteger(c.fromPortIndex) && c.fromPortIndex >= 0) {
              out.fromPortIndex = c.fromPortIndex;
            }
            if (Number.isInteger(c.toPortIndex) && c.toPortIndex >= 0) {
              out.toPortIndex = c.toPortIndex;
            }
            return out;
          })
          .filter((c) => nodeIds.has(c.from) && nodeIds.has(c.to) && c.from !== c.to);
      }
      if (typeof data.panX === "number") this.panX = data.panX;
      if (typeof data.panY === "number") this.panY = data.panY;
      // reset id counter above max
      for (const n of this.nodes) {
        const m = n.id.match(/^n(\d+)$/);
        if (m) _nextId = Math.max(_nextId, Number(m[1]) + 1);
      }
      this._normalizeSumPortIndexes();
      this.selectedId = null;
      this.draw();
    }

    /** Types already present in the chain. */
    usedTypes() {
      return new Set(this.nodes.filter((n) => !n.fixed).map((n) => n.type));
    }

    // ---- rendering ---------------------------------------------------------

    draw() {
      const canvas = this.canvas;
      const ctx    = this.ctx;
      const dpr    = window.devicePixelRatio || 1;
      const rect   = canvas.getBoundingClientRect();
      const w = rect.width;
      const h = rect.height;

      if (canvas.width !== Math.round(w * dpr) || canvas.height !== Math.round(h * dpr)) {
        canvas.width  = Math.round(w * dpr);
        canvas.height = Math.round(h * dpr);
      }
      ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
      ctx.clearRect(0, 0, w, h);

      this._drawGrid(w, h);

      ctx.save();
      ctx.translate(this.panX, this.panY);

      // wires
      for (const conn of this.connections) {
        this._drawWire(conn, conn === this._hoveredWire);
      }

      // temp wire while connecting
      if (this._action === "connect" && this._connectEnd) {
        this._drawTempWire();
      }

      // nodes
      for (const node of this.nodes) {
        this._drawNode(node);
      }

      ctx.restore();
    }

    _drawGrid(w, h) {
      const ctx = this.ctx;
      const step = 24;
      const offX = ((this.panX % step) + step) % step;
      const offY = ((this.panY % step) + step) % step;
      ctx.strokeStyle = isDark() ? "rgba(255,255,255,0.04)" : "rgba(0,0,0,0.05)";
      ctx.lineWidth = 1;
      ctx.beginPath();
      for (let x = offX; x < w; x += step) {
        ctx.moveTo(Math.round(x) + 0.5, 0);
        ctx.lineTo(Math.round(x) + 0.5, h);
      }
      for (let y = offY; y < h; y += step) {
        ctx.moveTo(0, Math.round(y) + 0.5);
        ctx.lineTo(w, Math.round(y) + 0.5);
      }
      ctx.stroke();
    }

    _drawWire(conn, highlighted = false) {
      const fromNode = this._nodeById(conn.from);
      const toNode   = this._nodeById(conn.to);
      if (!fromNode || !toNode) return;

      const x1 = fromNode.x + NODE_W;
      const y1 = this._outputPortY(fromNode, conn.fromPortIndex);
      const x2 = toNode.x;
      const y2 = this._inputPortY(toNode, conn.toPortIndex);

      const ctx = this.ctx;
      const dx = Math.max(Math.abs(x2 - x1) * 0.45, 40);

      ctx.beginPath();
      ctx.moveTo(x1, y1);
      ctx.bezierCurveTo(x1 + dx, y1, x2 - dx, y2, x2, y2);
      if (highlighted) {
        ctx.strokeStyle = isDark() ? "rgba(200,220,245,0.85)" : "rgba(52,72,92,0.75)";
        ctx.lineWidth = 3.5;
      } else {
        ctx.strokeStyle = isDark() ? "rgba(140,170,200,0.55)" : "rgba(60,80,100,0.4)";
        ctx.lineWidth = 2.5;
      }
      ctx.stroke();
    }

    _drawTempWire() {
      if (!this._connectFrom || !this._connectEnd) return;
      const node = this._nodeById(this._connectFrom);
      if (!node) return;

      let x1, y1;
      if (this._connectPort === "output") {
        x1 = node.x + NODE_W;
        y1 = this._outputPortY(node, this._connectPortIndex);
      } else {
        x1 = node.x;
        y1 = this._inputPortY(node, this._connectPortIndex);
      }
      const x2 = this._connectEnd.x;
      const y2 = this._connectEnd.y;

      const ctx = this.ctx;
      const dx = Math.max(Math.abs(x2 - x1) * 0.45, 30);

      ctx.beginPath();
      if (this._connectPort === "output") {
        ctx.moveTo(x1, y1);
        ctx.bezierCurveTo(x1 + dx, y1, x2 - dx, y2, x2, y2);
      } else {
        ctx.moveTo(x1, y1);
        ctx.bezierCurveTo(x1 - dx, y1, x2 + dx, y2, x2, y2);
      }
      ctx.strokeStyle = isDark() ? "rgba(140,170,200,0.35)" : "rgba(60,80,100,0.25)";
      ctx.lineWidth = 2;
      ctx.setLineDash([6, 4]);
      ctx.stroke();
      ctx.setLineDash([]);
    }

    _drawNode(node) {
      const ctx = this.ctx;
      const x = node.x;
      const y = node.y;
      const h = this._nodeH(node);
      const selected = node.id === this.selectedId;
      const hovered  = node.id === this._hoveredNode;
      const bypassed = node.bypassed;
      const alpha    = bypassed ? 0.5 : 1;

      // shadow
      ctx.save();
      ctx.shadowColor = "rgba(0,0,0,0.12)";
      ctx.shadowBlur  = 8;
      ctx.shadowOffsetY = 2;

      // body
      const r = 10;
      ctx.beginPath();
      ctx.roundRect(x, y, NODE_W, h, r);
      ctx.fillStyle = nodeFill(node);
      ctx.globalAlpha = alpha;
      ctx.fill();
      ctx.restore();

      // border
      ctx.globalAlpha = alpha;
      ctx.beginPath();
      ctx.roundRect(x, y, NODE_W, h, r);
      if (selected) {
        ctx.strokeStyle = cssVar("--accent") || "#c24d2c";
        ctx.lineWidth = 2.5;
      } else if (hovered) {
        ctx.strokeStyle = nodeColor(node, 0.7);
        ctx.lineWidth = 1.8;
      } else {
        ctx.strokeStyle = isDark() ? "rgba(255,255,255,0.13)" : "rgba(0,0,0,0.12)";
        ctx.lineWidth = 1;
      }
      ctx.stroke();

      // label
      ctx.font = "600 13px 'IBM Plex Sans', 'Segoe UI', sans-serif";
      ctx.fillStyle = nodeColor(node, 1);
      ctx.textBaseline = "middle";
      const labelX = x + 14;
      const labelY = y + h / 2;
      ctx.fillText(node.label, labelX, labelY);

      // bypass button (only for effect nodes)
      if (!node.fixed && !FX_TYPES[node.type]?.utility) {
        this._drawBypassBtn(node);
      }

      // ports
      if (node.type !== "_input") {
        if (node.type === "sum") {
          const inputs = this._sumInputCount(node.id);
          for (let i = 0; i < inputs; i++) {
            const ipx = x;
            const ipy = this._inputPortY(node, i);
            const ihov = this._hoveredPort?.nodeId === node.id &&
              this._hoveredPort?.port === "input" &&
              this._hoveredPort?.portIndex === i;
            const connected = this._hasIncomingAtPort(node.id, i);
            const optional = i === 1 && !connected;
            this._drawPort(ipx, ipy, ihov, connected, optional);
          }
        } else {
          // input port (left)
          const ipx = x;
          const ipy = y + h / 2;
          const ihov = this._hoveredPort?.nodeId === node.id && this._hoveredPort?.port === "input";
          this._drawPort(ipx, ipy, ihov, this._hasIncoming(node.id));
        }
      }
      if (node.type !== "_output") {
        if (node.type === "split-freq") {
          for (let i = 0; i < 2; i++) {
            const opx = x + NODE_W;
            const opy = this._outputPortY(node, i);
            const ohov = this._hoveredPort?.nodeId === node.id &&
              this._hoveredPort?.port === "output" &&
              this._hoveredPort?.portIndex === i;
            this._drawPort(opx, opy, ohov, this._hasOutgoingAtPort(node.id, i));
          }
        } else {
          // output port (right)
          const opx = x + NODE_W;
          const opy = y + h / 2;
          const ohov = this._hoveredPort?.nodeId === node.id && this._hoveredPort?.port === "output";
          this._drawPort(opx, opy, ohov, this._hasOutgoing(node.id));
        }
      }

      ctx.globalAlpha = 1;
    }

    _drawBypassBtn(node) {
      const ctx = this.ctx;
      const bx = node.x + NODE_W - BYPASS_S - BYPASS_PAD;
      const by = node.y + (this._nodeH(node) - BYPASS_S) / 2;

      // button background
      ctx.beginPath();
      ctx.roundRect(bx, by, BYPASS_S, BYPASS_S, 4);
      if (node.bypassed) {
        ctx.fillStyle = isDark() ? "rgba(255,100,80,0.25)" : "rgba(200,60,40,0.15)";
      } else {
        ctx.fillStyle = isDark() ? "rgba(100,200,120,0.25)" : "rgba(40,160,60,0.15)";
      }
      ctx.fill();

      // power icon
      const cx = bx + BYPASS_S / 2;
      const cy = by + BYPASS_S / 2;
      const ir = 4;
      ctx.beginPath();
      ctx.arc(cx, cy, ir, -Math.PI * 0.7, Math.PI * 0.7, false);
      ctx.strokeStyle = node.bypassed
        ? (isDark() ? "rgba(255,120,100,0.7)" : "rgba(180,50,30,0.6)")
        : (isDark() ? "rgba(100,220,130,0.8)" : "rgba(40,140,60,0.7)");
      ctx.lineWidth = 1.5;
      ctx.stroke();
      ctx.beginPath();
      ctx.moveTo(cx, cy - ir);
      ctx.lineTo(cx, cy - ir + 3);
      ctx.stroke();
    }

    _drawPort(px, py, hovered, connected, optional = false) {
      const ctx = this.ctx;
      const r = hovered ? PORT_R + 1.5 : PORT_R;
      ctx.beginPath();
      ctx.arc(px, py, r, 0, Math.PI * 2);
      if (connected) {
        ctx.fillStyle = isDark() ? "rgba(140,180,220,0.7)" : "rgba(60,100,140,0.6)";
      } else if (optional) {
        ctx.fillStyle = isDark() ? "rgba(100,120,140,0.2)" : "rgba(120,140,160,0.18)";
      } else {
        ctx.fillStyle = isDark() ? "rgba(100,120,140,0.4)" : "rgba(120,140,160,0.35)";
      }
      ctx.fill();
      ctx.strokeStyle = isDark() ? "rgba(200,220,240,0.5)" : "rgba(255,255,255,0.8)";
      ctx.lineWidth = 1.5;
      ctx.stroke();
    }

    // ---- hit testing -------------------------------------------------------

    _hitNode(wx, wy) {
      // reverse order so topmost node wins
      for (let i = this.nodes.length - 1; i >= 0; i--) {
        const n = this.nodes[i];
        if (wx >= n.x && wx <= n.x + NODE_W && wy >= n.y && wy <= n.y + this._nodeH(n)) {
          return n;
        }
      }
      return null;
    }

    _hitBypass(wx, wy, node) {
      if (!node || node.fixed) return false;
      const bx = node.x + NODE_W - BYPASS_S - BYPASS_PAD;
      const by = node.y + (this._nodeH(node) - BYPASS_S) / 2;
      return wx >= bx && wx <= bx + BYPASS_S && wy >= by && wy <= by + BYPASS_S;
    }

    _hitPort(wx, wy) {
      const hitR = PORT_R + 6;
      for (let i = this.nodes.length - 1; i >= 0; i--) {
        const n = this.nodes[i];
        // input port
        if (n.type !== "_input") {
          if (n.type === "sum") {
            const inputs = this._sumInputCount(n.id);
            for (let i = 0; i < inputs; i++) {
              const ipx = n.x;
              const ipy = this._inputPortY(n, i);
              if (Math.hypot(wx - ipx, wy - ipy) <= hitR) {
                return { nodeId: n.id, port: "input", portIndex: i };
              }
            }
          } else {
            const ipx = n.x;
            const ipy = n.y + this._nodeH(n) / 2;
            if (Math.hypot(wx - ipx, wy - ipy) <= hitR) {
              return { nodeId: n.id, port: "input" };
            }
          }
        }
        // output port
        if (n.type !== "_output") {
          if (n.type === "split-freq") {
            for (let i = 0; i < 2; i++) {
              const opx = n.x + NODE_W;
              const opy = this._outputPortY(n, i);
              if (Math.hypot(wx - opx, wy - opy) <= hitR) {
                return { nodeId: n.id, port: "output", portIndex: i };
              }
            }
          } else {
            const opx = n.x + NODE_W;
            const opy = n.y + this._nodeH(n) / 2;
            if (Math.hypot(wx - opx, wy - opy) <= hitR) {
              return { nodeId: n.id, port: "output" };
            }
          }
        }
      }
      return null;
    }

    _hitWire(wx, wy) {
      const threshold = 8;
      for (let i = this.connections.length - 1; i >= 0; i--) {
        const conn = this.connections[i];
        const fromNode = this._nodeById(conn.from);
        const toNode = this._nodeById(conn.to);
        if (!fromNode || !toNode) continue;
        const x1 = fromNode.x + NODE_W;
        const y1 = this._outputPortY(fromNode, conn.fromPortIndex);
        const x2 = toNode.x;
        const y2 = this._inputPortY(toNode, conn.toPortIndex);
        const dx = Math.max(Math.abs(x2 - x1) * 0.45, 40);
        const c1x = x1 + dx;
        const c1y = y1;
        const c2x = x2 - dx;
        const c2y = y2;

        let prev = this._bezierPoint(x1, y1, c1x, c1y, c2x, c2y, x2, y2, 0);
        for (let s = 1; s <= 24; s++) {
          const t = s / 24;
          const cur = this._bezierPoint(x1, y1, c1x, c1y, c2x, c2y, x2, y2, t);
          if (this._distPointToSegment(wx, wy, prev.x, prev.y, cur.x, cur.y) <= threshold) {
            return conn;
          }
          prev = cur;
        }
      }
      return null;
    }

    _bezierPoint(x1, y1, c1x, c1y, c2x, c2y, x2, y2, t) {
      const u = 1 - t;
      const tt = t * t;
      const uu = u * u;
      const uuu = uu * u;
      const ttt = tt * t;
      return {
        x: uuu * x1 + 3 * uu * t * c1x + 3 * u * tt * c2x + ttt * x2,
        y: uuu * y1 + 3 * uu * t * c1y + 3 * u * tt * c2y + ttt * y2,
      };
    }

    _distPointToSegment(px, py, x1, y1, x2, y2) {
      const vx = x2 - x1;
      const vy = y2 - y1;
      const wx = px - x1;
      const wy = py - y1;
      const vv = vx * vx + vy * vy;
      if (vv <= 1e-9) return Math.hypot(px - x1, py - y1);
      let t = (wx * vx + wy * vy) / vv;
      if (t < 0) t = 0;
      if (t > 1) t = 1;
      const qx = x1 + t * vx;
      const qy = y1 + t * vy;
      return Math.hypot(px - qx, py - qy);
    }

    // ---- coordinate transforms ---------------------------------------------

    _toWorld(cx, cy) {
      return { x: cx - this.panX, y: cy - this.panY };
    }

    _mousePos(e) {
      const rect = this.canvas.getBoundingClientRect();
      return { cx: e.clientX - rect.left, cy: e.clientY - rect.top };
    }

    // ---- event binding -----------------------------------------------------

    _bind() {
      this.canvas.addEventListener("mousedown",   (e) => this._onMouseDown(e));
      this.canvas.addEventListener("mousemove",    (e) => this._onMouseMove(e));
      this.canvas.addEventListener("mouseup",      (e) => this._onMouseUp(e));
      this.canvas.addEventListener("dblclick",     (e) => this._onDoubleClick(e));
      document.addEventListener("mouseup",         (e) => {
        if (this._action && e.target !== this.canvas) this._onMouseUp(e);
      });
      this.canvas.addEventListener("contextmenu",  (e) => e.preventDefault());
      this.canvas.addEventListener("mouseleave",   ()  => this._onMouseLeave());
      document.addEventListener("keydown", (e) => this._onKeyDown(e));

      // close context menu on outside click
      document.addEventListener("mousedown", (e) => {
        const inMainMenu = this._menu && this._menu.contains(e.target);
        const inSubmenu = this._submenu && this._submenu.contains(e.target);
        if ((this._menu || this._submenu) && !inMainMenu && !inSubmenu) {
          this._hideMenu();
        }
      });
    }

    _onMouseDown(e) {
      this._hideMenu();

      const { cx, cy } = this._mousePos(e);
      const { x: wx, y: wy } = this._toWorld(cx, cy);

      this._startCX = cx;
      this._startCY = cy;
      this._moved = false;

      if (e.button === 2) {
        // right button: pan
        this._action = "pan";
        this._panStartX = this.panX;
        this._panStartY = this.panY;
        return;
      }

      if (e.button === 0) {
        // left button: check port first, then node, then deselect
        const port = this._hitPort(wx, wy);
        if (port) {
          this._action = "connect";
          this._connectFrom = port.nodeId;
          this._connectPort = port.port;
          this._connectPortIndex = port.portIndex;
          this._connectEnd = { x: wx, y: wy };
          return;
        }

        const node = this._hitNode(wx, wy);
        if (node) {
          // check bypass button
          if (this._hitBypass(wx, wy, node)) {
            node.bypassed = !node.bypassed;
            this._emitChange();
            this.draw();
            return;
          }
          this._action = "drag";
          this._dragNodeId = node.id;
          this._dragNodeStartX = node.x;
          this._dragNodeStartY = node.y;
          return;
        }

        // clicked empty space: deselect
        this.selectNode(null);
      }
    }

    _onMouseMove(e) {
      const { cx, cy } = this._mousePos(e);
      const { x: wx, y: wy } = this._toWorld(cx, cy);
      const dx = cx - this._startCX;
      const dy = cy - this._startCY;

      if (!this._moved && Math.hypot(dx, dy) > DRAG_THRESHOLD) {
        this._moved = true;
      }

      if (this._action === "pan") {
        this.panX = this._panStartX + dx;
        this.panY = this._panStartY + dy;
        this.draw();
        return;
      }

      if (this._action === "drag" && this._moved) {
        const node = this._nodeById(this._dragNodeId);
        if (node) {
          node.x = this._dragNodeStartX + dx;
          node.y = this._dragNodeStartY + dy;
          this.draw();
        }
        return;
      }

      if (this._action === "connect") {
        this._connectEnd = { x: wx, y: wy };
        this.draw();
        return;
      }

      // hover detection
      const port = this._hitPort(wx, wy);
      const node = this._hitNode(wx, wy);
      const wire = (!port && !node) ? this._hitWire(wx, wy) : null;
      const prevHovNode = this._hoveredNode;
      const prevHovPort = this._hoveredPort;
      const prevHovWire = this._hoveredWire;
      this._hoveredNode = node ? node.id : null;
      this._hoveredPort = port;
      this._hoveredWire = wire;
      if (this._hoveredNode !== prevHovNode || this._hoveredPort !== prevHovPort || this._hoveredWire !== prevHovWire) {
        this.canvas.style.cursor = port ? "crosshair" : (node ? "grab" : (wire ? "pointer" : "default"));
        this.draw();
      }
    }

    _onMouseUp(e) {
      const { cx, cy } = this._mousePos(e);
      const { x: wx, y: wy } = this._toWorld(cx, cy);
      const action = this._action;
      this._action = null;

      if (e.button === 2) {
        // right button release
        if (!this._moved) {
          // no drag: show context menu
          const node = this._hitNode(wx, wy);
          const wire = node ? null : this._hitWire(wx, wy);
          this._showMenu(e.clientX, e.clientY, node, wire);
        }
        return;
      }

      if (e.button === 0) {
        if (action === "connect") {
          // try to complete connection
          const port = this._hitPort(wx, wy);
          if (port && port.nodeId !== this._connectFrom) {
            this._createConnection(
              this._connectFrom,
              this._connectPort,
              port.nodeId,
              port.port,
              this._connectPortIndex,
              port.portIndex,
            );
          }
          this._connectFrom = null;
          this._connectEnd  = null;
          this._connectPort = null;
          this._connectPortIndex = null;
          this.draw();
          return;
        }

        if (action === "drag" && !this._moved) {
          // click without drag: select node and open detail
          const node = this._nodeById(this._dragNodeId);
          if (node) {
            this.selectNode(node.id);
          }
          return;
        }
      }
    }

    _onMouseLeave() {
      this._hoveredNode = null;
      this._hoveredPort = null;
      this._hoveredWire = null;
      this.canvas.style.cursor = "default";
      if (!this._action) this.draw();
    }

    _onDoubleClick(e) {
      if (e.button !== 0) return;
      const { cx, cy } = this._mousePos(e);
      const { x: wx, y: wy } = this._toWorld(cx, cy);

      const node = this._hitNode(wx, wy);
      if (node && !node.fixed) {
        this.removeNode(node.id);
        this._hideMenu();
        return;
      }

      const wire = this._hitWire(wx, wy);
      if (wire) {
        this.connections = this.connections.filter((c) => c !== wire);
        this._normalizeSumPortIndexes();
        this._emitChange();
        this._hideMenu();
        this.draw();
      }
    }

    _onKeyDown(e) {
      if (!this.selectedId) return;
      if (e.key === "Delete" || e.key === "Backspace") {
        const node = this._nodeById(this.selectedId);
        if (node && !node.fixed) {
          // prevent browser back-navigation
          e.preventDefault();
          this.removeNode(this.selectedId);
        }
      }
    }

    // ---- connection management ---------------------------------------------

    _createConnection(fromId, fromPort, toId, toPort, fromPortIndex = null, toPortIndex = null) {
      // normalise direction: always from output to input
      let srcId, dstId;
      let srcPortIndex = null;
      let dstPortIndex = null;
      if (fromPort === "output" && toPort === "input") {
        srcId = fromId; dstId = toId;
        srcPortIndex = fromPortIndex;
        dstPortIndex = toPortIndex;
      } else if (fromPort === "input" && toPort === "output") {
        srcId = toId; dstId = fromId;
        srcPortIndex = toPortIndex;
        dstPortIndex = fromPortIndex;
      } else {
        return; // same-type ports
      }
      // prevent self-connection
      if (srcId === dstId) return;
      // prevent _output as source or _input as destination
      const srcNode = this._nodeById(srcId);
      const dstNode = this._nodeById(dstId);
      if (!srcNode || !dstNode) return;
      if (srcNode.type === "_output" || dstNode.type === "_input") return;
      // prevent cycles: reject if dstId can already reach srcId
      if (this._canReach(dstId, srcId)) return;

      const srcOutLimit = this._outgoingLimit(srcNode.type);
      const dstInLimit = this._incomingLimit(dstNode.type);

      this.connections = this.connections.filter((c) => {
        if (c.from === srcId && srcOutLimit === 1) return false;
        if (dstNode.type === "sum" && c.to === dstId) {
          if (typeof dstPortIndex === "number" && c.toPortIndex === dstPortIndex) return false;
          return true;
        }
        if (c.to === dstId && dstInLimit === 1) return false;
        return true;
      });
      const newConn = { from: srcId, to: dstId };
      if (srcNode.type === "split-freq") {
        newConn.fromPortIndex = Number.isInteger(srcPortIndex)
          ? srcPortIndex
          : 0;
      }
      if (dstNode.type === "sum") {
        const preferredIndex = Number.isInteger(dstPortIndex) ? dstPortIndex : this._firstFreeSumPort(dstId);
        newConn.toPortIndex = preferredIndex;
      }
      this.connections.push(newConn);
      this._normalizeSumPortIndexes();
      this._emitChange();
    }

    _hasIncoming(nodeId) {
      return this.connections.some((c) => c.to === nodeId);
    }

    _hasOutgoing(nodeId) {
      return this.connections.some((c) => c.from === nodeId);
    }

    _canReach(fromId, targetId) {
      const visited = new Set();
      const queue = [fromId];
      while (queue.length) {
        const cur = queue.shift();
        if (cur === targetId) return true;
        if (visited.has(cur)) continue;
        visited.add(cur);
        for (const c of this.connections) {
          if (c.from === cur) queue.push(c.to);
        }
      }
      return false;
    }

    _isInChain(nodeId) {
      const queue = ["_input"];
      const visited = new Set();
      while (queue.length > 0) {
        const cur = queue.shift();
        if (!cur || visited.has(cur)) continue;
        if (cur === nodeId) return true;
        visited.add(cur);
        for (const conn of this.connections) {
          if (conn.from === cur && !visited.has(conn.to)) queue.push(conn.to);
        }
      }
      return false;
    }

    _autoInsert(nodeId) {
      // insert the new node into the chain based on its x position
      const node = this._nodeById(nodeId);
      if (!node) return;
      const nx = node.x + NODE_W / 2;

      // find the connection where we should insert
      // walk the chain and find adjacent pair whose x straddles nx
      let bestConn = null;
      for (const conn of this.connections) {
        const fromNode = this._nodeById(conn.from);
        const toNode   = this._nodeById(conn.to);
        if (!fromNode || !toNode) continue;
        const fromX = fromNode.x + NODE_W / 2;
        const toX   = toNode.x + NODE_W / 2;
        // insert between the pair that most closely straddles this node
        if (fromX <= nx && toX >= nx) {
          bestConn = conn;
          break;
        }
      }
      // fallback: insert before _output
      if (!bestConn) {
        bestConn = this.connections.find((c) => c.to === "_output");
      }
      if (!bestConn) {
        // last resort: find any connection from _input
        bestConn = this.connections.find((c) => c.from === "_input");
      }
      if (bestConn) {
        const oldTo = bestConn.to;
        const oldFromPortIndex = bestConn.fromPortIndex;
        const oldToPortIndex = bestConn.toPortIndex;
        bestConn.to = nodeId;
        if (Number.isInteger(oldFromPortIndex)) bestConn.fromPortIndex = oldFromPortIndex;
        if (Number.isInteger(oldToPortIndex)) delete bestConn.toPortIndex;
        const bridge = { from: nodeId, to: oldTo };
        if (Number.isInteger(oldToPortIndex)) bridge.toPortIndex = oldToPortIndex;
        this.connections.push(bridge);
      } else {
        // no chain yet, connect input -> node -> output
        this.connections.push({ from: "_input", to: nodeId });
        this.connections.push({ from: nodeId, to: "_output" });
      }
      this._normalizeSumPortIndexes();
    }

    _removeFromChain(nodeId) {
      const incoming = this.connections.filter((c) => c.to === nodeId);
      const outgoing = this.connections.filter((c) => c.from === nodeId);

      // reconnect neighbours
      if (incoming.length === 1 && outgoing.length === 1) {
        incoming[0].to = outgoing[0].to;
      }
      // remove all connections involving this node
      this.connections = this.connections.filter(
        (c) => c.from !== nodeId && c.to !== nodeId
      );
      this._normalizeSumPortIndexes();
    }

    // ---- context menu ------------------------------------------------------

    _showMenu(clientX, clientY, nodeUnderCursor, wireUnderCursor) {
      this._hideMenu();
      const menu = document.createElement("div");
      menu.className = "chain-context-menu";

      if (nodeUnderCursor && !nodeUnderCursor.fixed) {
        // node context menu
        const bypassItem = document.createElement("button");
        bypassItem.className = "chain-menu-item";
        bypassItem.textContent = nodeUnderCursor.bypassed ? "Enable" : "Bypass";
        bypassItem.addEventListener("click", () => {
          nodeUnderCursor.bypassed = !nodeUnderCursor.bypassed;
          this._emitChange();
          this._hideMenu();
          this.draw();
        });
        menu.appendChild(bypassItem);

        const removeItem = document.createElement("button");
        removeItem.className = "chain-menu-item chain-menu-item--danger";
        removeItem.textContent = "Remove";
        removeItem.addEventListener("click", () => {
          this.removeNode(nodeUnderCursor.id);
          this._hideMenu();
        });
        menu.appendChild(removeItem);
      } else if (wireUnderCursor) {
        const removeConnItem = document.createElement("button");
        removeConnItem.className = "chain-menu-item chain-menu-item--danger";
        removeConnItem.textContent = "Remove Connection";
        removeConnItem.addEventListener("click", () => {
          this.connections = this.connections.filter((c) => c !== wireUnderCursor);
          this._normalizeSumPortIndexes();
          this._emitChange();
          this._hideMenu();
          this.draw();
        });
        menu.appendChild(removeConnItem);

        const insertTitle = document.createElement("div");
        insertTitle.className = "chain-menu-title";
        insertTitle.textContent = "Insert Block";
        menu.appendChild(insertTitle);

        const grouped = new Map();
        for (const [type, def] of Object.entries(FX_TYPES)) {
          if (def.hidden) continue;
          const category = def.category || "Other";
          if (!grouped.has(category)) grouped.set(category, []);
          grouped.get(category).push([type, def]);
        }
        const categoryOrder = ["Filters", "Dynamics", "Modulation", "Time/Space", "Pitch", "Spatial", "Color", "Routing", "Other"];
        const hideSubmenu = () => {
          if (this._submenu) {
            this._submenu.remove();
            this._submenu = null;
          }
        };
        const placeSubmenu = (submenu, anchor) => {
          const ar = anchor.getBoundingClientRect();
          const sr = submenu.getBoundingClientRect();
          let left = ar.right + 6;
          let top = ar.top - 4;
          if (left + sr.width > window.innerWidth - 8) left = ar.left - sr.width - 6;
          if (left < 8) left = 8;
          if (top + sr.height > window.innerHeight - 8) top = window.innerHeight - sr.height - 8;
          if (top < 8) top = 8;
          submenu.style.left = left + "px";
          submenu.style.top = top + "px";
        };
        const showTypesFlyout = (category, anchor) => {
          hideSubmenu();
          const submenu = document.createElement("div");
          submenu.className = "chain-context-menu chain-context-submenu";
          const title2 = document.createElement("div");
          title2.className = "chain-menu-title";
          title2.textContent = category;
          submenu.appendChild(title2);
          const entries = grouped.get(category) || [];
          for (const [type, def] of entries) {
            const item = document.createElement("button");
            item.className = "chain-menu-item";
            item.textContent = def.label;
            item.addEventListener("click", () => {
              const { x: wx, y: wy } = this._toWorld(
                clientX - this.canvas.getBoundingClientRect().left,
                clientY - this.canvas.getBoundingClientRect().top,
              );
              const defNode = FX_TYPES[type];
              if (!defNode) return;
              const newId = genId();
              const instances = this.nodes.filter((n) => n.type === type).length;
              const label = instances > 0 ? `${defNode.label} ${instances + 1}` : defNode.label;
              const params = this.opts.createParams?.(type) || {};
              this.nodes.push({
                id: newId, type, label,
                x: wx - NODE_W / 2, y: wy - NODE_H / 2,
                bypassed: false, fixed: false, params,
              });
              this.connections = this.connections.filter((c) => c !== wireUnderCursor);
              this.connections.push({
                from: wireUnderCursor.from,
                to: newId,
                fromPortIndex: wireUnderCursor.fromPortIndex,
              });
              this.connections.push({
                from: newId,
                to: wireUnderCursor.to,
                toPortIndex: wireUnderCursor.toPortIndex,
              });
              this._normalizeSumPortIndexes();
              this._emitChange();
              this._hideMenu();
              this.draw();
            });
            submenu.appendChild(item);
          }
          document.body.appendChild(submenu);
          placeSubmenu(submenu, anchor);
          this._submenu = submenu;
        };
        for (const category of categoryOrder) {
          const entries = grouped.get(category);
          if (!entries || entries.length === 0) continue;
          const item = document.createElement("button");
          item.className = "chain-menu-item chain-menu-item--submenu";
          item.textContent = `${category} (${entries.length})`;
          item.addEventListener("mouseenter", () => showTypesFlyout(category, item));
          item.addEventListener("click", () => showTypesFlyout(category, item));
          menu.appendChild(item);
        }
      } else {
        // empty-space context menu: add effects
        const title = document.createElement("div");
        title.className = "chain-menu-title";
        title.textContent = "Add Effect";
        menu.appendChild(title);

        const grouped = new Map();
        for (const [type, def] of Object.entries(FX_TYPES)) {
          if (def.hidden) continue;
          const category = def.category || "Other";
          if (!grouped.has(category)) grouped.set(category, []);
          grouped.get(category).push([type, def]);
        }
        const categoryOrder = ["Filters", "Dynamics", "Modulation", "Time/Space", "Pitch", "Spatial", "Color", "Routing", "Other"];
        const hideSubmenu = () => {
          if (this._submenu) {
            this._submenu.remove();
            this._submenu = null;
          }
        };
        const placeSubmenu = (submenu, anchor) => {
          const ar = anchor.getBoundingClientRect();
          const sr = submenu.getBoundingClientRect();
          let left = ar.right + 6;
          let top = ar.top - 4;
          if (left + sr.width > window.innerWidth - 8) {
            left = ar.left - sr.width - 6;
          }
          if (left < 8) left = 8;
          if (top + sr.height > window.innerHeight - 8) {
            top = window.innerHeight - sr.height - 8;
          }
          if (top < 8) top = 8;
          submenu.style.left = left + "px";
          submenu.style.top = top + "px";
        };
        const showTypesFlyout = (category, anchor) => {
          hideSubmenu();
          const submenu = document.createElement("div");
          submenu.className = "chain-context-menu chain-context-submenu";

          const title2 = document.createElement("div");
          title2.className = "chain-menu-title";
          title2.textContent = category;
          submenu.appendChild(title2);

          const entries = grouped.get(category) || [];
          for (const [type, def] of entries) {
            const item = document.createElement("button");
            item.className = "chain-menu-item";
            item.textContent = def.label;
            item.addEventListener("click", () => {
              const { x: wx, y: wy } = this._toWorld(
                clientX - this.canvas.getBoundingClientRect().left,
                clientY - this.canvas.getBoundingClientRect().top,
              );
              this.addEffect(type, wx, wy);
              this._hideMenu();
            });
            submenu.appendChild(item);
          }

          document.body.appendChild(submenu);
          placeSubmenu(submenu, anchor);
          this._submenu = submenu;
        };

        for (const category of categoryOrder) {
          const entries = grouped.get(category);
          if (!entries || entries.length === 0) continue;
          const item = document.createElement("button");
          item.className = "chain-menu-item chain-menu-item--submenu";
          item.textContent = `${category} (${entries.length})`;
          item.addEventListener("mouseenter", () => showTypesFlyout(category, item));
          item.addEventListener("click", () => showTypesFlyout(category, item));
          menu.appendChild(item);
        }
      }

      // position
      menu.style.left = clientX + "px";
      menu.style.top  = clientY + "px";
      document.body.appendChild(menu);

      // keep within viewport
      requestAnimationFrame(() => {
        const mr = menu.getBoundingClientRect();
        if (mr.right > window.innerWidth)  menu.style.left = (window.innerWidth  - mr.width  - 8) + "px";
        if (mr.bottom > window.innerHeight) menu.style.top  = (window.innerHeight - mr.height - 8) + "px";
      });

      this._menu = menu;
    }

    _hideMenu() {
      if (this._submenu) {
        this._submenu.remove();
        this._submenu = null;
      }
      if (this._menu) {
        this._menu.remove();
        this._menu = null;
      }
    }

    // ---- helpers -----------------------------------------------------------

    _nodeById(id) {
      return this.nodes.find((n) => n.id === id) || null;
    }

    _addFixedNode(id, label, x, y) {
      this.nodes.push({ id, type: id, label, x, y, bypassed: false, fixed: true });
    }

    _incomingLimit(type) {
      if (type === "_input") return 0;
      if (type === "_output") return -1;
      return 1;
    }

    _outgoingLimit(type) {
      if (type === "_output") return 0;
      return -1;
    }

    _nodeH(node) {
      if (!node || node.type !== "sum") return NODE_H;
      const inputs = this._sumInputCount(node.id);
      const needed = SUM_PORT_PAD * 2 + (inputs - 1) * SUM_PORT_SPACING;
      return Math.max(NODE_H, needed);
    }

    _sumInputCount(nodeId) {
      const connected = this.connections.reduce((n, c) => n + (c.to === nodeId ? 1 : 0), 0);
      return Math.max(2, connected + 1);
    }

    _sumInputYs(node) {
      const count = this._sumInputCount(node.id);
      const h = this._nodeH(node);
      const startY = node.y + (h - (count - 1) * SUM_PORT_SPACING) / 2;
      const out = [];
      for (let i = 0; i < count; i++) out.push(startY + i * SUM_PORT_SPACING);
      return out;
    }

    _inputPortY(node, portIndex) {
      if (!node) return 0;
      if (node.type !== "sum") return node.y + this._nodeH(node) / 2;
      const ys = this._sumInputYs(node);
      if (ys.length === 0) return node.y + this._nodeH(node) / 2;
      if (!Number.isInteger(portIndex) || portIndex < 0) return ys[0];
      return ys[Math.min(portIndex, ys.length - 1)];
    }

    _hasIncomingAtPort(nodeId, portIndex) {
      return this.connections.some((c) => c.to === nodeId && c.toPortIndex === portIndex);
    }

    _firstFreeSumPort(nodeId) {
      const used = new Set(
        this.connections
          .filter((c) => c.to === nodeId && Number.isInteger(c.toPortIndex))
          .map((c) => c.toPortIndex),
      );
      let i = 0;
      while (used.has(i)) i++;
      return i;
    }

    _hasOutgoingAtPort(nodeId, portIndex) {
      return this.connections.some((c) => c.from === nodeId && c.fromPortIndex === portIndex);
    }

    _outputPortY(node, portIndex) {
      if (!node) return 0;
      if (node.type !== "split-freq") return node.y + this._nodeH(node) / 2;
      const h = this._nodeH(node);
      const mid = node.y + h / 2;
      const offset = SUM_PORT_SPACING / 2;
      return portIndex === 1 ? (mid + offset) : (mid - offset);
    }

    _normalizeSumPortIndexes() {
      const sumIds = new Set(this.nodes.filter((n) => n.type === "sum").map((n) => n.id));
      for (const sumId of sumIds) {
        const incoming = this.connections
          .map((c, idx) => ({ c, idx }))
          .filter((v) => v.c.to === sumId);
        incoming.sort((a, b) => {
          const ai = Number.isInteger(a.c.toPortIndex) ? a.c.toPortIndex : Number.MAX_SAFE_INTEGER;
          const bi = Number.isInteger(b.c.toPortIndex) ? b.c.toPortIndex : Number.MAX_SAFE_INTEGER;
          if (ai !== bi) return ai - bi;
          if (a.c.from !== b.c.from) return String(a.c.from).localeCompare(String(b.c.from));
          return a.idx - b.idx;
        });
        for (let i = 0; i < incoming.length; i++) {
          incoming[i].c.toPortIndex = i;
        }
      }
    }

    _emitChange() {
      this.opts.onChange?.();
    }
  }

  window.EffectChain = EffectChain;
})();
