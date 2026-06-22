package model

import (
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

type ValveType string

const (
	ValveTypeGate      ValveType = "gate"
	ValveTypeGlobe     ValveType = "globe"
	ValveTypeBall      ValveType = "ball"
	ValveTypeButterfly ValveType = "butterfly"
	ValveTypeCheck     ValveType = "check"
)

type ValveStatus string

const (
	ValveStatusNormal   ValveStatus = "normal"
	ValveStatusWarning  ValveStatus = "warning"
	ValveStatusCritical ValveStatus = "critical"
)

type DiagnosisStatus string

const (
	DiagnosisStatusPending  DiagnosisStatus = "pending"
	DiagnosisStatusRunning  DiagnosisStatus = "running"
	DiagnosisStatusComplete DiagnosisStatus = "complete"
	DiagnosisStatusFailed   DiagnosisStatus = "failed"
)

type AnomalyType string

const (
	AnomalyTypeJamming    AnomalyType = "jamming"
	AnomalyTypeCavitation AnomalyType = "cavitation"
	AnomalyTypeLooseness  AnomalyType = "looseness"
	AnomalyTypeBearing    AnomalyType = "bearing"
)

type Valve struct {
	ID             uint64         `gorm:"primaryKey;autoIncrement" json:"id"`
	DeviceNo       string         `gorm:"size:64;uniqueIndex;not null" json:"device_no"`
	Name           string         `gorm:"size:128;not null" json:"name"`
	Type           ValveType      `gorm:"size:32;not null;index" json:"type"`
	Location       string         `gorm:"size:256" json:"location"`
	Manufacturer   string         `gorm:"size:128" json:"manufacturer"`
	Model          string         `gorm:"size:64" json:"model"`
	InstallDate    *time.Time     `json:"install_date"`
	Status         ValveStatus    `gorm:"size:32;default:normal;index" json:"status"`
	LastCheckTime  *time.Time     `json:"last_check_time"`
	Description    string         `gorm:"type:text" json:"description"`
	CreatedAt      time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt      time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`
}

type Waveform struct {
	ID            uint64         `gorm:"primaryKey;autoIncrement" json:"id"`
	DeviceNo      string         `gorm:"size:64;not null;index" json:"device_no"`
	WaveformHash  string         `gorm:"size:64;uniqueIndex;not null" json:"waveform_hash"`
	SamplingRate  uint32         `gorm:"not null" json:"sampling_rate"`
	SamplingCount uint32         `gorm:"not null" json:"sampling_count"`
	ChannelNo     uint8          `gorm:"not null" json:"channel_no"`
	CollectTime   time.Time      `gorm:"not null;index" json:"collect_time"`
	RawDataSize   uint32         `gorm:"not null" json:"raw_data_size"`
	RawData       []byte         `gorm:"type:bytea" json:"-"`
	VibrationData []float64      `gorm:"-" json:"vibration_data,omitempty"`
	CRC           uint32         `gorm:"not null" json:"crc"`
	UploadTime    time.Time      `gorm:"autoCreateTime;index" json:"upload_time"`
	CreatedAt     time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt     time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
}

type DiagnosisResult struct {
	ID              uint64          `gorm:"primaryKey;autoIncrement" json:"id"`
	TaskID          string          `gorm:"size:64;uniqueIndex;not null" json:"task_id"`
	DeviceNo        string          `gorm:"size:64;not null;index" json:"device_no"`
	WaveformID      uint64          `gorm:"not null;index" json:"waveform_id"`
	Status          DiagnosisStatus `gorm:"size:32;not null;default:pending;index" json:"status"`
	Version         int64           `gorm:"not null;default:0" json:"version"`
	MainFrequency   float64         `json:"main_frequency"`
	MainEnergy      float64         `json:"main_energy"`
	HarmonicEnergies string         `gorm:"type:text" json:"harmonic_energies"`
	BandEnergies    string          `gorm:"type:text" json:"band_energies"`
	AnomalyScore    float64         `gorm:"default:0;index" json:"anomaly_score"`
	AnomalyType     *AnomalyType    `gorm:"size:32;index" json:"anomaly_type,omitempty"`
	AnomalyDetails  string          `gorm:"type:text" json:"anomaly_details"`
	FFTResult       string          `gorm:"type:text" json:"fft_result"`
	RuleVersion     string          `gorm:"size:32" json:"rule_version"`
	StartTime       time.Time       `json:"start_time"`
	EndTime         *time.Time      `json:"end_time"`
	ErrorMsg        string          `gorm:"type:text" json:"error_msg,omitempty"`
	CreatedAt       time.Time       `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt       time.Time       `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt       gorm.DeletedAt  `gorm:"index" json:"-"`

	Waveform *Waveform `gorm:"foreignKey:WaveformID" json:"waveform,omitempty"`
	Valve    *Valve    `gorm:"foreignKey:DeviceNo;references:DeviceNo" json:"valve,omitempty"`
}

const (
	StateTransitionError = "invalid state transition"
	TaskAlreadyCompleted = "task already completed"
	TaskAlreadyRunning   = "task already running"
	VersionMismatch      = "version mismatch"
)

type RuleConfig struct {
	ID            uint64         `gorm:"primaryKey;autoIncrement" json:"id"`
	ValveType     ValveType      `gorm:"size:32;not null;uniqueIndex:idx_valve_type_anomaly;index" json:"valve_type"`
	AnomalyType   AnomalyType    `gorm:"size:32;not null;uniqueIndex:idx_valve_type_anomaly;index" json:"anomaly_type"`
	Name          string         `gorm:"size:128;not null" json:"name"`
	Description   string         `gorm:"type:text" json:"description"`
	MinFrequency  float64        `json:"min_frequency"`
	MaxFrequency  float64        `json:"max_frequency"`
	Threshold     float64        `gorm:"not null" json:"threshold"`
	Weight        float64        `gorm:"default:1.0" json:"weight"`
	HarmonicCheck bool           `gorm:"default:false" json:"harmonic_check"`
	HarmonicCount int            `gorm:"default:3" json:"harmonic_count"`
	BandwidthLow  float64        `json:"bandwidth_low"`
	BandwidthHigh float64        `json:"bandwidth_high"`
	Enabled       bool           `gorm:"default:true;index" json:"enabled"`
	Version       string         `gorm:"size:32;not null" json:"version"`
	CreatedAt     time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt     time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
}

type DiagnosisTask struct {
	TaskID     string    `json:"task_id"`
	DeviceNo   string    `json:"device_no"`
	WaveformID uint64    `json:"waveform_id"`
	Priority   int       `json:"priority"`
	RetryCount int       `json:"retry_count"`
	CreatedAt  time.Time `json:"created_at"`
}

type FFTResult struct {
	Frequencies      []float64 `json:"frequencies"`
	Magnitudes       []float64 `json:"magnitudes"`
	MainFrequency    float64   `json:"main_frequency"`
	MainEnergy       float64   `json:"main_energy"`
	HarmonicEnergies []float64 `json:"harmonic_energies"`
	BandEnergies     []float64 `json:"band_energies"`
	TotalEnergy      float64   `json:"total_energy"`
}

type AnomalyDiagnosis struct {
	Type        AnomalyType `json:"type"`
	Name        string      `json:"name"`
	Score       float64     `json:"score"`
	Confidence  float64     `json:"confidence"`
	Description string      `json:"description"`
	MatchedRule *RuleConfig `json:"matched_rule,omitempty"`
}

type DiagnosisSummary struct {
	AnomalyScore float64            `json:"anomaly_score"`
	Anomalies    []AnomalyDiagnosis `json:"anomalies"`
	MainAnomaly  *AnomalyDiagnosis  `json:"main_anomaly,omitempty"`
	FFTResult    *FFTResult         `json:"fft_result,omitempty"`
}

func (s *DiagnosisSummary) ToJSON() (string, error) {
	data, err := json.Marshal(s)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (f *FFTResult) HarmonicEnergiesToJSON() (string, error) {
	data, err := json.Marshal(f.HarmonicEnergies)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (f *FFTResult) BandEnergiesToJSON() (string, error) {
	data, err := json.Marshal(f.BandEnergies)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (f *FFTResult) ToJSON() (string, error) {
	type simplifiedFFT struct {
		MainFrequency    float64   `json:"main_frequency"`
		MainEnergy       float64   `json:"main_energy"`
		HarmonicEnergies []float64 `json:"harmonic_energies"`
		BandEnergies     []float64 `json:"band_energies"`
		TotalEnergy      float64   `json:"total_energy"`
	}

	simplified := simplifiedFFT{
		MainFrequency:    f.MainFrequency,
		MainEnergy:       f.MainEnergy,
		HarmonicEnergies: f.HarmonicEnergies,
		BandEnergies:     f.BandEnergies,
		TotalEnergy:      f.TotalEnergy,
	}

	data, err := json.Marshal(simplified)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
