package repository

import (
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"nuclear-valve-vibration-api/internal/model"
)

type DiagnosisRepository interface {
	Create(result *model.DiagnosisResult) error
	GetByID(id uint64) (*model.DiagnosisResult, error)
	GetByTaskID(taskID string) (*model.DiagnosisResult, error)
	ListByDeviceNo(deviceNo string, page, pageSize int, startTime, endTime *time.Time, status *model.DiagnosisStatus) ([]*model.DiagnosisResult, int64, error)
	UpdateStatus(taskID string, status model.DiagnosisStatus) error
	UpdateResult(result *model.DiagnosisResult) error
	UpdateWithError(taskID string, errorMsg string) error
	ExistsByTaskID(taskID string) (bool, error)
	GetLatestByDeviceNo(deviceNo string, limit int) ([]*model.DiagnosisResult, error)
	GetStats(deviceNo string, startTime, endTime time.Time) (map[string]interface{}, error)

	AcquireTask(taskID string) (*model.DiagnosisResult, error)
	CompleteTask(result *model.DiagnosisResult, version int64) (*model.DiagnosisResult, error)
	FailTask(taskID string, errorMsg string, version int64) (*model.DiagnosisResult, error)
	GetDB() *gorm.DB
}

type diagnosisRepository struct {
	db *gorm.DB
}

func NewDiagnosisRepository(db *gorm.DB) DiagnosisRepository {
	return &diagnosisRepository{db: db}
}

func (r *diagnosisRepository) GetDB() *gorm.DB {
	return r.db
}

func (r *diagnosisRepository) Create(result *model.DiagnosisResult) error {
	return r.db.Create(result).Error
}

func (r *diagnosisRepository) GetByID(id uint64) (*model.DiagnosisResult, error) {
	var result model.DiagnosisResult
	err := r.db.Preload("Waveform").Preload("Valve").Where("id = ?", id).First(&result).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &result, nil
}

func (r *diagnosisRepository) GetByTaskID(taskID string) (*model.DiagnosisResult, error) {
	var result model.DiagnosisResult
	err := r.db.Preload("Waveform").Preload("Valve").Where("task_id = ?", taskID).First(&result).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &result, nil
}

func (r *diagnosisRepository) ListByDeviceNo(deviceNo string, page, pageSize int, startTime, endTime *time.Time, status *model.DiagnosisStatus) ([]*model.DiagnosisResult, int64, error) {
	var results []*model.DiagnosisResult
	var total int64

	query := r.db.Model(&model.DiagnosisResult{}).Where("device_no = ?", deviceNo)

	if startTime != nil {
		query = query.Where("created_at >= ?", *startTime)
	}
	if endTime != nil {
		query = query.Where("created_at <= ?", *endTime)
	}
	if status != nil {
		query = query.Where("status = ?", *status)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	err := query.Preload("Waveform").Preload("Valve").
		Offset(offset).Limit(pageSize).Order("created_at DESC").Find(&results).Error
	if err != nil {
		return nil, 0, err
	}

	return results, total, nil
}

func (r *diagnosisRepository) UpdateStatus(taskID string, status model.DiagnosisStatus) error {
	updates := map[string]interface{}{
		"status":     status,
		"updated_at": time.Now(),
	}
	if status == model.DiagnosisStatusRunning {
		updates["start_time"] = time.Now()
	}
	return r.db.Model(&model.DiagnosisResult{}).Where("task_id = ?", taskID).Updates(updates).Error
}

func (r *diagnosisRepository) UpdateResult(result *model.DiagnosisResult) error {
	now := time.Now()
	result.EndTime = &now
	return r.db.Save(result).Error
}

func (r *diagnosisRepository) UpdateWithError(taskID string, errorMsg string) error {
	now := time.Now()
	return r.db.Model(&model.DiagnosisResult{}).
		Where("task_id = ?", taskID).
		Updates(map[string]interface{}{
			"status":     model.DiagnosisStatusFailed,
			"error_msg":  errorMsg,
			"end_time":   now,
			"updated_at": now,
		}).Error
}

func (r *diagnosisRepository) ExistsByTaskID(taskID string) (bool, error) {
	var count int64
	err := r.db.Model(&model.DiagnosisResult{}).Where("task_id = ?", taskID).Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *diagnosisRepository) GetLatestByDeviceNo(deviceNo string, limit int) ([]*model.DiagnosisResult, error) {
	var results []*model.DiagnosisResult
	err := r.db.Preload("Waveform").
		Where("device_no = ?", deviceNo).
		Order("created_at DESC").
		Limit(limit).
		Find(&results).Error
	if err != nil {
		return nil, err
	}
	return results, nil
}

func (r *diagnosisRepository) GetStats(deviceNo string, startTime, endTime time.Time) (map[string]interface{}, error) {
	type stats struct {
		TotalCount     int64
		NormalCount    int64
		WarningCount   int64
		CriticalCount  int64
		FailedCount    int64
		AvgAnomalyScore float64
		MaxAnomalyScore float64
	}

	var result stats

	query := r.db.Model(&model.DiagnosisResult{}).
		Where("device_no = ? AND created_at >= ? AND created_at <= ?", deviceNo, startTime, endTime)

	if err := query.Count(&result.TotalCount).Error; err != nil {
		return nil, err
	}

	subQuery := r.db.Model(&model.DiagnosisResult{}).
		Where("device_no = ? AND created_at >= ? AND created_at <= ?", deviceNo, startTime, endTime)

	if err := subQuery.Where("anomaly_score < 30").Count(&result.NormalCount).Error; err != nil {
		return nil, err
	}

	if err := subQuery.Where("anomaly_score >= 30 AND anomaly_score < 70").Count(&result.WarningCount).Error; err != nil {
		return nil, err
	}

	if err := subQuery.Where("anomaly_score >= 70").Count(&result.CriticalCount).Error; err != nil {
		return nil, err
	}

	if err := subQuery.Where("status = ?", model.DiagnosisStatusFailed).Count(&result.FailedCount).Error; err != nil {
		return nil, err
	}

	type avgResult struct {
		Avg float64
		Max float64
	}
	var avgRes avgResult
	if err := r.db.Model(&model.DiagnosisResult{}).
		Select("COALESCE(AVG(anomaly_score), 0) as avg, COALESCE(MAX(anomaly_score), 0) as max").
		Where("device_no = ? AND created_at >= ? AND created_at <= ?", deviceNo, startTime, endTime).
		Scan(&avgRes).Error; err != nil {
		return nil, err
	}
	result.AvgAnomalyScore = avgRes.Avg
	result.MaxAnomalyScore = avgRes.Max

	return map[string]interface{}{
		"total_count":      result.TotalCount,
		"normal_count":     result.NormalCount,
		"warning_count":    result.WarningCount,
		"critical_count":   result.CriticalCount,
		"failed_count":     result.FailedCount,
		"avg_anomaly_score": result.AvgAnomalyScore,
		"max_anomaly_score": result.MaxAnomalyScore,
	}, nil
}

func (r *diagnosisRepository) AcquireTask(taskID string) (*model.DiagnosisResult, error) {
	var result model.DiagnosisResult

	err := r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Set("gorm:query_option", "FOR UPDATE").
			Where("task_id = ?", taskID).
			First(&result).Error; err != nil {
			return err
		}

		switch result.Status {
		case model.DiagnosisStatusComplete:
			return fmt.Errorf("%s: %s", model.TaskAlreadyCompleted, taskID)
		case model.DiagnosisStatusRunning:
			return fmt.Errorf("%s: %s", model.TaskAlreadyRunning, taskID)
		case model.DiagnosisStatusPending, model.DiagnosisStatusFailed:
		default:
			return fmt.Errorf("%s: cannot transition from %s", model.StateTransitionError, result.Status)
		}

		now := time.Now()
		result.Status = model.DiagnosisStatusRunning
		result.StartTime = now
		result.Version++
		result.UpdatedAt = now

		if err := tx.Save(&result).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return &result, nil
}

func (r *diagnosisRepository) CompleteTask(result *model.DiagnosisResult, version int64) (*model.DiagnosisResult, error) {
	var updated model.DiagnosisResult

	err := r.db.Transaction(func(tx *gorm.DB) error {
		var current model.DiagnosisResult
		if err := tx.Set("gorm:query_option", "FOR UPDATE").
			Where("task_id = ?", result.TaskID).
			First(&current).Error; err != nil {
			return err
		}

		if current.Version != version {
			return fmt.Errorf("%s: expected %d, got %d", model.VersionMismatch, version, current.Version)
		}

		if current.Status == model.DiagnosisStatusComplete {
			result = &current
			return nil
		}

		if current.Status != model.DiagnosisStatusRunning {
			return fmt.Errorf("%s: cannot complete from status %s", model.StateTransitionError, current.Status)
		}

		now := time.Now()
		current.Status = model.DiagnosisStatusComplete
		current.EndTime = &now
		current.Version++
		current.UpdatedAt = now

		current.MainFrequency = result.MainFrequency
		current.MainEnergy = result.MainEnergy
		current.HarmonicEnergies = result.HarmonicEnergies
		current.BandEnergies = result.BandEnergies
		current.AnomalyScore = result.AnomalyScore
		current.AnomalyType = result.AnomalyType
		current.AnomalyDetails = result.AnomalyDetails
		current.FFTResult = result.FFTResult
		current.RuleVersion = result.RuleVersion

		if err := tx.Save(&current).Error; err != nil {
			return err
		}

		updated = current
		return nil
	})

	if err != nil {
		return nil, err
	}

	return &updated, nil
}

func (r *diagnosisRepository) FailTask(taskID string, errorMsg string, version int64) (*model.DiagnosisResult, error) {
	var updated model.DiagnosisResult

	err := r.db.Transaction(func(tx *gorm.DB) error {
		var current model.DiagnosisResult
		if err := tx.Set("gorm:query_option", "FOR UPDATE").
			Where("task_id = ?", taskID).
			First(&current).Error; err != nil {
			return err
		}

		if current.Version != version {
			return fmt.Errorf("%s: expected %d, got %d", model.VersionMismatch, version, current.Version)
		}

		if current.Status == model.DiagnosisStatusComplete {
			updated = current
			return nil
		}

		if current.Status != model.DiagnosisStatusRunning {
			return fmt.Errorf("%s: cannot fail from status %s", model.StateTransitionError, current.Status)
		}

		now := time.Now()
		current.Status = model.DiagnosisStatusFailed
		current.EndTime = &now
		current.ErrorMsg = errorMsg
		current.Version++
		current.UpdatedAt = now

		if err := tx.Save(&current).Error; err != nil {
			return err
		}

		updated = current
		return nil
	})

	if err != nil {
		return nil, err
	}

	return &updated, nil
}
