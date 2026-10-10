"""Generate golden vectors for dsp/cqt and design.Firwin2.

Run from this directory with `uv run python gen.py`. The output goes to
dsp/cqt/testdata and dsp/filter/design/testdata as gzip-compressed JSON. The
Go tests only read those files; they never run Python.

References:

- firwin2: scipy.signal.firwin2, float64.
- CQT: nnAudio.features.cqt.CQT2010v2, which is the code spotify/basic-pitch
  ported to TensorFlow in basic_pitch/layers/nnaudio.py. Each transform case is
  run in float64 and, where marked, also as shipped:
  - "f32": nnAudio exactly as shipped (complex64 kernels, float32 low-pass,
    float32 torch arithmetic). This is what basic-pitch computes.
  - "f64": the same nnAudio code with its numpy float32/complex64 casts
    redirected to float64/complex128 and the module run in float64. This is
    the algorithm without float32 rounding and is the tight reference.
"""

from __future__ import annotations

import gzip
import json
import types
import warnings
from pathlib import Path

import librosa
import numpy as np
import scipy
import scipy.signal
import torch

import nnAudio
import nnAudio.utils as nn_utils
from nnAudio.features.cqt import CQT2010v2

ROOT = Path(__file__).resolve().parents[3]
CQT_OUT = ROOT / "dsp" / "cqt" / "testdata"
DESIGN_OUT = ROOT / "dsp" / "filter" / "design" / "testdata"

SIGNAL_LEN = 43844  # basic-pitch AUDIO_N_SAMPLES = 2 s * 22050 - 256
SIGNAL_SR = 22050


def versions() -> dict:
    return {
        "nnAudio": nnAudio.__version__,
        "torch": torch.__version__,
        "scipy": scipy.__version__,
        "librosa": librosa.__version__,
        "numpy": np.__version__,
    }


def write(path: Path, data: dict) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    raw = json.dumps(data, separators=(",", ":")).encode()
    # mtime=0 keeps the archive byte-identical across regenerations.
    with open(path, "wb") as fh, gzip.GzipFile(fileobj=fh, mode="wb", mtime=0, filename="") as gz:
        gz.write(raw)
    print(f"wrote {path.relative_to(ROOT)} ({path.stat().st_size} bytes)")


def flat(a) -> list:
    return [float(v) for v in np.asarray(a, dtype=np.float64).ravel()]


# ---------------------------------------------------------------------------
# firwin2


FIRWIN2_CASES = [
    # name, numtaps, freq, gain, kwargs
    ("nnaudio_lowpass", 256, [0.0, 0.5 / 1.001, 0.5 * 1.001, 1.0], [1.0, 1.0, 0.0, 0.0], {}),
    ("nnaudio_early_ds2", 256, [0.0, 0.5 / 1.03, 0.5 * 1.03, 1.0], [1.0, 1.0, 0.0, 0.0], {}),
    ("nnaudio_early_ds4", 256, [0.0, 0.25 / 1.03, 0.25 * 1.03, 1.0], [1.0, 1.0, 0.0, 0.0], {}),
    ("type1_lowpass", 101, [0.0, 0.2, 0.3, 1.0], [1.0, 1.0, 0.0, 0.0], {}),
    ("type1_highpass", 51, [0.0, 0.4, 0.5, 1.0], [0.0, 0.0, 1.0, 1.0], {}),
    ("type1_multiband", 63, [0.0, 0.1, 0.2, 0.35, 0.6, 0.8, 1.0], [0.5, 1.0, 0.25, 0.25, 2.0, 1.0, 0.75], {}),
    ("step_repeated_freq", 65, [0.0, 0.5, 0.5, 1.0], [1.0, 1.0, 0.0, 0.0], {}),
    ("type2_ramp", 6, [0.0, 0.5, 1.0], [1.0, 1.0, 0.0], {}),
    ("nfreqs_custom", 31, [0.0, 0.3, 0.4, 1.0], [1.0, 1.0, 0.0, 0.0], {"nfreqs": 100}),
    ("nfreqs_odd_len", 20, [0.0, 0.3, 0.4, 1.0], [1.0, 1.0, 0.0, 0.0], {"nfreqs": 33}),
    ("window_none", 41, [0.0, 0.3, 0.4, 1.0], [1.0, 1.0, 0.0, 0.0], {"window": None}),
    ("window_hann", 41, [0.0, 0.3, 0.4, 1.0], [1.0, 1.0, 0.0, 0.0], {"window": "hann"}),
    ("window_blackman", 40, [0.0, 0.3, 0.4, 1.0], [1.0, 1.0, 0.0, 0.0], {"window": "blackman"}),
    ("window_kaiser", 45, [0.0, 0.3, 0.4, 1.0], [1.0, 1.0, 0.0, 0.0], {"window": ("kaiser", 8.6)}),
    ("fs_hz", 51, [0.0, 1000.0, 1500.0, 4000.0], [1.0, 1.0, 0.0, 0.0], {"fs": 8000.0}),
    ("one_tap", 1, [0.0, 1.0], [1.0, 1.0], {}),
    ("two_taps", 2, [0.0, 1.0], [1.0, 0.0], {}),
]


def gen_firwin2() -> None:
    cases = []
    for name, numtaps, freq, gain, kwargs in FIRWIN2_CASES:
        taps = scipy.signal.firwin2(numtaps, freq, gain, **kwargs)
        case = {"name": name, "numtaps": numtaps, "freq": freq, "gain": gain, "taps": flat(taps)}
        if "nfreqs" in kwargs:
            case["nfreqs"] = kwargs["nfreqs"]
        if "fs" in kwargs:
            case["fs"] = kwargs["fs"]
        if "window" in kwargs:
            w = kwargs["window"]
            if w is None:
                case["window"] = "none"
            elif isinstance(w, tuple):
                case["window"] = w[0]
                case["window_param"] = w[1]
            else:
                case["window"] = w
        cases.append(case)
    write(DESIGN_OUT / "firwin2.json.gz", {"versions": versions(), "cases": cases})


# ---------------------------------------------------------------------------
# CQT


def make_signal() -> np.ndarray:
    """Deterministic chirp plus tones plus noise, rounded to float32.

    The float32 rounding makes the stored float64 values exactly what the
    float32 nnAudio run sees.
    """
    n = np.arange(SIGNAL_LEN, dtype=np.float64)
    t = n / SIGNAL_SR
    dur = SIGNAL_LEN / SIGNAL_SR
    f0, f1 = 20.0, 10000.0
    k = np.log(f1 / f0)
    chirp = np.sin(2 * np.pi * f0 * dur / k * (np.exp(k * t / dur) - 1.0))
    tone = 0.3 * np.sin(2 * np.pi * 440.0 * t) + 0.2 * np.sin(2 * np.pi * 55.0 * t)
    rng = np.random.default_rng(20261010)
    noise = 0.05 * rng.standard_normal(SIGNAL_LEN)
    x = 0.5 * chirp + tone + noise
    return x.astype(np.float32).astype(np.float64)


class _F64Numpy(types.ModuleType):
    """numpy proxy that turns nnAudio's float32/complex64 casts into float64."""

    def __init__(self):
        super().__init__("numpy_f64_proxy")
        self.float32 = np.float64
        self.complex64 = np.complex128

    def __getattr__(self, name):
        return getattr(np, name)


def build(cfg: dict, f64: bool) -> CQT2010v2:
    kwargs = dict(
        sr=cfg["sr"],
        hop_length=cfg["hop"],
        fmin=cfg["fmin"],
        n_bins=cfg["n_bins"],
        bins_per_octave=cfg["bins_per_octave"],
        filter_scale=cfg["filter_scale"],
        basis_norm=cfg["basis_norm"],
        pad_mode=cfg["pad_mode"],
        earlydownsample=cfg["earlydownsample"],
        verbose=False,
    )
    orig_np = nn_utils.np
    if f64:
        nn_utils.np = _F64Numpy()
    try:
        m = CQT2010v2(**kwargs)
        if cfg["window"] != "hann":
            # CQT2010v2 never forwards its `window` argument to
            # create_cqt_kernels, so its kernels are always Hann. For other
            # windows, rebuild the kernels with nnAudio's own kernel builder.
            q = float(cfg["filter_scale"]) / (2 ** (1 / cfg["bins_per_octave"]) - 1)
            sr_eff = cfg["sr"] / float(m.downsample_factor)
            basis, n_fft, _, _ = nn_utils.create_cqt_kernels(
                q,
                sr_eff,
                m.fmin_t,
                min(cfg["bins_per_octave"], cfg["n_bins"]),
                cfg["bins_per_octave"],
                norm=cfg["basis_norm"],
                window=cfg["window"],
                topbin_check=False,
            )
            assert n_fft == m.n_fft
            m.cqt_kernels_real = torch.tensor(basis.real).unsqueeze(1)
            m.cqt_kernels_imag = torch.tensor(basis.imag).unsqueeze(1)
            m.basis = basis
    finally:
        nn_utils.np = orig_np
    if f64:
        m = m.double()
    return m


def run(m: CQT2010v2, x: np.ndarray, cfg: dict, f64: bool):
    dtype = torch.float64 if f64 else torch.float32
    xt = torch.tensor(x, dtype=dtype)[None, :]
    fmt = "Complex" if cfg["output"] == "complex" else "Magnitude"
    with warnings.catch_warnings(record=True) as caught:
        warnings.simplefilter("always")
        with torch.no_grad():
            y = m(xt, output_format=fmt, normalization_type=cfg["normalization"])
    fallback = any("padding with reflection mode" in str(w.message) for w in caught)
    y = y[0].numpy().astype(np.float64)
    # nnAudio is (bins, frames[, 2]); the Go package is frame-major.
    y = np.swapaxes(y, 0, 1)
    return y, fallback


BASIC_PITCH = dict(
    sr=22050,
    hop=256,
    fmin=27.5,
    n_bins=309,
    bins_per_octave=36,
    filter_scale=1,
    basis_norm=1,
    window="hann",
    pad_mode="reflect",
    earlydownsample=True,
    normalization="librosa",
    output="magnitude",
)

NNAUDIO_DEFAULTS = dict(
    sr=22050,
    hop=512,
    fmin=32.70,
    n_bins=84,
    bins_per_octave=12,
    filter_scale=1,
    basis_norm=1,
    window="hann",
    pad_mode="reflect",
    earlydownsample=True,
    normalization="librosa",
    output="magnitude",
)

EARLY_DS8 = dict(NNAUDIO_DEFAULTS, hop=1024, fmin=30.0, n_bins=48, output="complex")

TRANSFORM_CASES = [
    # name, config, signal length, also run nnAudio as shipped (float32)
    # basic-pitch's front end; the f32 reference is what basic-pitch computes.
    ("basic_pitch", dict(BASIC_PITCH), SIGNAL_LEN, True),
    # Same configuration on a short signal: the lower octaves are shorter than
    # n_fft/2, so torch's reflection pad raises and nnAudio falls back to zeros.
    ("basic_pitch_short", dict(BASIC_PITCH), 4000, False),
    # nnAudio's defaults (early downsampling evaluates to a factor of 1).
    ("nnaudio_defaults", dict(NNAUDIO_DEFAULTS), SIGNAL_LEN, True),
    # Early downsampling by 8 with complex output.
    ("early_ds8_complex", dict(EARLY_DS8), SIGNAL_LEN, False),
    # Same configuration with early downsampling disabled.
    ("no_early_ds", dict(EARLY_DS8, earlydownsample=False), SIGNAL_LEN, False),
    # Constant padding, L2 kernels, convolutional normalization, a partial top
    # octave, non-unit filter scale and complex output.
    (
        "const_l2_conv_complex",
        dict(
            sr=16000,
            hop=128,
            fmin=50.0,
            n_bins=100,
            bins_per_octave=24,
            filter_scale=0.8,
            basis_norm=2,
            window="hann",
            pad_mode="constant",
            earlydownsample=True,
            normalization="convolutional",
            output="complex",
        ),
        20000,
        False,
    ),
    # Unnormalized kernels, "wrap" normalization and a Hamming window.
    (
        "hamming_nonorm_wrap",
        dict(
            sr=22050,
            hop=64,
            fmin=100.0,
            n_bins=60,
            bins_per_octave=12,
            filter_scale=1.5,
            basis_norm=0,
            window="hamming",
            pad_mode="reflect",
            earlydownsample=True,
            normalization="wrap",
            output="magnitude",
        ),
        10000,
        False,
    ),
    # Blackman window on part of basic-pitch's range.
    ("blackman", dict(BASIC_PITCH, window="blackman", n_bins=72, fmin=110.0), SIGNAL_LEN, False),
]


def gen_cqt() -> None:
    signal = make_signal()
    write(CQT_OUT / "signal.json.gz", {"versions": versions(), "sample_rate": SIGNAL_SR, "x": flat(signal)})

    for name, cfg, length, with_f32 in TRANSFORM_CASES:
        x = signal[:length]
        m64 = build(cfg, f64=True)
        y64, fallback = run(m64, x, cfg, f64=True)
        basis = np.asarray(m64.basis)
        data = {
            "versions": versions(),
            "config": cfg,
            "length": length,
            "n_fft": int(m64.n_fft),
            "n_octaves": int(m64.n_octaves),
            "downsample_factor": int(m64.downsample_factor),
            "hop_effective": int(m64.hop_length),
            "reflect_fallback": bool(fallback),
            "frequencies": flat(m64.frequencies),
            "lengths": flat(m64.lenghts.numpy()),
            "kernels_real": flat(basis.real),
            "kernels_imag": flat(basis.imag),
            "lowpass": flat(m64.lowpass_filter.numpy()),
            "frames": int(y64.shape[0]),
            "f64": flat(y64),
        }
        if getattr(m64, "early_downsample_filter", None) is not None:
            data["early_lowpass"] = flat(m64.early_downsample_filter.numpy())
        if with_f32:
            m32 = build(cfg, f64=False)
            y32, fallback32 = run(m32, x, cfg, f64=False)
            assert fallback32 == fallback
            data["f32"] = flat(y32)
            scale = np.max(np.abs(y64))
            print(f"  {name}: f32 vs f64 max abs diff / max = {np.max(np.abs(y32 - y64)) / scale:.3e}")
        if name == "basic_pitch":
            # librosa's own CQT (recursive downsampling with soxr, different
            # kernel placement) is a loose sanity check of the librosa
            # normalization, not a bit-level reference.
            lib = np.abs(
                librosa.cqt(
                    x,
                    sr=cfg["sr"],
                    hop_length=cfg["hop"],
                    fmin=cfg["fmin"],
                    n_bins=cfg["n_bins"],
                    bins_per_octave=cfg["bins_per_octave"],
                    filter_scale=cfg["filter_scale"],
                    norm=1,
                    window="hann",
                    pad_mode="reflect",
                )
            ).T
            data["librosa"] = flat(lib)
            rel = np.linalg.norm(lib - y64) / np.linalg.norm(lib)
            print(f"  {name}: librosa vs nnAudio relative Frobenius error = {rel:.3e}")
        print(
            f"  {name}: n_fft={data['n_fft']} octaves={data['n_octaves']} ds={data['downsample_factor']} "
            f"hop={data['hop_effective']} frames={data['frames']} fallback={fallback}"
        )
        write(CQT_OUT / f"{name}.json.gz", data)


if __name__ == "__main__":
    gen_firwin2()
    gen_cqt()
