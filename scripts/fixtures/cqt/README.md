# CQT and firwin2 golden vectors

`gen.py` writes the reference data for `dsp/cqt` and `design.Firwin2`:

- `dsp/filter/design/testdata/firwin2.json.gz`: `scipy.signal.firwin2` taps.
- `dsp/cqt/testdata/signal.json.gz`: the shared input, a deterministic chirp
  plus two tones plus noise of 43844 samples (basic-pitch's window length),
  rounded to float32.
- `dsp/cqt/testdata/<case>.json.gz`: nnAudio `CQT2010v2` kernels, filters and
  transforms, one file per configuration. `f64` is nnAudio's code run in
  float64; `f32`, where present, is nnAudio exactly as shipped (the numbers
  basic-pitch computes).

The Go tests only read the committed files and never run Python. To
regenerate, install [uv](https://docs.astral.sh/uv/) and run:

```bash
cd scripts/fixtures/cqt
uv run python gen.py
```

All dependencies are pinned in `pyproject.toml` and `uv.lock` (nnAudio 0.3.4,
torch 2.14.1, scipy 1.18.1, librosa 1.0.0, numpy 2.5.3). The archives are
written with a zero mtime, so a regeneration with the same versions on the
same platform is byte-identical.

Notes on the reference:

- nnAudio's `CQT2010v2` never passes its `window` argument to
  `create_cqt_kernels`, so its kernels are always Hann. The Hamming and
  Blackman cases rebuild the kernels with nnAudio's own `create_cqt_kernels`.
- For the `f64` runs, `gen.py` redirects nnAudio's `np.float32`/`np.complex64`
  casts to float64 and runs the module in double precision.
- The `librosa` array in `basic_pitch.json.gz` is `librosa.cqt` with the same
  parameters. librosa uses a different algorithm, so it is only a loose check
  of the overall level.
