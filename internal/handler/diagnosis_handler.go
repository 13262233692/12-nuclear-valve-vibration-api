package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"nuclear-valve-vibration-api/internal/model"
	"nuclear-valve-vibration-api/internal/service"
)

type DiagnosisHandler struct {
	service service.DiagnosisService
}

func NewDiagnosisHandler(service service.DiagnosisService) *DiagnosisHandler {
	return &DiagnosisHandler{service: service}
}

func (h *DiagnosisHandler) GetByID(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Invalid diagnosis ID",
			"error":   err.Error(),
		})
		return
	}

	result, err := h.service.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to get diagnosis result",
			"error":   err.Error(),
		})
		return
	}

	if result == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "Diagnosis result not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "Success",
		"data":    result,
	})
}

func (h *DiagnosisHandler) GetByTaskID(c *gin.Context) {
	taskID := c.Param("taskId")
	if taskID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Task ID is required",
		})
		return
	}

	result, err := h.service.GetByTaskID(c.Request.Context(), taskID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to get diagnosis result",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "Success",
		"data":    result,
	})
}

func (h *DiagnosisHandler) ListByDeviceNo(c *gin.Context) {
	deviceNo := c.Param("deviceNo")
	if deviceNo == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Device number is required",
		})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))

	var startTime, endTime *time.Time
	var status *model.DiagnosisStatus

	startTimeStr := c.Query("start_time")
	endTimeStr := c.Query("end_time")
	statusStr := c.Query("status")

	if startTimeStr != "" {
		if t, err := time.Parse(time.RFC3339, startTimeStr); err == nil {
			startTime = &t
		}
	}
	if endTimeStr != "" {
		if t, err := time.Parse(time.RFC3339, endTimeStr); err == nil {
			endTime = &t
		}
	}
	if statusStr != "" {
		s := model.DiagnosisStatus(statusStr)
		status = &s
	}

	results, total, err := h.service.ListByDeviceNo(c.Request.Context(), deviceNo, page, pageSize, startTime, endTime, status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to list diagnosis results",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "Success",
		"data": gin.H{
			"list":     results,
			"total":    total,
			"page":     page,
			"pageSize": pageSize,
		},
	})
}

func (h *DiagnosisHandler) GetLatest(c *gin.Context) {
	deviceNo := c.Param("deviceNo")
	if deviceNo == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Device number is required",
		})
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))

	results, err := h.service.GetLatestByDeviceNo(c.Request.Context(), deviceNo, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to get latest diagnosis results",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "Success",
		"data":    results,
	})
}

func (h *DiagnosisHandler) GetStats(c *gin.Context) {
	deviceNo := c.Param("deviceNo")
	if deviceNo == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Device number is required",
		})
		return
	}

	startTimeStr := c.Query("start_time")
	endTimeStr := c.Query("end_time")

	var startTime, endTime time.Time
	var err error

	if startTimeStr != "" {
		startTime, err = time.Parse(time.RFC3339, startTimeStr)
	} else {
		startTime = time.Now().AddDate(0, 0, -7)
	}

	if endTimeStr != "" {
		endTime, err = time.Parse(time.RFC3339, endTimeStr)
	} else {
		endTime = time.Now()
	}

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Invalid time format",
			"error":   err.Error(),
		})
		return
	}

	stats, err := h.service.GetStats(c.Request.Context(), deviceNo, startTime, endTime)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to get diagnosis stats",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "Success",
		"data":    stats,
	})
}

func (h *DiagnosisHandler) SubmitTask(c *gin.Context) {
	deviceNo := c.Param("deviceNo")
	if deviceNo == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Device number is required",
		})
		return
	}

	var req struct {
		WaveformID uint64 `json:"waveform_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Invalid request parameters",
			"error":   err.Error(),
		})
		return
	}

	taskID, err := h.service.SubmitTask(c.Request.Context(), deviceNo, req.WaveformID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to submit diagnosis task",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"code":    200,
		"message": "Success",
		"data": gin.H{
			"task_id": taskID,
		},
	})
}
