package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"nuclear-valve-vibration-api/internal/cache"
	"nuclear-valve-vibration-api/internal/config"
	"nuclear-valve-vibration-api/internal/model"
	"nuclear-valve-vibration-api/internal/repository"
)

type mockDiagnosisRepoDiag struct {
	mock.Mock
	repository.DiagnosisRepository
}

func (m *mockDiagnosisRepoDiag) Create(result *model.DiagnosisResult) error {
	args := m.Called(result)
	return args.Error(0)
}

func (m *mockDiagnosisRepoDiag) ExistsByTaskID(taskID string) (bool, error) {
	args := m.Called(taskID)
	return args.Bool(0), args.Error(1)
}

func (m *mockDiagnosisRepoDiag) GetByTaskID(taskID string) (*model.DiagnosisResult, error) {
	args := m.Called(taskID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.DiagnosisResult), args.Error(1)
}

func (m *mockDiagnosisRepoDiag) UpdateStatus(taskID string, status model.DiagnosisStatus) error {
	args := m.Called(taskID, status)
	return args.Error(0)
}

func (m *mockDiagnosisRepoDiag) UpdateResult(result *model.DiagnosisResult) error {
	args := m.Called(result)
	return args.Error(0)
}

func (m *mockDiagnosisRepoDiag) UpdateWithError(taskID string, errorMsg string) error {
	args := m.Called(taskID, errorMsg)
	return args.Error(0)
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

type mockCacheDiag struct {
	mock.Mock
	cache.Cache
}

func (m *mockCacheDiag) MarkTaskProcessing(ctx context.Context, taskID string) (bool, error) {
	args := m.Called(ctx, taskID)
	return args.Bool(0), args.Error(1)
}

func (m *mockCacheDiag) UnmarkTaskProcessing(ctx context.Context, taskID string) error {
	args := m.Called(ctx, taskID)
	return args.Error(0)
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

func TestDiagnosisProcessTask_DuplicateMessage(t *testing.T) {
	ctx := context.Background()
	taskID := "task-dup-001"
	task := &model.DiagnosisTask{
		TaskID:     taskID,
		DeviceNo:   "VLV-001",
		WaveformID: 1,
	}

	mockDiagRepo := new(mockDiagnosisRepoDiag)
	mockWfRepo := new(mockWaveformRepoDiag)
	mockVRepo := new(mockValveRepoDiag)
	mockRRepo := new(mockRuleRepoDiag)
	mockCache := new(mockCacheDiag)

	cfg := &config.Config{
		FFT: config.FFTConfig{
			WindowSize: 1024,
			Overlap:    0.5,
		},
	}

	service := NewDiagnosisService(mockDiagRepo, mockWfRepo, mockVRepo, mockRRepo, mockCache, cfg)

	mockCache.On("MarkTaskProcessing", ctx, taskID).Return(false, nil)

	err := service.ProcessTask(ctx, task)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already being processed")

	mockCache.AssertCalled(t, "MarkTaskProcessing", ctx, taskID)
	mockDiagRepo.AssertNotCalled(t, "ExistsByTaskID", mock.Anything)
	mockDiagRepo.AssertNotCalled(t, "UpdateStatus", mock.Anything, mock.Anything)
}

func TestDiagnosisProcessTask_TaskAlreadyCompleted(t *testing.T) {
	ctx := context.Background()
	taskID := "task-done-001"
	task := &model.DiagnosisTask{
		TaskID:     taskID,
		DeviceNo:   "VLV-001",
		WaveformID: 1,
	}

	mockDiagRepo := new(mockDiagnosisRepoDiag)
	mockWfRepo := new(mockWaveformRepoDiag)
	mockVRepo := new(mockValveRepoDiag)
	mockRRepo := new(mockRuleRepoDiag)
	mockCache := new(mockCacheDiag)

	cfg := &config.Config{
		FFT: config.FFTConfig{
			WindowSize: 1024,
			Overlap:    0.5,
		},
	}

	service := NewDiagnosisService(mockDiagRepo, mockWfRepo, mockVRepo, mockRRepo, mockCache, cfg)

	completedResult := &model.DiagnosisResult{
		TaskID: taskID,
		Status: model.DiagnosisStatusComplete,
	}

	mockCache.On("MarkTaskProcessing", ctx, taskID).Return(true, nil)
	mockCache.On("UnmarkTaskProcessing", ctx, taskID).Return(nil)
	mockDiagRepo.On("ExistsByTaskID", taskID).Return(true, nil)
	mockDiagRepo.On("GetByTaskID", taskID).Return(completedResult, nil)

	err := service.ProcessTask(ctx, task)

	assert.NoError(t, err)

	mockDiagRepo.AssertCalled(t, "ExistsByTaskID", taskID)
	mockDiagRepo.AssertNotCalled(t, "UpdateStatus", mock.Anything, mock.Anything)
}

func TestDiagnosisProcessTask_RedisError(t *testing.T) {
	ctx := context.Background()
	taskID := "task-redis-err-001"
	task := &model.DiagnosisTask{
		TaskID:     taskID,
		DeviceNo:   "VLV-001",
		WaveformID: 1,
	}

	mockDiagRepo := new(mockDiagnosisRepoDiag)
	mockWfRepo := new(mockWaveformRepoDiag)
	mockVRepo := new(mockValveRepoDiag)
	mockRRepo := new(mockRuleRepoDiag)
	mockCache := new(mockCacheDiag)

	cfg := &config.Config{
		FFT: config.FFTConfig{
			WindowSize: 1024,
			Overlap:    0.5,
		},
	}

	service := NewDiagnosisService(mockDiagRepo, mockWfRepo, mockVRepo, mockRRepo, mockCache, cfg)

	redisErr := errors.New("redis connection refused")
	mockCache.On("MarkTaskProcessing", ctx, taskID).Return(false, redisErr)

	err := service.ProcessTask(ctx, task)

	assert.Error(t, err)
	assert.Equal(t, redisErr, err)

	mockDiagRepo.AssertNotCalled(t, "ExistsByTaskID", mock.Anything)
}

func TestDiagnosisProcessTask_TaskNotFound(t *testing.T) {
	ctx := context.Background()
	taskID := "task-notfound-001"
	task := &model.DiagnosisTask{
		TaskID:     taskID,
		DeviceNo:   "VLV-001",
		WaveformID: 1,
	}

	mockDiagRepo := new(mockDiagnosisRepoDiag)
	mockWfRepo := new(mockWaveformRepoDiag)
	mockVRepo := new(mockValveRepoDiag)
	mockRRepo := new(mockRuleRepoDiag)
	mockCache := new(mockCacheDiag)

	cfg := &config.Config{
		FFT: config.FFTConfig{
			WindowSize: 1024,
			Overlap:    0.5,
		},
	}

	service := NewDiagnosisService(mockDiagRepo, mockWfRepo, mockVRepo, mockRRepo, mockCache, cfg)

	mockCache.On("MarkTaskProcessing", ctx, taskID).Return(true, nil)
	mockCache.On("UnmarkTaskProcessing", ctx, taskID).Return(nil)
	mockDiagRepo.On("ExistsByTaskID", taskID).Return(false, nil)

	err := service.ProcessTask(ctx, task)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found in database")

	mockDiagRepo.AssertCalled(t, "ExistsByTaskID", taskID)
}

func TestDiagnosisProcessTask_WaveformNotFound(t *testing.T) {
	ctx := context.Background()
	taskID := "task-wf-missing-001"
	task := &model.DiagnosisTask{
		TaskID:     taskID,
		DeviceNo:   "VLV-001",
		WaveformID: 999,
	}

	mockDiagRepo := new(mockDiagnosisRepoDiag)
	mockWfRepo := new(mockWaveformRepoDiag)
	mockVRepo := new(mockValveRepoDiag)
	mockRRepo := new(mockRuleRepoDiag)
	mockCache := new(mockCacheDiag)

	cfg := &config.Config{
		FFT: config.FFTConfig{
			WindowSize: 1024,
			Overlap:    0.5,
		},
	}

	service := NewDiagnosisService(mockDiagRepo, mockWfRepo, mockVRepo, mockRRepo, mockCache, cfg)

	pendingResult := &model.DiagnosisResult{
		TaskID: taskID,
		Status: model.DiagnosisStatusPending,
	}

	mockCache.On("MarkTaskProcessing", ctx, taskID).Return(true, nil)
	mockCache.On("UnmarkTaskProcessing", ctx, taskID).Return(nil)
	mockDiagRepo.On("ExistsByTaskID", taskID).Return(true, nil)
	mockDiagRepo.On("GetByTaskID", taskID).Return(pendingResult, nil)
	mockDiagRepo.On("UpdateStatus", taskID, model.DiagnosisStatusRunning).Return(nil)
	mockRRepo.On("ListAllEnabled").Return([]*model.RuleConfig{}, nil)
	mockRRepo.On("GetLatestVersion").Return("v1.0", nil)
	mockCache.On("SetRuleVersion", ctx, "v1.0").Return(nil)
	mockWfRepo.On("GetByID", uint64(999)).Return(nil, nil)
	mockDiagRepo.On("UpdateWithError", taskID, mock.Anything).Return(nil)

	err := service.ProcessTask(ctx, task)

	assert.Error(t, err)
	mockDiagRepo.AssertCalled(t, "UpdateWithError", taskID, mock.Anything)
}

func TestDiagnosisProcessTask_IdempotencyViaDoubleCall(t *testing.T) {
	ctx := context.Background()
	taskID := "task-idem-001"
	task := &model.DiagnosisTask{
		TaskID:     taskID,
		DeviceNo:   "VLV-001",
		WaveformID: 1,
	}

	mockDiagRepo := new(mockDiagnosisRepoDiag)
	mockWfRepo := new(mockWaveformRepoDiag)
	mockVRepo := new(mockValveRepoDiag)
	mockRRepo := new(mockRuleRepoDiag)
	mockCache := new(mockCacheDiag)

	cfg := &config.Config{
		FFT: config.FFTConfig{
			WindowSize: 1024,
			Overlap:    0.5,
		},
	}

	service := NewDiagnosisService(mockDiagRepo, mockWfRepo, mockVRepo, mockRRepo, mockCache, cfg)

	completedResult := &model.DiagnosisResult{
		TaskID:     taskID,
		DeviceNo:   "VLV-001",
		WaveformID: 1,
		Status:     model.DiagnosisStatusComplete,
	}

	mockCache.On("MarkTaskProcessing", ctx, taskID).Return(true, nil).Once()
	mockCache.On("UnmarkTaskProcessing", ctx, taskID).Return(nil).Once()
	mockDiagRepo.On("ExistsByTaskID", taskID).Return(true, nil).Once()
	mockDiagRepo.On("GetByTaskID", taskID).Return(completedResult, nil).Once()

	err1 := service.ProcessTask(ctx, task)
	assert.NoError(t, err1)

	mockCache.On("MarkTaskProcessing", ctx, taskID).Return(false, nil).Once()
	err2 := service.ProcessTask(ctx, task)
	assert.Error(t, err2)
	assert.Contains(t, err2.Error(), "already being processed")

	mockCache.AssertNumberOfCalls(t, "MarkTaskProcessing", 2)
	mockDiagRepo.AssertNumberOfCalls(t, "ExistsByTaskID", 1)
}
