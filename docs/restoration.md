# Restoration and time stretch

`dsp/effects/restoration` owns the algorithms; callers own sources, undo and UI.
`SpectralProcessor.Step` reads one FFT window and emits contiguous borrowed
samples through normalized inverse STFT. Workspace is bounded, independent of
file duration. Capture and reduction use identical sample rate, FFT size, Hann
window and quarter-window hop. At least two captured noise-only frames are
required. Editor defaults: 2048-point FFT, Wiener filtering, 24 dB maximum
attenuation. Profiles are frozen before processing target audio.

The legacy lineage is `legacy/Source/DSP/DAV_DspSpectralNoiseReduction.pas`, used
by `Examples/Plugins/FFT Effects/{Noise Reduction,Spectral Noise Gate}`. Mean
powers and per-bin smoothed gains are retained. DC profile capture deliberately
corrects the legacy typo that averaged filter power instead of source power.
Normalized overlap-add replaces the legacy filter-IR/window/crossfade path; the
port is algorithmically compatible rather than bit-exact with Delphi FFT output.
Wiener mode adds decision-directed SNR estimation (Ephraim and Malah, 1984,
https://malah.net.technion.ac.il/files/2017/08/Ephraim_Speech_Enhancement_ASSP84.pdf).

Spectral masks use sample-frame and Hz coordinates. Heal fits an autoregressive
model to intact context and solves unknown residuals before replacing selected
inverse-STFT bins. The principle follows Janssen, Veldhuis and Vries (1986,
https://pure.tue.nl/ws/files/3077308/Metis235417.pdf). Healing is bounded to 256
samples and requires two-sided context; longer or edge gaps are rejected.
Singular or insufficient context falls back to cubic Hermite interpolation.
Automatic clicks use local second-difference outliers; declipping replaces only
bounded interior saturation runs. Long or edge saturation remains unchanged.
Preview is advisable on intentional transients and complex material.

`PitchShifter.TimeStretch` exposes existing WSOLA without pitch-shift resampling.
`NewStretchStream` bounds work/storage and shares alignment across every channel.
The duration ratio is [0.25,4], ratio 1 is exact, and output duration is rounded
once. WSOLA quality depends on material; transients and polyphonic audio can
have artefacts.

Reproduce with `go test -race -v ./dsp/effects/restoration ./dsp/effects/pitch`.
Reference click healing requires RMS residual below -80 dBFS and maximum error
below 1e-4. Default noise reduction requires >=15 dB on stationary Gaussian noise,
50 ms residual-power variation <=6 dB and no isolated line above 12 times its
neighbourhood mean. Wanted-tone amplitude is separately bounded. Noise/hum fast
paths have allocation benchmarks. These deterministic checks are perceptual
proxies; arbitrary material still needs listening evaluation.
