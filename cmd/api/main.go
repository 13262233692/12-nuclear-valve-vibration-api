package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"nuclear-valve-vibration-api/internal/app"
	"nuclear-valve-vibration-api/internal/cache"
	"nuclear-valve-vibration-api/internal/config"
	"nuclear-valve-vibration-api/internal/diagnosis"
	"nuclear-valve-vibration-api/internal/handler"
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

	diagnosisEngine := diagnosis.NewEngine()
	rules, err := ruleRepo.ListAllEnabled()
	if err == nil && len(rules) > 0 {
		diagnosisEngine.LoadRules(rules)
	} else {
		defaultRules := diagnosis.GetDefaultRules()
		for _, rule := range defaultRules {
			rule.Enabled = true
			_ = ruleRepo.Create(rule)
		}
		diagnosisEngine.LoadRules(defaultRules)
	}

	valveService := service.NewValveService(valveRepo, redisCache)
	waveformService := service.NewWaveformService(waveformRepo, valveRepo, diagnosisRepo, natsMQ)
	diagnosisService := service.NewDiagnosisService(diagnosisRepo, waveformRepo, valveRepo, ruleRepo, redisCache, cfg)
	ruleService := service.NewRuleService(ruleRepo, redisCache, diagnosisEngine)

	valveHandler := handler.NewValveHandler(valveService)
	waveformHandler := handler.NewWaveformHandler(waveformService)
	diagnosisHandler := handler.NewDiagnosisHandler(diagnosisService)
	ruleHandler := handler.NewRuleHandler(ruleService)

	r := app.SetupRouter(valveHandler, waveformHandler, diagnosisHandler, ruleHandler)

	srv := &http.Server{
		Addr:    cfg.Server.Addr(),
		Handler: r,
	}

	go func() {
		log.Printf("Server starting on %s", cfg.Server.Addr())
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	ctxShutDown, cancelShutDown := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelShutDown()

	if err := srv.Shutdown(ctxShutDown); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exiting")
}
