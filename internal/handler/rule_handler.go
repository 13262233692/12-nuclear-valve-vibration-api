package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"nuclear-valve-vibration-api/internal/model"
	"nuclear-valve-vibration-api/internal/service"
)

type RuleHandler struct {
	service service.RuleService
}

func NewRuleHandler(service service.RuleService) *RuleHandler {
	return &RuleHandler{service: service}
}

func (h *RuleHandler) Create(c *gin.Context) {
	var rule model.RuleConfig
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Invalid request parameters",
			"error":   err.Error(),
		})
		return
	}

	result, err := h.service.Create(c.Request.Context(), &rule)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to create rule",
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

func (h *RuleHandler) GetByID(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Invalid rule ID",
			"error":   err.Error(),
		})
		return
	}

	rule, err := h.service.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to get rule",
			"error":   err.Error(),
		})
		return
	}

	if rule == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "Rule not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "Success",
		"data":    rule,
	})
}

func (h *RuleHandler) GetByTypeAndAnomaly(c *gin.Context) {
	valveType := model.ValveType(c.Param("valveType"))
	anomalyType := model.AnomalyType(c.Param("anomalyType"))

	if valveType == "" || anomalyType == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Valve type and anomaly type are required",
		})
		return
	}

	rule, err := h.service.GetByTypeAndAnomaly(c.Request.Context(), valveType, anomalyType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to get rule",
			"error":   err.Error(),
		})
		return
	}

	if rule == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "Rule not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "Success",
		"data":    rule,
	})
}

func (h *RuleHandler) List(c *gin.Context) {
	var valveType *model.ValveType
	var anomalyType *model.AnomalyType

	vt := c.Query("valve_type")
	at := c.Query("anomaly_type")
	enabledOnly := c.Query("enabled_only") == "true"

	if vt != "" {
		v := model.ValveType(vt)
		valveType = &v
	}
	if at != "" {
		a := model.AnomalyType(at)
		anomalyType = &a
	}

	rules, err := h.service.List(c.Request.Context(), valveType, anomalyType, enabledOnly)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to list rules",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "Success",
		"data":    rules,
	})
}

func (h *RuleHandler) Update(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Invalid rule ID",
			"error":   err.Error(),
		})
		return
	}

	var rule model.RuleConfig
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Invalid request parameters",
			"error":   err.Error(),
		})
		return
	}

	rule.ID = id
	result, err := h.service.Update(c.Request.Context(), &rule)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to update rule",
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

func (h *RuleHandler) Delete(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Invalid rule ID",
			"error":   err.Error(),
		})
		return
	}

	if err := h.service.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to delete rule",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "Success",
	})
}

func (h *RuleHandler) ToggleEnabled(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Invalid rule ID",
			"error":   err.Error(),
		})
		return
	}

	var req struct {
		Enabled bool `json:"enabled" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Invalid request parameters",
			"error":   err.Error(),
		})
		return
	}

	if err := h.service.ToggleEnabled(c.Request.Context(), id, req.Enabled); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to toggle rule",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "Success",
	})
}

func (h *RuleHandler) InitDefaultRules(c *gin.Context) {
	if err := h.service.InitDefaultRules(c.Request.Context()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to initialize default rules",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "Default rules initialized successfully",
	})
}

func (h *RuleHandler) ReloadRules(c *gin.Context) {
	if err := h.service.ReloadRules(c.Request.Context()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to reload rules",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "Rules reloaded successfully",
	})
}
