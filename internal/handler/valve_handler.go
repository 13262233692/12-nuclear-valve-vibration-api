package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"nuclear-valve-vibration-api/internal/model"
	"nuclear-valve-vibration-api/internal/service"
)

type ValveHandler struct {
	service service.ValveService
}

func NewValveHandler(service service.ValveService) *ValveHandler {
	return &ValveHandler{service: service}
}

type RegisterValveRequest struct {
	DeviceNo     string             `json:"device_no" binding:"required"`
	Name         string             `json:"name" binding:"required"`
	Type         model.ValveType    `json:"type" binding:"required"`
	Location     string             `json:"location"`
	Manufacturer string             `json:"manufacturer"`
	Model        string             `json:"model"`
	InstallDate  *time.Time         `json:"install_date"`
	Description  string             `json:"description"`
}

func (h *ValveHandler) Register(c *gin.Context) {
	var req RegisterValveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Invalid request parameters",
			"error":   err.Error(),
		})
		return
	}

	valve := &model.Valve{
		DeviceNo:     req.DeviceNo,
		Name:         req.Name,
		Type:         req.Type,
		Location:     req.Location,
		Manufacturer: req.Manufacturer,
		Model:        req.Model,
		InstallDate:  req.InstallDate,
		Description:  req.Description,
	}

	result, err := h.service.Register(c.Request.Context(), valve)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to register valve",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"code":    200,
		"message": "Success",
		"data":    result,
	})
}

func (h *ValveHandler) GetByID(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Invalid valve ID",
			"error":   err.Error(),
		})
		return
	}

	valve, err := h.service.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to get valve",
			"error":   err.Error(),
		})
		return
	}

	if valve == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "Valve not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "Success",
		"data":    valve,
	})
}

func (h *ValveHandler) GetByDeviceNo(c *gin.Context) {
	deviceNo := c.Param("deviceNo")
	if deviceNo == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Device number is required",
		})
		return
	}

	valve, err := h.service.GetByDeviceNo(c.Request.Context(), deviceNo)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to get valve",
			"error":   err.Error(),
		})
		return
	}

	if valve == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "Valve not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "Success",
		"data":    valve,
	})
}

func (h *ValveHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	valveType := model.ValveType(c.Query("type"))
	status := model.ValveStatus(c.Query("status"))

	valves, total, err := h.service.List(c.Request.Context(), page, pageSize, valveType, status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to list valves",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "Success",
		"data": gin.H{
			"list":     valves,
			"total":    total,
			"page":     page,
			"pageSize": pageSize,
		},
	})
}

func (h *ValveHandler) Update(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Invalid valve ID",
			"error":   err.Error(),
		})
		return
	}

	var valve model.Valve
	if err := c.ShouldBindJSON(&valve); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Invalid request parameters",
			"error":   err.Error(),
		})
		return
	}

	valve.ID = id
	result, err := h.service.Update(c.Request.Context(), &valve)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to update valve",
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

func (h *ValveHandler) Delete(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Invalid valve ID",
			"error":   err.Error(),
		})
		return
	}

	if err := h.service.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to delete valve",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "Success",
	})
}

func (h *ValveHandler) UpdateStatus(c *gin.Context) {
	deviceNo := c.Param("deviceNo")
	if deviceNo == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Device number is required",
		})
		return
	}

	var req struct {
		Status model.ValveStatus `json:"status" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Invalid request parameters",
			"error":   err.Error(),
		})
		return
	}

	if err := h.service.UpdateStatus(c.Request.Context(), deviceNo, req.Status); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to update valve status",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "Success",
	})
}
