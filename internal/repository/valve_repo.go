package repository

import (
	"errors"
	"time"

	"gorm.io/gorm"

	"nuclear-valve-vibration-api/internal/model"
)

type ValveRepository interface {
	Create(valve *model.Valve) error
	GetByID(id uint64) (*model.Valve, error)
	GetByDeviceNo(deviceNo string) (*model.Valve, error)
	List(page, pageSize int, valveType model.ValveType, status model.ValveStatus) ([]*model.Valve, int64, error)
	Update(valve *model.Valve) error
	Delete(id uint64) error
	UpdateStatus(deviceNo string, status model.ValveStatus) error
	UpdateLastCheckTime(deviceNo string, checkTime time.Time) error
	ExistsByDeviceNo(deviceNo string) (bool, error)
}

type valveRepository struct {
	db *gorm.DB
}

func NewValveRepository(db *gorm.DB) ValveRepository {
	return &valveRepository{db: db}
}

func (r *valveRepository) Create(valve *model.Valve) error {
	return r.db.Create(valve).Error
}

func (r *valveRepository) GetByID(id uint64) (*model.Valve, error) {
	var valve model.Valve
	err := r.db.Where("id = ?", id).First(&valve).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &valve, nil
}

func (r *valveRepository) GetByDeviceNo(deviceNo string) (*model.Valve, error) {
	var valve model.Valve
	err := r.db.Where("device_no = ?", deviceNo).First(&valve).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &valve, nil
}

func (r *valveRepository) List(page, pageSize int, valveType model.ValveType, status model.ValveStatus) ([]*model.Valve, int64, error) {
	var valves []*model.Valve
	var total int64

	query := r.db.Model(&model.Valve{})

	if valveType != "" {
		query = query.Where("type = ?", valveType)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	err := query.Offset(offset).Limit(pageSize).Order("created_at DESC").Find(&valves).Error
	if err != nil {
		return nil, 0, err
	}

	return valves, total, nil
}

func (r *valveRepository) Update(valve *model.Valve) error {
	return r.db.Save(valve).Error
}

func (r *valveRepository) Delete(id uint64) error {
	return r.db.Delete(&model.Valve{}, id).Error
}

func (r *valveRepository) UpdateStatus(deviceNo string, status model.ValveStatus) error {
	return r.db.Model(&model.Valve{}).
		Where("device_no = ?", deviceNo).
		Updates(map[string]interface{}{
			"status":     status,
			"updated_at": time.Now(),
		}).Error
}

func (r *valveRepository) UpdateLastCheckTime(deviceNo string, checkTime time.Time) error {
	return r.db.Model(&model.Valve{}).
		Where("device_no = ?", deviceNo).
		Updates(map[string]interface{}{
			"last_check_time": checkTime,
			"updated_at":      time.Now(),
		}).Error
}

func (r *valveRepository) ExistsByDeviceNo(deviceNo string) (bool, error) {
	var count int64
	err := r.db.Model(&model.Valve{}).Where("device_no = ?", deviceNo).Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
