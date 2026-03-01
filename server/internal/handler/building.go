package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// BuildingHandler groups building-management route handlers.
type BuildingHandler struct{}

// NewBuildingHandler constructs a BuildingHandler.
func NewBuildingHandler() *BuildingHandler { return &BuildingHandler{} }

// List handles GET /api/v1/buildings
func (h *BuildingHandler) List(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": nil})
}

// Create handles POST /api/v1/buildings
func (h *BuildingHandler) Create(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": nil})
}

// UpgradeLevel handles PUT /api/v1/buildings/:id/level
func (h *BuildingHandler) UpgradeLevel(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": nil})
}

// FinishEarly handles POST /api/v1/buildings/:id/finish-early
// Spends premium currency to skip the remaining build timer.
func (h *BuildingHandler) FinishEarly(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": nil})
}
