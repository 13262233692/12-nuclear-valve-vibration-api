package repository

import (
	"errors"
	"time"

	"gorm.io/gorm"

	"nuclear-valve-vibration-api/internal/model"
)

type RuleRepository interface {
	Create(rule *model.RuleConfig) error
	GetByID(id uint64) (*model.RuleConfig, error)
	GetByTypeAndAnomaly(valveType model.ValveType, anomalyType model.AnomalyType) (*model.RuleConfig, error)
	List(valveType *model.ValveType, anomalyType *model.AnomalyType, enabledOnly bool) ([]*model.RuleConfig, error)
	ListAllEnabled() ([]*model.RuleConfig, error)
	Update(rule *model.RuleConfig) error
	Delete(id uint64) error
	ToggleEnabled(id uint64, enabled bool) error
	BatchUpsert(rules []*model.RuleConfig) error
	GetLatestVersion() (string, error)
}

type ruleRepository struct {
	db *gorm.DB
}

func NewRuleRepository(db *gorm.DB) RuleRepository {
	return &ruleRepository{db: db}
}

func (r *ruleRepository) Create(rule *model.RuleConfig) error {
	return r.db.Create(rule).Error
}

func (r *ruleRepository) GetByID(id uint64) (*model.RuleConfig, error) {
	var rule model.RuleConfig
	err := r.db.Where("id = ?", id).First(&rule).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &rule, nil
}

func (r *ruleRepository) GetByTypeAndAnomaly(valveType model.ValveType, anomalyType model.AnomalyType) (*model.RuleConfig, error) {
	var rule model.RuleConfig
	err := r.db.Where("valve_type = ? AND anomaly_type = ?", valveType, anomalyType).First(&rule).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &rule, nil
}

func (r *ruleRepository) List(valveType *model.ValveType, anomalyType *model.AnomalyType, enabledOnly bool) ([]*model.RuleConfig, error) {
	var rules []*model.RuleConfig

	query := r.db.Model(&model.RuleConfig{})

	if valveType != nil {
		query = query.Where("valve_type = ?", *valveType)
	}
	if anomalyType != nil {
		query = query.Where("anomaly_type = ?", *anomalyType)
	}
	if enabledOnly {
		query = query.Where("enabled = ?", true)
	}

	err := query.Order("valve_type, anomaly_type").Find(&rules).Error
	if err != nil {
		return nil, err
	}
	return rules, nil
}

func (r *ruleRepository) ListAllEnabled() ([]*model.RuleConfig, error) {
	var rules []*model.RuleConfig
	err := r.db.Where("enabled = ?", true).Find(&rules).Error
	if err != nil {
		return nil, err
	}
	return rules, nil
}

func (r *ruleRepository) Update(rule *model.RuleConfig) error {
	return r.db.Save(rule).Error
}

func (r *ruleRepository) Delete(id uint64) error {
	return r.db.Delete(&model.RuleConfig{}, id).Error
}

func (r *ruleRepository) ToggleEnabled(id uint64, enabled bool) error {
	return r.db.Model(&model.RuleConfig{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"enabled":    enabled,
			"updated_at": time.Now(),
		}).Error
}

func (r *ruleRepository) BatchUpsert(rules []*model.RuleConfig) error {
	if len(rules) == 0 {
		return nil
	}

	return r.db.Transaction(func(tx *gorm.DB) error {
		for _, rule := range rules {
			existing, err := r.GetByTypeAndAnomaly(rule.ValveType, rule.AnomalyType)
			if err != nil {
				return err
			}
			if existing != nil {
				rule.ID = existing.ID
				rule.CreatedAt = existing.CreatedAt
				if err := tx.Save(rule).Error; err != nil {
					return err
				}
			} else {
				if err := tx.Create(rule).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func (r *ruleRepository) GetLatestVersion() (string, error) {
	type versionResult struct {
		Version string
	}
	var result versionResult
	err := r.db.Model(&model.RuleConfig{}).
		Select("version").
		Order("updated_at DESC").
		Limit(1).
		Scan(&result).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", nil
		}
		return "", err
	}
	return result.Version, nil
}
