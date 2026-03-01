package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/liukai/farmer/server/ent"
	entuser "github.com/liukai/farmer/server/ent/user"
)

// staminaRecoveryRate is how often (in seconds) the player recovers 1 stamina point.
// 6 minutes per point = full 100 recovery in 10 hours.
const staminaRecoveryRate = 6 * 60

// computeStamina returns the current stamina after lazy recovery and whether the DB needs updating.
func computeStamina(stored, max int, updatedAt time.Time) (current int, changed bool) {
	elapsed := time.Since(updatedAt).Seconds()
	recovered := int(elapsed / staminaRecoveryRate)
	if recovered <= 0 || stored >= max {
		return stored, false
	}
	current = stored + recovered
	if current > max {
		current = max
	}
	return current, true
}

// UserHandler groups user-profile route handlers.
type UserHandler struct {
	db *ent.Client
}

// NewUserHandler constructs a UserHandler.
func NewUserHandler(db *ent.Client) *UserHandler { return &UserHandler{db: db} }

// GetMe handles GET /api/v1/users/me
func (h *UserHandler) GetMe(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "unauthorized", "data": nil})
		return
	}

	u, err := h.db.User.Query().
		Where(entuser.ID(userID)).
		Only(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "user not found", "data": nil})
		return
	}

	// Lazy stamina recovery: compute how much has regenerated since last update
	currentStamina, staminaChanged := computeStamina(u.Stamina, u.MaxStamina, u.StaminaUpdatedAt)
	if staminaChanged {
		u, _ = h.db.User.UpdateOne(u).
			SetStamina(currentStamina).
			SetStaminaUpdatedAt(time.Now()).
			Save(c.Request.Context())
		if u == nil {
			currentStamina = currentStamina // keep computed value even if save failed
		}
	}

	stamina := currentStamina

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "ok",
		"data": gin.H{
			"id":         u.ID,
			"nickname":   u.Nickname,
			"avatar":     u.Avatar,
			"level":      u.Level,
			"exp":        u.Exp,
			"coins":      u.Coins,
			"diamonds":   u.Diamonds,
			"stamina":    stamina,
			"maxStamina": u.MaxStamina,
		},
	})
}

// UpdateMe handles PUT /api/v1/users/me
func (h *UserHandler) UpdateMe(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": nil})
}

// GetByID handles GET /api/v1/users/:id
func (h *UserHandler) GetByID(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid user id", "data": nil})
		return
	}
	u, err := h.db.User.Get(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "user not found", "data": nil})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code": 0, "message": "ok",
		"data": gin.H{"id": u.ID, "nickname": u.Nickname, "avatar": u.Avatar, "level": u.Level},
	})
}
