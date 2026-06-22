package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"

	"nuclear-valve-vibration-api/internal/config"
	"nuclear-valve-vibration-api/internal/model"
)

const (
	ValveStatusKey      = "valve:status:%s"
	DiagnosisLatestKey  = "diagnosis:latest:%s"
	DiagnosisStatsKey   = "diagnosis:stats:%s"
	TaskProcessingKey   = "task:processing:%s"
	RuleVersionKey      = "rule:version"

	DefaultTTL      = 5 * time.Minute
	StatusTTL       = 15 * time.Minute
	ProcessingTTL   = 10 * time.Minute
	LatestTTL       = 1 * time.Hour
)

type Cache interface {
	GetValveStatus(ctx context.Context, deviceNo string) (*model.ValveStatus, error)
	SetValveStatus(ctx context.Context, deviceNo string, status model.ValveStatus) error
	DeleteValveStatus(ctx context.Context, deviceNo string) error

	GetLatestDiagnosis(ctx context.Context, deviceNo string) (*model.DiagnosisResult, error)
	SetLatestDiagnosis(ctx context.Context, deviceNo string, result *model.DiagnosisResult) error
	DeleteLatestDiagnosis(ctx context.Context, deviceNo string) error

	MarkTaskProcessing(ctx context.Context, taskID string) (bool, error)
	UnmarkTaskProcessing(ctx context.Context, taskID string) error

	GetRuleVersion(ctx context.Context) (string, error)
	SetRuleVersion(ctx context.Context, version string) error

	Close() error
}

type redisCache struct {
	client *redis.Client
}

func NewRedisCache(cfg *config.RedisConfig) (Cache, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr(),
		Password: cfg.Password,
		DB:       cfg.DB,
		PoolSize: 50,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &redisCache{client: client}, nil
}

func (r *redisCache) GetValveStatus(ctx context.Context, deviceNo string) (*model.ValveStatus, error) {
	key := fmt.Sprintf(ValveStatusKey, deviceNo)
	val, err := r.client.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil
		}
		return nil, err
	}
	status := model.ValveStatus(val)
	return &status, nil
}

func (r *redisCache) SetValveStatus(ctx context.Context, deviceNo string, status model.ValveStatus) error {
	key := fmt.Sprintf(ValveStatusKey, deviceNo)
	return r.client.Set(ctx, key, string(status), StatusTTL).Err()
}

func (r *redisCache) DeleteValveStatus(ctx context.Context, deviceNo string) error {
	key := fmt.Sprintf(ValveStatusKey, deviceNo)
	return r.client.Del(ctx, key).Err()
}

func (r *redisCache) GetLatestDiagnosis(ctx context.Context, deviceNo string) (*model.DiagnosisResult, error) {
	key := fmt.Sprintf(DiagnosisLatestKey, deviceNo)
	data, err := r.client.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil
		}
		return nil, err
	}

	var result model.DiagnosisResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (r *redisCache) SetLatestDiagnosis(ctx context.Context, deviceNo string, result *model.DiagnosisResult) error {
	key := fmt.Sprintf(DiagnosisLatestKey, deviceNo)
	data, err := json.Marshal(result)
	if err != nil {
		return err
	}
	return r.client.Set(ctx, key, data, LatestTTL).Err()
}

func (r *redisCache) DeleteLatestDiagnosis(ctx context.Context, deviceNo string) error {
	key := fmt.Sprintf(DiagnosisLatestKey, deviceNo)
	return r.client.Del(ctx, key).Err()
}

func (r *redisCache) MarkTaskProcessing(ctx context.Context, taskID string) (bool, error) {
	key := fmt.Sprintf(TaskProcessingKey, taskID)
	ok, err := r.client.SetNX(ctx, key, "1", ProcessingTTL).Result()
	if err != nil {
		return false, err
	}
	return ok, nil
}

func (r *redisCache) UnmarkTaskProcessing(ctx context.Context, taskID string) error {
	key := fmt.Sprintf(TaskProcessingKey, taskID)
	return r.client.Del(ctx, key).Err()
}

func (r *redisCache) GetRuleVersion(ctx context.Context) (string, error) {
	val, err := r.client.Get(ctx, RuleVersionKey).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return "", nil
		}
		return "", err
	}
	return val, nil
}

func (r *redisCache) SetRuleVersion(ctx context.Context, version string) error {
	return r.client.Set(ctx, RuleVersionKey, version, DefaultTTL).Err()
}

func (r *redisCache) Close() error {
	return r.client.Close()
}
