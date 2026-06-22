package service

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"nuclear-valve-vibration-api/internal/cache"
	"nuclear-valve-vibration-api/internal/config"
	"nuclear-valve-vibration-api/internal/diagnosis"
	"nuclear-valve-vibration-api/internal/fft"
	"nuclear-valve-vibration-api/internal/model"
	"nuclear-valve-vibration-api/internal/repository"
)

type DiagnosisService interface {
	GetByID(ctx context.Context, id uint64) (*model.DiagnosisResult, error)
	GetByTaskID(ctx context.Context, taskID string) (*model.DiagnosisResult, error)
	ListByDeviceNo(ctx context.Context, deviceNo string, page, pageSize int, startTime, endTime *time.Time, status *model.DiagnosisStatus) ([]*model.DiagnosisResult, int64, error)
	GetLatestByDeviceNo(ctx context.Context, deviceNo string, limit int) ([]*model.DiagnosisResult, error)
	GetStats(ctx context.Context, deviceNo string, startTime, endTime time.Time) (map[string]interface{}, error)
	ProcessTask(ctx context.Context, task *model.DiagnosisTask) error
	SubmitTask(ctx context.Context, deviceNo string, waveformID uint64) (string, error)
}

type diagnosisService struct {
	repo            repository.DiagnosisRepository
	waveformRepo    repository.WaveformRepository
	valveRepo       repository.ValveRepository
	ruleRepo        repository.RuleRepository
	cache           cache.Cache
	fftAnalyzer     *fft.Analyzer
	diagnosisEngine *diagnosis.Engine
	cfg             *config.Config
}

func NewDiagnosisService(
	repo repository.DiagnosisRepository,
	waveformRepo repository.WaveformRepository,
	valveRepo repository.ValveRepository,
	ruleRepo repository.RuleRepository,
	cache cache.Cache,
	cfg *config.Config,
) DiagnosisService {
	engine := diagnosis.NewEngine()
	return &diagnosisService{
		repo:            repo,
		waveformRepo:    waveformRepo,
		valveRepo:       valveRepo,
		ruleRepo:        ruleRepo,
		cache:           cache,
		fftAnalyzer:     fft.NewAnalyzer(&cfg.FFT),
		diagnosisEngine: engine,
		cfg:             cfg,
	}
}

func (s *diagnosisService) reloadRules() error {
	rules, err := s.ruleRepo.ListAllEnabled()
	if err != nil {
		return err
	}
	s.diagnosisEngine.LoadRules(rules)

	version, _ := s.ruleRepo.GetLatestVersion()
	_ = s.cache.SetRuleVersion(context.Background(), version)

	return nil
}

func (s *diagnosisService) GetByID(ctx context.Context, id uint64) (*model.DiagnosisResult, error) {
	return s.repo.GetByID(id)
}

func (s *diagnosisService) GetByTaskID(ctx context.Context, taskID string) (*model.DiagnosisResult, error) {
	result, err := s.repo.GetByTaskID(taskID)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, errors.New("diagnosis result not found")
	}
	return result, nil
}

func (s *diagnosisService) ListByDeviceNo(ctx context.Context, deviceNo string, page, pageSize int, startTime, endTime *time.Time, status *model.DiagnosisStatus) ([]*model.DiagnosisResult, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	return s.repo.ListByDeviceNo(deviceNo, page, pageSize, startTime, endTime, status)
}

func (s *diagnosisService) GetLatestByDeviceNo(ctx context.Context, deviceNo string, limit int) ([]*model.DiagnosisResult, error) {
	if limit < 1 || limit > 100 {
		limit = 10
	}
	return s.repo.GetLatestByDeviceNo(deviceNo, limit)
}

func (s *diagnosisService) GetStats(ctx context.Context, deviceNo string, startTime, endTime time.Time) (map[string]interface{}, error) {
	return s.repo.GetStats(deviceNo, startTime, endTime)
}

func (s *diagnosisService) SubmitTask(ctx context.Context, deviceNo string, waveformID uint64) (string, error) {
	_, err := s.valveRepo.GetByDeviceNo(deviceNo)
	if err != nil {
		return "", err
	}

	wf, err := s.waveformRepo.GetByID(waveformID)
	if err != nil {
		return "", err
	}
	if wf == nil {
		return "", errors.New("waveform not found")
	}
	if wf.DeviceNo != deviceNo {
		return "", errors.New("waveform does not belong to device")
	}

	taskID := fmt.Sprintf("manual-%d-%d", waveformID, time.Now().UnixNano())

	diagResult := &model.DiagnosisResult{
		TaskID:     taskID,
		DeviceNo:   deviceNo,
		WaveformID: waveformID,
		Status:     model.DiagnosisStatusPending,
		StartTime:  time.Now(),
	}

	if err := s.repo.Create(diagResult); err != nil {
		return "", err
	}

	return taskID, nil
}

func (s *diagnosisService) ProcessTask(ctx context.Context, task *model.DiagnosisTask) error {
	acquired, err := s.repo.AcquireTask(task.TaskID)
	if err != nil {
		if strings.Contains(err.Error(), model.TaskAlreadyCompleted) {
			return nil
		}
		if strings.Contains(err.Error(), model.TaskAlreadyRunning) {
			return fmt.Errorf("task %s is already being processed", task.TaskID)
		}
		return fmt.Errorf("failed to acquire task %s: %w", task.TaskID, err)
	}

	currentVersion := acquired.Version

	defer func() {
		if r := recover(); r != nil {
			_, _ = s.repo.FailTask(task.TaskID, fmt.Sprintf("panic: %v", r), currentVersion)
		}
	}()

	if err := s.reloadRules(); err != nil {
		_, _ = s.repo.FailTask(task.TaskID, fmt.Sprintf("failed to reload rules: %v", err), currentVersion)
		return err
	}

	wf, err := s.waveformRepo.GetByID(task.WaveformID)
	if err != nil {
		_, _ = s.repo.FailTask(task.TaskID, fmt.Sprintf("failed to get waveform: %v", err), currentVersion)
		return err
	}
	if wf == nil {
		_, _ = s.repo.FailTask(task.TaskID, "waveform not found", currentVersion)
		return errors.New("waveform not found")
	}

	vibrationData, err := s.extractVibrationData(wf)
	if err != nil {
		_, _ = s.repo.FailTask(task.TaskID, fmt.Sprintf("failed to extract vibration data: %v", err), currentVersion)
		return err
	}

	fftResult, err := s.fftAnalyzer.Analyze(vibrationData, wf.SamplingRate)
	if err != nil {
		_, _ = s.repo.FailTask(task.TaskID, fmt.Sprintf("FFT analysis failed: %v", err), currentVersion)
		return err
	}

	valve, err := s.valveRepo.GetByDeviceNo(task.DeviceNo)
	if err != nil {
		_, _ = s.repo.FailTask(task.TaskID, fmt.Sprintf("failed to get valve info: %v", err), currentVersion)
		return err
	}

	valveType := model.ValveType("")
	if valve != nil {
		valveType = valve.Type
	}

	diagSummary, err := s.diagnosisEngine.Diagnose(valveType, fftResult, vibrationData)
	if err != nil {
		_, _ = s.repo.FailTask(task.TaskID, fmt.Sprintf("diagnosis failed: %v", err), currentVersion)
		return err
	}

	result := &model.DiagnosisResult{
		TaskID:       task.TaskID,
		DeviceNo:     task.DeviceNo,
		WaveformID:   task.WaveformID,
		MainFrequency: fftResult.MainFrequency,
		MainEnergy:   fftResult.MainEnergy,
		AnomalyScore: diagSummary.AnomalyScore,
	}

	harmonicJSON, _ := fftResult.HarmonicEnergiesToJSON()
	result.HarmonicEnergies = harmonicJSON
	bandJSON, _ := fftResult.BandEnergiesToJSON()
	result.BandEnergies = bandJSON
	fftJSON, _ := fftResult.ToJSON()
	result.FFTResult = fftJSON

	if diagSummary.MainAnomaly != nil {
		result.AnomalyType = &diagSummary.MainAnomaly.Type
		details, _ := diagSummary.ToJSON()
		result.AnomalyDetails = details
	}

	version, _ := s.ruleRepo.GetLatestVersion()
	result.RuleVersion = version

	finalResult, err := s.repo.CompleteTask(result, currentVersion)
	if err != nil {
		if strings.Contains(err.Error(), model.TaskAlreadyCompleted) || strings.Contains(err.Error(), model.VersionMismatch) {
			return nil
		}
		return fmt.Errorf("failed to complete task: %w", err)
	}

	if finalResult != nil && finalResult.Status == model.DiagnosisStatusComplete {
		if valve != nil {
			status := model.ValveStatusNormal
			if diagSummary.AnomalyScore >= 70 {
				status = model.ValveStatusCritical
			} else if diagSummary.AnomalyScore >= 30 {
				status = model.ValveStatusWarning
			}
			_ = s.valveRepo.UpdateStatus(valve.DeviceNo, status)
			_ = s.cache.SetValveStatus(ctx, valve.DeviceNo, status)
			_ = s.valveRepo.UpdateLastCheckTime(valve.DeviceNo, time.Now())
		}

		_ = s.cache.SetLatestDiagnosis(ctx, task.DeviceNo, finalResult)
	}

	return nil
}

func (s *diagnosisService) extractVibrationData(wf *model.Waveform) ([]float64, error) {
	if wf.VibrationData != nil && len(wf.VibrationData) > 0 {
		return wf.VibrationData, nil
	}

	if wf.RawData == nil || len(wf.RawData) == 0 {
		return nil, errors.New("no raw data available")
	}

	parsed, err := parseRawData(wf.RawData)
	if err != nil {
		return nil, err
	}

	return parsed.VibrationData, nil
}

type rawParsedData struct {
	VibrationData []float64
}

func parseRawData(data []byte) (*rawParsedData, error) {
	if len(data) < 16 {
		return nil, errors.New("data too short")
	}

	headerOffset := 2 + 1 + 1 + 4 + 4 + 8 + 1 + 1
	deviceNoLen := int(data[3])
	if deviceNoLen == 0 || deviceNoLen > 64 {
		return nil, errors.New("invalid device number length")
	}

	vibOffset := headerOffset + deviceNoLen
	remaining := len(data) - vibOffset - 4

	samplingCount := binary.LittleEndian.Uint32(data[8:12])
	expectedSize := int(samplingCount) * 4

	if remaining < expectedSize {
		return nil, fmt.Errorf("insufficient vibration data: expected %d bytes, got %d", expectedSize, remaining)
	}

	vibrationData := make([]float64, samplingCount)
	for i := uint32(0); i < samplingCount; i++ {
		offset := vibOffset + int(i)*4
		bits := binary.LittleEndian.Uint32(data[offset : offset+4])
		vibrationData[i] = float64(math.Float32frombits(bits))
	}

	return &rawParsedData{VibrationData: vibrationData}, nil
}
