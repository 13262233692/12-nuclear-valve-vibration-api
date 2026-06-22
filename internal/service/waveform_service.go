package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"

	"nuclear-valve-vibration-api/internal/model"
	"nuclear-valve-vibration-api/internal/mq"
	"nuclear-valve-vibration-api/internal/repository"
	"nuclear-valve-vibration-api/internal/waveform"
)

type WaveformService interface {
	Upload(ctx context.Context, data []byte) (*model.Waveform, string, error)
	UploadRaw(ctx context.Context, deviceNo string, samplingRate, samplingCount uint32, channelNo uint8, collectTime time.Time, vibrationData []float64) (*model.Waveform, string, error)
	GetByID(ctx context.Context, id uint64) (*model.Waveform, error)
	GetByHash(ctx context.Context, hash string) (*model.Waveform, error)
	ListByDeviceNo(ctx context.Context, deviceNo string, page, pageSize int, startTime, endTime *time.Time) ([]*model.Waveform, int64, error)
	Delete(ctx context.Context, id uint64) error
}

type waveformService struct {
	repo        repository.WaveformRepository
	valveRepo   repository.ValveRepository
	diagRepo    repository.DiagnosisRepository
	mq          mq.MessageQueue
}

func NewWaveformService(repo repository.WaveformRepository, valveRepo repository.ValveRepository, diagRepo repository.DiagnosisRepository, mq mq.MessageQueue) WaveformService {
	return &waveformService{
		repo:      repo,
		valveRepo: valveRepo,
		diagRepo:  diagRepo,
		mq:        mq,
	}
}

func (s *waveformService) Upload(ctx context.Context, data []byte) (*model.Waveform, string, error) {
	parsed, err := waveform.Parse(data)
	if err != nil {
		return nil, "", fmt.Errorf("failed to parse waveform: %w", err)
	}

	exists, err := s.repo.ExistsByHash(parsed.WaveformHash)
	if err != nil {
		return nil, "", err
	}
	if exists {
		existing, err := s.repo.GetByHash(parsed.WaveformHash)
		if err != nil {
			return nil, "", err
		}
		return existing, "", errors.New("duplicate waveform upload")
	}

	valveExists, err := s.valveRepo.ExistsByDeviceNo(parsed.DeviceNo)
	if err != nil {
		return nil, "", err
	}
	if !valveExists {
		return nil, "", errors.New("device not registered")
	}

	wf := parsed.ToModel()
	if err := s.repo.Create(wf); err != nil {
		return nil, "", err
	}

	taskID, err := s.createDiagnosisTask(ctx, wf)
	if err != nil {
		return wf, "", fmt.Errorf("waveform uploaded but failed to create diagnosis task: %w", err)
	}

	return wf, taskID, nil
}

func (s *waveformService) UploadRaw(ctx context.Context, deviceNo string, samplingRate, samplingCount uint32, channelNo uint8, collectTime time.Time, vibrationData []float64) (*model.Waveform, string, error) {
	binData, err := waveform.Generate(deviceNo, samplingRate, samplingCount, channelNo, collectTime, vibrationData)
	if err != nil {
		return nil, "", err
	}
	return s.Upload(ctx, binData)
}

func (s *waveformService) createDiagnosisTask(ctx context.Context, wf *model.Waveform) (string, error) {
	taskID := uuid.New().String()

	diagResult := &model.DiagnosisResult{
		TaskID:     taskID,
		DeviceNo:   wf.DeviceNo,
		WaveformID: wf.ID,
		Status:     model.DiagnosisStatusPending,
		StartTime:  time.Now(),
	}

	if err := s.diagRepo.Create(diagResult); err != nil {
		return "", err
	}

	task := &model.DiagnosisTask{
		TaskID:     taskID,
		DeviceNo:   wf.DeviceNo,
		WaveformID: wf.ID,
		Priority:   0,
		RetryCount: 0,
		CreatedAt:  time.Now(),
	}

	if err := s.mq.Publish(ctx, task); err != nil {
		_ = s.diagRepo.UpdateWithError(taskID, fmt.Sprintf("failed to publish to MQ: %v", err))
		return "", err
	}

	return taskID, nil
}

func (s *waveformService) GetByID(ctx context.Context, id uint64) (*model.Waveform, error) {
	return s.repo.GetByID(id)
}

func (s *waveformService) GetByHash(ctx context.Context, hash string) (*model.Waveform, error) {
	return s.repo.GetByHash(hash)
}

func (s *waveformService) ListByDeviceNo(ctx context.Context, deviceNo string, page, pageSize int, startTime, endTime *time.Time) ([]*model.Waveform, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	return s.repo.ListByDeviceNo(deviceNo, page, pageSize, startTime, endTime)
}

func (s *waveformService) Delete(ctx context.Context, id uint64) error {
	_, err := s.repo.GetByID(id)
	if err != nil {
		return err
	}
	return s.repo.Delete(id)
}

func GenerateTestWaveform(deviceNo string, anomalyType model.AnomalyType, duration float64) ([]byte, error) {
	samplingRate := uint32(10000)
	samplingCount := uint32(float64(samplingRate) * duration)
	if samplingCount < 1024 {
		samplingCount = 1024
	}

	vibrationData := make([]float64, samplingCount)
	mainFreq := 50.0
	amplitude := 0.1

	switch anomalyType {
	case model.AnomalyTypeJamming:
		mainFreq = 15
		amplitude = 0.15
		for i := range vibrationData {
			t := float64(i) / float64(samplingRate)
			vibrationData[i] = amplitude*math.Sin(2*math.Pi*mainFreq*t) +
				0.05*math.Sin(2*math.Pi*mainFreq*2*t) +
				0.03*math.Sin(2*math.Pi*mainFreq*3*t)
		}
	case model.AnomalyTypeCavitation:
		for i := range vibrationData {
			t := float64(i) / float64(samplingRate)
			highFreq := 2000 + float64(i%1000)
			vibrationData[i] = 0.02*math.Sin(2*math.Pi*50*t) +
				0.08*math.Sin(2*math.Pi*highFreq*t)
		}
	case model.AnomalyTypeLooseness:
		for i := range vibrationData {
			t := float64(i) / float64(samplingRate)
			vibrationData[i] = 0.2*math.Sin(2*math.Pi*100*t) +
				0.1*math.Sin(2*math.Pi*200*t) +
				0.05*math.Sin(2*math.Pi*300*t)
		}
	case model.AnomalyTypeBearing:
		for i := range vibrationData {
			t := float64(i) / float64(samplingRate)
			vibrationData[i] = 0.05*math.Sin(2*math.Pi*50*t) +
				0.1*math.Sin(2*math.Pi*500*t) +
				0.08*math.Sin(2*math.Pi*1000*t) +
				0.06*math.Sin(2*math.Pi*1500*t)
		}
	default:
		for i := range vibrationData {
			t := float64(i) / float64(samplingRate)
			vibrationData[i] = 0.05 * math.Sin(2*math.Pi*50*t)
		}
	}

	return waveform.Generate(deviceNo, samplingRate, samplingCount, 1, time.Now(), vibrationData)
}
