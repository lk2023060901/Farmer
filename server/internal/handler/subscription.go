package handler

// SubscriptionHandler handles WeChat message subscription (消息订阅) management.
//
// WeChat "订阅消息" allows users to authorise push notifications per template.
// The user's preferences are persisted in-process (sync.Map) so they survive
// within a single server process lifetime. In production this should be
// replaced with a dedicated DB table or Redis hash.
//
// Routes (all JWT-protected):
//
//	GET  /api/v1/subscriptions         — list all templates + subscription status
//	POST /api/v1/subscriptions         — subscribe / unsubscribe a template
//	POST /api/v1/subscriptions/send    — internal: create a notification record
import (
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/liukai/farmer/server/ent"
	"github.com/liukai/farmer/server/internal/ws"
)

// subscriptionTemplates are the four WeChat message templates supported by
// the 农趣村 mini-program.
var subscriptionTemplates = []map[string]interface{}{
	{
		"id":          "crop_mature",
		"name":        "作物成熟通知",
		"description": "作物成熟时提醒收获",
	},
	{
		"id":          "friend_visit",
		"name":        "好友到访通知",
		"description": "好友来农场时通知",
	},
	{
		"id":          "season_result",
		"name":        "赛季结算通知",
		"description": "赛季结算后发送排名",
	},
	{
		"id":          "event_reminder",
		"name":        "活动提醒",
		"description": "游戏活动开始前提醒",
	},
}

// userSubKey is the composite key for the subscription map.
type userSubKey struct {
	userID     uuid.UUID
	templateID string
}

// subscriptionStore holds per-user template subscription flags in memory.
// Keys are userSubKey; values are bool (subscribed).
var subscriptionStore sync.Map

// SubscriptionHandler groups WeChat message-subscription route handlers.
type SubscriptionHandler struct {
	db  *ent.Client
	hub *ws.Hub
}

// NewSubscriptionHandler constructs a SubscriptionHandler.
func NewSubscriptionHandler(db *ent.Client, hub *ws.Hub) *SubscriptionHandler {
	return &SubscriptionHandler{db: db, hub: hub}
}

// templateStatus returns the subscription status (bool) for a given user and template.
func templateStatus(userID uuid.UUID, templateID string) bool {
	v, ok := subscriptionStore.Load(userSubKey{userID: userID, templateID: templateID})
	if !ok {
		return false
	}
	return v.(bool)
}

// templateIDs returns the set of valid template IDs for quick lookup.
func validTemplateID(id string) bool {
	for _, t := range subscriptionTemplates {
		if t["id"] == id {
			return true
		}
	}
	return false
}

// GetSubscriptions handles GET /api/v1/subscriptions
//
// Returns the list of all four message templates together with the calling
// user's current subscription status for each one.
func (h *SubscriptionHandler) GetSubscriptions(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "unauthorized", "data": nil})
		return
	}

	type templateItem struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Subscribed  bool   `json:"subscribed"`
	}

	items := make([]templateItem, 0, len(subscriptionTemplates))
	for _, t := range subscriptionTemplates {
		items = append(items, templateItem{
			ID:          t["id"].(string),
			Name:        t["name"].(string),
			Description: t["description"].(string),
			Subscribed:  templateStatus(userID, t["id"].(string)),
		})
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": items})
}

// updateSubReq is the request body for UpdateSubscription.
type updateSubReq struct {
	TemplateID string `json:"templateId" binding:"required"`
	Subscribed bool   `json:"subscribed"`
}

// UpdateSubscription handles POST /api/v1/subscriptions
//
// Creates or updates the calling user's subscription preference for a single
// message template. In a full WeChat integration, subscribing would also call
// wx.requestSubscribeMessage to obtain the user's explicit consent on device;
// here we just persist the preference server-side.
func (h *SubscriptionHandler) UpdateSubscription(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "unauthorized", "data": nil})
		return
	}

	var req updateSubReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid request body", "data": nil})
		return
	}

	if !validTemplateID(req.TemplateID) {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "unknown templateId", "data": nil})
		return
	}

	subscriptionStore.Store(userSubKey{userID: userID, templateID: req.TemplateID}, req.Subscribed)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "ok",
		"data": gin.H{
			"templateId": req.TemplateID,
			"subscribed": req.Subscribed,
		},
	})
}

// sendSubMsgReq is the request body for SendSubscriptionMessage.
type sendSubMsgReq struct {
	UserID     string                 `json:"userId"     binding:"required"`
	TemplateID string                 `json:"templateId" binding:"required"`
	Data       map[string]interface{} `json:"data"`
}

// SendSubscriptionMessage handles POST /api/v1/subscriptions/send
//
// Internal endpoint used by game systems (e.g. tick/cron) to push a WeChat
// template message to a subscribed user.
//
// Production flow:
//  1. Verify the user is subscribed to the requested template.
//  2. Call the WeChat cloud.sendSubscribeMessage (or direct API) with the
//     user's openid and template parameters.
//
// Current implementation: creates an in-app Notification record so the
// behaviour is testable without WeChat credentials.
func (h *SubscriptionHandler) SendSubscriptionMessage(c *gin.Context) {
	var req sendSubMsgReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid request body", "data": nil})
		return
	}

	targetUserID, err := uuid.Parse(req.UserID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid userId", "data": nil})
		return
	}

	if !validTemplateID(req.TemplateID) {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "unknown templateId", "data": nil})
		return
	}

	// Respect the user's subscription preference; silently skip if not subscribed.
	if !templateStatus(targetUserID, req.TemplateID) {
		c.JSON(http.StatusOK, gin.H{
			"code":    0,
			"message": "user not subscribed, message skipped",
			"data":    gin.H{"sent": false},
		})
		return
	}

	// Resolve a human-readable title from the template registry.
	title := req.TemplateID
	for _, t := range subscriptionTemplates {
		if t["id"] == req.TemplateID {
			title = t["name"].(string)
			break
		}
	}

	// Build notification content from the data map if available.
	content := "您有一条新消息"
	if req.Data != nil {
		if v, ok := req.Data["content"]; ok {
			content = v.(string)
		}
	}

	ctx := c.Request.Context()

	// Persist as an in-app notification so it shows up in the notification centre.
	n, dbErr := h.db.Notification.Create().
		SetUserID(targetUserID).
		SetType("wx_subscription").
		SetTitle(title).
		SetContent(content).
		Save(ctx)

	if dbErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "failed to create notification", "data": nil})
		return
	}

	// Push via WebSocket if the user is connected.
	if h.hub != nil {
		h.hub.Send(targetUserID, &ws.Message{
			Type:   ws.EventNotification,
			UserID: targetUserID,
			Payload: ws.NotificationPayload{
				ID:      n.ID.String(),
				Type:    "wx_subscription",
				Title:   title,
				Content: content,
			},
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "ok",
		"data":    gin.H{"sent": true, "notificationId": n.ID.String()},
	})
}
