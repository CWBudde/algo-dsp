# Web Demo

This folder contains a static GitHub Pages demo:

- 16-step pure-tone sequencer with exponential decay envelope
- Realtime 3-band EQ (low shelf, mid peak, high shelf)
- Chart.js magnitude response display from Web Audio filter responses

## Local run

```bash
python3 -m http.server 8080 -d web
```

Open <http://localhost:8080>.

## GitHub Pages

This repository deploys `web/` automatically via `.github/workflows/pages.yml` to GitHub Pages (`gh-pages` environment).
