package service

import (
	"context"
	"errors"

	"nuclear-valve-vibration-api/internal/cache"
	"nuclear-valve-vibration-api/internal/model"
	"nuclear-valve-vibration-api/internal/repository"
)

type ValveService interface {
	Register(ctx context.Context, valve *model.Valve) (*model.Valve, error)
	GetByID(ctx context.Context, id uint64) (*model.Valve, error)
	GetByDeviceNo(ctx context.Context, deviceNo string) (*model.Valve, error)
	List(ctx context.Context, page, pageSize int, valveType model.ValveType, status model.ValveStatus) ([]*model.Valve, int64, error)
	Update(ctx context.Context, valve *model.Valve) (*model.Valve, error)
	Delete(ctx context.Context, id uint64) error
	UpdateStatus(ctx context.Context, deviceNo string, status model.ValveStatus) error
}

type valveService struct {
	repo  repository.ValveRepository
	cache cache.Cache
}

func NewValveService(repo repository.ValveRepository, cache cache.Cache) ValveService {
	return &valveService{
		repo:  repo,
		cache: cache,
	}
}

func (s *valveService) Register(ctx context.Context, valve *model.Valve) (*model.Valve, error) {
	if valve.DeviceNo == "" {
		return nil, errors.New("device_no is required")
	}
	if valve.Name == "" {
		return nil, errors.New("name is required")
	}
	if valve.Type == "" {
		return nil, errors.New("type is required")
	}

	exists, err := s.repo.ExistsByDeviceNo(valve.DeviceNo)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, errors.New("device already registered")
	}

	if valve.Status == "" {
		valve.Status = model.ValveStatusNormal
	}

	if err := s.repo.Create(valve); err != nil {
		return nil, err
	}

	_ = s.cache.SetValveStatus(ctx, valve.DeviceNo, valve.Status)

	return valve, nil
}

func (s *valveService) GetByID(ctx context.Context, id uint64) (*model.Valve, error) {
	return s.repo.GetByID(id)
}

func (s *valveService) GetByDeviceNo(ctx context.Context, deviceNo string) (*model.Valve, error) {
	return s.repo.GetByDeviceNo(deviceNo)
}

func (s *valveService) List(ctx context.Context, page, pageSize int, valveType model.ValveType, status model.ValveStatus) ([]*model.Valve, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	return s.repo.List(page, pageSize, valveType, status)
}

func (s *valveService) Update(ctx context.Context, valve *model.Valve) (*model.Valve, error) {
	existing, err := s.repo.GetByID(valve.ID)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, errors.New("valve not found")
	}

	if err := s.repo.Update(valve); err != nil {
		return nil, err
	}

	_ = s.cache.DeleteValveStatus(ctx, valve.DeviceNo)
	_ = s.cache.SetValveStatus(ctx, valve.DeviceNo, valve.Status)

	return valve, nil
}

func (s *valveService) Delete(ctx context.Context, id uint64) error {
	valve, err := s.repo.GetByID(id)
	if err != nil {
		return err
	}
	if valve == nil {
		return errors.New("valve not found")
	}

	if err := s.repo.Delete(id); err != nil {
		return err
	}

	_ = s.cache.DeleteValveStatus(ctx, valve.DeviceNo)
	_ = s.cache.DeleteLatestDiagnosis(ctx, valve.DeviceNo)

	return nil
}

func (s *valveService) UpdateStatus(ctx context.Context, deviceNo string, status model.ValveStatus) error {
	exists, err := s.repo.ExistsByDeviceNo(deviceNo)
	if err != nil {
		return err
	}
	if !exists {
		return errors.New("valve not found")
	}

	if err := s.repo.UpdateStatus(deviceNo, status); err != nil {
		return err
	}

	_ = s.cache.SetValveStatus(ctx, deviceNo, status)

	return nil
}
