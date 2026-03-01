package handler

import (
	"bytes"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/liukai/farmer/server/ent"
	entchat "github.com/liukai/farmer/server/ent/chatlog"
)

// ChatHandler groups chat route handlers.
type ChatHandler struct {
	db *ent.Client
}

// NewChatHandler constructs a ChatHandler.
func NewChatHandler(db *ent.Client) *ChatHandler { return &ChatHandler{db: db} }

// normalizeChatIDs returns (a, b) with a <= b (lexicographic by UUID bytes).
// This ensures the same conversation always uses the same (user_a_id, user_b_id) pair.
func normalizeChatIDs(x, y uuid.UUID) (uuid.UUID, uuid.UUID) {
	if bytes.Compare(x[:], y[:]) <= 0 {
		return x, y
	}
	return y, x
}

type sendMsgReq struct {
	ToUserID string `json:"toUserId" binding:"required"`
	Content  string `json:"content"  binding:"required,max=500"`
}

// GetHistory handles GET /api/v1/chat/history?friendId=...
// Returns up to 50 messages for the conversation between the current user and friendId,
// ordered chronologically (oldest first).
func (h *ChatHandler) GetHistory(c *gin.Context) {
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

	aID, bID := normalizeChatIDs(myID, friendID)
	ctx := c.Request.Context()

	logs, err := h.db.ChatLog.Query().
		Where(entchat.UserAID(aID), entchat.UserBID(bID)).
		Order(entchat.ByCreatedAt()).
		Limit(50).
		All(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "query failed", "data": nil})
		return
	}

	result := make([]map[string]any, len(logs))
	for i, l := range logs {
		result[i] = map[string]any{
			"id":             l.ID,
			"speakerUserId":  l.SpeakerUserID,
			"scene":          l.Scene,
			"content":        l.Content,
			"isLlmGenerated": l.IsLlmGenerated,
			"createdAt":      l.CreatedAt.Format(time.RFC3339),
		}
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": gin.H{
		"messages": result,
		"total":    len(result),
	}})
}

// Send handles POST /api/v1/chat/send
// Creates a new chat message from the current user to toUserId.
func (h *ChatHandler) Send(c *gin.Context) {
	myID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "unauthorized", "data": nil})
		return
	}

	var req sendMsgReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid request", "data": nil})
		return
	}

	toID, err := uuid.Parse(req.ToUserID)
	if err != nil || myID == toID {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid toUserId", "data": nil})
		return
	}

	aID, bID := normalizeChatIDs(myID, toID)
	ctx := c.Request.Context()

	msg, err := h.db.ChatLog.Create().
		SetUserAID(aID).
		SetUserBID(bID).
		SetSpeakerUserID(myID).
		SetScene("chat").
		SetContent(req.Content).
		Save(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "send failed", "data": nil})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": map[string]any{
		"id":             msg.ID,
		"speakerUserId":  msg.SpeakerUserID,
		"scene":          msg.Scene,
		"content":        msg.Content,
		"isLlmGenerated": msg.IsLlmGenerated,
		"createdAt":      msg.CreatedAt.Format(time.RFC3339),
	}})
}
