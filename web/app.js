const SCALES = {
  pentatonic: [0, 2, 4, 7, 9],
  pentatonicMinor: [0, 3, 5, 7, 10],
  major: [0, 2, 4, 5, 7, 9, 11],
  minor: [0, 2, 3, 5, 7, 8, 10],
  dorian: [0, 2, 3, 5, 7, 9, 10],
  phrygian: [0, 1, 3, 5, 7, 8, 10],
  lydian: [0, 2, 4, 6, 7, 9, 11],
  mixolydian: [0, 2, 4, 5, 7, 9, 10],
  blues: [0, 3, 5, 6, 7, 10],
  hijazkiar: [0, 1, 4, 5, 7, 8, 11],
  chromatic: [0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11],
};

const ROOT_NOTES = [
  "C",
  "C#",
  "D",
  "D#",
  "E",
  "F",
  "F#",
  "G",
  "G#",
  "A",
  "A#",
  "B",
];

function getNoteFreq(noteIndex) {
  // A4 (index 57) = 440Hz.
  return 440 * Math.pow(2, (noteIndex - 57) / 12);
}

function generateNotes(rootName, scaleKey) {
  const rootOffset = ROOT_NOTES.indexOf(rootName);
  const intervals = SCALES[scaleKey] || SCALES.pentatonic;
  const notes = [];

  // Generate 2 octaves starting from octave 3
  for (let octave = 3; octave <= 4; octave++) {
    for (const interval of intervals) {
      const noteIdx = octave * 12 + rootOffset + interval;
      const freq = getNoteFreq(noteIdx);
      const noteName = ROOT_NOTES[(rootOffset + interval) % 12];
      const label = `${noteName}${octave}`;
      notes.push([label, freq]);
    }
  }
  // Add one more root note at the top
  const topIdx = 5 * 12 + rootOffset;
  notes.push([`${rootName}5`, getNoteFreq(topIdx)]);

  return notes;
}

let currentNotes = generateNotes("C", "pentatonic");

const STEP_COUNT = 16;

const state = {
  audioCtx: null,
  outputNode: null,
  isRunning: false,
  currentStep: 0,
  nextNoteTime: 0,
  scheduler: null,
  steps: [],
  eqUI: null,
  compUI: null,
  limUI: null,
  chain: null,
  hoverInfo: null,
  eqParams: {
    hpFamily: "rbj",
    hpType: "highpass",
    hpOrder: 4,
    hpFreq: 40,
    hpGain: 0,
    hpQ: 0.707,
    lowFamily: "rbj",
    lowType: "lowshelf",
    lowOrder: 4,
    lowFreq: 120,
    lowGain: 0,
    lowQ: 0.707,
    midFamily: "rbj",
    midType: "peak",
    midOrder: 4,
    midFreq: 1000,
    midGain: 0,
    midQ: 1.2,
    highFamily: "rbj",
    highType: "highshelf",
    highOrder: 4,
    highFreq: 5000,
    highGain: 0,
    highQ: 0.707,
    lpFamily: "rbj",
    lpType: "lowpass",
    lpOrder: 4,
    lpFreq: 12000,
    lpGain: 0,
    lpQ: 0.707,
    master: 1,
  },
  effectsParams: {
    chorusEnabled: false,
    chorusMix: 0.18,
    chorusDepth: 0.003,
    chorusSpeedHz: 0.35,
    chorusStages: 3,
    flangerEnabled: false,
    flangerRateHz: 0.25,
    flangerDepth: 0.0015,
    flangerBaseDelay: 0.001,
    flangerFeedback: 0.25,
    flangerMix: 0.5,
    ringModEnabled: false,
    ringModCarrierHz: 440,
    ringModMix: 1,
    bitCrusherEnabled: false,
    bitCrusherBitDepth: 8,
    bitCrusherDownsample: 4,
    bitCrusherMix: 1,
    distortionEnabled: false,
    transformerEnabled: false,
    widenerEnabled: false,
    widenerWidth: 1,
    widenerMix: 0.5,
    phaserEnabled: false,
    phaserRateHz: 0.4,
    phaserMinFreqHz: 300,
    phaserMaxFreqHz: 1600,
    phaserStages: 6,
    phaserFeedback: 0.2,
    phaserMix: 0.5,
    tremoloEnabled: false,
    tremoloRateHz: 4,
    tremoloDepth: 0.6,
    tremoloSmoothingMs: 5,
    tremoloMix: 1,
    delayEnabled: false,
    delayTime: 0.25,
    delayFeedback: 0.35,
    delayMix: 0.25,
    timePitchEnabled: false,
    timePitchSemitones: 0,
    timePitchSequence: 40,
    timePitchOverlap: 10,
    timePitchSearch: 15,
    spectralPitchEnabled: false,
    spectralPitchSemitones: 0,
    spectralPitchFrameSize: 1024,
    spectralPitchHopRatio: 0.25,
    spectralPitchHop: 256,
    expanderEnabled: false,
    deesserEnabled: false,
    multibandEnabled: false,
    harmonicBassEnabled: false,
    harmonicBassFrequency: 80,
    harmonicBassInputGain: 1,
    harmonicBassHighGain: 1,
    harmonicBassOriginal: 1,
    harmonicBassHarmonic: 0,
    harmonicBassDecay: 0,
    harmonicBassResponseMs: 20,
    harmonicBassHighpass: 0,
    reverbEnabled: false,
    reverbModel: "freeverb",
    reverbWet: 0.42,
    reverbDry: 1.0,
    reverbRoomSize: 0.72,
    reverbDamp: 0.45,
    reverbGain: 0.015,
    reverbRT60: 1.8,
    reverbPreDelay: 0.01,
    reverbModDepth: 0.002,
    reverbModRate: 0.1,
    chainGraphJSON: "",
  },
  compParams: {
    enabled: false,
    thresholdDB: -20,
    ratio: 4,
    kneeDB: 6,
    attackMs: 10,
    releaseMs: 100,
    makeupGainDB: 0,
    autoMakeup: true,
  },
  limParams: {
    enabled: true,
    threshold: -0.1,
    release: 100,
  },
  analyzerParams: {
    fftSize: 2048,
    overlap: 0.75,
    window: "blackmanharris",
    smoothing: 0.65,
  },
  dsp: {
    ready: false,
    api: null,
    go: null,
    sampleRate: 0,
  },
  waveform: "sine",
  eqDrawLoopHandle: null,
  eqLastDrawTimeMS: 0,
};

const el = {
  runToggle: document.getElementById("run-toggle"),
  waveform: document.getElementById("waveform"),
  tempo: document.getElementById("tempo"),
  tempoValue: document.getElementById("tempo-value"),
  decay: document.getElementById("decay"),
  decayValue: document.getElementById("decay-value"),
  shuffle: document.getElementById("shuffle"),
  shuffleValue: document.getElementById("shuffle-value"),
  steps: document.getElementById("steps"),
  scale: document.getElementById("scale"),
  rootNote: document.getElementById("root-note"),
  randomizeSteps: document.getElementById("randomize-steps"),
  eqCanvas: document.getElementById("eq-canvas"),
  eqReadout: document.getElementById("eq-readout"),
  chainCanvas: document.getElementById("chain-canvas"),
  chainDetail: document.getElementById("chain-detail"),
  chorusMix: document.getElementById("chorus-mix"),
  chorusMixValue: document.getElementById("chorus-mix-value"),
  chorusDepth: document.getElementById("chorus-depth"),
  chorusDepthValue: document.getElementById("chorus-depth-value"),
  chorusSpeed: document.getElementById("chorus-speed"),
  chorusSpeedValue: document.getElementById("chorus-speed-value"),
  chorusStages: document.getElementById("chorus-stages"),
  chorusStagesValue: document.getElementById("chorus-stages-value"),
  flangerRate: document.getElementById("flanger-rate"),
  flangerRateValue: document.getElementById("flanger-rate-value"),
  flangerDepth: document.getElementById("flanger-depth"),
  flangerDepthValue: document.getElementById("flanger-depth-value"),
  flangerBaseDelay: document.getElementById("flanger-base-delay"),
  flangerBaseDelayValue: document.getElementById("flanger-base-delay-value"),
  flangerFeedback: document.getElementById("flanger-feedback"),
  flangerFeedbackValue: document.getElementById("flanger-feedback-value"),
  flangerMix: document.getElementById("flanger-mix"),
  flangerMixValue: document.getElementById("flanger-mix-value"),
  ringModCarrier: document.getElementById("ringmod-carrier"),
  ringModCarrierValue: document.getElementById("ringmod-carrier-value"),
  ringModMix: document.getElementById("ringmod-mix"),
  ringModMixValue: document.getElementById("ringmod-mix-value"),
  bitCrusherBits: document.getElementById("bitcrusher-bits"),
  bitCrusherBitsValue: document.getElementById("bitcrusher-bits-value"),
  bitCrusherDownsample: document.getElementById("bitcrusher-downsample"),
  bitCrusherDownsampleValue: document.getElementById("bitcrusher-downsample-value"),
  bitCrusherMix: document.getElementById("bitcrusher-mix"),
  bitCrusherMixValue: document.getElementById("bitcrusher-mix-value"),
  distortionMode: document.getElementById("distortion-mode"),
  distortionApprox: document.getElementById("distortion-approx"),
  distortionDrive: document.getElementById("distortion-drive"),
  distortionDriveValue: document.getElementById("distortion-drive-value"),
  distortionMix: document.getElementById("distortion-mix"),
  distortionMixValue: document.getElementById("distortion-mix-value"),
  distortionOutput: document.getElementById("distortion-output"),
  distortionOutputValue: document.getElementById("distortion-output-value"),
  distortionClip: document.getElementById("distortion-clip"),
  distortionClipValue: document.getElementById("distortion-clip-value"),
  distortionShape: document.getElementById("distortion-shape"),
  distortionShapeValue: document.getElementById("distortion-shape-value"),
  distortionBias: document.getElementById("distortion-bias"),
  distortionBiasValue: document.getElementById("distortion-bias-value"),
  distortionChebOrder: document.getElementById("distortion-cheb-order"),
  distortionChebOrderValue: document.getElementById("distortion-cheb-order-value"),
  distortionChebHarmonic: document.getElementById("distortion-cheb-harmonic"),
  distortionChebInvert: document.getElementById("distortion-cheb-invert"),
  distortionChebGain: document.getElementById("distortion-cheb-gain"),
  distortionChebGainValue: document.getElementById("distortion-cheb-gain-value"),
  distortionChebDCBypass: document.getElementById("distortion-cheb-dc-bypass"),
  transformerQuality: document.getElementById("transformer-quality"),
  transformerDrive: document.getElementById("transformer-drive"),
  transformerDriveValue: document.getElementById("transformer-drive-value"),
  transformerMix: document.getElementById("transformer-mix"),
  transformerMixValue: document.getElementById("transformer-mix-value"),
  transformerOutput: document.getElementById("transformer-output"),
  transformerOutputValue: document.getElementById("transformer-output-value"),
  transformerHighpass: document.getElementById("transformer-highpass"),
  transformerHighpassValue: document.getElementById("transformer-highpass-value"),
  transformerDamping: document.getElementById("transformer-damping"),
  transformerDampingValue: document.getElementById("transformer-damping-value"),
  transformerOversampling: document.getElementById("transformer-oversampling"),
  fxFilterFamily: document.getElementById("fx-filter-family"),
  fxFilterKind: document.getElementById("fx-filter-kind"),
  fxFilterOrder: document.getElementById("fx-filter-order"),
  fxFilterOrderValue: document.getElementById("fx-filter-order-value"),
  fxFilterFreq: document.getElementById("fx-filter-freq"),
  fxFilterFreqValue: document.getElementById("fx-filter-freq-value"),
  fxFilterQ: document.getElementById("fx-filter-q"),
  fxFilterQValue: document.getElementById("fx-filter-q-value"),
  fxFilterGain: document.getElementById("fx-filter-gain"),
  fxFilterGainValue: document.getElementById("fx-filter-gain-value"),
  fxCompThresh: document.getElementById("fx-comp-thresh"),
  fxCompThreshValue: document.getElementById("fx-comp-thresh-value"),
  fxCompRatio: document.getElementById("fx-comp-ratio"),
  fxCompRatioValue: document.getElementById("fx-comp-ratio-value"),
  fxCompKnee: document.getElementById("fx-comp-knee"),
  fxCompKneeValue: document.getElementById("fx-comp-knee-value"),
  fxCompAttack: document.getElementById("fx-comp-attack"),
  fxCompAttackValue: document.getElementById("fx-comp-attack-value"),
  fxCompRelease: document.getElementById("fx-comp-release"),
  fxCompReleaseValue: document.getElementById("fx-comp-release-value"),
  fxCompMakeup: document.getElementById("fx-comp-makeup"),
  fxCompMakeupValue: document.getElementById("fx-comp-makeup-value"),
  fxLimThresh: document.getElementById("fx-lim-thresh"),
  fxLimThreshValue: document.getElementById("fx-lim-thresh-value"),
  fxLimRelease: document.getElementById("fx-lim-release"),
  fxLimReleaseValue: document.getElementById("fx-lim-release-value"),
  fxLookaheadThresh: document.getElementById("fx-lookahead-thresh"),
  fxLookaheadThreshValue: document.getElementById("fx-lookahead-thresh-value"),
  fxLookaheadRelease: document.getElementById("fx-lookahead-release"),
  fxLookaheadReleaseValue: document.getElementById("fx-lookahead-release-value"),
  fxLookaheadMs: document.getElementById("fx-lookahead-ms"),
  fxLookaheadMsValue: document.getElementById("fx-lookahead-ms-value"),
  fxGateMode: document.getElementById("fx-gate-mode"),
  fxGateThresh: document.getElementById("fx-gate-thresh"),
  fxGateThreshValue: document.getElementById("fx-gate-thresh-value"),
  fxGateRatio: document.getElementById("fx-gate-ratio"),
  fxGateRatioValue: document.getElementById("fx-gate-ratio-value"),
  fxGateKnee: document.getElementById("fx-gate-knee"),
  fxGateKneeValue: document.getElementById("fx-gate-knee-value"),
  fxGateAttack: document.getElementById("fx-gate-attack"),
  fxGateAttackValue: document.getElementById("fx-gate-attack-value"),
  fxGateHold: document.getElementById("fx-gate-hold"),
  fxGateHoldValue: document.getElementById("fx-gate-hold-value"),
  fxGateRelease: document.getElementById("fx-gate-release"),
  fxGateReleaseValue: document.getElementById("fx-gate-release-value"),
  fxGateRange: document.getElementById("fx-gate-range"),
  fxGateRangeValue: document.getElementById("fx-gate-range-value"),
  fxExpTopology: document.getElementById("fx-exp-topology"),
  fxExpDetector: document.getElementById("fx-exp-detector"),
  fxExpRMS: document.getElementById("fx-exp-rms"),
  fxExpRMSValue: document.getElementById("fx-exp-rms-value"),
  fxExpThresh: document.getElementById("fx-exp-thresh"),
  fxExpThreshValue: document.getElementById("fx-exp-thresh-value"),
  fxExpRatio: document.getElementById("fx-exp-ratio"),
  fxExpRatioValue: document.getElementById("fx-exp-ratio-value"),
  fxExpKnee: document.getElementById("fx-exp-knee"),
  fxExpKneeValue: document.getElementById("fx-exp-knee-value"),
  fxExpAttack: document.getElementById("fx-exp-attack"),
  fxExpAttackValue: document.getElementById("fx-exp-attack-value"),
  fxExpRelease: document.getElementById("fx-exp-release"),
  fxExpReleaseValue: document.getElementById("fx-exp-release-value"),
  fxExpRange: document.getElementById("fx-exp-range"),
  fxExpRangeValue: document.getElementById("fx-exp-range-value"),
  fxDeessMode: document.getElementById("fx-deess-mode"),
  fxDeessDetector: document.getElementById("fx-deess-detector"),
  fxDeessListen: document.getElementById("fx-deess-listen"),
  fxDeessFreq: document.getElementById("fx-deess-freq"),
  fxDeessFreqValue: document.getElementById("fx-deess-freq-value"),
  fxDeessQ: document.getElementById("fx-deess-q"),
  fxDeessQValue: document.getElementById("fx-deess-q-value"),
  fxDeessOrder: document.getElementById("fx-deess-order"),
  fxDeessOrderValue: document.getElementById("fx-deess-order-value"),
  fxDeessThresh: document.getElementById("fx-deess-thresh"),
  fxDeessThreshValue: document.getElementById("fx-deess-thresh-value"),
  fxDeessRatio: document.getElementById("fx-deess-ratio"),
  fxDeessRatioValue: document.getElementById("fx-deess-ratio-value"),
  fxDeessKnee: document.getElementById("fx-deess-knee"),
  fxDeessKneeValue: document.getElementById("fx-deess-knee-value"),
  fxDeessAttack: document.getElementById("fx-deess-attack"),
  fxDeessAttackValue: document.getElementById("fx-deess-attack-value"),
  fxDeessRelease: document.getElementById("fx-deess-release"),
  fxDeessReleaseValue: document.getElementById("fx-deess-release-value"),
  fxDeessRange: document.getElementById("fx-deess-range"),
  fxDeessRangeValue: document.getElementById("fx-deess-range-value"),
  fxTransientAttack: document.getElementById("fx-transient-attack"),
  fxTransientAttackValue: document.getElementById("fx-transient-attack-value"),
  fxTransientSustain: document.getElementById("fx-transient-sustain"),
  fxTransientSustainValue: document.getElementById("fx-transient-sustain-value"),
  fxTransientAttackMs: document.getElementById("fx-transient-attack-ms"),
  fxTransientAttackMsValue: document.getElementById("fx-transient-attack-ms-value"),
  fxTransientReleaseMs: document.getElementById("fx-transient-release-ms"),
  fxTransientReleaseMsValue: document.getElementById("fx-transient-release-ms-value"),
  fxMBBands: document.getElementById("fx-mb-bands"),
  fxMBCross1: document.getElementById("fx-mb-cross1"),
  fxMBCross1Value: document.getElementById("fx-mb-cross1-value"),
  fxMBCross2: document.getElementById("fx-mb-cross2"),
  fxMBCross2Value: document.getElementById("fx-mb-cross2-value"),
  fxMBOrder: document.getElementById("fx-mb-order"),
  fxMBOrderValue: document.getElementById("fx-mb-order-value"),
  fxMBAttack: document.getElementById("fx-mb-attack"),
  fxMBAttackValue: document.getElementById("fx-mb-attack-value"),
  fxMBRelease: document.getElementById("fx-mb-release"),
  fxMBReleaseValue: document.getElementById("fx-mb-release-value"),
  fxMBKnee: document.getElementById("fx-mb-knee"),
  fxMBKneeValue: document.getElementById("fx-mb-knee-value"),
  fxMBAutoMakeup: document.getElementById("fx-mb-auto-makeup"),
  fxMBMakeup: document.getElementById("fx-mb-makeup"),
  fxMBMakeupValue: document.getElementById("fx-mb-makeup-value"),
  fxMBLowThresh: document.getElementById("fx-mb-low-thresh"),
  fxMBLowThreshValue: document.getElementById("fx-mb-low-thresh-value"),
  fxMBLowRatio: document.getElementById("fx-mb-low-ratio"),
  fxMBLowRatioValue: document.getElementById("fx-mb-low-ratio-value"),
  fxMBMidThresh: document.getElementById("fx-mb-mid-thresh"),
  fxMBMidThreshValue: document.getElementById("fx-mb-mid-thresh-value"),
  fxMBMidRatio: document.getElementById("fx-mb-mid-ratio"),
  fxMBMidRatioValue: document.getElementById("fx-mb-mid-ratio-value"),
  fxMBHighThresh: document.getElementById("fx-mb-high-thresh"),
  fxMBHighThreshValue: document.getElementById("fx-mb-high-thresh-value"),
  fxMBHighRatio: document.getElementById("fx-mb-high-ratio"),
  fxMBHighRatioValue: document.getElementById("fx-mb-high-ratio-value"),
  splitFreq: document.getElementById("split-freq"),
  splitFreqValue: document.getElementById("split-freq-value"),
  widenerWidth: document.getElementById("widener-width"),
  widenerWidthValue: document.getElementById("widener-width-value"),
  widenerMix: document.getElementById("widener-mix"),
  widenerMixValue: document.getElementById("widener-mix-value"),
  phaserRate: document.getElementById("phaser-rate"),
  phaserRateValue: document.getElementById("phaser-rate-value"),
  phaserMinFreq: document.getElementById("phaser-min-freq"),
  phaserMinFreqValue: document.getElementById("phaser-min-freq-value"),
  phaserMaxFreq: document.getElementById("phaser-max-freq"),
  phaserMaxFreqValue: document.getElementById("phaser-max-freq-value"),
  phaserStages: document.getElementById("phaser-stages"),
  phaserStagesValue: document.getElementById("phaser-stages-value"),
  phaserFeedback: document.getElementById("phaser-feedback"),
  phaserFeedbackValue: document.getElementById("phaser-feedback-value"),
  phaserMix: document.getElementById("phaser-mix"),
  phaserMixValue: document.getElementById("phaser-mix-value"),
  tremoloRate: document.getElementById("tremolo-rate"),
  tremoloRateValue: document.getElementById("tremolo-rate-value"),
  tremoloDepth: document.getElementById("tremolo-depth"),
  tremoloDepthValue: document.getElementById("tremolo-depth-value"),
  tremoloSmoothing: document.getElementById("tremolo-smoothing"),
  tremoloSmoothingValue: document.getElementById("tremolo-smoothing-value"),
  tremoloMix: document.getElementById("tremolo-mix"),
  tremoloMixValue: document.getElementById("tremolo-mix-value"),
  delayTime: document.getElementById("delay-time"),
  delayTimeValue: document.getElementById("delay-time-value"),
  delayFeedback: document.getElementById("delay-feedback"),
  delayFeedbackValue: document.getElementById("delay-feedback-value"),
  delayMix: document.getElementById("delay-mix"),
  delayMixValue: document.getElementById("delay-mix-value"),
  simpleDelayMs: document.getElementById("simple-delay-ms"),
  simpleDelayMsValue: document.getElementById("simple-delay-ms-value"),
  timePitchSemitones: document.getElementById("time-pitch-semitones"),
  timePitchSemitonesValue: document.getElementById("time-pitch-semitones-value"),
  timePitchSequence: document.getElementById("time-pitch-sequence"),
  timePitchSequenceValue: document.getElementById("time-pitch-sequence-value"),
  timePitchOverlap: document.getElementById("time-pitch-overlap"),
  timePitchOverlapValue: document.getElementById("time-pitch-overlap-value"),
  timePitchSearch: document.getElementById("time-pitch-search"),
  timePitchSearchValue: document.getElementById("time-pitch-search-value"),
  spectralPitchSemitones: document.getElementById("spectral-pitch-semitones"),
  spectralPitchSemitonesValue: document.getElementById("spectral-pitch-semitones-value"),
  spectralPitchFrame: document.getElementById("spectral-pitch-frame"),
  spectralPitchFrameValue: document.getElementById("spectral-pitch-frame-value"),
  spectralPitchHopRatio: document.getElementById("spectral-pitch-hop-ratio"),
  spectralPitchHopRatioValue: document.getElementById("spectral-pitch-hop-ratio-value"),
  harmonicFrequency: document.getElementById("harmonic-frequency"),
  harmonicFrequencyValue: document.getElementById("harmonic-frequency-value"),
  harmonicInput: document.getElementById("harmonic-input"),
  harmonicInputValue: document.getElementById("harmonic-input-value"),
  harmonicHigh: document.getElementById("harmonic-high"),
  harmonicHighValue: document.getElementById("harmonic-high-value"),
  harmonicOriginal: document.getElementById("harmonic-original"),
  harmonicOriginalValue: document.getElementById("harmonic-original-value"),
  harmonicHarmonic: document.getElementById("harmonic-harmonic"),
  harmonicHarmonicValue: document.getElementById("harmonic-harmonic-value"),
  harmonicDecay: document.getElementById("harmonic-decay"),
  harmonicDecayValue: document.getElementById("harmonic-decay-value"),
  harmonicResponse: document.getElementById("harmonic-response"),
  harmonicResponseValue: document.getElementById("harmonic-response-value"),
  harmonicHighpass: document.getElementById("harmonic-highpass"),
  harmonicHighpassValue: document.getElementById("harmonic-highpass-value"),
  reverbModel: document.getElementById("reverb-model"),
  reverbWet: document.getElementById("reverb-wet"),
  reverbWetValue: document.getElementById("reverb-wet-value"),
  reverbDry: document.getElementById("reverb-dry"),
  reverbDryValue: document.getElementById("reverb-dry-value"),
  reverbRoom: document.getElementById("reverb-room"),
  reverbRoomValue: document.getElementById("reverb-room-value"),
  reverbDamp: document.getElementById("reverb-damp"),
  reverbDampValue: document.getElementById("reverb-damp-value"),
  reverbRT60: document.getElementById("reverb-rt60"),
  reverbRT60Value: document.getElementById("reverb-rt60-value"),
  reverbPreDelay: document.getElementById("reverb-predelay"),
  reverbPreDelayValue: document.getElementById("reverb-predelay-value"),
  reverbModDepth: document.getElementById("reverb-mod-depth"),
  reverbModDepthValue: document.getElementById("reverb-mod-depth-value"),
  reverbModRate: document.getElementById("reverb-mod-rate"),
  reverbModRateValue: document.getElementById("reverb-mod-rate-value"),
  compEnabled: document.getElementById("comp-enabled"),
  compThresh: document.getElementById("comp-thresh"),
  compThreshValue: document.getElementById("comp-thresh-value"),
  compRatio: document.getElementById("comp-ratio"),
  compRatioValue: document.getElementById("comp-ratio-value"),
  compKnee: document.getElementById("comp-knee"),
  compKneeValue: document.getElementById("comp-knee-value"),
  compAttack: document.getElementById("comp-attack"),
  compAttackValue: document.getElementById("comp-attack-value"),
  compRelease: document.getElementById("comp-release"),
  compReleaseValue: document.getElementById("comp-release-value"),
  compAuto: document.getElementById("comp-auto"),
  compMakeup: document.getElementById("comp-makeup"),
  compMakeupValue: document.getElementById("comp-makeup-value"),
  limEnabled: document.getElementById("lim-enabled"),
  limThresh: document.getElementById("lim-thresh"),
  limThreshValue: document.getElementById("lim-thresh-value"),
  limRelease: document.getElementById("lim-release"),
  limReleaseValue: document.getElementById("lim-release-value"),
  compGraph: document.getElementById("comp-graph"),
  limGraph: document.getElementById("lim-graph"),
  analyzerFFT: document.getElementById("analyzer-fft"),
  analyzerOverlap: document.getElementById("analyzer-overlap"),
  analyzerOverlapValue: document.getElementById("analyzer-overlap-value"),
  analyzerWindow: document.getElementById("analyzer-window"),
  analyzerSmoothing: document.getElementById("analyzer-smoothing"),
  analyzerSmoothingValue: document.getElementById("analyzer-smoothing-value"),
  themeToggle: document.getElementById("theme-toggle"),
};

const THEME_STORAGE_KEY = "algo-dsp-theme";
const THEME_MODES = ["system", "light", "dark"];
const SETTINGS_STORAGE_KEY = "algo-dsp-settings";
const FLANGER_MIN_DELAY_SECONDS = 0.0001;
const FLANGER_MAX_DELAY_SECONDS = 0.01;
const FLANGER_MAX_DEPTH_SECONDS = 0.009;
const EFFECT_NODE_DEFAULTS = {
  chorus: { mix: 0.18, depth: 0.003, speedHz: 0.35, stages: 3 },
  flanger: {
    rateHz: 0.25,
    depth: 0.0015,
    baseDelay: 0.001,
    feedback: 0.25,
    mix: 0.5,
  },
  ringmod: { carrierHz: 440, mix: 1.0 },
  bitcrusher: { bitDepth: 8, downsample: 4, mix: 1.0 },
  distortion: {
    mode: "softclip",
    approx: "exact",
    drive: 1.8,
    mix: 1.0,
    output: 1.0,
    clip: 1.0,
    shape: 0.5,
    bias: 0.0,
    chebOrder: 3,
    chebHarmonic: "all",
    chebInvert: 0,
    chebGain: 1.0,
    chebDCBypass: 0,
  },
  transformer: {
    quality: "high",
    drive: 2.0,
    mix: 1.0,
    output: 1.0,
    highpassHz: 25,
    dampingHz: 9000,
    oversampling: 4,
  },
  filter: { family: "rbj", kind: "lowpass", order: 2, freq: 1200, q: 0.707, gain: 0 },
  "dyn-compressor": {
    thresholdDB: -20,
    ratio: 4,
    kneeDB: 6,
    attackMs: 10,
    releaseMs: 100,
    makeupGainDB: 0,
  },
  "dyn-limiter": { thresholdDB: -0.1, releaseMs: 100 },
  "dyn-lookahead": { thresholdDB: -1.0, releaseMs: 100, lookaheadMs: 3.0 },
  "dyn-gate": {
    mode: "gate",
    thresholdDB: -40,
    ratio: 10,
    kneeDB: 6,
    attackMs: 0.1,
    holdMs: 50,
    releaseMs: 100,
    rangeDB: -80,
  },
  "dyn-expander": {
    topology: "feedforward",
    detector: "peak",
    rmsWindowMs: 30,
    thresholdDB: -35,
    ratio: 2.0,
    kneeDB: 6,
    attackMs: 1,
    releaseMs: 100,
    rangeDB: -60,
  },
  "dyn-deesser": {
    mode: "splitband",
    detector: "bandpass",
    listen: 0,
    freqHz: 6000,
    q: 1.5,
    filterOrder: 2,
    thresholdDB: -20,
    ratio: 4,
    kneeDB: 3,
    attackMs: 0.5,
    releaseMs: 20,
    rangeDB: -24,
  },
  "dyn-transient": {
    attack: 0.0,
    sustain: 0.0,
    attackMs: 10.0,
    releaseMs: 120.0,
  },
  "dyn-multiband": {
    bands: 3,
    cross1Hz: 250,
    cross2Hz: 3000,
    order: 4,
    attackMs: 8,
    releaseMs: 120,
    kneeDB: 6,
    autoMakeup: 0,
    makeupGainDB: 0,
    lowThresholdDB: -20,
    lowRatio: 2.5,
    midThresholdDB: -18,
    midRatio: 3.0,
    highThresholdDB: -14,
    highRatio: 4.0,
  },
  "split-freq": { freqHz: 1200 },
  widener: { width: 1.0, mix: 0.5 },
  phaser: {
    rateHz: 0.4,
    minFreqHz: 300,
    maxFreqHz: 1600,
    stages: 6,
    feedback: 0.2,
    mix: 0.5,
  },
  tremolo: { rateHz: 4, depth: 0.6, smoothingMs: 5, mix: 1.0 },
  delay: { time: 0.25, feedback: 0.35, mix: 0.25 },
  "delay-simple": { delayMs: 20 },
  bass: {
    frequency: 80,
    inputGain: 1,
    highGain: 1,
    original: 1,
    harmonic: 0,
    decay: 0,
    responseMs: 20,
    highpass: 0,
  },
  "pitch-time": { semitones: 0, sequence: 40, overlap: 10, search: 15 },
  "pitch-spectral": { semitones: 0, frameSize: 1024, hopRatio: 0.25 },
  reverb: {
    model: "freeverb",
    wet: 0.22,
    dry: 1.0,
    roomSize: 0.72,
    damp: 0.45,
    rt60: 1.8,
    preDelay: 0.01,
    modDepth: 0.002,
    modRate: 0.1,
  },
};

function defaultNodeParams(type) {
  const src = EFFECT_NODE_DEFAULTS[type] || {};
  return { ...src };
}

function getSelectedEffectNode() {
  const node = state.chain?.getSelectedNode?.();
  if (!node || node.fixed) return null;
  return node;
}

function applyNodeParamsToUI(node) {
  if (!node || node.fixed) return;
  const p = { ...defaultNodeParams(node.type), ...(node.params || {}) };
  switch (node.type) {
    case "chorus":
      el.chorusMix.value = p.mix;
      el.chorusDepth.value = p.depth;
      el.chorusSpeed.value = p.speedHz;
      el.chorusStages.value = p.stages;
      break;
    case "flanger":
      el.flangerRate.value = p.rateHz;
      el.flangerDepth.value = p.depth;
      el.flangerBaseDelay.value = p.baseDelay;
      el.flangerFeedback.value = p.feedback;
      el.flangerMix.value = p.mix;
      sanitizeFlangerControls();
      break;
    case "ringmod":
      el.ringModCarrier.value = p.carrierHz;
      el.ringModMix.value = p.mix;
      break;
    case "bitcrusher":
      el.bitCrusherBits.value = p.bitDepth;
      el.bitCrusherDownsample.value = p.downsample;
      el.bitCrusherMix.value = p.mix;
      break;
    case "distortion":
      el.distortionMode.value = p.mode || "softclip";
      el.distortionApprox.value = p.approx || "exact";
      el.distortionDrive.value = p.drive;
      el.distortionMix.value = p.mix;
      el.distortionOutput.value = p.output;
      el.distortionClip.value = p.clip;
      el.distortionShape.value = p.shape;
      el.distortionBias.value = p.bias;
      el.distortionChebOrder.value = p.chebOrder;
      el.distortionChebHarmonic.value = p.chebHarmonic || "all";
      el.distortionChebInvert.value = String(Number(p.chebInvert || 0));
      el.distortionChebGain.value = p.chebGain;
      el.distortionChebDCBypass.value = String(Number(p.chebDCBypass || 0));
      break;
    case "transformer":
      el.transformerQuality.value = p.quality || "high";
      el.transformerDrive.value = p.drive;
      el.transformerMix.value = p.mix;
      el.transformerOutput.value = p.output;
      el.transformerHighpass.value = p.highpassHz;
      el.transformerDamping.value = p.dampingHz;
      el.transformerOversampling.value = String(p.oversampling ?? 4);
      break;
    case "filter":
      el.fxFilterFamily.value = p.family || "rbj";
      el.fxFilterKind.value = p.kind || "lowpass";
      el.fxFilterOrder.value = p.order ?? 2;
      el.fxFilterFreq.value = p.freq;
      el.fxFilterQ.value = p.q;
      el.fxFilterGain.value = p.gain;
      break;
    case "dyn-compressor":
      el.fxCompThresh.value = p.thresholdDB;
      el.fxCompRatio.value = p.ratio;
      el.fxCompKnee.value = p.kneeDB;
      el.fxCompAttack.value = p.attackMs;
      el.fxCompRelease.value = p.releaseMs;
      el.fxCompMakeup.value = p.makeupGainDB;
      break;
    case "dyn-limiter":
      el.fxLimThresh.value = p.thresholdDB;
      el.fxLimRelease.value = p.releaseMs;
      break;
    case "dyn-lookahead":
      el.fxLookaheadThresh.value = p.thresholdDB;
      el.fxLookaheadRelease.value = p.releaseMs;
      el.fxLookaheadMs.value = p.lookaheadMs;
      break;
    case "dyn-gate":
      el.fxGateMode.value = p.mode || "gate";
      el.fxGateThresh.value = p.thresholdDB;
      el.fxGateRatio.value = p.ratio;
      el.fxGateKnee.value = p.kneeDB;
      el.fxGateAttack.value = p.attackMs;
      el.fxGateHold.value = p.holdMs;
      el.fxGateRelease.value = p.releaseMs;
      el.fxGateRange.value = p.rangeDB;
      break;
    case "dyn-expander":
      el.fxExpTopology.value = p.topology || "feedforward";
      el.fxExpDetector.value = p.detector || "peak";
      el.fxExpRMS.value = p.rmsWindowMs;
      el.fxExpThresh.value = p.thresholdDB;
      el.fxExpRatio.value = p.ratio;
      el.fxExpKnee.value = p.kneeDB;
      el.fxExpAttack.value = p.attackMs;
      el.fxExpRelease.value = p.releaseMs;
      el.fxExpRange.value = p.rangeDB;
      break;
    case "dyn-deesser":
      el.fxDeessMode.value = p.mode || "splitband";
      el.fxDeessDetector.value = p.detector || "bandpass";
      el.fxDeessListen.value = String(Number(p.listen || 0));
      el.fxDeessFreq.value = p.freqHz;
      el.fxDeessQ.value = p.q;
      el.fxDeessOrder.value = p.filterOrder;
      el.fxDeessThresh.value = p.thresholdDB;
      el.fxDeessRatio.value = p.ratio;
      el.fxDeessKnee.value = p.kneeDB;
      el.fxDeessAttack.value = p.attackMs;
      el.fxDeessRelease.value = p.releaseMs;
      el.fxDeessRange.value = p.rangeDB;
      break;
    case "dyn-transient":
      el.fxTransientAttack.value = p.attack;
      el.fxTransientSustain.value = p.sustain;
      el.fxTransientAttackMs.value = p.attackMs;
      el.fxTransientReleaseMs.value = p.releaseMs;
      break;
    case "dyn-multiband":
      el.fxMBBands.value = String(p.bands ?? 3);
      el.fxMBCross1.value = p.cross1Hz;
      el.fxMBCross2.value = p.cross2Hz;
      el.fxMBOrder.value = p.order;
      el.fxMBAttack.value = p.attackMs;
      el.fxMBRelease.value = p.releaseMs;
      el.fxMBKnee.value = p.kneeDB;
      el.fxMBAutoMakeup.value = String(Number(p.autoMakeup || 0));
      el.fxMBMakeup.value = p.makeupGainDB;
      el.fxMBLowThresh.value = p.lowThresholdDB;
      el.fxMBLowRatio.value = p.lowRatio;
      el.fxMBMidThresh.value = p.midThresholdDB;
      el.fxMBMidRatio.value = p.midRatio;
      el.fxMBHighThresh.value = p.highThresholdDB;
      el.fxMBHighRatio.value = p.highRatio;
      break;
    case "split-freq":
      el.splitFreq.value = p.freqHz;
      break;
    case "widener":
      el.widenerWidth.value = p.width;
      el.widenerMix.value = p.mix;
      break;
    case "phaser":
      el.phaserRate.value = p.rateHz;
      el.phaserMinFreq.value = p.minFreqHz;
      el.phaserMaxFreq.value = p.maxFreqHz;
      el.phaserStages.value = p.stages;
      el.phaserFeedback.value = p.feedback;
      el.phaserMix.value = p.mix;
      break;
    case "tremolo":
      el.tremoloRate.value = p.rateHz;
      el.tremoloDepth.value = p.depth;
      el.tremoloSmoothing.value = p.smoothingMs;
      el.tremoloMix.value = p.mix;
      break;
    case "delay":
      el.delayTime.value = p.time;
      el.delayFeedback.value = p.feedback;
      el.delayMix.value = p.mix;
      break;
    case "delay-simple":
      el.simpleDelayMs.value = p.delayMs;
      break;
    case "bass":
      el.harmonicFrequency.value = p.frequency;
      el.harmonicInput.value = p.inputGain;
      el.harmonicHigh.value = p.highGain;
      el.harmonicOriginal.value = p.original;
      el.harmonicHarmonic.value = p.harmonic;
      el.harmonicDecay.value = p.decay;
      el.harmonicResponse.value = p.responseMs;
      el.harmonicHighpass.value = p.highpass;
      break;
    case "pitch-time":
      el.timePitchSemitones.value = p.semitones;
      el.timePitchSequence.value = p.sequence;
      el.timePitchOverlap.value = p.overlap;
      el.timePitchSearch.value = p.search;
      break;
    case "pitch-spectral":
      el.spectralPitchSemitones.value = p.semitones;
      el.spectralPitchFrame.value = p.frameSize;
      el.spectralPitchHopRatio.value = String(p.hopRatio);
      break;
    case "reverb":
      el.reverbModel.value = p.model || "freeverb";
      el.reverbWet.value = p.wet;
      el.reverbDry.value = p.dry;
      el.reverbRoom.value = p.roomSize;
      el.reverbDamp.value = p.damp;
      el.reverbRT60.value = p.rt60;
      el.reverbPreDelay.value = p.preDelay;
      el.reverbModDepth.value = p.modDepth;
      el.reverbModRate.value = p.modRate;
      break;
    default:
      break;
  }
}

function collectNodeParamsFromUI(nodeType) {
  switch (nodeType) {
    case "chorus":
      return {
        mix: Number(el.chorusMix.value),
        depth: Number(el.chorusDepth.value),
        speedHz: Number(el.chorusSpeed.value),
        stages: Number(el.chorusStages.value),
      };
    case "flanger": {
      const flanger = sanitizeFlangerControls();
      return {
        rateHz: Number(el.flangerRate.value),
        depth: flanger.depth,
        baseDelay: flanger.baseDelay,
        feedback: Number(el.flangerFeedback.value),
        mix: Number(el.flangerMix.value),
      };
    }
    case "ringmod":
      return {
        carrierHz: Number(el.ringModCarrier.value),
        mix: Number(el.ringModMix.value),
      };
    case "bitcrusher":
      return {
        bitDepth: Number(el.bitCrusherBits.value),
        downsample: Number(el.bitCrusherDownsample.value),
        mix: Number(el.bitCrusherMix.value),
      };
    case "distortion": {
      const chebHarmonic = String(el.distortionChebHarmonic.value || "all");
      let chebOrder = Math.max(1, Math.min(16, Math.round(Number(el.distortionChebOrder.value))));
      if (chebHarmonic === "odd" && chebOrder % 2 === 0) {
        chebOrder = Math.max(1, chebOrder - 1);
      }
      if (chebHarmonic === "even" && chebOrder % 2 !== 0) {
        chebOrder = Math.min(16, chebOrder + 1);
      }
      if (Number(el.distortionChebOrder.value) !== chebOrder) {
        el.distortionChebOrder.value = String(chebOrder);
      }
      return {
        mode: String(el.distortionMode.value || "softclip"),
        approx: String(el.distortionApprox.value || "exact"),
        drive: Number(el.distortionDrive.value),
        mix: Number(el.distortionMix.value),
        output: Number(el.distortionOutput.value),
        clip: Number(el.distortionClip.value),
        shape: Number(el.distortionShape.value),
        bias: Number(el.distortionBias.value),
        chebOrder,
        chebHarmonic,
        chebInvert: Number(el.distortionChebInvert.value),
        chebGain: Number(el.distortionChebGain.value),
        chebDCBypass: Number(el.distortionChebDCBypass.value),
      };
    }
    case "transformer":
      return {
        quality: String(el.transformerQuality.value || "high"),
        drive: Number(el.transformerDrive.value),
        mix: Number(el.transformerMix.value),
        output: Number(el.transformerOutput.value),
        highpassHz: Number(el.transformerHighpass.value),
        dampingHz: Number(el.transformerDamping.value),
        oversampling: Number(el.transformerOversampling.value),
      };
    case "filter":
      return {
        family: String(el.fxFilterFamily.value || "rbj"),
        kind: String(el.fxFilterKind.value || "lowpass"),
        order: Number(el.fxFilterOrder.value),
        freq: Number(el.fxFilterFreq.value),
        q: Number(el.fxFilterQ.value),
        gain: Number(el.fxFilterGain.value),
      };
    case "dyn-compressor":
      return {
        thresholdDB: Number(el.fxCompThresh.value),
        ratio: Number(el.fxCompRatio.value),
        kneeDB: Number(el.fxCompKnee.value),
        attackMs: Number(el.fxCompAttack.value),
        releaseMs: Number(el.fxCompRelease.value),
        makeupGainDB: Number(el.fxCompMakeup.value),
      };
    case "dyn-limiter":
      return {
        thresholdDB: Number(el.fxLimThresh.value),
        releaseMs: Number(el.fxLimRelease.value),
      };
    case "dyn-lookahead":
      return {
        thresholdDB: Number(el.fxLookaheadThresh.value),
        releaseMs: Number(el.fxLookaheadRelease.value),
        lookaheadMs: Number(el.fxLookaheadMs.value),
      };
    case "dyn-gate":
      return {
        mode: String(el.fxGateMode.value || "gate"),
        thresholdDB: Number(el.fxGateThresh.value),
        ratio: Number(el.fxGateRatio.value),
        kneeDB: Number(el.fxGateKnee.value),
        attackMs: Number(el.fxGateAttack.value),
        holdMs: Number(el.fxGateHold.value),
        releaseMs: Number(el.fxGateRelease.value),
        rangeDB: Number(el.fxGateRange.value),
      };
    case "dyn-expander":
      return {
        topology: String(el.fxExpTopology.value || "feedforward"),
        detector: String(el.fxExpDetector.value || "peak"),
        rmsWindowMs: Number(el.fxExpRMS.value),
        thresholdDB: Number(el.fxExpThresh.value),
        ratio: Number(el.fxExpRatio.value),
        kneeDB: Number(el.fxExpKnee.value),
        attackMs: Number(el.fxExpAttack.value),
        releaseMs: Number(el.fxExpRelease.value),
        rangeDB: Number(el.fxExpRange.value),
      };
    case "dyn-deesser":
      return {
        mode: String(el.fxDeessMode.value || "splitband"),
        detector: String(el.fxDeessDetector.value || "bandpass"),
        listen: Number(el.fxDeessListen.value),
        freqHz: Number(el.fxDeessFreq.value),
        q: Number(el.fxDeessQ.value),
        filterOrder: Number(el.fxDeessOrder.value),
        thresholdDB: Number(el.fxDeessThresh.value),
        ratio: Number(el.fxDeessRatio.value),
        kneeDB: Number(el.fxDeessKnee.value),
        attackMs: Number(el.fxDeessAttack.value),
        releaseMs: Number(el.fxDeessRelease.value),
        rangeDB: Number(el.fxDeessRange.value),
      };
    case "dyn-transient":
      return {
        attack: Number(el.fxTransientAttack.value),
        sustain: Number(el.fxTransientSustain.value),
        attackMs: Number(el.fxTransientAttackMs.value),
        releaseMs: Number(el.fxTransientReleaseMs.value),
      };
    case "dyn-multiband":
      return {
        bands: Number(el.fxMBBands.value),
        cross1Hz: Number(el.fxMBCross1.value),
        cross2Hz: Number(el.fxMBCross2.value),
        order: Number(el.fxMBOrder.value),
        attackMs: Number(el.fxMBAttack.value),
        releaseMs: Number(el.fxMBRelease.value),
        kneeDB: Number(el.fxMBKnee.value),
        autoMakeup: Number(el.fxMBAutoMakeup.value),
        makeupGainDB: Number(el.fxMBMakeup.value),
        lowThresholdDB: Number(el.fxMBLowThresh.value),
        lowRatio: Number(el.fxMBLowRatio.value),
        midThresholdDB: Number(el.fxMBMidThresh.value),
        midRatio: Number(el.fxMBMidRatio.value),
        highThresholdDB: Number(el.fxMBHighThresh.value),
        highRatio: Number(el.fxMBHighRatio.value),
      };
    case "split-freq":
      return {
        freqHz: Number(el.splitFreq.value),
      };
    case "widener":
      return {
        width: Number(el.widenerWidth.value),
        mix: Number(el.widenerMix.value),
      };
    case "phaser":
      return {
        rateHz: Number(el.phaserRate.value),
        minFreqHz: Number(el.phaserMinFreq.value),
        maxFreqHz: Number(el.phaserMaxFreq.value),
        stages: Number(el.phaserStages.value),
        feedback: Number(el.phaserFeedback.value),
        mix: Number(el.phaserMix.value),
      };
    case "tremolo":
      return {
        rateHz: Number(el.tremoloRate.value),
        depth: Number(el.tremoloDepth.value),
        smoothingMs: Number(el.tremoloSmoothing.value),
        mix: Number(el.tremoloMix.value),
      };
    case "delay":
      return {
        time: Number(el.delayTime.value),
        feedback: Number(el.delayFeedback.value),
        mix: Number(el.delayMix.value),
      };
    case "delay-simple":
      return {
        delayMs: Number(el.simpleDelayMs.value),
      };
    case "bass":
      return {
        frequency: Number(el.harmonicFrequency.value),
        inputGain: Number(el.harmonicInput.value),
        highGain: Number(el.harmonicHigh.value),
        original: Number(el.harmonicOriginal.value),
        harmonic: Number(el.harmonicHarmonic.value),
        decay: Number(el.harmonicDecay.value),
        responseMs: Number(el.harmonicResponse.value),
        highpass: Number(el.harmonicHighpass.value),
      };
    case "pitch-time": {
      const sequence = Number(el.timePitchSequence.value);
      const overlapRaw = Number(el.timePitchOverlap.value);
      const overlap = Math.min(sequence - 1, Math.max(4, overlapRaw));
      if (overlap !== overlapRaw) el.timePitchOverlap.value = String(overlap);
      return {
        semitones: Number(el.timePitchSemitones.value),
        sequence,
        overlap,
        search: Number(el.timePitchSearch.value),
      };
    }
    case "pitch-spectral":
      return {
        semitones: Number(el.spectralPitchSemitones.value),
        frameSize: Number(el.spectralPitchFrame.value),
        hopRatio: Number(el.spectralPitchHopRatio.value),
      };
    case "reverb":
      return {
        model: String(el.reverbModel.value || "freeverb"),
        wet: Number(el.reverbWet.value),
        dry: Number(el.reverbDry.value),
        roomSize: Number(el.reverbRoom.value),
        damp: Number(el.reverbDamp.value),
        rt60: Number(el.reverbRT60.value),
        preDelay: Number(el.reverbPreDelay.value),
        modDepth: Number(el.reverbModDepth.value),
        modRate: Number(el.reverbModRate.value),
      };
    default:
      return {};
  }
}

function commitSelectedNodeParamsFromUI() {
  const node = getSelectedEffectNode();
  if (!node) return false;
  const params = collectNodeParamsFromUI(node.type);
  return state.chain?.updateNodeParams(node.id, params) || false;
}

function saveSettings() {
  try {
    const settings = {
      effectsParams: state.effectsParams,
      compParams: state.compParams,
      limParams: state.limParams,
      chainState: state.chain ? state.chain.getState() : null,
    };
    localStorage.setItem(SETTINGS_STORAGE_KEY, JSON.stringify(settings));
  } catch (e) {
    // Ignore storage failures.
  }
}

function loadSettings() {
  let stored = null;
  try {
    stored = localStorage.getItem(SETTINGS_STORAGE_KEY);
  } catch (e) {
    return;
  }
  if (!stored) return;

  let settings;
  try {
    settings = JSON.parse(stored);
  } catch (e) {
    return;
  }

  if (settings.effectsParams) {
    Object.assign(state.effectsParams, settings.effectsParams);
    if (el.chorusMix) el.chorusMix.value = state.effectsParams.chorusMix;
    if (el.chorusDepth) el.chorusDepth.value = state.effectsParams.chorusDepth;
    if (el.chorusSpeed) el.chorusSpeed.value = state.effectsParams.chorusSpeedHz;
    if (el.chorusStages) el.chorusStages.value = state.effectsParams.chorusStages;
    if (el.flangerRate) el.flangerRate.value = state.effectsParams.flangerRateHz;
    if (el.flangerDepth) el.flangerDepth.value = state.effectsParams.flangerDepth;
    if (el.flangerBaseDelay) el.flangerBaseDelay.value = state.effectsParams.flangerBaseDelay;
    if (el.flangerFeedback) el.flangerFeedback.value = state.effectsParams.flangerFeedback;
    if (el.flangerMix) el.flangerMix.value = state.effectsParams.flangerMix;
    if (el.ringModCarrier) el.ringModCarrier.value = state.effectsParams.ringModCarrierHz;
    if (el.ringModMix) el.ringModMix.value = state.effectsParams.ringModMix;
    if (el.bitCrusherBits) el.bitCrusherBits.value = state.effectsParams.bitCrusherBitDepth;
    if (el.bitCrusherDownsample)
      el.bitCrusherDownsample.value = state.effectsParams.bitCrusherDownsample;
    if (el.bitCrusherMix) el.bitCrusherMix.value = state.effectsParams.bitCrusherMix;
    if (el.widenerWidth) el.widenerWidth.value = state.effectsParams.widenerWidth;
    if (el.widenerMix) el.widenerMix.value = state.effectsParams.widenerMix;
    if (el.phaserRate) el.phaserRate.value = state.effectsParams.phaserRateHz;
    if (el.phaserMinFreq) el.phaserMinFreq.value = state.effectsParams.phaserMinFreqHz;
    if (el.phaserMaxFreq) el.phaserMaxFreq.value = state.effectsParams.phaserMaxFreqHz;
    if (el.phaserStages) el.phaserStages.value = state.effectsParams.phaserStages;
    if (el.phaserFeedback) el.phaserFeedback.value = state.effectsParams.phaserFeedback;
    if (el.phaserMix) el.phaserMix.value = state.effectsParams.phaserMix;
    if (el.tremoloRate) el.tremoloRate.value = state.effectsParams.tremoloRateHz;
    if (el.tremoloDepth) el.tremoloDepth.value = state.effectsParams.tremoloDepth;
    if (el.tremoloSmoothing) el.tremoloSmoothing.value = state.effectsParams.tremoloSmoothingMs;
    if (el.tremoloMix) el.tremoloMix.value = state.effectsParams.tremoloMix;
    if (el.delayTime) el.delayTime.value = state.effectsParams.delayTime;
    if (el.delayFeedback) el.delayFeedback.value = state.effectsParams.delayFeedback;
    if (el.delayMix) el.delayMix.value = state.effectsParams.delayMix;
    if (el.timePitchSemitones)
      el.timePitchSemitones.value = state.effectsParams.timePitchSemitones;
    if (el.timePitchSequence)
      el.timePitchSequence.value = state.effectsParams.timePitchSequence;
    if (el.timePitchOverlap)
      el.timePitchOverlap.value = state.effectsParams.timePitchOverlap;
    if (el.timePitchSearch)
      el.timePitchSearch.value = state.effectsParams.timePitchSearch;
    if (el.spectralPitchSemitones)
      el.spectralPitchSemitones.value = state.effectsParams.spectralPitchSemitones;
    if (el.spectralPitchFrame)
      el.spectralPitchFrame.value = state.effectsParams.spectralPitchFrameSize;
    if (el.spectralPitchHopRatio)
      el.spectralPitchHopRatio.value = String(
        state.effectsParams.spectralPitchHopRatio ?? 0.25,
      );
    if (el.harmonicFrequency)
      el.harmonicFrequency.value = state.effectsParams.harmonicBassFrequency;
    if (el.harmonicInput)
      el.harmonicInput.value = state.effectsParams.harmonicBassInputGain;
    if (el.harmonicHigh)
      el.harmonicHigh.value = state.effectsParams.harmonicBassHighGain;
    if (el.harmonicOriginal)
      el.harmonicOriginal.value = state.effectsParams.harmonicBassOriginal;
    if (el.harmonicHarmonic)
      el.harmonicHarmonic.value = state.effectsParams.harmonicBassHarmonic;
    if (el.harmonicDecay)
      el.harmonicDecay.value = state.effectsParams.harmonicBassDecay;
    if (el.harmonicResponse)
      el.harmonicResponse.value = state.effectsParams.harmonicBassResponseMs;
    if (el.harmonicHighpass)
      el.harmonicHighpass.value = state.effectsParams.harmonicBassHighpass;
    if (el.reverbModel) el.reverbModel.value = state.effectsParams.reverbModel || "freeverb";
    if (el.reverbWet) el.reverbWet.value = state.effectsParams.reverbWet;
    if (el.reverbDry) el.reverbDry.value = state.effectsParams.reverbDry;
    if (el.reverbRoom) el.reverbRoom.value = state.effectsParams.reverbRoomSize;
    if (el.reverbDamp) el.reverbDamp.value = state.effectsParams.reverbDamp;
    if (el.reverbRT60) el.reverbRT60.value = state.effectsParams.reverbRT60;
    if (el.reverbPreDelay) el.reverbPreDelay.value = state.effectsParams.reverbPreDelay;
    if (el.reverbModDepth) el.reverbModDepth.value = state.effectsParams.reverbModDepth;
    if (el.reverbModRate) el.reverbModRate.value = state.effectsParams.reverbModRate;
    sanitizeFlangerControls();
    updateEffectsText();
  }

  if (settings.chainState && state.chain) {
    state.chain.setState(settings.chainState);
    readEffectsFromChain();
  }

  if (settings.compParams) {
    Object.assign(state.compParams, settings.compParams);
    if (el.compEnabled) el.compEnabled.checked = !!state.compParams.enabled;
    if (el.compThresh) el.compThresh.value = state.compParams.thresholdDB;
    if (el.compRatio) el.compRatio.value = state.compParams.ratio;
    if (el.compKnee) el.compKnee.value = state.compParams.kneeDB;
    if (el.compAttack) el.compAttack.value = state.compParams.attackMs;
    if (el.compRelease) el.compRelease.value = state.compParams.releaseMs;
    if (el.compAuto) el.compAuto.checked = !!state.compParams.autoMakeup;
    if (el.compMakeup) el.compMakeup.value = state.compParams.makeupGainDB;
    updateCompressorText();
  }

  if (settings.limParams) {
    Object.assign(state.limParams, settings.limParams);
    if (el.limEnabled) el.limEnabled.checked = !!state.limParams.enabled;
    if (el.limThresh) el.limThresh.value = state.limParams.threshold;
    if (el.limRelease) el.limRelease.value = state.limParams.release;
    updateLimiterText();
  }
}

function sanitizeFlangerControls() {
  if (!el.flangerBaseDelay || !el.flangerDepth) {
    return {
      baseDelay: state.effectsParams.flangerBaseDelay,
      depth: state.effectsParams.flangerDepth,
    };
  }

  const baseMin = Number.isFinite(Number(el.flangerBaseDelay.min))
    ? Number(el.flangerBaseDelay.min)
    : FLANGER_MIN_DELAY_SECONDS;
  const baseMaxRaw = Number.isFinite(Number(el.flangerBaseDelay.max))
    ? Number(el.flangerBaseDelay.max)
    : FLANGER_MAX_DELAY_SECONDS;
  const baseMax = Math.min(baseMaxRaw, FLANGER_MAX_DELAY_SECONDS);
  let baseDelay = Number(el.flangerBaseDelay.value);
  if (!Number.isFinite(baseDelay)) {
    baseDelay = state.effectsParams.flangerBaseDelay;
  }
  baseDelay = Math.max(baseMin, Math.min(baseMax, baseDelay));

  const depthMaxAllowed = Math.max(0, FLANGER_MAX_DELAY_SECONDS - baseDelay);
  const depthMax = Math.min(FLANGER_MAX_DEPTH_SECONDS, depthMaxAllowed);
  let depth = Number(el.flangerDepth.value);
  if (!Number.isFinite(depth)) {
    depth = state.effectsParams.flangerDepth;
  }
  depth = Math.max(0, Math.min(depthMax, depth));

  if (Math.abs(Number(el.flangerBaseDelay.value) - baseDelay) > 1e-12) {
    el.flangerBaseDelay.value = String(baseDelay);
  }
  if (Math.abs(Number(el.flangerDepth.max) - depthMax) > 1e-12) {
    el.flangerDepth.max = String(depthMax);
  }
  if (Math.abs(Number(el.flangerDepth.value) - depth) > 1e-12) {
    el.flangerDepth.value = String(depth);
  }

  return { baseDelay, depth };
}

function getThemeIconMarkup(mode, resolvedMode = mode) {
  const effectiveMode = mode === "system" ? resolvedMode : mode;
  if (effectiveMode === "light") {
    return `
      <circle cx="12" cy="12" r="5"></circle>
      <line x1="12" y1="1" x2="12" y2="3"></line>
      <line x1="12" y1="21" x2="12" y2="23"></line>
      <line x1="4.22" y1="4.22" x2="5.64" y2="5.64"></line>
      <line x1="18.36" y1="18.36" x2="19.78" y2="19.78"></line>
      <line x1="1" y1="12" x2="3" y2="12"></line>
      <line x1="21" y1="12" x2="23" y2="12"></line>
      <line x1="4.22" y1="19.78" x2="5.64" y2="18.36"></line>
      <line x1="18.36" y1="5.64" x2="19.78" y2="4.22"></line>
    `;
  }
  if (effectiveMode === "dark") {
    return `<path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z"></path>`;
  }
  return `
    <rect x="3" y="4" width="18" height="12" rx="2"></rect>
    <line x1="8" y1="20" x2="16" y2="20"></line>
    <line x1="12" y1="16" x2="12" y2="20"></line>
  `;
}

function updateThemeToggleButton(mode) {
  if (!el.themeToggle) return;
  const icon = el.themeToggle.querySelector(".theme-toggle-icon");
  const label = el.themeToggle.querySelector(".theme-toggle-label");
  const labels = { system: "Auto", light: "Light", dark: "Dark" };
  const text = labels[mode] || labels.system;
  const resolved = document.documentElement.dataset.resolvedTheme || "light";
  if (icon) icon.innerHTML = getThemeIconMarkup(mode, resolved);
  if (label) label.textContent = text;
  el.themeToggle.setAttribute("aria-label", `Theme: ${text}. Click to cycle.`);
  el.themeToggle.title = `Theme: ${text} (resolved ${resolved})`;
}

function resolveTheme(theme, mql) {
  return theme === "system" ? (mql.matches ? "dark" : "light") : theme;
}

function applyTheme(theme, mql) {
  const selected =
    theme === "light" || theme === "dark" || theme === "system"
      ? theme
      : "system";
  const resolved = resolveTheme(selected, mql);
  const root = document.documentElement;
  root.dataset.theme = selected;
  root.dataset.resolvedTheme = resolved;
}

function initTheme() {
  if (!el.themeToggle) return;
  const mql = window.matchMedia("(prefers-color-scheme: dark)");
  let stored = null;
  try {
    stored = localStorage.getItem(THEME_STORAGE_KEY);
  } catch {
    stored = null;
  }
  let currentTheme = THEME_MODES.includes(stored) ? stored : "system";
  applyTheme(currentTheme, mql);
  updateThemeToggleButton(currentTheme);

  el.themeToggle.addEventListener("click", () => {
    const currentIdx = THEME_MODES.indexOf(currentTheme);
    currentTheme = THEME_MODES[(currentIdx + 1) % THEME_MODES.length];
    applyTheme(currentTheme, mql);
    updateThemeToggleButton(currentTheme);
    try {
      localStorage.setItem(THEME_STORAGE_KEY, currentTheme);
    } catch {
      // Ignore storage failures (private mode / disabled storage).
    }
    state.eqUI?.draw();
  });

  mql.addEventListener("change", () => {
    if (currentTheme !== "system") return;
    applyTheme("system", mql);
    updateThemeToggleButton("system");
    state.eqUI?.draw();
  });
}

function buildStepUI() {
  for (let i = 0; i < STEP_COUNT; i += 1) {
    const step = document.createElement("div");
    step.className = "step";

    const head = document.createElement("div");
    head.className = "step-head";
    head.innerHTML = `<strong>${i + 1}</strong>`;

    const enabled = document.createElement("input");
    enabled.type = "checkbox";
    enabled.checked = i % 4 === 0;
    head.appendChild(enabled);

    const noteSelect = document.createElement("select");
    currentNotes.forEach(([label, freq], idx) => {
      const opt = document.createElement("option");
      opt.value = String(freq);
      opt.textContent = label;
      if (idx === (i % currentNotes.length)) opt.selected = true;
      noteSelect.appendChild(opt);
    });

    step.appendChild(head);
    step.appendChild(noteSelect);
    el.steps.appendChild(step);

    const stateStep = { enabled, noteSelect, node: step };
    state.steps.push(stateStep);

    enabled.addEventListener("change", syncStepsToDSP);
    noteSelect.addEventListener("change", syncStepsToDSP);
  }
}

async function ensureDSP(sampleRate) {
  if (state.dsp.ready) {
    if (Math.abs(state.dsp.sampleRate - sampleRate) > 1) {
      const initErr = state.dsp.api.init(sampleRate);
      if (typeof initErr === "string" && initErr.length > 0)
        throw new Error(initErr);
      state.dsp.sampleRate = sampleRate;
      syncTransportToDSP();
      syncWaveformToDSP();
      syncStepsToDSP();
      syncEQToDSP();
      syncEffectsToDSP();
      syncCompressorToDSP();
      syncLimiterToDSP();
      syncSpectrumToDSP();
      state.eqUI?.draw();
      state.compUI?.draw();
      state.limUI?.draw();
    }
    return;
  }
  if (typeof Go === "undefined")
    throw new Error("wasm_exec.js missing. Build wasm assets first.");

  const go = new Go();
  let result;
  try {
    result = await WebAssembly.instantiateStreaming(
      fetch("algo_dsp_demo.wasm"),
      go.importObject,
    );
  } catch {
    const response = await fetch("algo_dsp_demo.wasm");
    const bytes = await response.arrayBuffer();
    result = await WebAssembly.instantiate(bytes, go.importObject);
  }

  go.run(result.instance);

  const api = window.AlgoDSPDemo;
  if (!api) throw new Error("AlgoDSPDemo API not found after wasm init");

  const initErr = api.init(sampleRate);
  if (typeof initErr === "string" && initErr.length > 0)
    throw new Error(initErr);

  state.dsp.ready = true;
  state.dsp.api = api;
  state.dsp.go = go;
  state.dsp.sampleRate = sampleRate;

  syncTransportToDSP();
  syncWaveformToDSP();
  syncStepsToDSP();
  syncEQToDSP();
  syncEffectsToDSP();
  syncCompressorToDSP();
  syncLimiterToDSP();
  syncSpectrumToDSP();
}

async function setupAudio() {
  if (state.audioCtx) return;

  const ctx = new AudioContext();
  await ensureDSP(ctx.sampleRate);

  const node = ctx.createScriptProcessor(1024, 0, 1);
  node.onaudioprocess = (event) => {
    const out = event.outputBuffer.getChannelData(0);
    if (!state.dsp.ready || !state.dsp.api) {
      out.fill(0);
      return;
    }

    const chunk = state.dsp.api.render(out.length);
    out.set(chunk);
  };

  node.connect(ctx.destination);

  state.audioCtx = ctx;
  state.outputNode = node;
  state.eqUI?.draw();
}

function updateEQText() {
  const h = state.hoverInfo;
  if (!h) {
    el.eqReadout.textContent =
      "Hover a node for details. Mouse wheel adjusts shape (Q / bandwidth / ripple). Right-click a node to change filter type.";
    return;
  }

  const family = typeof h.family === "string" ? h.family.toUpperCase() : "RBJ";
  const orderPart = Number(h.order) > 1 ? `, Order ${Number(h.order)}` : "";
  const shape = Number.isFinite(Number(h.shape)) ? Number(h.shape) : Number(h.q);
  let shapeLabel = `Q ${shape.toFixed(2)}`;
  if (h.shapeMode === "bandwidth") shapeLabel = `Bandwidth ${shape.toFixed(1)} Hz`;
  if (h.shapeMode === "ripple") shapeLabel = `Ripple ${shape.toFixed(2)} dB`;
  el.eqReadout.textContent = `${h.label} [${family}${orderPart}]: ${Math.round(h.freq)} Hz, ${h.gain.toFixed(1)} dB, ${shapeLabel}`;
}

function stepDurationSeconds(stepIndex) {
  const base = 60 / Number(el.tempo.value) / 4;
  const ratio = shuffleRatio(Number(el.shuffle.value));
  if (ratio <= 0) return base;
  return stepIndex % 2 === 0 ? base * (1 + ratio) : base * (1 - ratio);
}

function shuffleRatio(shuffleValue) {
  const shuffle = Math.max(0, Math.min(1, shuffleValue));
  // Map 0..1 control to 0..1/3 timing ratio with a gentle curve.
  return (1 / 3) * Math.pow(shuffle, 1.6);
}

function schedule() {
  const lookahead = 0.1;
  while (state.nextNoteTime < state.audioCtx.currentTime + lookahead) {
    const stepIndex = state.currentStep;
    highlightStep(stepIndex);
    state.nextNoteTime += stepDurationSeconds(stepIndex);
    state.currentStep = (stepIndex + 1) % STEP_COUNT;
  }
}

function highlightStep(index) {
  state.steps.forEach((s, i) => {
    s.node.classList.toggle("current", i === index);
  });
}

function syncTransportToDSP() {
  if (!state.dsp.ready || !state.dsp.api) return;
  state.dsp.api.setTransport(
    Number(el.tempo.value),
    Number(el.decay.value),
    Number(el.shuffle.value),
  );
}

function syncWaveformToDSP() {
  if (!state.dsp.ready || !state.dsp.api) return;
  const waveform = String(el.waveform.value || "sine");
  state.waveform = waveform;
  state.dsp.api.setWaveform(waveform);
}

function syncStepsToDSP() {
  if (!state.dsp.ready || !state.dsp.api) return;
  const steps = state.steps.map((step) => ({
    enabled: step.enabled.checked,
    freq: Number(step.noteSelect.value),
  }));
  state.dsp.api.setSteps(steps);
}

function syncEQToDSP() {
  if (!state.dsp.ready || !state.dsp.api) return;
  const err = state.dsp.api.setEQ(state.eqParams);
  if (typeof err === "string" && err.length > 0)
    console.error("setEQ failed", err);
}

function syncEffectsToDSP() {
  if (!state.dsp.ready || !state.dsp.api) return;
  const err = state.dsp.api.setEffects(state.effectsParams);
  if (typeof err === "string" && err.length > 0)
    console.error("setEffects failed", err);
}

function syncSpectrumToDSP() {
  if (!state.dsp.ready || !state.dsp.api) return;
  const err = state.dsp.api.setSpectrum(state.analyzerParams);
  if (typeof err === "string" && err.length > 0)
    console.error("setSpectrum failed", err);
}

function readSpectrumFromUI() {
  state.analyzerParams = {
    fftSize: Number(el.analyzerFFT.value),
    overlap: Number(el.analyzerOverlap.value) / 100,
    window: String(el.analyzerWindow.value),
    smoothing: Number(el.analyzerSmoothing.value),
  };
}

function updateSpectrumText() {
  const overlapPct = Math.round(Number(el.analyzerOverlap.value));
  const hopPct = Math.max(1, 100 - overlapPct);
  el.analyzerOverlapValue.textContent = `${overlapPct}% overlap (${hopPct}% hop)`;
  el.analyzerSmoothingValue.textContent = Number(
    el.analyzerSmoothing.value,
  ).toFixed(2);
}

function spectralPitchHopSamples() {
  const frame = Number(el.spectralPitchFrame.value);
  const ratio = Number(el.spectralPitchHopRatio.value);
  const hop = Math.round(frame * ratio);
  return Math.max(1, Math.min(frame - 1, hop));
}

function readEffectsFromChain() {
  const enabled = state.chain ? state.chain.getEnabledEffects() : new Set();
  const chainState = state.chain ? state.chain.getState() : null;

  state.effectsParams = {
    ...state.effectsParams,
    chorusEnabled: enabled.has("chorus"),
    flangerEnabled: enabled.has("flanger"),
    ringModEnabled: enabled.has("ringmod"),
    bitCrusherEnabled: enabled.has("bitcrusher"),
    distortionEnabled: enabled.has("distortion"),
    transformerEnabled: enabled.has("transformer"),
    widenerEnabled: enabled.has("widener"),
    phaserEnabled: enabled.has("phaser"),
    tremoloEnabled: enabled.has("tremolo"),
    delayEnabled: enabled.has("delay"),
    timePitchEnabled: enabled.has("pitch-time"),
    spectralPitchEnabled: enabled.has("pitch-spectral"),
    expanderEnabled: enabled.has("dyn-expander"),
    deesserEnabled: enabled.has("dyn-deesser"),
    multibandEnabled: enabled.has("dyn-multiband"),
    harmonicBassEnabled: enabled.has("bass"),
    reverbEnabled: enabled.has("reverb"),
    chainGraphJSON: chainState ? JSON.stringify(chainState) : "",
  };
}

function updateEffectsText() {
  sanitizeFlangerControls();
  el.chorusMixValue.textContent = `${Math.round(Number(el.chorusMix.value) * 100)}%`;
  el.chorusDepthValue.textContent = `${(Number(el.chorusDepth.value) * 1000).toFixed(1)} ms`;
  el.chorusSpeedValue.textContent = `${Number(el.chorusSpeed.value).toFixed(2)} Hz`;
  el.chorusStagesValue.textContent = `${Number(el.chorusStages.value)}`;
  el.flangerRateValue.textContent = `${Number(el.flangerRate.value).toFixed(2)} Hz`;
  el.flangerDepthValue.textContent = `${(Number(el.flangerDepth.value) * 1000).toFixed(2)} ms`;
  el.flangerBaseDelayValue.textContent =
    `${(Number(el.flangerBaseDelay.value) * 1000).toFixed(2)} ms`;
  el.flangerFeedbackValue.textContent =
    `${Math.round(Number(el.flangerFeedback.value) * 100)}%`;
  el.flangerMixValue.textContent = `${Math.round(Number(el.flangerMix.value) * 100)}%`;
  el.ringModCarrierValue.textContent = `${Number(el.ringModCarrier.value).toFixed(1)} Hz`;
  el.ringModMixValue.textContent = `${Math.round(Number(el.ringModMix.value) * 100)}%`;
  el.bitCrusherBitsValue.textContent = `${Number(el.bitCrusherBits.value).toFixed(1)} bit`;
  el.bitCrusherDownsampleValue.textContent = `${Number(el.bitCrusherDownsample.value).toFixed(0)}x`;
  el.bitCrusherMixValue.textContent = `${Math.round(Number(el.bitCrusherMix.value) * 100)}%`;
  if (el.distortionDriveValue) {
    el.distortionDriveValue.textContent = `${Number(el.distortionDrive.value).toFixed(2)}x`;
  }
  if (el.distortionMixValue) {
    el.distortionMixValue.textContent = `${Math.round(Number(el.distortionMix.value) * 100)}%`;
  }
  if (el.distortionOutputValue) {
    el.distortionOutputValue.textContent = `${Number(el.distortionOutput.value).toFixed(2)}x`;
  }
  if (el.distortionClipValue) {
    el.distortionClipValue.textContent = Number(el.distortionClip.value).toFixed(2);
  }
  if (el.distortionShapeValue) {
    el.distortionShapeValue.textContent = Number(el.distortionShape.value).toFixed(2);
  }
  if (el.distortionBiasValue) {
    el.distortionBiasValue.textContent = Number(el.distortionBias.value).toFixed(2);
  }
  if (el.distortionChebOrderValue) {
    el.distortionChebOrderValue.textContent = `${Math.round(Number(el.distortionChebOrder.value))}`;
  }
  if (el.distortionChebGainValue) {
    el.distortionChebGainValue.textContent = `${Number(el.distortionChebGain.value).toFixed(2)}x`;
  }
  if (el.transformerDriveValue) {
    el.transformerDriveValue.textContent = `${Number(el.transformerDrive.value).toFixed(2)}x`;
  }
  if (el.transformerMixValue) {
    el.transformerMixValue.textContent = `${Math.round(Number(el.transformerMix.value) * 100)}%`;
  }
  if (el.transformerOutputValue) {
    el.transformerOutputValue.textContent = `${Number(el.transformerOutput.value).toFixed(2)}x`;
  }
  if (el.transformerHighpassValue) {
    el.transformerHighpassValue.textContent = `${Number(el.transformerHighpass.value).toFixed(0)} Hz`;
  }
  if (el.transformerDampingValue) {
    el.transformerDampingValue.textContent = `${Number(el.transformerDamping.value).toFixed(0)} Hz`;
  }
  if (el.fxFilterFreqValue) {
    el.fxFilterFreqValue.textContent = `${Number(el.fxFilterFreq.value).toFixed(0)} Hz`;
  }
  if (el.fxFilterOrderValue) {
    el.fxFilterOrderValue.textContent = `${Number(el.fxFilterOrder.value).toFixed(0)}`;
  }
  if (el.fxFilterQValue) {
    el.fxFilterQValue.textContent = Number(el.fxFilterQ.value).toFixed(2);
  }
  if (el.fxFilterGainValue) {
    el.fxFilterGainValue.textContent = `${Number(el.fxFilterGain.value).toFixed(1)} dB`;
  }
  if (el.fxCompThreshValue) {
    el.fxCompThreshValue.textContent = `${Number(el.fxCompThresh.value).toFixed(1)} dB`;
  }
  if (el.fxCompRatioValue) {
    el.fxCompRatioValue.textContent = `${Number(el.fxCompRatio.value).toFixed(1)}:1`;
  }
  if (el.fxCompKneeValue) {
    el.fxCompKneeValue.textContent = `${Number(el.fxCompKnee.value).toFixed(1)} dB`;
  }
  if (el.fxCompAttackValue) {
    el.fxCompAttackValue.textContent = `${Number(el.fxCompAttack.value).toFixed(1)} ms`;
  }
  if (el.fxCompReleaseValue) {
    el.fxCompReleaseValue.textContent = `${Number(el.fxCompRelease.value).toFixed(0)} ms`;
  }
  if (el.fxCompMakeupValue) {
    el.fxCompMakeupValue.textContent = `${Number(el.fxCompMakeup.value).toFixed(1)} dB`;
  }
  if (el.fxLimThreshValue) {
    el.fxLimThreshValue.textContent = `${Number(el.fxLimThresh.value).toFixed(1)} dB`;
  }
  if (el.fxLimReleaseValue) {
    el.fxLimReleaseValue.textContent = `${Number(el.fxLimRelease.value).toFixed(0)} ms`;
  }
  if (el.fxLookaheadThreshValue) {
    el.fxLookaheadThreshValue.textContent = `${Number(el.fxLookaheadThresh.value).toFixed(1)} dB`;
  }
  if (el.fxLookaheadReleaseValue) {
    el.fxLookaheadReleaseValue.textContent = `${Number(el.fxLookaheadRelease.value).toFixed(0)} ms`;
  }
  if (el.fxLookaheadMsValue) {
    el.fxLookaheadMsValue.textContent = `${Number(el.fxLookaheadMs.value).toFixed(1)} ms`;
  }
  if (el.fxGateThreshValue) {
    el.fxGateThreshValue.textContent = `${Number(el.fxGateThresh.value).toFixed(1)} dB`;
  }
  if (el.fxGateRatioValue) {
    el.fxGateRatioValue.textContent = `${Number(el.fxGateRatio.value).toFixed(1)}:1`;
  }
  if (el.fxGateKneeValue) {
    el.fxGateKneeValue.textContent = `${Number(el.fxGateKnee.value).toFixed(1)} dB`;
  }
  if (el.fxGateAttackValue) {
    el.fxGateAttackValue.textContent = `${Number(el.fxGateAttack.value).toFixed(1)} ms`;
  }
  if (el.fxGateHoldValue) {
    el.fxGateHoldValue.textContent = `${Number(el.fxGateHold.value).toFixed(0)} ms`;
  }
  if (el.fxGateReleaseValue) {
    el.fxGateReleaseValue.textContent = `${Number(el.fxGateRelease.value).toFixed(0)} ms`;
  }
  if (el.fxGateRangeValue) {
    el.fxGateRangeValue.textContent = `${Number(el.fxGateRange.value).toFixed(0)} dB`;
  }
  if (el.fxExpRMSValue) {
    el.fxExpRMSValue.textContent = `${Number(el.fxExpRMS.value).toFixed(0)} ms`;
  }
  if (el.fxExpThreshValue) {
    el.fxExpThreshValue.textContent = `${Number(el.fxExpThresh.value).toFixed(1)} dB`;
  }
  if (el.fxExpRatioValue) {
    el.fxExpRatioValue.textContent = `${Number(el.fxExpRatio.value).toFixed(1)}:1`;
  }
  if (el.fxExpKneeValue) {
    el.fxExpKneeValue.textContent = `${Number(el.fxExpKnee.value).toFixed(1)} dB`;
  }
  if (el.fxExpAttackValue) {
    el.fxExpAttackValue.textContent = `${Number(el.fxExpAttack.value).toFixed(1)} ms`;
  }
  if (el.fxExpReleaseValue) {
    el.fxExpReleaseValue.textContent = `${Number(el.fxExpRelease.value).toFixed(0)} ms`;
  }
  if (el.fxExpRangeValue) {
    el.fxExpRangeValue.textContent = `${Number(el.fxExpRange.value).toFixed(0)} dB`;
  }
  if (el.fxDeessFreqValue) {
    el.fxDeessFreqValue.textContent = `${Number(el.fxDeessFreq.value).toFixed(0)} Hz`;
  }
  if (el.fxDeessQValue) {
    el.fxDeessQValue.textContent = Number(el.fxDeessQ.value).toFixed(2);
  }
  if (el.fxDeessOrderValue) {
    el.fxDeessOrderValue.textContent = `${Number(el.fxDeessOrder.value).toFixed(0)}`;
  }
  if (el.fxDeessThreshValue) {
    el.fxDeessThreshValue.textContent = `${Number(el.fxDeessThresh.value).toFixed(1)} dB`;
  }
  if (el.fxDeessRatioValue) {
    el.fxDeessRatioValue.textContent = `${Number(el.fxDeessRatio.value).toFixed(1)}:1`;
  }
  if (el.fxDeessKneeValue) {
    el.fxDeessKneeValue.textContent = `${Number(el.fxDeessKnee.value).toFixed(1)} dB`;
  }
  if (el.fxDeessAttackValue) {
    el.fxDeessAttackValue.textContent = `${Number(el.fxDeessAttack.value).toFixed(2)} ms`;
  }
  if (el.fxDeessReleaseValue) {
    el.fxDeessReleaseValue.textContent = `${Number(el.fxDeessRelease.value).toFixed(0)} ms`;
  }
  if (el.fxDeessRangeValue) {
    el.fxDeessRangeValue.textContent = `${Number(el.fxDeessRange.value).toFixed(0)} dB`;
  }
  if (el.fxTransientAttackValue) {
    el.fxTransientAttackValue.textContent = Number(el.fxTransientAttack.value).toFixed(2);
  }
  if (el.fxTransientSustainValue) {
    el.fxTransientSustainValue.textContent = Number(el.fxTransientSustain.value).toFixed(2);
  }
  if (el.fxTransientAttackMsValue) {
    el.fxTransientAttackMsValue.textContent = `${Number(el.fxTransientAttackMs.value).toFixed(1)} ms`;
  }
  if (el.fxTransientReleaseMsValue) {
    el.fxTransientReleaseMsValue.textContent = `${Number(el.fxTransientReleaseMs.value).toFixed(0)} ms`;
  }
  if (el.fxMBCross1Value) {
    el.fxMBCross1Value.textContent = `${Number(el.fxMBCross1.value).toFixed(0)} Hz`;
  }
  if (el.fxMBCross2Value) {
    el.fxMBCross2Value.textContent = `${Number(el.fxMBCross2.value).toFixed(0)} Hz`;
  }
  if (el.fxMBOrderValue) {
    el.fxMBOrderValue.textContent = `${Number(el.fxMBOrder.value).toFixed(0)}`;
  }
  if (el.fxMBAttackValue) {
    el.fxMBAttackValue.textContent = `${Number(el.fxMBAttack.value).toFixed(1)} ms`;
  }
  if (el.fxMBReleaseValue) {
    el.fxMBReleaseValue.textContent = `${Number(el.fxMBRelease.value).toFixed(0)} ms`;
  }
  if (el.fxMBKneeValue) {
    el.fxMBKneeValue.textContent = `${Number(el.fxMBKnee.value).toFixed(1)} dB`;
  }
  if (el.fxMBMakeupValue) {
    el.fxMBMakeupValue.textContent = `${Number(el.fxMBMakeup.value).toFixed(1)} dB`;
  }
  if (el.fxMBLowThreshValue) {
    el.fxMBLowThreshValue.textContent = `${Number(el.fxMBLowThresh.value).toFixed(1)} dB`;
  }
  if (el.fxMBLowRatioValue) {
    el.fxMBLowRatioValue.textContent = `${Number(el.fxMBLowRatio.value).toFixed(1)}:1`;
  }
  if (el.fxMBMidThreshValue) {
    el.fxMBMidThreshValue.textContent = `${Number(el.fxMBMidThresh.value).toFixed(1)} dB`;
  }
  if (el.fxMBMidRatioValue) {
    el.fxMBMidRatioValue.textContent = `${Number(el.fxMBMidRatio.value).toFixed(1)}:1`;
  }
  if (el.fxMBHighThreshValue) {
    el.fxMBHighThreshValue.textContent = `${Number(el.fxMBHighThresh.value).toFixed(1)} dB`;
  }
  if (el.fxMBHighRatioValue) {
    el.fxMBHighRatioValue.textContent = `${Number(el.fxMBHighRatio.value).toFixed(1)}:1`;
  }
  if (el.splitFreqValue) {
    el.splitFreqValue.textContent = `${Number(el.splitFreq.value).toFixed(0)} Hz`;
  }
  el.widenerWidthValue.textContent = `${Number(el.widenerWidth.value).toFixed(2)}x`;
  el.widenerMixValue.textContent = `${Math.round(Number(el.widenerMix.value) * 100)}%`;
  el.phaserRateValue.textContent = `${Number(el.phaserRate.value).toFixed(2)} Hz`;
  el.phaserMinFreqValue.textContent = `${Number(el.phaserMinFreq.value).toFixed(0)} Hz`;
  el.phaserMaxFreqValue.textContent = `${Number(el.phaserMaxFreq.value).toFixed(0)} Hz`;
  el.phaserStagesValue.textContent = `${Number(el.phaserStages.value)}`;
  el.phaserFeedbackValue.textContent =
    `${Math.round(Number(el.phaserFeedback.value) * 100)}%`;
  el.phaserMixValue.textContent = `${Math.round(Number(el.phaserMix.value) * 100)}%`;
  el.tremoloRateValue.textContent = `${Number(el.tremoloRate.value).toFixed(2)} Hz`;
  el.tremoloDepthValue.textContent = `${Math.round(Number(el.tremoloDepth.value) * 100)}%`;
  el.tremoloSmoothingValue.textContent =
    `${Number(el.tremoloSmoothing.value).toFixed(1)} ms`;
  el.tremoloMixValue.textContent = `${Math.round(Number(el.tremoloMix.value) * 100)}%`;
  el.delayTimeValue.textContent = `${(Number(el.delayTime.value) * 1000).toFixed(0)} ms`;
  el.delayFeedbackValue.textContent = `${Math.round(Number(el.delayFeedback.value) * 100)}%`;
  el.delayMixValue.textContent = `${Math.round(Number(el.delayMix.value) * 100)}%`;
  if (el.simpleDelayMsValue) {
    el.simpleDelayMsValue.textContent = `${Number(el.simpleDelayMs.value).toFixed(0)} ms`;
  }
  el.timePitchSemitonesValue.textContent = `${Number(el.timePitchSemitones.value).toFixed(1)} st`;
  el.timePitchSequenceValue.textContent = `${Number(el.timePitchSequence.value).toFixed(0)} ms`;
  el.timePitchOverlapValue.textContent = `${Number(el.timePitchOverlap.value).toFixed(0)} ms`;
  el.timePitchSearchValue.textContent = `${Number(el.timePitchSearch.value).toFixed(0)} ms`;
  el.spectralPitchSemitonesValue.textContent =
    `${Number(el.spectralPitchSemitones.value).toFixed(1)} st`;
  const spectralFrame = Number(el.spectralPitchFrame.value);
  const spectralRatio = Number(el.spectralPitchHopRatio.value);
  const spectralHop = spectralPitchHopSamples();
  el.spectralPitchFrameValue.textContent = `${spectralFrame} samples`;
  el.spectralPitchHopRatioValue.textContent =
    `${spectralHop} samples (${Math.round(spectralRatio * 100)}%)`;
  el.harmonicFrequencyValue.textContent = `${Number(el.harmonicFrequency.value).toFixed(0)} Hz`;
  el.harmonicInputValue.textContent = Number(el.harmonicInput.value).toFixed(2);
  el.harmonicHighValue.textContent = Number(el.harmonicHigh.value).toFixed(2);
  el.harmonicOriginalValue.textContent = Number(el.harmonicOriginal.value).toFixed(2);
  el.harmonicHarmonicValue.textContent = Number(el.harmonicHarmonic.value).toFixed(2);
  el.harmonicDecayValue.textContent = Number(el.harmonicDecay.value).toFixed(2);
  el.harmonicResponseValue.textContent = `${Number(el.harmonicResponse.value).toFixed(0)} ms`;
  if (el.harmonicHighpassValue) {
    const mode = Number(el.harmonicHighpass.value);
    const labels = ["DC", "1st Order", "2nd Order"];
    el.harmonicHighpassValue.textContent = labels[mode] || "DC";
  }
  el.reverbWetValue.textContent = `${Math.round(Number(el.reverbWet.value) * 100)}%`;
  el.reverbDryValue.textContent = Number(el.reverbDry.value).toFixed(2);
  el.reverbRoomValue.textContent = Number(el.reverbRoom.value).toFixed(2);
  el.reverbDampValue.textContent = Number(el.reverbDamp.value).toFixed(2);
  if (el.reverbRT60Value) {
    el.reverbRT60Value.textContent = `${Number(el.reverbRT60.value).toFixed(2)} s`;
  }
  if (el.reverbPreDelayValue) {
    el.reverbPreDelayValue.textContent = `${(Number(el.reverbPreDelay.value) * 1000).toFixed(1)} ms`;
  }
  if (el.reverbModDepthValue) {
    el.reverbModDepthValue.textContent = `${(Number(el.reverbModDepth.value) * 1000).toFixed(1)} ms`;
  }
  if (el.reverbModRateValue) {
    el.reverbModRateValue.textContent = `${Number(el.reverbModRate.value).toFixed(2)} Hz`;
  }
  updateReverbModelUI();
}

function updateReverbModelUI() {
  const model = el.reverbModel?.value || "freeverb";
  const fdnVisible = model === "fdn";
  document.querySelectorAll(".reverb-fdn").forEach((node) => {
    node.hidden = !fdnVisible;
  });
  document.querySelectorAll(".reverb-freeverb").forEach((node) => {
    node.hidden = fdnVisible;
  });
}

function syncCompressorToDSP() {
  if (!state.dsp.ready || !state.dsp.api) return;
  const err = state.dsp.api.setCompressor(state.compParams);
  if (typeof err === "string" && err.length > 0)
    console.error("setCompressor failed", err);
}

function readCompressorFromUI() {
  state.compParams = {
    enabled: el.compEnabled.checked,
    thresholdDB: Number(el.compThresh.value),
    ratio: Number(el.compRatio.value),
    kneeDB: Number(el.compKnee.value),
    attackMs: Number(el.compAttack.value),
    releaseMs: Number(el.compRelease.value),
    autoMakeup: el.compAuto.checked,
    makeupGainDB: Number(el.compMakeup.value),
  };
}

function updateCompressorText() {
  el.compThreshValue.textContent = `${Number(el.compThresh.value).toFixed(1)} dB`;
  el.compRatioValue.textContent = `${Number(el.compRatio.value).toFixed(1)}:1`;
  el.compKneeValue.textContent = `${Number(el.compKnee.value).toFixed(1)} dB`;
  el.compAttackValue.textContent = `${Number(el.compAttack.value).toFixed(1)} ms`;
  el.compReleaseValue.textContent = `${Number(el.compRelease.value).toFixed(0)} ms`;
  el.compMakeupValue.textContent = `${Number(el.compMakeup.value).toFixed(1)} dB`;

  if (el.compAuto.checked) {
    el.compMakeup.disabled = true;
    el.compMakeupValue.style.opacity = "0.5";
  } else {
    el.compMakeup.disabled = false;
    el.compMakeupValue.style.opacity = "1";
  }

  state.compUI?.draw();
}

function syncLimiterToDSP() {
  if (!state.dsp.ready || !state.dsp.api) return;
  const err = state.dsp.api.setLimiter(state.limParams);
  if (typeof err === "string" && err.length > 0)
    console.error("setLimiter failed", err);
}

function readLimiterFromUI() {
  state.limParams = {
    enabled: el.limEnabled.checked,
    threshold: Number(el.limThresh.value),
    release: Number(el.limRelease.value),
  };
}

function updateLimiterText() {
  el.limThreshValue.textContent = `${Number(el.limThresh.value).toFixed(1)} dB`;
  el.limReleaseValue.textContent = `${Number(el.limRelease.value).toFixed(0)} ms`;

  state.limUI?.draw();
}

function startSequencer() {
  if (!state.audioCtx) return;
  if (state.audioCtx.state === "suspended") state.audioCtx.resume();
  if (state.isRunning) return;

  state.isRunning = true;
  state.currentStep = 0;
  state.nextNoteTime = state.audioCtx.currentTime + 0.05;
  state.scheduler = setInterval(schedule, 25);
  if (state.dsp.ready && state.dsp.api) state.dsp.api.setRunning(true);
  const sr = el.runToggle.querySelector(".sr-only");
  if (sr) sr.textContent = "Stop";
  el.runToggle.setAttribute("aria-label", "Stop");
  el.runToggle.classList.add("active");
}

function stopSequencer() {
  if (!state.isRunning) return;
  clearInterval(state.scheduler);
  state.scheduler = null;
  state.isRunning = false;
  if (state.dsp.ready && state.dsp.api) state.dsp.api.setRunning(false);
  const sr = el.runToggle.querySelector(".sr-only");
  if (sr) sr.textContent = "Play";
  el.runToggle.setAttribute("aria-label", "Play");
  el.runToggle.classList.remove("active");
  highlightStep(-1);
}

function initEQCanvas() {
  state.eqUI = new window.EQCanvas(el.eqCanvas, {
    initialParams: state.eqParams,
    onChange: (params) => {
      state.eqParams = { ...params };
      syncEQToDSP();
      updateEQText();
    },
    onHover: (info) => {
      state.hoverInfo = info;
      updateEQText();
    },
    getSampleRate: () => state.audioCtx?.sampleRate ?? 48000,
    getResponseDB: (freqs) => {
      if (!state.dsp.ready || !state.dsp.api) return null;
      return state.dsp.api.responseCurve(freqs);
    },
    getNodeResponseDB: (key, freqs) => {
      if (!state.dsp.ready || !state.dsp.api) return null;
      return state.dsp.api.nodeResponseCurve(key, freqs);
    },
    getSpectrumDB: (freqs) => {
      if (!state.dsp.ready || !state.dsp.api) return null;
      return state.dsp.api.spectrumCurve(freqs);
    },
  });
}

function startEQDrawLoop() {
  if (state.eqDrawLoopHandle !== null) return;
  const targetFrameMS = 1000 / 24;

  const tick = (now) => {
    if (state.eqUI && now - state.eqLastDrawTimeMS >= targetFrameMS) {
      state.eqUI.draw();
      state.eqLastDrawTimeMS = now;
    }
    state.eqDrawLoopHandle = requestAnimationFrame(tick);
  };

  state.eqDrawLoopHandle = requestAnimationFrame(tick);
}

function initDynamicsGraphs() {
  state.compUI = new window.DynamicsGraph(el.compGraph, {
    type: "compressor",
    getParams: () => state.compParams,
    getCurve: (inputs) => {
      if (!state.dsp.ready || !state.dsp.api) return null;
      return state.dsp.api.compressorCurve(inputs);
    },
  });
  state.limUI = new window.DynamicsGraph(el.limGraph, {
    type: "limiter",
    getParams: () => state.limParams,
    getCurve: (inputs) => {
      if (!state.dsp.ready || !state.dsp.api) return null;
      return state.dsp.api.limiterCurve(inputs);
    },
  });
}

function updateStepOptions() {
  currentNotes = generateNotes(el.rootNote.value, el.scale.value);
  state.steps.forEach((step, i) => {
    const prevIndex = step.noteSelect.selectedIndex;
    step.noteSelect.innerHTML = "";
    currentNotes.forEach(([label, freq]) => {
      const opt = document.createElement("option");
      opt.value = String(freq);
      opt.textContent = label;
      step.noteSelect.appendChild(opt);
    });
    if (prevIndex >= 0 && prevIndex < currentNotes.length) {
      step.noteSelect.selectedIndex = prevIndex;
    } else {
      step.noteSelect.selectedIndex = i % currentNotes.length;
    }
  });
  syncStepsToDSP();
}

function randomizeSteps() {
  const intervals = SCALES[el.scale.value] || SCALES.pentatonic;
  const hasFifth = intervals.includes(7);
  
  // Find indices in currentNotes for root and fifth (using octave 3 as base)
  const rootIndex = 0; // First note in currentNotes is root octave 3
  let fifthIndex = -1;
  if (hasFifth) {
    fifthIndex = intervals.indexOf(7);
  }

  state.steps.forEach((step, i) => {
    // 1-indexed steps: 1, 5, 9, 13
    // 0-indexed: 0, 4, 8, 12
    if (i === 0 || i === 8) {
      step.enabled.checked = true;
      step.noteSelect.selectedIndex = rootIndex;
    } else if ((i === 4 || i === 12) && hasFifth) {
      step.enabled.checked = true;
      step.noteSelect.selectedIndex = fifthIndex;
    } else {
      // Randomize other steps
      step.enabled.checked = Math.random() > 0.6; // ~40% chance to be enabled
      step.noteSelect.selectedIndex = Math.floor(Math.random() * currentNotes.length);
    }
  });
  syncStepsToDSP();
}

function bindEvents() {
  el.runToggle.addEventListener("click", async () => {
    if (!state.audioCtx) {
      try {
        await setupAudio();
      } catch (err) {
        console.error(err);
        return;
      }
    }
    if (state.isRunning) stopSequencer();
    else startSequencer();
  });

  el.scale.addEventListener("change", updateStepOptions);
  el.rootNote.addEventListener("change", updateStepOptions);
  el.randomizeSteps.addEventListener("click", randomizeSteps);

  [el.tempo, el.decay, el.shuffle].forEach((control) => {
    control.addEventListener("input", () => {
      el.tempoValue.textContent = `${Number(el.tempo.value)} BPM`;
      el.decayValue.textContent = `${Number(el.decay.value).toFixed(2)} s`;
      el.shuffleValue.textContent = `${Math.round(Number(el.shuffle.value) * 100)}%`;
      syncTransportToDSP();
    });
  });

  el.waveform.addEventListener("change", () => {
    syncWaveformToDSP();
  });

  // Effects parameter sliders (enable state is driven by the chain graph).
  [
    el.chorusMix,
    el.chorusDepth,
    el.chorusSpeed,
    el.chorusStages,
    el.flangerRate,
    el.flangerDepth,
    el.flangerBaseDelay,
    el.flangerFeedback,
    el.flangerMix,
    el.ringModCarrier,
    el.ringModMix,
    el.bitCrusherBits,
    el.bitCrusherDownsample,
    el.bitCrusherMix,
    el.distortionMode,
    el.distortionApprox,
    el.distortionDrive,
    el.distortionMix,
    el.distortionOutput,
    el.distortionClip,
    el.distortionShape,
    el.distortionBias,
    el.distortionChebOrder,
    el.distortionChebHarmonic,
    el.distortionChebInvert,
    el.distortionChebGain,
    el.distortionChebDCBypass,
    el.transformerQuality,
    el.transformerDrive,
    el.transformerMix,
    el.transformerOutput,
    el.transformerHighpass,
    el.transformerDamping,
    el.transformerOversampling,
    el.fxFilterFamily,
    el.fxFilterKind,
    el.fxFilterOrder,
    el.fxFilterFreq,
    el.fxFilterQ,
    el.fxFilterGain,
    el.fxCompThresh,
    el.fxCompRatio,
    el.fxCompKnee,
    el.fxCompAttack,
    el.fxCompRelease,
    el.fxCompMakeup,
    el.fxLimThresh,
    el.fxLimRelease,
    el.fxLookaheadThresh,
    el.fxLookaheadRelease,
    el.fxLookaheadMs,
    el.fxGateMode,
    el.fxGateThresh,
    el.fxGateRatio,
    el.fxGateKnee,
    el.fxGateAttack,
    el.fxGateHold,
    el.fxGateRelease,
    el.fxGateRange,
    el.fxExpTopology,
    el.fxExpDetector,
    el.fxExpRMS,
    el.fxExpThresh,
    el.fxExpRatio,
    el.fxExpKnee,
    el.fxExpAttack,
    el.fxExpRelease,
    el.fxExpRange,
    el.fxDeessMode,
    el.fxDeessDetector,
    el.fxDeessListen,
    el.fxDeessFreq,
    el.fxDeessQ,
    el.fxDeessOrder,
    el.fxDeessThresh,
    el.fxDeessRatio,
    el.fxDeessKnee,
    el.fxDeessAttack,
    el.fxDeessRelease,
    el.fxDeessRange,
    el.fxTransientAttack,
    el.fxTransientSustain,
    el.fxTransientAttackMs,
    el.fxTransientReleaseMs,
    el.fxMBBands,
    el.fxMBCross1,
    el.fxMBCross2,
    el.fxMBOrder,
    el.fxMBAttack,
    el.fxMBRelease,
    el.fxMBKnee,
    el.fxMBAutoMakeup,
    el.fxMBMakeup,
    el.fxMBLowThresh,
    el.fxMBLowRatio,
    el.fxMBMidThresh,
    el.fxMBMidRatio,
    el.fxMBHighThresh,
    el.fxMBHighRatio,
    el.splitFreq,
    el.widenerWidth,
    el.widenerMix,
    el.phaserRate,
    el.phaserMinFreq,
    el.phaserMaxFreq,
    el.phaserStages,
    el.phaserFeedback,
    el.phaserMix,
    el.tremoloRate,
    el.tremoloDepth,
    el.tremoloSmoothing,
    el.tremoloMix,
    el.delayTime,
    el.delayFeedback,
    el.delayMix,
    el.simpleDelayMs,
    el.timePitchSemitones,
    el.timePitchSequence,
    el.timePitchOverlap,
    el.timePitchSearch,
    el.spectralPitchSemitones,
    el.spectralPitchFrame,
    el.spectralPitchHopRatio,
    el.harmonicFrequency,
    el.harmonicInput,
    el.harmonicHigh,
    el.harmonicOriginal,
    el.harmonicHarmonic,
    el.harmonicDecay,
    el.harmonicResponse,
    el.harmonicHighpass,
    el.reverbModel,
    el.reverbWet,
    el.reverbDry,
    el.reverbRoom,
    el.reverbDamp,
    el.reverbRT60,
    el.reverbPreDelay,
    el.reverbModDepth,
    el.reverbModRate,
  ].forEach((control) => {
    const eventName =
      control.tagName === "SELECT" ? "change" : "input";
    control.addEventListener(eventName, () => {
      updateEffectsText();
      const committed = commitSelectedNodeParamsFromUI();
      if (!committed) {
        readEffectsFromChain();
        syncEffectsToDSP();
        saveSettings();
      }
    });
  });

  [
    el.compEnabled,
    el.compThresh,
    el.compRatio,
    el.compKnee,
    el.compAttack,
    el.compRelease,
    el.compAuto,
    el.compMakeup,
  ].forEach((control) => {
    const eventName = control.type === "checkbox" ? "change" : "input";
    control.addEventListener(eventName, () => {
      readCompressorFromUI();
      updateCompressorText();
      syncCompressorToDSP();
      saveSettings();
    });
  });

  [el.limEnabled, el.limThresh, el.limRelease].forEach((control) => {
    const eventName = control.type === "checkbox" ? "change" : "input";
    control.addEventListener(eventName, () => {
      readLimiterFromUI();
      updateLimiterText();
      syncLimiterToDSP();
      saveSettings();
    });
  });

  [el.analyzerFFT, el.analyzerWindow].forEach((control) => {
    control.addEventListener("change", () => {
      readSpectrumFromUI();
      updateSpectrumText();
      syncSpectrumToDSP();
    });
  });

  [el.analyzerOverlap, el.analyzerSmoothing].forEach((control) => {
    control.addEventListener("input", () => {
      readSpectrumFromUI();
      updateSpectrumText();
      syncSpectrumToDSP();
    });
  });

  el.tempoValue.textContent = `${Number(el.tempo.value)} BPM`;
  el.decayValue.textContent = `${Number(el.decay.value).toFixed(2)} s`;
  el.shuffleValue.textContent = `${Math.round(Number(el.shuffle.value) * 100)}%`;
  el.waveform.value = state.waveform;
  updateEffectsText();
  readEffectsFromChain();
  updateCompressorText();
  readCompressorFromUI();
  updateLimiterText();
  readLimiterFromUI();
  el.analyzerFFT.value = String(state.analyzerParams.fftSize);
  el.analyzerOverlap.value = String(
    Math.round(state.analyzerParams.overlap * 100),
  );
  el.analyzerWindow.value = state.analyzerParams.window;
  el.analyzerSmoothing.value = String(state.analyzerParams.smoothing);
  readSpectrumFromUI();
  updateSpectrumText();
  updateEQText();

  loadSettings();
}

// ---- Effect Chain Canvas Initialisation ----

function initEffectChain() {
  state.chain = new window.EffectChain(el.chainCanvas, {
    createParams: (type) => defaultNodeParams(type),
    onChange: () => {
      readEffectsFromChain();
      syncEffectsToDSP();
      saveSettings();
      // Keep detail panel in sync when canvas slider is dragged
      const sel = state.chain?.getSelectedNode?.();
      if (sel && !sel.fixed) {
        applyNodeParamsToUI(sel);
      }
    },
    onSelect: (node) => {
      showChainDetail(node);
    },
  });
}

/** Show/hide the detail panel for the selected chain node. */
function showChainDetail(node) {
  const detail = el.chainDetail;
  if (!node || node.fixed) {
    detail.hidden = true;
    return;
  }
  // Map node type to the data-chain-detail attribute
  const type = node.type;
  document.querySelectorAll("[data-chain-detail]").forEach((card) => {
    card.hidden = card.dataset.chainDetail !== type;
  });
  applyNodeParamsToUI(node);
  updatePinButtonStates(node);
  detail.hidden = false;
  updateEffectsText();
}

// ---- Pin-to-block buttons for chain detail sliders ----

// Maps HTML input element IDs to { effectType, paramKey }.
const PIN_MAP = {
  "chorus-mix": { type: "chorus", param: "mix" },
  "chorus-depth": { type: "chorus", param: "depth" },
  "chorus-speed": { type: "chorus", param: "speedHz" },
  "chorus-stages": { type: "chorus", param: "stages" },
  "flanger-rate": { type: "flanger", param: "rateHz" },
  "flanger-depth": { type: "flanger", param: "depth" },
  "flanger-base-delay": { type: "flanger", param: "baseDelay" },
  "flanger-feedback": { type: "flanger", param: "feedback" },
  "flanger-mix": { type: "flanger", param: "mix" },
  "ringmod-carrier": { type: "ringmod", param: "carrierHz" },
  "ringmod-mix": { type: "ringmod", param: "mix" },
  "bitcrusher-bits": { type: "bitcrusher", param: "bitDepth" },
  "bitcrusher-downsample": { type: "bitcrusher", param: "downsample" },
  "bitcrusher-mix": { type: "bitcrusher", param: "mix" },
  "distortion-drive": { type: "distortion", param: "drive" },
  "distortion-mix": { type: "distortion", param: "mix" },
  "distortion-output": { type: "distortion", param: "output" },
  "distortion-clip": { type: "distortion", param: "clip" },
  "distortion-shape": { type: "distortion", param: "shape" },
  "distortion-bias": { type: "distortion", param: "bias" },
  "transformer-drive": { type: "transformer", param: "drive" },
  "transformer-mix": { type: "transformer", param: "mix" },
  "transformer-output": { type: "transformer", param: "output" },
  "transformer-highpass": { type: "transformer", param: "highpassHz" },
  "transformer-damping": { type: "transformer", param: "dampingHz" },
  "fx-filter-freq": { type: "filter", param: "freq" },
  "fx-filter-q": { type: "filter", param: "q" },
  "fx-filter-gain": { type: "filter", param: "gain" },
  "fx-filter-order": { type: "filter", param: "order" },
  "fx-comp-thresh": { type: "dyn-compressor", param: "thresholdDB" },
  "fx-comp-ratio": { type: "dyn-compressor", param: "ratio" },
  "fx-comp-knee": { type: "dyn-compressor", param: "kneeDB" },
  "fx-comp-attack": { type: "dyn-compressor", param: "attackMs" },
  "fx-comp-release": { type: "dyn-compressor", param: "releaseMs" },
  "fx-comp-makeup": { type: "dyn-compressor", param: "makeupGainDB" },
  "fx-lim-thresh": { type: "dyn-limiter", param: "thresholdDB" },
  "fx-lim-release": { type: "dyn-limiter", param: "releaseMs" },
  "fx-lookahead-thresh": { type: "dyn-lookahead", param: "thresholdDB" },
  "fx-lookahead-release": { type: "dyn-lookahead", param: "releaseMs" },
  "fx-lookahead-ms": { type: "dyn-lookahead", param: "lookaheadMs" },
  "fx-gate-thresh": { type: "dyn-gate", param: "thresholdDB" },
  "fx-gate-ratio": { type: "dyn-gate", param: "ratio" },
  "fx-gate-knee": { type: "dyn-gate", param: "kneeDB" },
  "fx-gate-attack": { type: "dyn-gate", param: "attackMs" },
  "fx-gate-hold": { type: "dyn-gate", param: "holdMs" },
  "fx-gate-release": { type: "dyn-gate", param: "releaseMs" },
  "fx-gate-range": { type: "dyn-gate", param: "rangeDB" },
  "fx-exp-rms": { type: "dyn-expander", param: "rmsWindowMs" },
  "fx-exp-thresh": { type: "dyn-expander", param: "thresholdDB" },
  "fx-exp-ratio": { type: "dyn-expander", param: "ratio" },
  "fx-exp-knee": { type: "dyn-expander", param: "kneeDB" },
  "fx-exp-attack": { type: "dyn-expander", param: "attackMs" },
  "fx-exp-release": { type: "dyn-expander", param: "releaseMs" },
  "fx-exp-range": { type: "dyn-expander", param: "rangeDB" },
  "fx-deess-freq": { type: "dyn-deesser", param: "freqHz" },
  "fx-deess-q": { type: "dyn-deesser", param: "q" },
  "fx-deess-thresh": { type: "dyn-deesser", param: "thresholdDB" },
  "fx-deess-ratio": { type: "dyn-deesser", param: "ratio" },
  "fx-deess-knee": { type: "dyn-deesser", param: "kneeDB" },
  "fx-deess-attack": { type: "dyn-deesser", param: "attackMs" },
  "fx-deess-release": { type: "dyn-deesser", param: "releaseMs" },
  "fx-deess-range": { type: "dyn-deesser", param: "rangeDB" },
  "fx-transient-attack": { type: "dyn-transient", param: "attack" },
  "fx-transient-sustain": { type: "dyn-transient", param: "sustain" },
  "fx-transient-attack-ms": { type: "dyn-transient", param: "attackMs" },
  "fx-transient-release-ms": { type: "dyn-transient", param: "releaseMs" },
  "fx-mb-cross1": { type: "dyn-multiband", param: "cross1Hz" },
  "fx-mb-cross2": { type: "dyn-multiband", param: "cross2Hz" },
  "fx-mb-attack": { type: "dyn-multiband", param: "attackMs" },
  "fx-mb-release": { type: "dyn-multiband", param: "releaseMs" },
  "fx-mb-knee": { type: "dyn-multiband", param: "kneeDB" },
  "fx-mb-makeup": { type: "dyn-multiband", param: "makeupGainDB" },
  "fx-mb-low-thresh": { type: "dyn-multiband", param: "lowThresholdDB" },
  "fx-mb-low-ratio": { type: "dyn-multiband", param: "lowRatio" },
  "fx-mb-mid-thresh": { type: "dyn-multiband", param: "midThresholdDB" },
  "fx-mb-mid-ratio": { type: "dyn-multiband", param: "midRatio" },
  "fx-mb-high-thresh": { type: "dyn-multiband", param: "highThresholdDB" },
  "fx-mb-high-ratio": { type: "dyn-multiband", param: "highRatio" },
  "split-freq": { type: "split-freq", param: "freqHz" },
  "widener-width": { type: "widener", param: "width" },
  "widener-mix": { type: "widener", param: "mix" },
  "phaser-rate": { type: "phaser", param: "rateHz" },
  "phaser-min-freq": { type: "phaser", param: "minFreqHz" },
  "phaser-max-freq": { type: "phaser", param: "maxFreqHz" },
  "phaser-stages": { type: "phaser", param: "stages" },
  "phaser-feedback": { type: "phaser", param: "feedback" },
  "phaser-mix": { type: "phaser", param: "mix" },
  "tremolo-rate": { type: "tremolo", param: "rateHz" },
  "tremolo-depth": { type: "tremolo", param: "depth" },
  "tremolo-smoothing": { type: "tremolo", param: "smoothingMs" },
  "tremolo-mix": { type: "tremolo", param: "mix" },
  "delay-time": { type: "delay", param: "time" },
  "delay-feedback": { type: "delay", param: "feedback" },
  "delay-mix": { type: "delay", param: "mix" },
  "simple-delay-ms": { type: "delay-simple", param: "delayMs" },
  "harmonic-frequency": { type: "bass", param: "frequency" },
  "harmonic-input": { type: "bass", param: "inputGain" },
  "harmonic-high": { type: "bass", param: "highGain" },
  "harmonic-original": { type: "bass", param: "original" },
  "harmonic-harmonic": { type: "bass", param: "harmonic" },
  "harmonic-decay": { type: "bass", param: "decay" },
  "harmonic-response": { type: "bass", param: "responseMs" },
  "time-pitch-semitones": { type: "pitch-time", param: "semitones" },
  "time-pitch-sequence": { type: "pitch-time", param: "sequence" },
  "time-pitch-overlap": { type: "pitch-time", param: "overlap" },
  "time-pitch-search": { type: "pitch-time", param: "search" },
  "spectral-pitch-semitones": { type: "pitch-spectral", param: "semitones" },
  "reverb-wet": { type: "reverb", param: "wet" },
  "reverb-dry": { type: "reverb", param: "dry" },
  "reverb-room": { type: "reverb", param: "roomSize" },
  "reverb-damp": { type: "reverb", param: "damp" },
  "reverb-rt60": { type: "reverb", param: "rt60" },
  "reverb-predelay": { type: "reverb", param: "preDelay" },
  "reverb-mod-depth": { type: "reverb", param: "modDepth" },
  "reverb-mod-rate": { type: "reverb", param: "modRate" },
};

const PIN_SVG = `<svg viewBox="0 0 16 16" xmlns="http://www.w3.org/2000/svg"><path d="M9.828 1.172a2 2 0 0 1 2.828 0l2.172 2.172a2 2 0 0 1 0 2.828l-3.5 3.5a1 1 0 0 1-.354.232l-2 .75a1 1 0 0 1-1.118-.226L6.5 9.072 2.354 13.22a.5.5 0 0 1-.708-.708L5.8 8.358 4.572 7.146a1 1 0 0 1-.226-1.118l.75-2a1 1 0 0 1 .232-.354l3.5-3.5z"/></svg>`;

function initPinButtons() {
  for (const [inputId, { param }] of Object.entries(PIN_MAP)) {
    const input = document.getElementById(inputId);
    if (!input) continue;
    const label = input.closest("label");
    if (!label) continue;
    const btn = document.createElement("button");
    btn.className = "pin-param";
    btn.dataset.inputId = inputId;
    btn.dataset.param = param;
    btn.title = "Pin to block";
    btn.innerHTML = PIN_SVG;
    btn.addEventListener("click", (e) => {
      e.preventDefault();
      e.stopPropagation();
      const node = getSelectedEffectNode();
      if (!node || !state.chain) return;
      if (state.chain.isPinned(node.id, param)) {
        state.chain.unpinParam(node.id, param);
        btn.classList.remove("pinned");
      } else {
        state.chain.pinParam(node.id, param);
        btn.classList.add("pinned");
      }
    });
    label.appendChild(btn);
  }
}

/** Update pin button states when a node's detail panel is shown. */
function updatePinButtonStates(node) {
  if (!node) return;
  document.querySelectorAll(".pin-param").forEach((btn) => {
    const mapping = PIN_MAP[btn.dataset.inputId];
    if (!mapping || mapping.type !== node.type) return;
    const pinned = state.chain?.isPinned(node.id, mapping.param);
    btn.classList.toggle("pinned", !!pinned);
  });
}

buildStepUI();
initDynamicsGraphs();
initEQCanvas();
startEQDrawLoop();
initTheme();
initEffectChain();
initPinButtons();
bindEvents();
ensureDSP(48000)
  .then(() => {
    state.eqUI?.draw();
    state.compUI?.draw();
    state.limUI?.draw();
  })
  .catch((err) => console.error(err));
