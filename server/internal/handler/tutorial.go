package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/liukai/farmer/server/ent"
	enttutor "github.com/liukai/farmer/server/ent/tutorialprogress"
)

// TutorialHandler groups tutorial-progress route handlers.
type TutorialHandler struct {
	db *ent.Client
}

// NewTutorialHandler constructs a TutorialHandler.
func NewTutorialHandler(db *ent.Client) *TutorialHandler { return &TutorialHandler{db: db} }

// GetProgress handles GET /api/v1/tutorial/progress
// Returns the list of completed step keys for the current user.
func (h *TutorialHandler) GetProgress(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "unauthorized", "data": nil})
		return
	}
	ctx := c.Request.Context()

	records, err := h.db.TutorialProgress.Query().
		Where(enttutor.UserID(userID)).
		All(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "query failed", "data": nil})
		return
	}

	steps := make([]string, 0, len(records))
	for _, r := range records {
		steps = append(steps, r.Step)
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "ok",
		"data":    gin.H{"completedSteps": steps},
	})
}

// CompleteStep handles POST /api/v1/tutorial/complete-step
// Marks a tutorial step as done for the current user (idempotent).
// Body: { "step": "plant" }
func (h *TutorialHandler) CompleteStep(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "unauthorized", "data": nil})
		return
	}
	ctx := c.Request.Context()

	var body struct {
		Step string `json:"step" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "step is required", "data": nil})
		return
	}

	_, err := h.db.TutorialProgress.Create().
		SetUserID(userID).
		SetStep(body.Step).
		Save(ctx)
	if err != nil && !ent.IsConstraintError(err) {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "save failed", "data": nil})
		return
	}
	// If err is a constraint violation, the step was already recorded — treat as success.

	// Re-query all completed steps so the response is always up-to-date.
	records, err := h.db.TutorialProgress.Query().
		Where(enttutor.UserID(userID)).
		All(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "query failed", "data": nil})
		return
	}

	steps := make([]string, 0, len(records))
	for _, r := range records {
		steps = append(steps, r.Step)
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "ok",
		"data": gin.H{
			"step":           body.Step,
			"completedSteps": steps,
		},
	})
}
