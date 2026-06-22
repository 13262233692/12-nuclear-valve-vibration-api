package service

import (
	"context"
	"errors"
	"fmt"

	"nuclear-valve-vibration-api/internal/cache"
	"nuclear-valve-vibration-api/internal/diagnosis"
	"nuclear-valve-vibration-api/internal/model"
	"nuclear-valve-vibration-api/internal/repository"
)

type RuleService interface {
	Create(ctx context.Context, rule *model.RuleConfig) (*model.RuleConfig, error)
	GetByID(ctx context.Context, id uint64) (*model.RuleConfig, error)
	GetByTypeAndAnomaly(ctx context.Context, valveType model.ValveType, anomalyType model.AnomalyType) (*model.RuleConfig, error)
	List(ctx context.Context, valveType *model.ValveType, anomalyType *model.AnomalyType, enabledOnly bool) ([]*model.RuleConfig, error)
	Update(ctx context.Context, rule *model.RuleConfig) (*model.RuleConfig, error)
	Delete(ctx context.Context, id uint64) error
	ToggleEnabled(ctx context.Context, id uint64, enabled bool) error
	InitDefaultRules(ctx context.Context) error
	ReloadRules(ctx context.Context) error
}

type ruleService struct {
	repo    repository.RuleRepository
	cache   cache.Cache
	engine  *diagnosis.Engine
}

func NewRuleService(repo repository.RuleRepository, cache cache.Cache, engine *diagnosis.Engine) RuleService {
	return &ruleService{
		repo:   repo,
		cache:  cache,
		engine: engine,
	}
}

func (s *ruleService) Create(ctx context.Context, rule *model.RuleConfig) (*model.RuleConfig, error) {
	if rule.ValveType == "" {
		return nil, errors.New("valve_type is required")
	}
	if rule.AnomalyType == "" {
		return nil, errors.New("anomaly_type is required")
	}
	if rule.Name == "" {
		return nil, errors.New("name is required")
	}
	if rule.Threshold <= 0 {
		return nil, errors.New("threshold must be greater than 0")
	}
	if rule.Version == "" {
		rule.Version = "v1.0"
	}

	existing, err := s.repo.GetByTypeAndAnomaly(rule.ValveType, rule.AnomalyType)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, fmt.Errorf("rule already exists for valve_type=%s, anomaly_type=%s", rule.ValveType, rule.AnomalyType)
	}

	if err := s.repo.Create(rule); err != nil {
		return nil, err
	}

	_ = s.reloadEngine(ctx)

	return rule, nil
}

func (s *ruleService) GetByID(ctx context.Context, id uint64) (*model.RuleConfig, error) {
	return s.repo.GetByID(id)
}

func (s *ruleService) GetByTypeAndAnomaly(ctx context.Context, valveType model.ValveType, anomalyType model.AnomalyType) (*model.RuleConfig, error) {
	return s.repo.GetByTypeAndAnomaly(valveType, anomalyType)
}

func (s *ruleService) List(ctx context.Context, valveType *model.ValveType, anomalyType *model.AnomalyType, enabledOnly bool) ([]*model.RuleConfig, error) {
	return s.repo.List(valveType, anomalyType, enabledOnly)
}

func (s *ruleService) Update(ctx context.Context, rule *model.RuleConfig) (*model.RuleConfig, error) {
	existing, err := s.repo.GetByID(rule.ID)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, errors.New("rule not found")
	}

	if err := s.repo.Update(rule); err != nil {
		return nil, err
	}

	_ = s.reloadEngine(ctx)

	return rule, nil
}

func (s *ruleService) Delete(ctx context.Context, id uint64) error {
	_, err := s.repo.GetByID(id)
	if err != nil {
		return err
	}

	if err := s.repo.Delete(id); err != nil {
		return err
	}

	_ = s.reloadEngine(ctx)

	return nil
}

func (s *ruleService) ToggleEnabled(ctx context.Context, id uint64, enabled bool) error {
	_, err := s.repo.GetByID(id)
	if err != nil {
		return err
	}

	if err := s.repo.ToggleEnabled(id, enabled); err != nil {
		return err
	}

	_ = s.reloadEngine(ctx)

	return nil
}

func (s *ruleService) InitDefaultRules(ctx context.Context) error {
	defaultRules := diagnosis.GetDefaultRules()

	count, err := s.repo.ListAllEnabled()
	if err != nil {
		return err
	}
	if len(count) > 0 {
		return errors.New("rules already initialized")
	}

	for _, rule := range defaultRules {
		rule.Enabled = true
		if err := s.repo.Create(rule); err != nil {
			return fmt.Errorf("failed to create rule %s: %w", rule.Name, err)
		}
	}

	_ = s.reloadEngine(ctx)

	return nil
}

func (s *ruleService) ReloadRules(ctx context.Context) error {
	return s.reloadEngine(ctx)
}

func (s *ruleService) reloadEngine(ctx context.Context) error {
	rules, err := s.repo.ListAllEnabled()
	if err != nil {
		return err
	}

	s.engine.LoadRules(rules)

	version, _ := s.repo.GetLatestVersion()
	_ = s.cache.SetRuleVersion(ctx, version)

	return nil
}
