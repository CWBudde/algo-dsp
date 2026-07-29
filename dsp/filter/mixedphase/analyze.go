package mixedphase

import (
	"fmt"
	"math"
	"math/cmplx"
)

// Analyze compares a candidate FIR with a reference magnitude response and
// measures the candidate's temporal energy distribution.
//
// fftSize may be zero to select a dense grid automatically.
func Analyze(reference, candidate []float64, fftSize int) (Metrics, error) {
	if len(reference) == 0 {
		return Metrics{}, ErrEmptyPrototype
	}

	if len(candidate) == 0 {
		return Metrics{}, ErrInvalidLength
	}

	size, err := nextDesignFFTSize(max(len(reference), len(candidate)), fftSize)
	if err != nil {
		return Metrics{}, err
	}

	w, err := newFFTWorkspace(size)
	if err != nil {
		return Metrics{}, err
	}

	referenceSpectrum, err := w.forwardReal(reference)
	if err != nil {
		return Metrics{}, err
	}

	return analyzeAgainstSpectrum(w, referenceSpectrum, candidate)
}

func analyzeAgainstSpectrum(
	w *fftWorkspace,
	referenceSpectrum []complex128,
	candidate []float64,
) (Metrics, error) {
	candidateSpectrum, err := w.forwardReal(candidate)
	if err != nil {
		return Metrics{}, err
	}

	peakMagnitude := 0.0
	for _, value := range referenceSpectrum {
		peakMagnitude = max(peakMagnitude, cmplx.Abs(value))
	}

	if peakMagnitude == 0 {
		return Metrics{}, fmt.Errorf(
			"mixedphase: reference magnitude is identically zero",
		)
	}

	floor := peakMagnitude * 1e-6
	sumSquared := 0.0
	maxError := 0.0
	linearErrorSquared := 0.0
	referenceSquared := 0.0
	binCount := w.size/2 + 1

	for i := 0; i < binCount; i++ {
		referenceMagnitudeLinear := cmplx.Abs(referenceSpectrum[i])
		candidateMagnitudeLinear := cmplx.Abs(candidateSpectrum[i])
		referenceMagnitude := max(referenceMagnitudeLinear, floor)
		candidateMagnitude := max(candidateMagnitudeLinear, floor)
		errorDB := 20 * math.Log10(candidateMagnitude/referenceMagnitude)
		linearError := candidateMagnitudeLinear - referenceMagnitudeLinear

		sumSquared += errorDB * errorDB
		maxError = max(maxError, math.Abs(errorDB))
		linearErrorSquared += linearError * linearError
		referenceSquared += referenceMagnitudeLinear * referenceMagnitudeLinear
	}

	peakIndex := 0
	peak := 0.0
	totalEnergy := 0.0
	weightedEnergy := 0.0

	for i, value := range candidate {
		absolute := math.Abs(value)
		if absolute > peak {
			peak = absolute
			peakIndex = i
		}

		energy := value * value
		totalEnergy += energy
		weightedEnergy += float64(i) * energy
	}

	prePeakEnergy := 0.0
	for _, value := range candidate[:peakIndex] {
		prePeakEnergy += value * value
	}

	energyCentroid := 0.0
	prePeakRatio := 0.0

	if totalEnergy > 0 {
		energyCentroid = weightedEnergy / totalEnergy
		prePeakRatio = prePeakEnergy / totalEnergy
	}

	return Metrics{
		RMSMagnitudeErrorDB:    math.Sqrt(sumSquared / float64(binCount)),
		MaxMagnitudeErrorDB:    maxError,
		RelativeMagnitudeError: math.Sqrt(linearErrorSquared / referenceSquared),
		PeakIndex:              peakIndex,
		EnergyCentroid:         energyCentroid,
		PrePeakEnergyRatio:     prePeakRatio,
	}, nil
}
