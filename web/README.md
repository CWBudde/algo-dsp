# Web Demo

This folder contains a GitHub Pages demo where DSP runs in Go/WASM and WebAudio only outputs raw PCM:

- 16-step pure-tone sequencer with exponential decay envelope (Go engine)
- Realtime 5-node EQ (highpass, low shelf, peak, high shelf, lowpass) using `dsp/filter/*` packages
- Interactive canvas EQ graph with draggable band nodes (frequency/gain), response curve queried from Go

## Local run

```bash
./web/build-wasm.sh
python3 -m http.server 8080 -d web
```

Open <http://localhost:8080>.

Or run:

```bash
just web-demo
```

## GitHub Pages

This repository deploys `web/` automatically via `.github/workflows/pages.yml` to GitHub Pages (`gh-pages` environment).
