package handler

import (
	"fmt"
	"math/rand"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/liukai/farmer/server/ent"
	entschema "github.com/liukai/farmer/server/ent/schema"
	entfarm "github.com/liukai/farmer/server/ent/farm"
	entuser "github.com/liukai/farmer/server/ent/user"
	"github.com/liukai/farmer/server/internal/service"
	"github.com/liukai/farmer/server/internal/tick"

)

// FarmHandler groups farm-management route handlers.
type FarmHandler struct {
	db *ent.Client
}

// NewFarmHandler constructs a FarmHandler.
func NewFarmHandler(db *ent.Client) *FarmHandler { return &FarmHandler{db: db} }

// currentUserID extracts the authenticated user's UUID from the Gin context.
func currentUserID(c *gin.Context) (uuid.UUID, bool) {
	raw, exists := c.Get("userID")
	if !exists {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(raw.(string))
	if err != nil {
		return uuid.Nil, false
	}
	return id, true
}

// GetMine handles GET /api/v1/farms/mine
func (h *FarmHandler) GetMine(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "unauthorized", "data": nil})
		return
	}

	farm, err := h.db.Farm.Query().
		Where(entfarm.OwnerID(userID)).
		Only(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "farm not found", "data": nil})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "ok",
		"data": gin.H{
			"id":     farm.ID,
			"name":   farm.Name,
			"level":  farm.Level,
			"plots":  farm.Plots,
		},
	})
}

// GetByID handles GET /api/v1/farms/:id
func (h *FarmHandler) GetByID(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid farm id", "data": nil})
		return
	}
	farm, err := h.db.Farm.Get(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "farm not found", "data": nil})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": gin.H{
		"id": farm.ID, "name": farm.Name, "level": farm.Level, "plots": farm.Plots, "ownerId": farm.OwnerID,
	}})
}

// plantReq is the request body for POST /api/v1/farms/:id/plant
type plantReq struct {
	X      int    `json:"x"      binding:"min=0,max=7"`
	Y      int    `json:"y"      binding:"min=0,max=7"`
	CropID string `json:"cropId" binding:"required"`
}

// Plant handles POST /api/v1/farms/:id/plant
func (h *FarmHandler) Plant(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "unauthorized", "data": nil})
		return
	}

	var req plantReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error(), "data": nil})
		return
	}

	crop := service.GetCrop(req.CropID)
	if crop == nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "unknown crop: " + req.CropID, "data": nil})
		return
	}

	// T-082: Seasonal restriction — some crops cannot be planted in winter.
	if service.IsCropForbidden(req.CropID) {
		info := service.CurrentSeasonInfo()
		c.JSON(http.StatusConflict, gin.H{
			"code":    409,
			"message": fmt.Sprintf("该作物在%s季无法种植", info["season"]),
			"data":    info,
		})
		return
	}

	farm, err := h.db.Farm.Query().
		Where(entfarm.OwnerID(userID)).
		Only(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "farm not found", "data": nil})
		return
	}

	plots := farm.Plots
	idx := req.Y*8 + req.X
	if idx < 0 || idx >= len(plots) {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid plot coordinates", "data": nil})
		return
	}

	if plots[idx].Type != "empty" && plots[idx].Type != "tilled" {
		c.JSON(http.StatusConflict, gin.H{
			"code":    409,
			"message": fmt.Sprintf("plot (%d,%d) is not empty (current: %s)", req.X, req.Y, plots[idx].Type),
			"data":    nil,
		})
		return
	}

	now := time.Now()
	plots[idx] = entschema.PlotState{
		X:         req.X,
		Y:         req.Y,
		Type:      "planted",
		CropID:    req.CropID,
		PlantedAt: now.Format(time.RFC3339),
		Stage:     "seedling",
	}

	_, err = h.db.Farm.UpdateOne(farm).SetPlots(plots).Save(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "save failed", "data": nil})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "planted",
		"data":    plots[idx],
	})
}

// harvestReq is the request body for POST /api/v1/farms/:id/harvest
type harvestReq struct {
	X int `json:"x" binding:"min=0,max=7"`
	Y int `json:"y" binding:"min=0,max=7"`
}

// Plant handles POST /api/v1/farms/:id/harvest
func (h *FarmHandler) Harvest(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "unauthorized", "data": nil})
		return
	}

	var req harvestReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error(), "data": nil})
		return
	}

	farm, err := h.db.Farm.Query().
		Where(entfarm.OwnerID(userID)).
		Only(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "farm not found", "data": nil})
		return
	}

	plots := farm.Plots
	idx := req.Y*8 + req.X
	if idx < 0 || idx >= len(plots) {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid plot coordinates", "data": nil})
		return
	}

	plot := &plots[idx]
	if plot.Type != "planted" && plot.Type != "mature" {
		c.JSON(http.StatusConflict, gin.H{
			"code":    409,
			"message": fmt.Sprintf("plot (%d,%d) has nothing to harvest (current: %s)", req.X, req.Y, plot.Type),
			"data":    nil,
		})
		return
	}

	// Check maturity
	crop := service.GetCrop(plot.CropID)
	if crop == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "unknown crop in db", "data": nil})
		return
	}

	plantedAt, err := time.Parse(time.RFC3339, plot.PlantedAt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "invalid planted_at", "data": nil})
		return
	}
	// Apply watering bonus: 20% faster growth if watered
	effectiveDuration := crop.GrowDuration
	if plot.WateredAt != "" {
		effectiveDuration = time.Duration(float64(crop.GrowDuration) * 0.8)
	}
	if time.Since(plantedAt) < effectiveDuration {
		remaining := effectiveDuration - time.Since(plantedAt)
		c.JSON(http.StatusConflict, gin.H{
			"code":    409,
			"message": fmt.Sprintf("crop not ready yet, %.0f seconds remaining", remaining.Seconds()),
			"data":    nil,
		})
		return
	}

	// T-084: Determine crop quality based on care (watering, fertilizing) + randomness.
	quality := determineQuality(plot.WateredAt != "", plot.Fertilized)

	// Calculate yield with seasonal bonus (T-082) + quality bonus.
	yield := crop.YieldMin + rand.Intn(crop.YieldMax-crop.YieldMin+1)
	seasonMult := service.GetSeasonCoinBonus(crop.ID) // 1.5× for seasonal bonus crops
	qualityMult := qualityMultiplier(quality)          // 1.0/1.5/2.0× for normal/good/excellent
	coinReward := int64(float64(crop.CoinReward) * seasonMult * qualityMult)

	// Reset the plot to empty
	plots[idx] = entschema.PlotState{X: req.X, Y: req.Y, Type: "empty"}

	_, err = h.db.Farm.UpdateOne(farm).SetPlots(plots).Save(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "save failed", "data": nil})
		return
	}

	// Add harvested crop to inventory
	if err := service.AddToInventory(c.Request.Context(), h.db, userID, crop.ID, "crop", int64(yield)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "inventory update failed", "data": nil})
		return
	}

	// Grant coins + exp to user and check for level-up
	u, err := h.db.User.Query().Where(entuser.ID(userID)).Only(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "user lookup failed", "data": nil})
		return
	}
	oldLevel := u.Level
	newExp := u.Exp + int64(crop.ExpReward)
	newLevel := tick.CalcLevel(newExp)

	upd := h.db.User.UpdateOne(u).
		AddCoins(coinReward).
		AddExp(int64(crop.ExpReward))
	if newLevel > oldLevel {
		upd = upd.SetLevel(newLevel)
	}
	u, err = upd.Save(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "reward failed", "data": nil})
		return
	}

	// Record harvest activity (fire-and-forget; don't fail the harvest on log error)
	content := fmt.Sprintf("收获了 %d 个%s（%s品质），获得 %d 金币", yield, crop.Name, qualityLabel(quality), coinReward)
	_ = service.CreateActivityLog(c.Request.Context(), h.db, userID, "harvest", content, map[string]any{
		"cropId": crop.ID, "yield": yield, "coinReward": coinReward, "quality": quality,
	})
	if newLevel > oldLevel {
		lvMsg := fmt.Sprintf("升级啦！达到 Lv.%d", newLevel)
		_ = service.CreateActivityLog(c.Request.Context(), h.db, userID, "level_up", lvMsg, map[string]any{
			"newLevel": newLevel,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "harvested",
		"data": gin.H{
			"cropId":       crop.ID,
			"cropName":     crop.Name,
			"yield":        yield,
			"quality":      quality,
			"qualityLabel": qualityLabel(quality),
			"coinReward":   coinReward,
			"expReward":    crop.ExpReward,
			"newCoins":     u.Coins,
			"newExp":       u.Exp,
			"newLevel":     u.Level,
			"levelUp":      newLevel > oldLevel,
		},
	})
}

// waterReq is the request body for POST /api/v1/farms/:id/water
type waterReq struct {
	X int `json:"x" binding:"min=0,max=7"`
	Y int `json:"y" binding:"min=0,max=7"`
}

const waterStaminaCost = 2

// Water handles POST /api/v1/farms/:id/water
// Consumes 2 stamina from the *caller* and records wateredAt on the target plot.
// Works for both self-watering and visit-assist watering.
func (h *FarmHandler) Water(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "unauthorized", "data": nil})
		return
	}

	farmID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid farm id", "data": nil})
		return
	}

	var req waterReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error(), "data": nil})
		return
	}

	// Check stamina
	u, err := h.db.User.Query().Where(entuser.ID(userID)).Only(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "user not found", "data": nil})
		return
	}
	if u.Stamina < waterStaminaCost {
		c.JSON(http.StatusConflict, gin.H{
			"code":    409,
			"message": fmt.Sprintf("not enough stamina (need %d, have %d)", waterStaminaCost, u.Stamina),
			"data":    nil,
		})
		return
	}

	farm, err := h.db.Farm.Get(c.Request.Context(), farmID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "farm not found", "data": nil})
		return
	}

	idx := req.Y*8 + req.X
	plots := farm.Plots
	if idx < 0 || idx >= len(plots) {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid plot coordinates", "data": nil})
		return
	}

	plot := &plots[idx]
	if plot.Type != "planted" {
		c.JSON(http.StatusConflict, gin.H{
			"code":    409,
			"message": fmt.Sprintf("plot (%d,%d) has no growing crop to water", req.X, req.Y),
			"data":    nil,
		})
		return
	}
	if plot.WateredAt != "" {
		c.JSON(http.StatusConflict, gin.H{"code": 409, "message": "plot already watered this cycle", "data": nil})
		return
	}

	plot.WateredAt = time.Now().Format(time.RFC3339)

	// Deduct stamina and save plot atomically via sequential DB writes
	_, err = h.db.User.UpdateOne(u).AddStamina(-waterStaminaCost).Save(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "stamina update failed", "data": nil})
		return
	}
	_, err = h.db.Farm.UpdateOne(farm).SetPlots(plots).Save(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "farm save failed", "data": nil})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "watered",
		"data": gin.H{
			"x":              req.X,
			"y":              req.Y,
			"wateredAt":      plot.WateredAt,
			"remainingStamina": u.Stamina - waterStaminaCost,
		},
	})
}

// Visit handles POST /api/v1/farms/:id/visit
func (h *FarmHandler) Visit(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": nil})
}

// ── T-084: Crop Quality Helpers ───────────────────────────────────────────────

// determineQuality computes a crop's quality at harvest time.
// Base probability: 10% excellent / 30% good / 60% normal.
// Watering adds +10%/+10% to excellent/good; fertilizing adds +5%/+10%.
func determineQuality(watered, fertilized bool) string {
	excellent, good := 10, 30
	if watered {
		excellent += 10
		good += 10
	}
	if fertilized {
		excellent += 5
		good += 10
	}
	roll := rand.Intn(100)
	if roll < excellent {
		return "excellent"
	} else if roll < excellent+good {
		return "good"
	}
	return "normal"
}

// qualityMultiplier returns the coin reward multiplier for a quality tier.
func qualityMultiplier(quality string) float64 {
	switch quality {
	case "excellent":
		return 2.0
	case "good":
		return 1.5
	default:
		return 1.0
	}
}

// qualityLabel returns the Chinese display label for a quality tier.
func qualityLabel(quality string) string {
	switch quality {
	case "excellent":
		return "极品"
	case "good":
		return "优良"
	default:
		return "普通"
	}
}
