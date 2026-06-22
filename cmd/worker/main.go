package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"nuclear-valve-vibration-api/internal/cache"
	"nuclear-valve-vibration-api/internal/config"
	"nuclear-valve-vibration-api/internal/model"
	"nuclear-valve-vibration-api/internal/mq"
	"nuclear-valve-vibration-api/internal/repository"
	"nuclear-valve-vibration-api/internal/service"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	db, err := repository.NewDatabase(&cfg.Postgres)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	redisCache, err := cache.NewRedisCache(&cfg.Redis)
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer redisCache.Close()

	natsMQ, err := mq.NewNATS(&cfg.NATS)
	if err != nil {
		log.Fatalf("Failed to connect to NATS: %v", err)
	}
	defer natsMQ.Close()

	valveRepo := repository.NewValveRepository(db.DB)
	waveformRepo := repository.NewWaveformRepository(db.DB)
	diagnosisRepo := repository.NewDiagnosisRepository(db.DB)
	ruleRepo := repository.NewRuleRepository(db.DB)

	diagnosisService := service.NewDiagnosisService(diagnosisRepo, waveformRepo, valveRepo, ruleRepo, redisCache, cfg)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	concurrency := cfg.Worker.Concurrency
	if concurrency < 1 {
		concurrency = 1
	}

	var wg sync.WaitGroup
	taskCh := make(chan *model.DiagnosisTask, concurrency*2)

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			log.Printf("Worker %d started", workerID)

			for task := range taskCh {
				log.Printf("Worker %d processing task %s", workerID, task.TaskID)

				if err := diagnosisService.ProcessTask(ctx, task); err != nil {
					log.Printf("Worker %d failed to process task %s: %v", workerID, task.TaskID, err)
					continue
				}

				log.Printf("Worker %d completed task %s", workerID, task.TaskID)
			}

			log.Printf("Worker %d stopped", workerID)
		}(i)
	}

	handler := func(task *model.DiagnosisTask) error {
		select {
		case taskCh <- task:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	if err := natsMQ.Subscribe(ctx, handler); err != nil {
		log.Fatalf("Failed to subscribe to NATS: %v", err)
	}

	log.Printf("Diagnosis worker started with %d concurrent workers", concurrency)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down worker...")

	cancel()

	close(taskCh)

	wg.Wait()

	log.Println("Worker exiting")
}
