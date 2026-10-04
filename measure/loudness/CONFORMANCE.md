# Streaming loudness and true-peak conformance vectors

The optional `TestStreamingEBU*` tests use the published EBU loudness test set
(© EBU and the programme copyright holders). Audio fixtures are not distributed
with this repository. Obtain the set from
[EBU](https://tech.ebu.ch/publications/ebu_loudness_test_set), accept its terms,
and extract it outside the repository. Run:

```sh
ALGO_DSP_EBU_TEST_SET=/path/to/extracted/set go test ./measure/loudness -run EBU -count=1 -v
```

The verified archive contains 70 WAV files, with 66 applicable conformance
sequences and four calibration/reference noise files. Archive size: 91,631,421
bytes. SHA-512 (verified against Gentoo's package manifest):

```text
60d022fdac47ad0be2688411be9daecbff85da994d6fa4921bba6cffab841b081d8b15d9ce284ad2253efb686463450a84a0d19cb0bad7a934546cc52dd73771
```

Tests preserve the tolerances in
[EBU Tech 3341](https://tech.ebu.ch/docs/tech/tech3341.pdf) and
[EBU Tech 3342](https://tech.ebu.ch/docs/tech/tech3342.pdf). They cover momentary,
short-term and integrated readings, channel weighting, 20 shifted maxima for
both window lengths, successive maxima, four synthetic LRA sequences, two real
programmes and nine true-peak sequences. The true-peak filter uses the four
12-tap phases published in
[ITU-R BS.1770-5](https://www.itu.int/rec/R-REC-BS.1770-5-202311-I/en).

All 66 sequences passed on native Go and actual JavaScript/WebAssembly. The
narrow-range programme measured -22.986159 LUFS and 4.978217 LU LRA; the
wide-range programme measured -22.997820 LUFS and 15.011135 LU LRA. These
vector results do not constitute product certification or a claim that every
requirement of EBU Mode has been evaluated.

Streaming loudness samples window maxima every 10 ms and gates completed
400 ms/3 s energy blocks every 100 ms. LRA is marked unstable during the first
60 seconds. `Snapshot` computes history gates and percentiles; `Reading`
returns current window readings with the most recent cached integrated/LRA
result. Neither method allocates. Configure enough `MaxFrames` at construction;
processing rejects a history overflow atomically. The reserved loudness history
is limited to 64 MiB. Explicit channel weights support five-channel surround
and six-channel layouts with an excluded LFE; omitted weights treat channels
equally.
