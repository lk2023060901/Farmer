package handler

import (
	"bytes"
	"fmt"
	"net/http"
	"time"

	"entgo.io/ent/dialect/sql"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/liukai/farmer/server/ent"
	entchat "github.com/liukai/farmer/server/ent/chatlog"
	entfarm "github.com/liukai/farmer/server/ent/farm"
	entuser "github.com/liukai/farmer/server/ent/user"
)

// DelegateHandler handles farm delegation endpoints.
type DelegateHandler struct {
	db *ent.Client
}

// NewDelegateHandler constructs a DelegateHandler.
func NewDelegateHandler(db *ent.Client) *DelegateHandler { return &DelegateHandler{db: db} }

// delegateFarmReq is the request body for POST /api/v1/farms/delegate.
type delegateFarmReq struct {
	FriendID string `json:"friendId" binding:"required"`
}

// DelegateFarm handles POST /api/v1/farms/delegate
// Performs delegation actions on a friend's farm: harvests mature crops and
// waters wilting crops, then records a ChatLog notification for the farm owner.
func (h *DelegateHandler) DelegateFarm(c *gin.Context) {
	myID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "unauthorized", "data": nil})
		return
	}

	var req delegateFarmReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error(), "data": nil})
		return
	}

	friendID, err := uuid.Parse(req.FriendID)
	if err != nil || friendID == myID {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid friendId", "data": nil})
		return
	}

	ctx := c.Request.Context()

	// Verify the friend exists and check offline status (≥ 24 hours via UpdatedAt).
	friend, err := h.db.User.Query().
		Where(entuser.ID(friendID)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "friend not found", "data": nil})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "user query failed", "data": nil})
		return
	}

	// Use UpdatedAt as a proxy for last activity. If the friend has been active
	// within the past 24 hours, deny the delegation request.
	if time.Since(friend.UpdatedAt) < 24*time.Hour {
		c.JSON(http.StatusConflict, gin.H{
			"code":    409,
			"message": "friend is not offline long enough (< 24 hours)",
			"data":    nil,
		})
		return
	}

	// Fetch the friend's farm.
	farm, err := h.db.Farm.Query().
		Where(entfarm.OwnerID(friendID)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "friend has no farm", "data": nil})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "farm query failed", "data": nil})
		return
	}

	// Iterate over all plots and perform delegation actions.
	// "mature"  → harvest: set type back to "tilled", clear crop fields.
	// "wilting" → water:   update wateredAt, reset type to "planted".
	plots := farm.Plots
	harvested := 0
	watered := 0
	now := time.Now().Format(time.RFC3339)

	for i := range plots {
		switch plots[i].Type {
		case "mature":
			plots[i].Type = "tilled"
			plots[i].CropID = ""
			plots[i].PlantedAt = ""
			plots[i].Stage = ""
			plots[i].Quality = ""
			plots[i].WateredAt = ""
			plots[i].Fertilized = false
			harvested++
		case "wilting":
			plots[i].Type = "planted"
			plots[i].WateredAt = now
			watered++
		}
	}

	// Persist the updated plots.
	if harvested > 0 || watered > 0 {
		if _, err := h.db.Farm.UpdateOne(farm).SetPlots(plots).Save(ctx); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "farm update failed", "data": nil})
			return
		}
	}

	// Record a ChatLog delegation notification so the farm owner can see it.
	aID, bID := normalizeDelegateIDs(myID, friendID)
	content := fmt.Sprintf("代管了您的农场：收获 %d 块，浇水 %d 块", harvested, watered)
	_, _ = h.db.ChatLog.Create().
		SetUserAID(aID).
		SetUserBID(bID).
		SetSpeakerUserID(myID).
		SetScene("delegation").
		SetContent(content).
		Save(ctx)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "delegation completed",
		"data": gin.H{
			"harvested": harvested,
			"watered":   watered,
			"friendId":  friendID,
		},
	})
}

// GetDelegateReport handles GET /api/v1/farms/delegate/report?friendId=uuid
// Returns the last 10 delegation log entries involving the caller and the given friend.
func (h *DelegateHandler) GetDelegateReport(c *gin.Context) {
	myID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "unauthorized", "data": nil})
		return
	}

	friendID, err := uuid.Parse(c.Query("friendId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid friendId", "data": nil})
		return
	}

	ctx := c.Request.Context()
	aID, bID := normalizeDelegateIDs(myID, friendID)

	logs, err := h.db.ChatLog.Query().
		Where(
			entchat.UserAID(aID),
			entchat.UserBID(bID),
			entchat.Scene("delegation"),
		).
		Order(entchat.ByCreatedAt(sql.OrderDesc())).
		Limit(10).
		All(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "query failed", "data": nil})
		return
	}

	result := make([]map[string]any, len(logs))
	for i, l := range logs {
		result[i] = map[string]any{
			"id":            l.ID,
			"speakerUserId": l.SpeakerUserID,
			"content":       l.Content,
			"createdAt":     l.CreatedAt.Format(time.RFC3339),
		}
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": result})
}

// normalizeDelegateIDs returns (a, b) with a ≤ b (lexicographic by UUID bytes),
// matching the convention used in ChatLog for deterministic pair storage.
func normalizeDelegateIDs(x, y uuid.UUID) (uuid.UUID, uuid.UUID) {
	if bytes.Compare(x[:], y[:]) <= 0 {
		return x, y
	}
	return y, x
}
