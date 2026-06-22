package diagnosis

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"nuclear-valve-vibration-api/internal/fft"
	"nuclear-valve-vibration-api/internal/model"
)

type Engine struct {
	rules map[model.ValveType][]*model.RuleConfig
}

func NewEngine() *Engine {
	return &Engine{
		rules: make(map[model.ValveType][]*model.RuleConfig),
	}
}

func (e *Engine) LoadRules(rules []*model.RuleConfig) {
	e.rules = make(map[model.ValveType][]*model.RuleConfig)
	for _, rule := range rules {
		if rule.Enabled {
			e.rules[rule.ValveType] = append(e.rules[rule.ValveType], rule)
		}
	}
}

func (e *Engine) GetRules(valveType model.ValveType) []*model.RuleConfig {
	return e.rules[valveType]
}

func (e *Engine) Diagnose(valveType model.ValveType, fftResult *model.FFTResult, vibrationData []float64) (*model.DiagnosisSummary, error) {
	rules := e.GetRules(valveType)
	if len(rules) == 0 {
		rules = e.GetRules("")
	}

	rms := fft.CalculateRMS(vibrationData)
	crestFactor := fft.CalculateCrestFactor(vibrationData)
	kurtosis := fft.CalculateKurtosis(vibrationData)

	var anomalies []model.AnomalyDiagnosis
	totalScore := 0.0
	totalWeight := 0.0

	for _, rule := range rules {
		anomaly, err := e.checkRule(rule, fftResult, rms, crestFactor, kurtosis)
		if err != nil {
			continue
		}
		if anomaly.Score > 0 {
			anomalies = append(anomalies, *anomaly)
			totalScore += anomaly.Score * rule.Weight
			totalWeight += rule.Weight
		}
	}

	var anomalyScore float64
	if totalWeight > 0 {
		anomalyScore = totalScore / totalWeight
	}

	for i := range anomalies {
		anomalies[i].Confidence = anomalies[i].Score / 100.0
	}

	sort.Slice(anomalies, func(i, j int) bool {
		return anomalies[i].Score > anomalies[j].Score
	})

	var mainAnomaly *model.AnomalyDiagnosis
	if len(anomalies) > 0 {
		mainAnomaly = &anomalies[0]
	}

	return &model.DiagnosisSummary{
		AnomalyScore: math.Round(anomalyScore*100) / 100,
		Anomalies:    anomalies,
		MainAnomaly:  mainAnomaly,
		FFTResult:    fftResult,
	}, nil
}

func (e *Engine) checkRule(rule *model.RuleConfig, fftResult *model.FFTResult, rms, crestFactor, kurtosis float64) (*model.AnomalyDiagnosis, error) {
	score := 0.0
	descriptionParts := []string{}

	if rule.MinFrequency > 0 || rule.MaxFrequency > 0 {
		minFreq := rule.MinFrequency
		maxFreq := rule.MaxFrequency
		if maxFreq == 0 {
			maxFreq = float64(1<<31) - 1
		}

		bandEnergy := calculateEnergyInBand(fftResult.Frequencies, fftResult.Magnitudes, minFreq, maxFreq)
		ratio := bandEnergy / fftResult.TotalEnergy

		if fftResult.TotalEnergy > 0 && ratio > 0 {
			normalizedRatio := math.Min(ratio/rule.Threshold, 1.0)
			score += normalizedRatio * 40
			if normalizedRatio > 0.5 {
				descriptionParts = append(descriptionParts,
					fmt.Sprintf("频段[%0.1f-%0.1fHz]能量占比%0.2f%%", minFreq, maxFreq, ratio*100))
			}
		}
	}

	if rule.BandwidthLow > 0 || rule.BandwidthHigh > 0 {
		bandEnergy := calculateEnergyInBand(fftResult.Frequencies, fftResult.Magnitudes, rule.BandwidthLow, rule.BandwidthHigh)
		if bandEnergy > rule.Threshold {
			normalizedValue := math.Min(bandEnergy/rule.Threshold, 1.0)
			score += normalizedValue * 30
			descriptionParts = append(descriptionParts,
				fmt.Sprintf("带宽[%0.1f-%0.1fHz]能量%0.4f超过阈值%0.4f", rule.BandwidthLow, rule.BandwidthHigh, bandEnergy, rule.Threshold))
		}
	}

	if rule.HarmonicCheck {
		harmonicScore := e.checkHarmonics(rule, fftResult)
		score += harmonicScore
		if harmonicScore > 10 {
			descriptionParts = append(descriptionParts,
				fmt.Sprintf("检测到%d次谐波异常", rule.HarmonicCount))
		}
	}

	switch rule.AnomalyType {
	case model.AnomalyTypeJamming:
		if kurtosis > 3.0 {
			kurtosisScore := math.Min((kurtosis-3.0)/3.0, 1.0) * 30
			score += kurtosisScore
			if kurtosisScore > 10 {
				descriptionParts = append(descriptionParts,
					fmt.Sprintf("峭度%0.2f偏高，可能存在卡涩", kurtosis))
			}
		}
	case model.AnomalyTypeCavitation:
		if crestFactor > 6.0 {
			cfScore := math.Min((crestFactor-6.0)/4.0, 1.0) * 30
			score += cfScore
			if cfScore > 10 {
				descriptionParts = append(descriptionParts,
					fmt.Sprintf("波峰因子%0.2f偏高，可能存在汽蚀", crestFactor))
			}
		}
	case model.AnomalyTypeLooseness:
		if rms > 0.1 {
			rmsScore := math.Min(rms/0.5, 1.0) * 25
			score += rmsScore
			if rmsScore > 10 {
				descriptionParts = append(descriptionParts,
					fmt.Sprintf("RMS值%0.4f偏高，可能存在松动", rms))
			}
		}
	case model.AnomalyTypeBearing:
		if kurtosis > 4.0 && crestFactor > 5.0 {
			bearingScore := math.Min(((kurtosis-4.0)+(crestFactor-5.0))/6.0, 1.0) * 35
			score += bearingScore
			if bearingScore > 10 {
				descriptionParts = append(descriptionParts,
					fmt.Sprintf("峭度%0.2f和波峰因子%0.2f均偏高，可能存在轴承异常", kurtosis, crestFactor))
			}
		}
	}

	if score < 5.0 {
		score = 0
	}

	description := ""
	if len(descriptionParts) > 0 {
		description = strings.Join(descriptionParts, "; ")
	}

	return &model.AnomalyDiagnosis{
		Type:        rule.AnomalyType,
		Name:        rule.Name,
		Score:       math.Round(score*100) / 100,
		Confidence:  0,
		Description: description,
		MatchedRule: rule,
	}, nil
}

func (e *Engine) checkHarmonics(rule *model.RuleConfig, fftResult *model.FFTResult) float64 {
	if len(fftResult.HarmonicEnergies) == 0 {
		return 0
	}

	harmonicCount := rule.HarmonicCount
	if harmonicCount > len(fftResult.HarmonicEnergies) {
		harmonicCount = len(fftResult.HarmonicEnergies)
	}

	matchCount := 0
	totalHarmonicEnergy := 0.0
	for i := 0; i < harmonicCount; i++ {
		if fftResult.HarmonicEnergies[i] > rule.Threshold*0.3 {
			matchCount++
		}
		totalHarmonicEnergy += fftResult.HarmonicEnergies[i]
	}

	ratio := float64(matchCount) / float64(harmonicCount)
	if fftResult.TotalEnergy > 0 {
		energyRatio := totalHarmonicEnergy / fftResult.TotalEnergy
		return (ratio*0.6 + energyRatio*0.4) * 30
	}

	return ratio * 30
}

func calculateEnergyInBand(frequencies, magnitudes []float64, lowFreq, highFreq float64) float64 {
	energy := 0.0
	for i, f := range frequencies {
		if f >= lowFreq && f < highFreq {
			energy += magnitudes[i] * magnitudes[i]
		}
	}
	return math.Sqrt(energy)
}

func GetDefaultRules() []*model.RuleConfig {
	return []*model.RuleConfig{
		{ValveType: model.ValveTypeGate, AnomalyType: model.AnomalyTypeJamming, Name: "闸阀卡涩检测",
			Description: "检测闸阀卡涩异常，主要表现为低频高峭度", MinFrequency: 5, MaxFrequency: 50,
			Threshold: 0.15, Weight: 1.2, HarmonicCheck: true, HarmonicCount: 3, Version: "v1.0"},
		{ValveType: model.ValveTypeGate, AnomalyType: model.AnomalyTypeCavitation, Name: "闸阀汽蚀检测",
			Description: "检测闸阀汽蚀异常，主要表现为高频能量和高波峰因子", MinFrequency: 1000, MaxFrequency: 10000,
			Threshold: 0.1, Weight: 1.0, Version: "v1.0"},
		{ValveType: model.ValveTypeGate, AnomalyType: model.AnomalyTypeLooseness, Name: "闸阀松动检测",
			Description: "检测闸阀松动异常，主要表现为中高频能量和高RMS", MinFrequency: 50, MaxFrequency: 500,
			Threshold: 0.08, Weight: 1.1, Version: "v1.0"},
		{ValveType: model.ValveTypeGate, AnomalyType: model.AnomalyTypeBearing, Name: "闸阀轴承检测",
			Description: "检测闸阀轴承异常，主要表现为特定频段和谐波", MinFrequency: 200, MaxFrequency: 2000,
			Threshold: 0.12, Weight: 1.3, HarmonicCheck: true, HarmonicCount: 5, Version: "v1.0"},

		{ValveType: model.ValveTypeGlobe, AnomalyType: model.AnomalyTypeJamming, Name: "截止阀卡涩检测",
			Description: "检测截止阀卡涩异常", MinFrequency: 3, MaxFrequency: 30,
			Threshold: 0.18, Weight: 1.2, HarmonicCheck: true, Version: "v1.0"},
		{ValveType: model.ValveTypeGlobe, AnomalyType: model.AnomalyTypeCavitation, Name: "截止阀汽蚀检测",
			Description: "检测截止阀汽蚀异常", MinFrequency: 500, MaxFrequency: 8000,
			Threshold: 0.12, Weight: 1.1, Version: "v1.0"},
		{ValveType: model.ValveTypeGlobe, AnomalyType: model.AnomalyTypeLooseness, Name: "截止阀松动检测",
			Description: "检测截止阀松动异常", MinFrequency: 30, MaxFrequency: 300,
			Threshold: 0.1, Weight: 1.0, Version: "v1.0"},
		{ValveType: model.ValveTypeGlobe, AnomalyType: model.AnomalyTypeBearing, Name: "截止阀轴承检测",
			Description: "检测截止阀轴承异常", MinFrequency: 150, MaxFrequency: 1500,
			Threshold: 0.15, Weight: 1.3, HarmonicCheck: true, Version: "v1.0"},

		{ValveType: model.ValveTypeBall, AnomalyType: model.AnomalyTypeJamming, Name: "球阀卡涩检测",
			Description: "检测球阀卡涩异常", MinFrequency: 8, MaxFrequency: 80,
			Threshold: 0.16, Weight: 1.1, HarmonicCheck: true, Version: "v1.0"},
		{ValveType: model.ValveTypeBall, AnomalyType: model.AnomalyTypeCavitation, Name: "球阀汽蚀检测",
			Description: "检测球阀汽蚀异常", MinFrequency: 800, MaxFrequency: 12000,
			Threshold: 0.11, Weight: 1.0, Version: "v1.0"},
		{ValveType: model.ValveTypeBall, AnomalyType: model.AnomalyTypeLooseness, Name: "球阀松动检测",
			Description: "检测球阀松动异常", MinFrequency: 60, MaxFrequency: 600,
			Threshold: 0.09, Weight: 1.1, Version: "v1.0"},
		{ValveType: model.ValveTypeBall, AnomalyType: model.AnomalyTypeBearing, Name: "球阀轴承检测",
			Description: "检测球阀轴承异常", MinFrequency: 180, MaxFrequency: 1800,
			Threshold: 0.13, Weight: 1.2, HarmonicCheck: true, Version: "v1.0"},

		{ValveType: model.ValveTypeButterfly, AnomalyType: model.AnomalyTypeJamming, Name: "蝶阀卡涩检测",
			Description: "检测蝶阀卡涩异常", MinFrequency: 2, MaxFrequency: 40,
			Threshold: 0.14, Weight: 1.2, HarmonicCheck: true, Version: "v1.0"},
		{ValveType: model.ValveTypeButterfly, AnomalyType: model.AnomalyTypeCavitation, Name: "蝶阀汽蚀检测",
			Description: "检测蝶阀汽蚀异常", MinFrequency: 600, MaxFrequency: 9000,
			Threshold: 0.13, Weight: 1.0, Version: "v1.0"},
		{ValveType: model.ValveTypeButterfly, AnomalyType: model.AnomalyTypeLooseness, Name: "蝶阀松动检测",
			Description: "检测蝶阀松动异常", MinFrequency: 40, MaxFrequency: 400,
			Threshold: 0.085, Weight: 1.1, Version: "v1.0"},
		{ValveType: model.ValveTypeButterfly, AnomalyType: model.AnomalyTypeBearing, Name: "蝶阀轴承检测",
			Description: "检测蝶阀轴承异常", MinFrequency: 160, MaxFrequency: 1600,
			Threshold: 0.14, Weight: 1.3, HarmonicCheck: true, Version: "v1.0"},

		{ValveType: model.ValveTypeCheck, AnomalyType: model.AnomalyTypeJamming, Name: "止回阀卡涩检测",
			Description: "检测止回阀卡涩异常", MinFrequency: 4, MaxFrequency: 60,
			Threshold: 0.17, Weight: 1.1, HarmonicCheck: true, Version: "v1.0"},
		{ValveType: model.ValveTypeCheck, AnomalyType: model.AnomalyTypeCavitation, Name: "止回阀汽蚀检测",
			Description: "检测止回阀汽蚀异常", MinFrequency: 700, MaxFrequency: 10000,
			Threshold: 0.115, Weight: 1.0, Version: "v1.0"},
		{ValveType: model.ValveTypeCheck, AnomalyType: model.AnomalyTypeLooseness, Name: "止回阀松动检测",
			Description: "检测止回阀松动异常", MinFrequency: 45, MaxFrequency: 450,
			Threshold: 0.095, Weight: 1.1, Version: "v1.0"},
		{ValveType: model.ValveTypeCheck, AnomalyType: model.AnomalyTypeBearing, Name: "止回阀轴承检测",
			Description: "检测止回阀轴承异常", MinFrequency: 170, MaxFrequency: 1700,
			Threshold: 0.135, Weight: 1.2, HarmonicCheck: true, Version: "v1.0"},
	}
}
