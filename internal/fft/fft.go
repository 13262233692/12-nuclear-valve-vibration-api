package fft

import (
	"errors"
	"math"
	"math/cmplx"

	"github.com/mjibson/go-dsp/fft"
	"github.com/mjibson/go-dsp/window"

	"nuclear-valve-vibration-api/internal/config"
	"nuclear-valve-vibration-api/internal/model"
)

type BandRange struct {
	Low  float64
	High float64
	Name string
}

var StandardBands = []BandRange{
	{Low: 0, High: 10, Name: "ultra_low"},
	{Low: 10, High: 50, Name: "low"},
	{Low: 50, High: 200, Name: "medium"},
	{Low: 200, High: 1000, Name: "high"},
	{Low: 1000, High: 5000, Name: "very_high"},
	{Low: 5000, High: 20000, Name: "ultra_high"},
}

var (
	ErrInvalidInput      = errors.New("invalid input data")
	ErrInvalidSampleRate = errors.New("invalid sample rate")
	ErrEmptyData         = errors.New("empty data")
)

type Analyzer struct {
	windowSize int
	overlap    float64
}

func NewAnalyzer(cfg *config.FFTConfig) *Analyzer {
	return &Analyzer{
		windowSize: cfg.WindowSize,
		overlap:    cfg.Overlap,
	}
}

func (a *Analyzer) Analyze(vibrationData []float64, samplingRate uint32) (*model.FFTResult, error) {
	if len(vibrationData) == 0 {
		return nil, ErrEmptyData
	}
	if samplingRate == 0 {
		return nil, ErrInvalidSampleRate
	}

	data := make([]float64, len(vibrationData))
	copy(data, vibrationData)

	removeDC(data)
	applyHanningWindow(data)

	complexData := make([]complex128, len(data))
	for i, v := range data {
		complexData[i] = complex(v, 0)
	}

	fftResult := fft.FFT(complexData)

	n := len(fftResult)
	freqStep := float64(samplingRate) / float64(n)

	frequencies := make([]float64, n/2)
	magnitudes := make([]float64, n/2)

	for i := 0; i < n/2; i++ {
		frequencies[i] = float64(i) * freqStep
		magnitudes[i] = cmplx.Abs(fftResult[i]) * 2.0 / float64(n)
	}

	mainFreqIdx := findMaxIndex(magnitudes)
	mainFrequency := frequencies[mainFreqIdx]
	mainEnergy := magnitudes[mainFreqIdx]

	totalEnergy := 0.0
	for _, m := range magnitudes {
		totalEnergy += m * m
	}

	harmonicCount := 5
	harmonicEnergies := make([]float64, harmonicCount)
	for h := 0; h < harmonicCount; h++ {
		harmonicFreq := mainFrequency * float64(h+1)
		harmonicEnergies[h] = getEnergyAroundFrequency(frequencies, magnitudes, harmonicFreq, freqStep*2)
	}

	bandEnergies := make([]float64, len(StandardBands))
	for i, band := range StandardBands {
		bandEnergies[i] = calculateBandEnergy(frequencies, magnitudes, band.Low, band.High)
	}

	return &model.FFTResult{
		Frequencies:      frequencies,
		Magnitudes:       magnitudes,
		MainFrequency:    mainFrequency,
		MainEnergy:       mainEnergy,
		HarmonicEnergies: harmonicEnergies,
		BandEnergies:     bandEnergies,
		TotalEnergy:      totalEnergy,
	}, nil
}

func (a *Analyzer) AnalyzeSTFT(vibrationData []float64, samplingRate uint32) ([]*model.FFTResult, error) {
	if len(vibrationData) == 0 {
		return nil, ErrEmptyData
	}
	if samplingRate == 0 {
		return nil, ErrInvalidSampleRate
	}
	if a.windowSize <= 0 {
		return nil, ErrInvalidInput
	}

	overlapSize := int(float64(a.windowSize) * a.overlap)
	stepSize := a.windowSize - overlapSize

	if stepSize <= 0 {
		stepSize = a.windowSize
	}

	var results []*model.FFTResult

	for i := 0; i+a.windowSize <= len(vibrationData); i += stepSize {
		segment := vibrationData[i : i+a.windowSize]
		result, err := a.Analyze(segment, samplingRate)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}

	if len(results) == 0 && len(vibrationData) > 0 {
		result, err := a.Analyze(vibrationData, samplingRate)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}

	return results, nil
}

func removeDC(data []float64) {
	if len(data) == 0 {
		return
	}

	mean := 0.0
	for _, v := range data {
		mean += v
	}
	mean /= float64(len(data))

	for i := range data {
		data[i] -= mean
	}
}

func applyHanningWindow(data []float64) {
	win := window.Hann(len(data))
	for i := range data {
		data[i] *= win[i]
	}
}

func findMaxIndex(data []float64) int {
	if len(data) == 0 {
		return 0
	}

	maxIdx := 1
	maxVal := data[1]

	for i := 2; i < len(data); i++ {
		if data[i] > maxVal {
			maxVal = data[i]
			maxIdx = i
		}
	}

	return maxIdx
}

func getEnergyAroundFrequency(frequencies, magnitudes []float64, targetFreq, tolerance float64) float64 {
	energy := 0.0
	for i, f := range frequencies {
		if math.Abs(f-targetFreq) <= tolerance {
			energy += magnitudes[i] * magnitudes[i]
		}
	}
	return math.Sqrt(energy)
}

func calculateBandEnergy(frequencies, magnitudes []float64, lowFreq, highFreq float64) float64 {
	energy := 0.0
	for i, f := range frequencies {
		if f >= lowFreq && f < highFreq {
			energy += magnitudes[i] * magnitudes[i]
		}
	}
	return math.Sqrt(energy)
}

func CalculateRMS(data []float64) float64 {
	if len(data) == 0 {
		return 0
	}

	sum := 0.0
	for _, v := range data {
		sum += v * v
	}
	return math.Sqrt(sum / float64(len(data)))
}

func CalculatePeak(data []float64) float64 {
	if len(data) == 0 {
		return 0
	}

	peak := 0.0
	for _, v := range data {
		absV := math.Abs(v)
		if absV > peak {
			peak = absV
		}
	}
	return peak
}

func CalculateCrestFactor(data []float64) float64 {
	rms := CalculateRMS(data)
	if rms == 0 {
		return 0
	}
	return CalculatePeak(data) / rms
}

func CalculateKurtosis(data []float64) float64 {
	if len(data) < 4 {
		return 0
	}

	mean := 0.0
	for _, v := range data {
		mean += v
	}
	mean /= float64(len(data))

	variance := 0.0
	for _, v := range data {
		diff := v - mean
		variance += diff * diff
	}
	variance /= float64(len(data))

	if variance == 0 {
		return 0
	}

	stdDev := math.Sqrt(variance)

	kurtosis := 0.0
	for _, v := range data {
		diff := (v - mean) / stdDev
		kurtosis += diff * diff * diff * diff
	}
	kurtosis /= float64(len(data))

	return kurtosis - 3
}
