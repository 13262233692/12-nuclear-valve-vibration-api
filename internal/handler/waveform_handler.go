package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"nuclear-valve-vibration-api/internal/model"
	"nuclear-valve-vibration-api/internal/service"
	"nuclear-valve-vibration-api/internal/waveform"
)

type WaveformHandler struct {
	service service.WaveformService
}

func NewWaveformHandler(service service.WaveformService) *WaveformHandler {
	return &WaveformHandler{service: service}
}

func (h *WaveformHandler) Upload(c *gin.Context) {
	data, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Failed to read request body",
			"error":   err.Error(),
		})
		return
	}

	if len(data) < waveform.HeaderSize+waveform.CRCSize {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Waveform data too short",
			"error":   "insufficient data",
		})
		return
	}

	wf, taskID, err := h.service.Upload(c.Request.Context(), data)
	if err != nil {
		if err.Error() == "duplicate waveform upload" {
			c.JSON(http.StatusConflict, gin.H{
				"code":    409,
				"message": "Duplicate waveform upload",
				"data": gin.H{
					"waveform_id": wf.ID,
					"hash":        wf.WaveformHash,
				},
			})
			return
		}

		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Failed to process waveform",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"code":    200,
		"message": "Success",
		"data": gin.H{
			"waveform_id": wf.ID,
			"device_no":   wf.DeviceNo,
			"task_id":     taskID,
			"hash":        wf.WaveformHash,
			"collect_time": wf.CollectTime,
		},
	})
}

func (h *WaveformHandler) GetByID(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Invalid waveform ID",
			"error":   err.Error(),
		})
		return
	}

	wf, err := h.service.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to get waveform",
			"error":   err.Error(),
		})
		return
	}

	if wf == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "Waveform not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "Success",
		"data":    wf,
	})
}

func (h *WaveformHandler) GetByHash(c *gin.Context) {
	hash := c.Param("hash")
	if hash == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Waveform hash is required",
		})
		return
	}

	wf, err := h.service.GetByHash(c.Request.Context(), hash)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to get waveform",
			"error":   err.Error(),
		})
		return
	}

	if wf == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "Waveform not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "Success",
		"data":    wf,
	})
}

func (h *WaveformHandler) ListByDeviceNo(c *gin.Context) {
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
	startTimeStr := c.Query("start_time")
	endTimeStr := c.Query("end_time")

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

	waveforms, total, err := h.service.ListByDeviceNo(c.Request.Context(), deviceNo, page, pageSize, startTime, endTime)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to list waveforms",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "Success",
		"data": gin.H{
			"list":     waveforms,
			"total":    total,
			"page":     page,
			"pageSize": pageSize,
		},
	})
}

func (h *WaveformHandler) Delete(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Invalid waveform ID",
			"error":   err.Error(),
		})
		return
	}

	if err := h.service.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to delete waveform",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "Success",
	})
}

func (h *WaveformHandler) GenerateTest(c *gin.Context) {
	deviceNo := c.Query("device_no")
	anomalyType := model.AnomalyType(c.Query("anomaly_type"))
	duration, _ := strconv.ParseFloat(c.DefaultQuery("duration", "1.0"), 64)

	if deviceNo == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Device number is required",
		})
		return
	}

	data, err := service.GenerateTestWaveform(deviceNo, anomalyType, duration)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to generate test waveform",
			"error":   err.Error(),
		})
		return
	}

	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Disposition", "attachment; filename=test_waveform.bin")
	c.Data(http.StatusOK, "application/octet-stream", data)
}
