package repository

import (
	"errors"
	"time"

	"gorm.io/gorm"

	"nuclear-valve-vibration-api/internal/model"
)

type WaveformRepository interface {
	Create(waveform *model.Waveform) error
	GetByID(id uint64) (*model.Waveform, error)
	GetByHash(hash string) (*model.Waveform, error)
	ListByDeviceNo(deviceNo string, page, pageSize int, startTime, endTime *time.Time) ([]*model.Waveform, int64, error)
	ExistsByHash(hash string) (bool, error)
	Delete(id uint64) error
}

type waveformRepository struct {
	db *gorm.DB
}

func NewWaveformRepository(db *gorm.DB) WaveformRepository {
	return &waveformRepository{db: db}
}

func (r *waveformRepository) Create(waveform *model.Waveform) error {
	return r.db.Create(waveform).Error
}

func (r *waveformRepository) GetByID(id uint64) (*model.Waveform, error) {
	var waveform model.Waveform
	err := r.db.Where("id = ?", id).First(&waveform).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &waveform, nil
}

func (r *waveformRepository) GetByHash(hash string) (*model.Waveform, error) {
	var waveform model.Waveform
	err := r.db.Where("waveform_hash = ?", hash).First(&waveform).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &waveform, nil
}

func (r *waveformRepository) ListByDeviceNo(deviceNo string, page, pageSize int, startTime, endTime *time.Time) ([]*model.Waveform, int64, error) {
	var waveforms []*model.Waveform
	var total int64

	query := r.db.Model(&model.Waveform{}).Where("device_no = ?", deviceNo)

	if startTime != nil {
		query = query.Where("collect_time >= ?", *startTime)
	}
	if endTime != nil {
		query = query.Where("collect_time <= ?", *endTime)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	err := query.Offset(offset).Limit(pageSize).Order("collect_time DESC").Find(&waveforms).Error
	if err != nil {
		return nil, 0, err
	}

	return waveforms, total, nil
}

func (r *waveformRepository) ExistsByHash(hash string) (bool, error) {
	var count int64
	err := r.db.Model(&model.Waveform{}).Where("waveform_hash = ?", hash).Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *waveformRepository) Delete(id uint64) error {
	return r.db.Delete(&model.Waveform{}, id).Error
}
