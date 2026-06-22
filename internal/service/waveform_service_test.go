package service

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"nuclear-valve-vibration-api/internal/model"
	"nuclear-valve-vibration-api/internal/mq"
	"nuclear-valve-vibration-api/internal/repository"
	"nuclear-valve-vibration-api/internal/waveform"
)

type mockWaveformRepo struct {
	mock.Mock
	repository.WaveformRepository
}

func (m *mockWaveformRepo) Create(wf *model.Waveform) error {
	args := m.Called(wf)
	return args.Error(0)
}

func (m *mockWaveformRepo) ExistsByHash(hash string) (bool, error) {
	args := m.Called(hash)
	return args.Bool(0), args.Error(1)
}

func (m *mockWaveformRepo) GetByHash(hash string) (*model.Waveform, error) {
	args := m.Called(hash)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Waveform), args.Error(1)
}

type mockValveRepo struct {
	mock.Mock
	repository.ValveRepository
}

func (m *mockValveRepo) ExistsByDeviceNo(deviceNo string) (bool, error) {
	args := m.Called(deviceNo)
	return args.Bool(0), args.Error(1)
}

type mockDiagnosisRepo struct {
	mock.Mock
	repository.DiagnosisRepository
}

func (m *mockDiagnosisRepo) Create(result *model.DiagnosisResult) error {
	args := m.Called(result)
	return args.Error(0)
}

func (m *mockDiagnosisRepo) UpdateWithError(taskID string, errorMsg string) error {
	args := m.Called(taskID, errorMsg)
	return args.Error(0)
}

type mockMQ struct {
	mock.Mock
	mq.MessageQueue
}

func (m *mockMQ) Publish(ctx context.Context, task *model.DiagnosisTask) error {
	args := m.Called(ctx, task)
	return args.Error(0)
}

func generateValidWaveformData(deviceNo string) ([]byte, *model.Waveform) {
	samplingRate := uint32(10000)
	samplingCount := uint32(2048)
	vibrationData := make([]float64, samplingCount)
	for i := range vibrationData {
		t := float64(i) / float64(samplingRate)
		vibrationData[i] = 0.1 * math.Sin(2*math.Pi*50*t)
	}

	data, _ := waveform.Generate(deviceNo, samplingRate, samplingCount, 1, time.Now(), vibrationData)

	parsed, _ := waveform.Parse(data)
	wf := parsed.ToModel()
	return data, wf
}

func TestWaveformUpload_DuplicateDetection(t *testing.T) {
	ctx := context.Background()
	deviceNo := "VLV-DUP-001"
	waveData, expectedWf := generateValidWaveformData(deviceNo)

	mockWfRepo := new(mockWaveformRepo)
	mockVRepo := new(mockValveRepo)
	mockDiagRepo := new(mockDiagnosisRepo)
	mockMQ := new(mockMQ)

	service := NewWaveformService(mockWfRepo, mockVRepo, mockDiagRepo, mockMQ)

	mockWfRepo.On("ExistsByHash", expectedWf.WaveformHash).Return(true, nil)
	mockWfRepo.On("GetByHash", expectedWf.WaveformHash).Return(expectedWf, nil)

	wf, taskID, err := service.Upload(ctx, waveData)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate")
	assert.NotNil(t, wf)
	assert.Equal(t, expectedWf.WaveformHash, wf.WaveformHash)
	assert.Equal(t, "", taskID)

	mockWfRepo.AssertCalled(t, "ExistsByHash", expectedWf.WaveformHash)
	mockWfRepo.AssertNotCalled(t, "Create", mock.Anything)
	mockDiagRepo.AssertNotCalled(t, "Create", mock.Anything)
	mockMQ.AssertNotCalled(t, "Publish", mock.Anything, mock.Anything)
}

func TestWaveformUpload_DeviceNotRegistered(t *testing.T) {
	ctx := context.Background()
	deviceNo := "VLV-NOREG-001"
	waveData, expectedWf := generateValidWaveformData(deviceNo)

	mockWfRepo := new(mockWaveformRepo)
	mockVRepo := new(mockValveRepo)
	mockDiagRepo := new(mockDiagnosisRepo)
	mockMQ := new(mockMQ)

	service := NewWaveformService(mockWfRepo, mockVRepo, mockDiagRepo, mockMQ)

	mockWfRepo.On("ExistsByHash", expectedWf.WaveformHash).Return(false, nil)
	mockVRepo.On("ExistsByDeviceNo", deviceNo).Return(false, nil)

	_, _, err := service.Upload(ctx, waveData)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "device not registered")

	mockWfRepo.AssertCalled(t, "ExistsByHash", expectedWf.WaveformHash)
	mockVRepo.AssertCalled(t, "ExistsByDeviceNo", deviceNo)
	mockWfRepo.AssertNotCalled(t, "Create", mock.Anything)
}

func TestWaveformUpload_Success(t *testing.T) {
	ctx := context.Background()
	deviceNo := "VLV-OK-001"
	waveData, _ := generateValidWaveformData(deviceNo)

	mockWfRepo := new(mockWaveformRepo)
	mockVRepo := new(mockValveRepo)
	mockDiagRepo := new(mockDiagnosisRepo)
	mockMQ := new(mockMQ)

	service := NewWaveformService(mockWfRepo, mockVRepo, mockDiagRepo, mockMQ)

	mockWfRepo.On("ExistsByHash", mock.Anything).Return(false, nil)
	mockVRepo.On("ExistsByDeviceNo", deviceNo).Return(true, nil)
	mockWfRepo.On("Create", mock.Anything).Return(nil)
	mockDiagRepo.On("Create", mock.Anything).Return(nil)
	mockMQ.On("Publish", ctx, mock.Anything).Return(nil)

	wf, taskID, err := service.Upload(ctx, waveData)

	assert.NoError(t, err)
	assert.NotNil(t, wf)
	assert.Equal(t, deviceNo, wf.DeviceNo)
	assert.NotEmpty(t, taskID)

	mockWfRepo.AssertCalled(t, "Create", mock.Anything)
	mockDiagRepo.AssertCalled(t, "Create", mock.Anything)
	mockMQ.AssertCalled(t, "Publish", ctx, mock.Anything)
}

func TestWaveformUpload_InvalidParse(t *testing.T) {
	ctx := context.Background()
	badData := []byte{0x00, 0x00, 0x01, 0x02, 0x03}

	mockWfRepo := new(mockWaveformRepo)
	mockVRepo := new(mockValveRepo)
	mockDiagRepo := new(mockDiagnosisRepo)
	mockMQ := new(mockMQ)

	service := NewWaveformService(mockWfRepo, mockVRepo, mockDiagRepo, mockMQ)

	_, _, err := service.Upload(ctx, badData)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse waveform")

	mockWfRepo.AssertNotCalled(t, "ExistsByHash", mock.Anything)
	mockWfRepo.AssertNotCalled(t, "Create", mock.Anything)
}

func TestWaveformUpload_DuplicateHash_RepoError(t *testing.T) {
	ctx := context.Background()
	deviceNo := "VLV-ERR-001"
	waveData, expectedWf := generateValidWaveformData(deviceNo)

	mockWfRepo := new(mockWaveformRepo)
	mockVRepo := new(mockValveRepo)
	mockDiagRepo := new(mockDiagnosisRepo)
	mockMQ := new(mockMQ)

	service := NewWaveformService(mockWfRepo, mockVRepo, mockDiagRepo, mockMQ)

	dbErr := errors.New("database connection error")
	mockWfRepo.On("ExistsByHash", expectedWf.WaveformHash).Return(false, dbErr)

	_, _, err := service.Upload(ctx, waveData)

	assert.Error(t, err)
	assert.Equal(t, dbErr, err)

	mockWfRepo.AssertNotCalled(t, "Create", mock.Anything)
}

func TestWaveformUpload_MQPublishFails(t *testing.T) {
	ctx := context.Background()
	deviceNo := "VLV-MQERR-001"
	waveData, _ := generateValidWaveformData(deviceNo)

	mockWfRepo := new(mockWaveformRepo)
	mockVRepo := new(mockValveRepo)
	mockDiagRepo := new(mockDiagnosisRepo)
	mockMQ := new(mockMQ)

	service := NewWaveformService(mockWfRepo, mockVRepo, mockDiagRepo, mockMQ)

	mqErr := errors.New("NATS connection failed")
	mockWfRepo.On("ExistsByHash", mock.Anything).Return(false, nil)
	mockVRepo.On("ExistsByDeviceNo", deviceNo).Return(true, nil)
	mockWfRepo.On("Create", mock.Anything).Return(nil)
	mockDiagRepo.On("Create", mock.Anything).Return(nil)
	mockMQ.On("Publish", ctx, mock.Anything).Return(mqErr)
	mockDiagRepo.On("UpdateWithError", mock.Anything, mock.Anything).Return(nil)

	wf, taskID, err := service.Upload(ctx, waveData)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create diagnosis task")
	assert.NotNil(t, wf)
	assert.Equal(t, "", taskID)

	mockWfRepo.AssertCalled(t, "Create", mock.Anything)
	mockDiagRepo.AssertCalled(t, "UpdateWithError", mock.Anything, mock.Anything)
}

func TestGenerateTestWaveform(t *testing.T) {
	tests := []struct {
		name        string
		anomalyType model.AnomalyType
		duration    float64
	}{
		{"normal waveform", "", 1.0},
		{"jamming waveform", model.AnomalyTypeJamming, 0.5},
		{"cavitation waveform", model.AnomalyTypeCavitation, 2.0},
		{"looseness waveform", model.AnomalyTypeLooseness, 1.0},
		{"bearing waveform", model.AnomalyTypeBearing, 1.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := GenerateTestWaveform("VLV-TEST-001", tt.anomalyType, tt.duration)
			assert.NoError(t, err)
			assert.NotEmpty(t, data)

			parsed, err := waveform.Parse(data)
			assert.NoError(t, err)
			assert.NotNil(t, parsed)
			assert.Equal(t, "VLV-TEST-001", parsed.DeviceNo)
			assert.Greater(t, parsed.SamplingCount, uint32(0))
			assert.Greater(t, len(parsed.VibrationData), 0)
		})
	}
}
