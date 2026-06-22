package app

import (
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"nuclear-valve-vibration-api/internal/handler"
	"nuclear-valve-vibration-api/internal/middleware"
)

func SetupRouter(
	valveHandler *handler.ValveHandler,
	waveformHandler *handler.WaveformHandler,
	diagnosisHandler *handler.DiagnosisHandler,
	ruleHandler *handler.RuleHandler,
) *gin.Engine {
	r := gin.Default()

	r.Use(middleware.CORS())
	r.Use(middleware.Logger())
	r.Use(middleware.Recovery())

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"code":    200,
			"message": "ok",
			"data": gin.H{
				"status": "running",
			},
		})
	})

	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	api := r.Group("/api/v1")
	{
		valves := api.Group("/valves")
		{
			valves.POST("", valveHandler.Register)
			valves.GET("", valveHandler.List)
			valves.GET("/:id", valveHandler.GetByID)
			valves.GET("/device/:deviceNo", valveHandler.GetByDeviceNo)
			valves.PUT("/:id", valveHandler.Update)
			valves.DELETE("/:id", valveHandler.Delete)
			valves.PATCH("/device/:deviceNo/status", valveHandler.UpdateStatus)
		}

		waveforms := api.Group("/waveforms")
		{
			waveforms.POST("/upload", waveformHandler.Upload)
			waveforms.GET("/test/generate", waveformHandler.GenerateTest)
			waveforms.GET("/:id", waveformHandler.GetByID)
			waveforms.GET("/hash/:hash", waveformHandler.GetByHash)
			waveforms.GET("/device/:deviceNo", waveformHandler.ListByDeviceNo)
			waveforms.DELETE("/:id", waveformHandler.Delete)
		}

		diagnosis := api.Group("/diagnosis")
		{
			diagnosis.POST("/device/:deviceNo/submit", diagnosisHandler.SubmitTask)
			diagnosis.GET("/:id", diagnosisHandler.GetByID)
			diagnosis.GET("/task/:taskId", diagnosisHandler.GetByTaskID)
			diagnosis.GET("/device/:deviceNo", diagnosisHandler.ListByDeviceNo)
			diagnosis.GET("/device/:deviceNo/latest", diagnosisHandler.GetLatest)
			diagnosis.GET("/device/:deviceNo/stats", diagnosisHandler.GetStats)
		}

		rules := api.Group("/rules")
		{
			rules.POST("", ruleHandler.Create)
			rules.GET("", ruleHandler.List)
			rules.GET("/:id", ruleHandler.GetByID)
			rules.GET("/:valveType/:anomalyType", ruleHandler.GetByTypeAndAnomaly)
			rules.PUT("/:id", ruleHandler.Update)
			rules.DELETE("/:id", ruleHandler.Delete)
			rules.PATCH("/:id/enabled", ruleHandler.ToggleEnabled)
			rules.POST("/init-default", ruleHandler.InitDefaultRules)
			rules.POST("/reload", ruleHandler.ReloadRules)
		}
	}

	return r
}
