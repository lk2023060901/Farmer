package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"entgo.io/ent/dialect/sql"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/liukai/farmer/server/ent"
	entlog "github.com/liukai/farmer/server/ent/activitylog"
	entinv "github.com/liukai/farmer/server/ent/inventoryitem"
	entuser "github.com/liukai/farmer/server/ent/user"
	"github.com/liukai/farmer/server/internal/service"
)

// WorkshopHandler handles craft/processing endpoints.
type WorkshopHandler struct {
	db *ent.Client
}

// NewWorkshopHandler constructs a WorkshopHandler.
func NewWorkshopHandler(db *ent.Client) *WorkshopHandler { return &WorkshopHandler{db: db} }

// Recipe defines a processing formula.
type Recipe struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	InputID     string `json:"inputId"`   // inventory item ID
	InputQty    int    `json:"inputQty"`  // required quantity
	OutputID    string `json:"outputId"`  // produced item ID
	OutputQty   int    `json:"outputQty"` // output quantity
	ProcessSecs int    `json:"processSecs"` // processing time in seconds
	UnlockLevel int    `json:"unlockLevel"`
}

// Recipes is the full catalogue of workshop processing formulas.
var Recipes = []Recipe{
	{ID: "bread", Name: "面包", InputID: "wheat", InputQty: 3, OutputID: "bread", OutputQty: 1, ProcessSecs: 300, UnlockLevel: 5},
	{ID: "cheese", Name: "奶酪", InputID: "milk", InputQty: 4, OutputID: "cheese", OutputQty: 1, ProcessSecs: 600, UnlockLevel: 8},
	{ID: "jam", Name: "草莓酱", InputID: "strawberry", InputQty: 5, OutputID: "jam", OutputQty: 2, ProcessSecs: 480, UnlockLevel: 10},
	{ID: "flower_bouquet", Name: "花束", InputID: "corn", InputQty: 3, OutputID: "flower_bouquet", OutputQty: 1, ProcessSecs: 240, UnlockLevel: 6},
	{ID: "cake", Name: "蛋糕", InputID: "egg", InputQty: 6, OutputID: "cake", OutputQty: 1, ProcessSecs: 900, UnlockLevel: 12},
}

// recipeByID looks up a recipe by its ID.
func recipeByID(id string) *Recipe {
	for i := range Recipes {
		if Recipes[i].ID == id {
			return &Recipes[i]
		}
	}
	return nil
}

// processingMeta is the JSON structure stored in ActivityLog.meta for processing jobs.
type processingMeta struct {
	RecipeID  string `json:"recipeId"`
	FinishAt  string `json:"finishAt"`  // RFC3339
	Collected bool   `json:"collected"`
}

// ListRecipes handles GET /api/v1/workshop/recipes
// Returns all recipes enriched with the user's current ingredient quantities.
func (h *WorkshopHandler) ListRecipes(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "unauthorized", "data": nil})
		return
	}

	ctx := c.Request.Context()

	// Fetch user to check level.
	u, err := h.db.User.Query().Where(entuser.ID(userID)).Only(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "user lookup failed", "data": nil})
		return
	}

	// Collect all ingredient IDs required by recipes.
	inputIDs := make([]string, 0, len(Recipes))
	for _, r := range Recipes {
		inputIDs = append(inputIDs, r.InputID)
	}

	// Query inventory for all relevant items in one shot.
	items, err := h.db.InventoryItem.Query().
		Where(entinv.UserID(userID), entinv.ItemIDIn(inputIDs...)).
		All(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "inventory query failed", "data": nil})
		return
	}
	qty := make(map[string]int64, len(items))
	for _, it := range items {
		qty[it.ItemID] = it.Quantity
	}

	type recipeDTO struct {
		Recipe
		HaveQty    int64 `json:"haveQty"`
		CanCraft   bool  `json:"canCraft"`
		IsUnlocked bool  `json:"isUnlocked"`
	}
	result := make([]recipeDTO, 0, len(Recipes))
	for _, r := range Recipes {
		have := qty[r.InputID]
		unlocked := u.Level >= r.UnlockLevel
		result = append(result, recipeDTO{
			Recipe:     r,
			HaveQty:    have,
			CanCraft:   unlocked && have >= int64(r.InputQty),
			IsUnlocked: unlocked,
		})
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": result})
}

type startProcessingReq struct {
	RecipeID string `json:"recipeId" binding:"required"`
}

// StartProcessing handles POST /api/v1/workshop/start
// Deducts ingredients and records a processing job via ActivityLog.
func (h *WorkshopHandler) StartProcessing(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "unauthorized", "data": nil})
		return
	}

	var req startProcessingReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid request", "data": nil})
		return
	}

	r := recipeByID(req.RecipeID)
	if r == nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "recipe not found", "data": nil})
		return
	}

	ctx := c.Request.Context()

	// Check user level.
	u, err := h.db.User.Query().Where(entuser.ID(userID)).Only(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "user lookup failed", "data": nil})
		return
	}
	if u.Level < r.UnlockLevel {
		c.JSON(http.StatusForbidden, gin.H{
			"code": 403, "message": fmt.Sprintf("recipe requires level %d", r.UnlockLevel), "data": nil,
		})
		return
	}

	// Check sufficient ingredients.
	item, err := h.db.InventoryItem.Query().
		Where(entinv.UserID(userID), entinv.ItemID(r.InputID)).
		Only(ctx)
	if err != nil || item.Quantity < int64(r.InputQty) {
		have := int64(0)
		if err == nil {
			have = item.Quantity
		}
		c.JSON(http.StatusConflict, gin.H{
			"code": 409, "message": "insufficient ingredients",
			"data": gin.H{"have": have, "need": r.InputQty},
		})
		return
	}

	// Deduct ingredients.
	if err := h.db.InventoryItem.UpdateOne(item).
		AddQuantity(-int64(r.InputQty)).
		Exec(ctx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "deduct ingredients failed", "data": nil})
		return
	}

	// Record job in ActivityLog.
	finishAt := time.Now().Add(time.Duration(r.ProcessSecs) * time.Second)
	meta := map[string]interface{}{
		"recipeId":  r.ID,
		"finishAt":  finishAt.UTC().Format(time.RFC3339),
		"collected": false,
	}
	job, err := h.db.ActivityLog.Create().
		SetUserID(userID).
		SetType("processing_job").
		SetContent(fmt.Sprintf("开始制作: %s", r.Name)).
		SetMeta(meta).
		Save(ctx)
	if err != nil {
		// Attempt to refund ingredients on failure.
		_ = service.AddToInventory(ctx, h.db, userID, r.InputID, "crop", int64(r.InputQty))
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "create job failed", "data": nil})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": gin.H{
		"jobId":    job.ID,
		"recipeId": r.ID,
		"name":     r.Name,
		"finishAt": finishAt.UTC().Format(time.RFC3339),
	}})
}

type collectProductReq struct {
	JobID string `json:"jobId" binding:"required"`
}

// CollectProduct handles POST /api/v1/workshop/collect
// Checks processing time elapsed and grants output to inventory.
func (h *WorkshopHandler) CollectProduct(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "unauthorized", "data": nil})
		return
	}

	var req collectProductReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid request", "data": nil})
		return
	}

	jobID, err := uuid.Parse(req.JobID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid jobId", "data": nil})
		return
	}

	ctx := c.Request.Context()

	job, err := h.db.ActivityLog.Get(ctx, jobID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "job not found", "data": nil})
		return
	}
	if job.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"code": 403, "message": "not your job", "data": nil})
		return
	}
	if job.Type != "processing_job" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "not a processing job", "data": nil})
		return
	}

	// Decode meta.
	metaBytes, err := json.Marshal(job.Meta)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "meta decode failed", "data": nil})
		return
	}
	var meta processingMeta
	if err := json.Unmarshal(metaBytes, &meta); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "meta parse failed", "data": nil})
		return
	}

	if meta.Collected {
		c.JSON(http.StatusConflict, gin.H{"code": 409, "message": "already collected", "data": nil})
		return
	}

	finishAt, err := time.Parse(time.RFC3339, meta.FinishAt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "finishAt parse failed", "data": nil})
		return
	}
	if time.Now().Before(finishAt) {
		remaining := int(time.Until(finishAt).Seconds())
		c.JSON(http.StatusConflict, gin.H{
			"code": 409, "message": "not ready yet",
			"data": gin.H{"remainingSecs": remaining, "finishAt": meta.FinishAt},
		})
		return
	}

	r := recipeByID(meta.RecipeID)
	if r == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "recipe not found", "data": nil})
		return
	}

	// Mark collected.
	newMeta := map[string]interface{}{
		"recipeId":  meta.RecipeID,
		"finishAt":  meta.FinishAt,
		"collected": true,
	}
	if err := h.db.ActivityLog.UpdateOneID(jobID).
		SetMeta(newMeta).
		Exec(ctx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "mark collected failed", "data": nil})
		return
	}

	// Add output to inventory.
	if err := service.AddToInventory(ctx, h.db, userID, r.OutputID, "product", int64(r.OutputQty)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "grant product failed", "data": nil})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": gin.H{
		"jobId":    jobID,
		"outputId": r.OutputID,
		"outputQty": r.OutputQty,
		"name":     r.Name,
	}})
}

// ListJobs handles GET /api/v1/workshop/jobs
// Returns all active (uncollected) processing jobs for the current user.
func (h *WorkshopHandler) ListJobs(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "unauthorized", "data": nil})
		return
	}

	ctx := c.Request.Context()

	jobs, err := h.db.ActivityLog.Query().
		Where(
			entlog.UserID(userID),
			entlog.Type("processing_job"),
		).
		Order(entlog.ByCreatedAt(sql.OrderDesc())).
		Limit(20).
		All(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "query failed", "data": nil})
		return
	}

	type jobDTO struct {
		JobID     uuid.UUID `json:"jobId"`
		RecipeID  string    `json:"recipeId"`
		Name      string    `json:"name"`
		FinishAt  string    `json:"finishAt"`
		Collected bool      `json:"collected"`
		IsReady   bool      `json:"isReady"`
	}

	result := make([]jobDTO, 0, len(jobs))
	for _, j := range jobs {
		metaBytes, _ := json.Marshal(j.Meta)
		var meta processingMeta
		if err := json.Unmarshal(metaBytes, &meta); err != nil {
			continue
		}
		finishAt, _ := time.Parse(time.RFC3339, meta.FinishAt)
		r := recipeByID(meta.RecipeID)
		name := meta.RecipeID
		if r != nil {
			name = r.Name
		}
		result = append(result, jobDTO{
			JobID:     j.ID,
			RecipeID:  meta.RecipeID,
			Name:      name,
			FinishAt:  meta.FinishAt,
			Collected: meta.Collected,
			IsReady:   !time.Now().Before(finishAt),
		})
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": result})
}
