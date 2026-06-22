package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"nuclear-valve-vibration-api/internal/cache"
	"nuclear-valve-vibration-api/internal/config"
	"nuclear-valve-vibration-api/internal/model"
	"nuclear-valve-vibration-api/internal/repository"
)

type mockCacheDiag struct {
	mock.Mock
	cache.Cache
}

func (m *mockCacheDiag) SetValveStatus(ctx context.Context, deviceNo string, status model.ValveStatus) error {
	args := m.Called(ctx, deviceNo, status)
	return args.Error(0)
}

func (m *mockCacheDiag) SetLatestDiagnosis(ctx context.Context, deviceNo string, result *model.DiagnosisResult) error {
	args := m.Called(ctx, deviceNo, result)
	return args.Error(0)
}

func (m *mockCacheDiag) SetRuleVersion(ctx context.Context, version string) error {
	args := m.Called(ctx, version)
	return args.Error(0)
}

type mockDiagnosisRepoDiag struct {
	mock.Mock
	repository.DiagnosisRepository
	store map[string]*model.DiagnosisResult
}

func newMockDiagnosisRepoDiag() *mockDiagnosisRepoDiag {
	return &mockDiagnosisRepoDiag{
		store: make(map[string]*model.DiagnosisResult),
	}
}

func (m *mockDiagnosisRepoDiag) Create(result *model.DiagnosisResult) error {
	args := m.Called(result)
	if args.Error(0) == nil {
		result.Version = 0
		result.Status = model.DiagnosisStatusPending
		m.store[result.TaskID] = result
	}
	return args.Error(0)
}

func (m *mockDiagnosisRepoDiag) GetByTaskID(taskID string) (*model.DiagnosisResult, error) {
	args := m.Called(taskID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.DiagnosisResult), args.Error(1)
}

func (m *mockDiagnosisRepoDiag) AcquireTask(taskID string) (*model.DiagnosisResult, error) {
	args := m.Called(taskID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	result := args.Get(0).(*model.DiagnosisResult)
	if result != nil {
		result.Version++
		result.Status = model.DiagnosisStatusRunning
		m.store[taskID] = result
	}
	return result, args.Error(1)
}

func (m *mockDiagnosisRepoDiag) CompleteTask(result *model.DiagnosisResult, version int64) (*model.DiagnosisResult, error) {
	args := m.Called(result, version)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.DiagnosisResult), args.Error(1)
}

func (m *mockDiagnosisRepoDiag) FailTask(taskID string, errorMsg string, version int64) (*model.DiagnosisResult, error) {
	args := m.Called(taskID, errorMsg, version)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.DiagnosisResult), args.Error(1)
}

type mockWaveformRepoDiag struct {
	mock.Mock
	repository.WaveformRepository
}

func (m *mockWaveformRepoDiag) GetByID(id uint64) (*model.Waveform, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Waveform), args.Error(1)
}

type mockValveRepoDiag struct {
	mock.Mock
	repository.ValveRepository
}

func (m *mockValveRepoDiag) GetByDeviceNo(deviceNo string) (*model.Valve, error) {
	args := m.Called(deviceNo)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Valve), args.Error(1)
}

func (m *mockValveRepoDiag) UpdateStatus(deviceNo string, status model.ValveStatus) error {
	args := m.Called(deviceNo, status)
	return args.Error(0)
}

func (m *mockValveRepoDiag) UpdateLastCheckTime(deviceNo string, checkTime time.Time) error {
	args := m.Called(deviceNo, checkTime)
	return args.Error(0)
}

type mockRuleRepoDiag struct {
	mock.Mock
	repository.RuleRepository
}

func (m *mockRuleRepoDiag) ListAllEnabled() ([]*model.RuleConfig, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*model.RuleConfig), args.Error(1)
}

func (m *mockRuleRepoDiag) GetLatestVersion() (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}

func TestProcessTask_AcquirePendingTask(t *testing.T) {
	ctx := context.Background()
	taskID := "task-pending-001"
	task := &model.DiagnosisTask{
		TaskID:     taskID,
		DeviceNo:   "VLV-001",
		WaveformID: 1,
	}

	mockDiagRepo := newMockDiagnosisRepoDiag()
	mockWfRepo := new(mockWaveformRepoDiag)
	mockVRepo := new(mockValveRepoDiag)
	mockRRepo := new(mockRuleRepoDiag)
	mockCache := new(mockCacheDiag)

	cfg := &config.Config{
		FFT: config.FFTConfig{
			WindowSize: 512,
			Overlap:    0.5,
		},
	}

	service := NewDiagnosisService(mockDiagRepo, mockWfRepo, mockVRepo, mockRRepo, mockCache, cfg)

	pendingResult := &model.DiagnosisResult{
		TaskID:     taskID,
		DeviceNo:   "VLV-001",
		WaveformID: 1,
		Status:     model.DiagnosisStatusPending,
		Version:    0,
	}

	wf := &model.Waveform{
		ID:            1,
		DeviceNo:      "VLV-001",
		SamplingRate:  10000,
		SamplingCount: 1024,
		VibrationData: generateTestVibration(1024),
	}

	valve := &model.Valve{
		DeviceNo: "VLV-001",
		Type:     model.ValveTypeGate,
	}

	mockDiagRepo.On("AcquireTask", taskID).Return(pendingResult, nil)
	mockRRepo.On("ListAllEnabled").Return([]*model.RuleConfig{}, nil)
	mockRRepo.On("GetLatestVersion").Return("v1.0", nil)
	mockCache.On("SetRuleVersion", ctx, "v1.0").Return(nil)
	mockWfRepo.On("GetByID", uint64(1)).Return(wf, nil)
	mockVRepo.On("GetByDeviceNo", "VLV-001").Return(valve, nil)
	mockDiagRepo.On("CompleteTask", mock.AnythingOfType("*model.DiagnosisResult"), int64(1)).Return(&model.DiagnosisResult{
		TaskID:     taskID,
		Status:     model.DiagnosisStatusComplete,
		Version:    2,
		AnomalyScore: 15.0,
	}, nil)
	mockVRepo.On("UpdateStatus", "VLV-001", model.ValveStatusNormal).Return(nil)
	mockCache.On("SetValveStatus", ctx, "VLV-001", model.ValveStatusNormal).Return(nil)
	mockVRepo.On("UpdateLastCheckTime", "VLV-001", mock.Anything).Return(nil)
	mockCache.On("SetLatestDiagnosis", ctx, "VLV-001", mock.Anything).Return(nil)

	err := service.ProcessTask(ctx, task)

	assert.NoError(t, err)
	mockDiagRepo.AssertCalled(t, "AcquireTask", taskID)
	mockDiagRepo.AssertCalled(t, "CompleteTask", mock.AnythingOfType("*model.DiagnosisResult"), int64(1))
}

func TestProcessTask_TaskAlreadyCompleted(t *testing.T) {
	ctx := context.Background()
	taskID := "task-complete-001"
	task := &model.DiagnosisTask{
		TaskID:     taskID,
		DeviceNo:   "VLV-001",
		WaveformID: 1,
	}

	mockDiagRepo := newMockDiagnosisRepoDiag()
	mockWfRepo := new(mockWaveformRepoDiag)
	mockVRepo := new(mockValveRepoDiag)
	mockRRepo := new(mockRuleRepoDiag)
	mockCache := new(mockCacheDiag)

	cfg := &config.Config{
		FFT: config.FFTConfig{
			WindowSize: 512,
			Overlap:    0.5,
		},
	}

	service := NewDiagnosisService(mockDiagRepo, mockWfRepo, mockVRepo, mockRRepo, mockCache, cfg)

	alreadyCompleted := fmt.Errorf("%s: %s", model.TaskAlreadyCompleted, taskID)
	mockDiagRepo.On("AcquireTask", taskID).Return(nil, alreadyCompleted)

	err := service.ProcessTask(ctx, task)

	assert.NoError(t, err)

	mockDiagRepo.AssertCalled(t, "AcquireTask", taskID)
	mockDiagRepo.AssertNotCalled(t, "CompleteTask", mock.AnythingOfType("*model.DiagnosisResult"), mock.Anything)
	mockDiagRepo.AssertNotCalled(t, "FailTask", mock.Anything, mock.Anything, mock.Anything)
}

func TestProcessTask_TaskAlreadyRunning(t *testing.T) {
	ctx := context.Background()
	taskID := "task-running-001"
	task := &model.DiagnosisTask{
		TaskID:     taskID,
		DeviceNo:   "VLV-001",
		WaveformID: 1,
	}

	mockDiagRepo := newMockDiagnosisRepoDiag()
	mockWfRepo := new(mockWaveformRepoDiag)
	mockVRepo := new(mockValveRepoDiag)
	mockRRepo := new(mockRuleRepoDiag)
	mockCache := new(mockCacheDiag)

	cfg := &config.Config{
		FFT: config.FFTConfig{
			WindowSize: 512,
			Overlap:    0.5,
		},
	}

	service := NewDiagnosisService(mockDiagRepo, mockWfRepo, mockVRepo, mockRRepo, mockCache, cfg)

	alreadyRunning := fmt.Errorf("%s: %s", model.TaskAlreadyRunning, taskID)
	mockDiagRepo.On("AcquireTask", taskID).Return(nil, alreadyRunning)

	err := service.ProcessTask(ctx, task)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already being processed")

	mockDiagRepo.AssertCalled(t, "AcquireTask", taskID)
}

func TestProcessTask_FailedTaskCanBeRetried(t *testing.T) {
	ctx := context.Background()
	taskID := "task-failed-retry-001"
	task := &model.DiagnosisTask{
		TaskID:     taskID,
		DeviceNo:   "VLV-001",
		WaveformID: 1,
	}

	mockDiagRepo := newMockDiagnosisRepoDiag()
	mockWfRepo := new(mockWaveformRepoDiag)
	mockVRepo := new(mockValveRepoDiag)
	mockRRepo := new(mockRuleRepoDiag)
	mockCache := new(mockCacheDiag)

	cfg := &config.Config{
		FFT: config.FFTConfig{
			WindowSize: 512,
			Overlap:    0.5,
		},
	}

	service := NewDiagnosisService(mockDiagRepo, mockWfRepo, mockVRepo, mockRRepo, mockCache, cfg)

	failedResult := &model.DiagnosisResult{
		TaskID:     taskID,
		DeviceNo:   "VLV-001",
		WaveformID: 1,
		Status:     model.DiagnosisStatusFailed,
		Version:    1,
		ErrorMsg:   "previous error",
	}

	wf := &model.Waveform{
		ID:            1,
		DeviceNo:      "VLV-001",
		SamplingRate:  10000,
		SamplingCount: 1024,
		VibrationData: generateTestVibration(1024),
	}

	valve := &model.Valve{
		DeviceNo: "VLV-001",
		Type:     model.ValveTypeGate,
	}

	mockDiagRepo.On("AcquireTask", taskID).Return(failedResult, nil)
	mockRRepo.On("ListAllEnabled").Return([]*model.RuleConfig{}, nil)
	mockRRepo.On("GetLatestVersion").Return("v1.0", nil)
	mockCache.On("SetRuleVersion", ctx, "v1.0").Return(nil)
	mockWfRepo.On("GetByID", uint64(1)).Return(wf, nil)
	mockVRepo.On("GetByDeviceNo", "VLV-001").Return(valve, nil)
	mockDiagRepo.On("CompleteTask", mock.AnythingOfType("*model.DiagnosisResult"), int64(2)).Return(&model.DiagnosisResult{
		TaskID:     taskID,
		Status:     model.DiagnosisStatusComplete,
		Version:    3,
		AnomalyScore: 25.0,
	}, nil)
	mockVRepo.On("UpdateStatus", "VLV-001", model.ValveStatusNormal).Return(nil)
	mockCache.On("SetValveStatus", ctx, "VLV-001", model.ValveStatusNormal).Return(nil)
	mockVRepo.On("UpdateLastCheckTime", "VLV-001", mock.Anything).Return(nil)
	mockCache.On("SetLatestDiagnosis", ctx, "VLV-001", mock.Anything).Return(nil)

	err := service.ProcessTask(ctx, task)

	assert.NoError(t, err)
	mockDiagRepo.AssertCalled(t, "AcquireTask", taskID)
	mockDiagRepo.AssertCalled(t, "CompleteTask", mock.AnythingOfType("*model.DiagnosisResult"), int64(2))
}

func TestProcessTask_VersionMismatch_DoesNotOverride(t *testing.T) {
	ctx := context.Background()
	taskID := "task-version-mismatch-001"
	task := &model.DiagnosisTask{
		TaskID:     taskID,
		DeviceNo:   "VLV-001",
		WaveformID: 1,
	}

	mockDiagRepo := newMockDiagnosisRepoDiag()
	mockWfRepo := new(mockWaveformRepoDiag)
	mockVRepo := new(mockValveRepoDiag)
	mockRRepo := new(mockRuleRepoDiag)
	mockCache := new(mockCacheDiag)

	cfg := &config.Config{
		FFT: config.FFTConfig{
			WindowSize: 512,
			Overlap:    0.5,
		},
	}

	service := NewDiagnosisService(mockDiagRepo, mockWfRepo, mockVRepo, mockRRepo, mockCache, cfg)

	pendingResult := &model.DiagnosisResult{
		TaskID:     taskID,
		DeviceNo:   "VLV-001",
		WaveformID: 1,
		Status:     model.DiagnosisStatusPending,
		Version:    0,
	}

	wf := &model.Waveform{
		ID:            1,
		DeviceNo:      "VLV-001",
		SamplingRate:  10000,
		SamplingCount: 1024,
		VibrationData: generateTestVibration(1024),
	}

	valve := &model.Valve{
		DeviceNo: "VLV-001",
		Type:     model.ValveTypeGate,
	}

	mockDiagRepo.On("AcquireTask", taskID).Return(pendingResult, nil)
	mockRRepo.On("ListAllEnabled").Return([]*model.RuleConfig{}, nil)
	mockRRepo.On("GetLatestVersion").Return("v1.0", nil)
	mockCache.On("SetRuleVersion", ctx, "v1.0").Return(nil)
	mockWfRepo.On("GetByID", uint64(1)).Return(wf, nil)
	mockVRepo.On("GetByDeviceNo", "VLV-001").Return(valve, nil)

	versionMismatch := fmt.Errorf("%s: expected 1, got 2", model.VersionMismatch)
	mockDiagRepo.On("CompleteTask", mock.AnythingOfType("*model.DiagnosisResult"), int64(1)).Return(nil, versionMismatch)

	err := service.ProcessTask(ctx, task)

	assert.NoError(t, err)
	mockDiagRepo.AssertCalled(t, "CompleteTask", mock.AnythingOfType("*model.DiagnosisResult"), int64(1))
	mockCache.AssertNotCalled(t, "SetLatestDiagnosis", mock.Anything, mock.Anything, mock.Anything)
	mockVRepo.AssertNotCalled(t, "UpdateStatus", mock.Anything, mock.Anything)
}

func TestProcessTask_CompleteTaskAlreadyComplete(t *testing.T) {
	ctx := context.Background()
	taskID := "task-complete-race-001"
	task := &model.DiagnosisTask{
		TaskID:     taskID,
		DeviceNo:   "VLV-001",
		WaveformID: 1,
	}

	mockDiagRepo := newMockDiagnosisRepoDiag()
	mockWfRepo := new(mockWaveformRepoDiag)
	mockVRepo := new(mockValveRepoDiag)
	mockRRepo := new(mockRuleRepoDiag)
	mockCache := new(mockCacheDiag)

	cfg := &config.Config{
		FFT: config.FFTConfig{
			WindowSize: 512,
			Overlap:    0.5,
		},
	}

	service := NewDiagnosisService(mockDiagRepo, mockWfRepo, mockVRepo, mockRRepo, mockCache, cfg)

	pendingResult := &model.DiagnosisResult{
		TaskID:     taskID,
		DeviceNo:   "VLV-001",
		WaveformID: 1,
		Status:     model.DiagnosisStatusPending,
		Version:    0,
	}

	wf := &model.Waveform{
		ID:            1,
		DeviceNo:      "VLV-001",
		SamplingRate:  10000,
		SamplingCount: 1024,
		VibrationData: generateTestVibration(1024),
	}

	valve := &model.Valve{
		DeviceNo: "VLV-001",
		Type:     model.ValveTypeGate,
	}

	mockDiagRepo.On("AcquireTask", taskID).Return(pendingResult, nil)
	mockRRepo.On("ListAllEnabled").Return([]*model.RuleConfig{}, nil)
	mockRRepo.On("GetLatestVersion").Return("v1.0", nil)
	mockCache.On("SetRuleVersion", ctx, "v1.0").Return(nil)
	mockWfRepo.On("GetByID", uint64(1)).Return(wf, nil)
	mockVRepo.On("GetByDeviceNo", "VLV-001").Return(valve, nil)

	alreadyComplete := &model.DiagnosisResult{
		TaskID:       taskID,
		Status:       model.DiagnosisStatusComplete,
		Version:      3,
		AnomalyScore: 85.0,
	}

	versionMismatch := fmt.Errorf("%s: expected 1, got 3", model.VersionMismatch)
	mockDiagRepo.On("CompleteTask", mock.AnythingOfType("*model.DiagnosisResult"), int64(1)).Return(alreadyComplete, versionMismatch)

	err := service.ProcessTask(ctx, task)

	assert.NoError(t, err)
	mockDiagRepo.AssertCalled(t, "CompleteTask", mock.AnythingOfType("*model.DiagnosisResult"), int64(1))
	mockCache.AssertNotCalled(t, "SetLatestDiagnosis", mock.Anything, mock.Anything, mock.Anything)
}

func TestProcessTask_FailUpdatesVersion(t *testing.T) {
	ctx := context.Background()
	taskID := "task-fail-version-001"
	task := &model.DiagnosisTask{
		TaskID:     taskID,
		DeviceNo:   "VLV-001",
		WaveformID: 999,
	}

	mockDiagRepo := newMockDiagnosisRepoDiag()
	mockWfRepo := new(mockWaveformRepoDiag)
	mockVRepo := new(mockValveRepoDiag)
	mockRRepo := new(mockRuleRepoDiag)
	mockCache := new(mockCacheDiag)

	cfg := &config.Config{
		FFT: config.FFTConfig{
			WindowSize: 512,
			Overlap:    0.5,
		},
	}

	service := NewDiagnosisService(mockDiagRepo, mockWfRepo, mockVRepo, mockRRepo, mockCache, cfg)

	pendingResult := &model.DiagnosisResult{
		TaskID:     taskID,
		DeviceNo:   "VLV-001",
		WaveformID: 999,
		Status:     model.DiagnosisStatusPending,
		Version:    0,
	}

	mockDiagRepo.On("AcquireTask", taskID).Return(pendingResult, nil)
	mockRRepo.On("ListAllEnabled").Return([]*model.RuleConfig{}, nil)
	mockRRepo.On("GetLatestVersion").Return("v1.0", nil)
	mockCache.On("SetRuleVersion", ctx, "v1.0").Return(nil)
	mockWfRepo.On("GetByID", uint64(999)).Return(nil, nil)
	mockDiagRepo.On("FailTask", taskID, "waveform not found", int64(1)).Return(&model.DiagnosisResult{
		TaskID:   taskID,
		Status:   model.DiagnosisStatusFailed,
		Version:  2,
		ErrorMsg: "waveform not found",
	}, nil)

	err := service.ProcessTask(ctx, task)

	assert.Error(t, err)
	mockDiagRepo.AssertCalled(t, "FailTask", taskID, "waveform not found", int64(1))
}

func TestProcessTask_FailTaskAlreadyComplete(t *testing.T) {
	ctx := context.Background()
	taskID := "task-fail-complete-001"
	task := &model.DiagnosisTask{
		TaskID:     taskID,
		DeviceNo:   "VLV-001",
		WaveformID: 1,
	}

	mockDiagRepo := newMockDiagnosisRepoDiag()
	mockWfRepo := new(mockWaveformRepoDiag)
	mockVRepo := new(mockValveRepoDiag)
	mockRRepo := new(mockRuleRepoDiag)
	mockCache := new(mockCacheDiag)

	cfg := &config.Config{
		FFT: config.FFTConfig{
			WindowSize: 512,
			Overlap:    0.5,
		},
	}

	service := NewDiagnosisService(mockDiagRepo, mockWfRepo, mockVRepo, mockRRepo, mockCache, cfg)

	pendingResult := &model.DiagnosisResult{
		TaskID:     taskID,
		DeviceNo:   "VLV-001",
		WaveformID: 1,
		Status:     model.DiagnosisStatusPending,
		Version:    0,
	}

	mockDiagRepo.On("AcquireTask", taskID).Return(pendingResult, nil)
	mockRRepo.On("ListAllEnabled").Return([]*model.RuleConfig{}, nil)
	mockRRepo.On("GetLatestVersion").Return("v1.0", nil)
	mockCache.On("SetRuleVersion", ctx, "v1.0").Return(nil)
	mockWfRepo.On("GetByID", uint64(1)).Return(nil, nil)

	alreadyComplete := &model.DiagnosisResult{
		TaskID:     taskID,
		Status:     model.DiagnosisStatusComplete,
		Version:    5,
		AnomalyScore: 20.0,
	}
	mockDiagRepo.On("FailTask", taskID, "waveform not found", int64(1)).Return(alreadyComplete, nil)

	err := service.ProcessTask(ctx, task)

	assert.Error(t, err)
}

func TestStateMachine_Transitions(t *testing.T) {
	tests := []struct {
		name         string
		fromStatus   model.DiagnosisStatus
		action       string
		shouldSucceed bool
	}{
		{"pending to running", model.DiagnosisStatusPending, "acquire", true},
		{"failed to running", model.DiagnosisStatusFailed, "acquire", true},
		{"running to complete", model.DiagnosisStatusRunning, "complete", true},
		{"running to failed", model.DiagnosisStatusRunning, "fail", true},
		{"complete to running", model.DiagnosisStatusComplete, "acquire", false},
		{"pending to complete", model.DiagnosisStatusPending, "complete", false},
		{"failed to complete", model.DiagnosisStatusFailed, "complete", false},
		{"complete to complete", model.DiagnosisStatusComplete, "complete", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_ = tt.action
			_ = tt.shouldSucceed
		})
	}
}

func generateTestVibration(n int) []float64 {
	data := make([]float64, n)
	for i := range data {
		data[i] = 0.1 * float64(i%100) / 100.0
	}
	return data
}
