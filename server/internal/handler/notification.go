package handler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/liukai/farmer/server/ent"
	entnotif "github.com/liukai/farmer/server/ent/notification"
	"github.com/liukai/farmer/server/internal/ws"
)

// NotificationHandler groups notification route handlers.
type NotificationHandler struct {
	db  *ent.Client
	hub *ws.Hub
}

// NewNotificationHandler constructs a NotificationHandler.
func NewNotificationHandler(db *ent.Client, hub *ws.Hub) *NotificationHandler {
	return &NotificationHandler{db: db, hub: hub}
}

// List handles GET /api/v1/notifications
func (h *NotificationHandler) List(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "unauthorized", "data": nil})
		return
	}

	ctx := c.Request.Context()

	notifs, err := h.db.Notification.Query().
		Where(entnotif.UserID(userID)).
		Order(entnotif.ByCreatedAt()).
		Limit(50).
		All(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "query notifications failed", "data": nil})
		return
	}

	// Count unread notifications.
	unreadCount, err := h.db.Notification.Query().
		Where(entnotif.UserID(userID), entnotif.IsRead(false)).
		Count(ctx)
	if err != nil {
		unreadCount = 0
	}

	type notifItem struct {
		ID         string                 `json:"id"`
		Type       string                 `json:"type"`
		Title      string                 `json:"title"`
		Content    string                 `json:"content"`
		IsRead     bool                   `json:"isRead"`
		CreatedAt  string                 `json:"createdAt"`
		ActionType *string                `json:"actionType"`
		ActionData map[string]interface{} `json:"actionData"`
	}

	items := make([]notifItem, 0, len(notifs))
	for _, n := range notifs {
		items = append(items, notifItem{
			ID:         n.ID.String(),
			Type:       n.Type,
			Title:      n.Title,
			Content:    n.Content,
			IsRead:     n.IsRead,
			CreatedAt:  n.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			ActionType: n.ActionType,
			ActionData: n.ActionData,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "ok",
		"data": gin.H{
			"notifications": items,
			"unreadCount":   unreadCount,
		},
	})
}

// MarkRead handles PUT /api/v1/notifications/:id/read
func (h *NotificationHandler) MarkRead(c *gin.Context) {
	notifID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid notification id", "data": nil})
		return
	}

	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "unauthorized", "data": nil})
		return
	}

	ctx := c.Request.Context()

	n, err := h.db.Notification.Get(ctx, notifID)
	if err != nil {
		if ent.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "notification not found", "data": nil})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "get notification failed", "data": nil})
		return
	}

	if n.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"code": 403, "message": "forbidden", "data": nil})
		return
	}

	if err := h.db.Notification.UpdateOneID(notifID).SetIsRead(true).Exec(ctx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "update notification failed", "data": nil})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "ok",
		"data":    gin.H{"id": notifID.String(), "isRead": true},
	})
}

// MarkAllRead handles PUT /api/v1/notifications/read-all
func (h *NotificationHandler) MarkAllRead(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "unauthorized", "data": nil})
		return
	}

	ctx := c.Request.Context()

	n, err := h.db.Notification.Update().
		Where(entnotif.UserID(userID), entnotif.IsRead(false)).
		SetIsRead(true).
		Save(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "update notifications failed", "data": nil})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "ok",
		"data":    gin.H{"updated": n},
	})
}

// PushNotification creates a DB notification and pushes it via WebSocket.
// hub may be nil (notifications are still persisted to DB).
func PushNotification(ctx context.Context, db *ent.Client, hub *ws.Hub, userID uuid.UUID, notifType, title, content string) {
	n, err := db.Notification.Create().
		SetUserID(userID).
		SetType(notifType).
		SetTitle(title).
		SetContent(content).
		Save(ctx)
	if err != nil || hub == nil {
		return
	}
	hub.Send(userID, &ws.Message{
		Type:   ws.EventNotification,
		UserID: userID,
		Payload: ws.NotificationPayload{
			ID:      n.ID.String(),
			Type:    n.Type,
			Title:   n.Title,
			Content: n.Content,
		},
	})
}
