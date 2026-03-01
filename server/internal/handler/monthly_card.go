package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/liukai/farmer/server/ent"
	entlog "github.com/liukai/farmer/server/ent/activitylog"
	entuser "github.com/liukai/farmer/server/ent/user"
)

// MonthlyCardHandler handles monthly subscription card endpoints.
type MonthlyCardHandler struct {
	db *ent.Client
}

// NewMonthlyCardHandler constructs a MonthlyCardHandler.
func NewMonthlyCardHandler(db *ent.Client) *MonthlyCardHandler {
	return &MonthlyCardHandler{db: db}
}

// monthlyCardDurationDays is the validity period of a monthly card.
const monthlyCardDurationDays = 28

// monthlyCardAmountCents is the price of a monthly card in fen (30 CNY).
const monthlyCardAmountCents = 3000

// monthlyCardDailyDiamonds is the daily diamond grant for monthly card holders.
const monthlyCardDailyDiamonds = 5

// monthlyCardPerk describes a single benefit of the monthly card.
type monthlyCardPerk struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
}

// monthlyCardPerks is the static list of monthly card benefits.
var monthlyCardPerks = []monthlyCardPerk{
	{ID: "daily_diamonds", Name: "每日5钻石", Description: "月卡有效期内每日可领取5钻石", Icon: "💎"},
	{ID: "exclusive_frame", Name: "专属装饰框", Description: "专属头像装饰框彰显尊贵身份", Icon: "🖼️"},
	{ID: "double_ai_chat", Name: "AI对话次数翻倍", Description: "每日AI对话次数加倍享用", Icon: "🤖"},
	{ID: "workshop_speed", Name: "加工加速30%", Description: "加工坊处理时间缩短30%", Icon: "⚡"},
	{ID: "storage_expand", Name: "仓库扩容+20格", Description: "仓库额外增加20个格子", Icon: "📦"},
}

// isMonthlyCardActive reports whether the user's monthly card subscription is
// currently valid.
func isMonthlyCardActive(u *ent.User) bool {
	if u.SubscriptionExpiresAt == nil {
		return false
	}
	return time.Now().Before(*u.SubscriptionExpiresAt)
}

// GetStatus handles GET /api/v1/monthly-card/status
// Returns the user's monthly card status: active / expired / never.
func (h *MonthlyCardHandler) GetStatus(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "unauthorized", "data": nil})
		return
	}

	ctx := c.Request.Context()
	u, err := h.db.User.Query().Where(entuser.ID(userID)).Only(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "user lookup failed", "data": nil})
		return
	}

	var status string
	var expiresAt *string
	if u.SubscriptionExpiresAt == nil {
		status = "never"
	} else if time.Now().Before(*u.SubscriptionExpiresAt) {
		status = "active"
		s := u.SubscriptionExpiresAt.UTC().Format(time.RFC3339)
		expiresAt = &s
	} else {
		status = "expired"
		s := u.SubscriptionExpiresAt.UTC().Format(time.RFC3339)
		expiresAt = &s
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": gin.H{
		"status":    status,
		"expiresAt": expiresAt,
		"isActive":  status == "active",
	}})
}

// PurchaseCard handles POST /api/v1/monthly-card/purchase
// Creates a PaymentOrder for the monthly card (28 days, 30 yuan) and returns a
// mock prepay_id.  Activates the subscription immediately upon order creation
// (production would defer to the WeChat Pay callback).
func (h *MonthlyCardHandler) PurchaseCard(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "unauthorized", "data": nil})
		return
	}

	ctx := c.Request.Context()

	// Create payment order.
	order, err := h.db.PaymentOrder.Create().
		SetUserID(userID).
		SetPackageID("monthly_card").
		SetAmountCents(monthlyCardAmountCents).
		SetDiamondsToGrant(0).
		SetStatus("paid"). // Mock: mark paid immediately in development.
		Save(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "create order failed", "data": nil})
		return
	}

	// Extend or set the subscription expiry.
	u, err := h.db.User.Query().Where(entuser.ID(userID)).Only(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "user lookup failed", "data": nil})
		return
	}

	var newExpiry time.Time
	if isMonthlyCardActive(u) {
		// Extend from current expiry.
		newExpiry = u.SubscriptionExpiresAt.Add(time.Duration(monthlyCardDurationDays) * 24 * time.Hour)
	} else {
		newExpiry = time.Now().Add(time.Duration(monthlyCardDurationDays) * 24 * time.Hour)
	}

	if err := h.db.User.UpdateOneID(userID).
		SetSubscriptionExpiresAt(newExpiry).
		Exec(ctx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "activate card failed", "data": nil})
		return
	}

	mockPrepayID := "mock_mc_prepay_" + order.ID.String()

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": gin.H{
		"orderId":     order.ID,
		"amountCents": monthlyCardAmountCents,
		"expiresAt":   newExpiry.UTC().Format(time.RFC3339),
		"prepayId":    mockPrepayID,
	}})
}

// DailyReward handles POST /api/v1/monthly-card/daily-reward
// Grants 5 diamonds if the monthly card is active and the reward has not been
// claimed today.
func (h *MonthlyCardHandler) DailyReward(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "unauthorized", "data": nil})
		return
	}

	ctx := c.Request.Context()

	// Verify active subscription.
	u, err := h.db.User.Query().Where(entuser.ID(userID)).Only(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "user lookup failed", "data": nil})
		return
	}
	if !isMonthlyCardActive(u) {
		c.JSON(http.StatusForbidden, gin.H{"code": 403, "message": "monthly card not active", "data": nil})
		return
	}

	// Check if already claimed today.
	today := todayUTC()
	tomorrow := today.Add(24 * time.Hour)
	existing, err := h.db.ActivityLog.Query().
		Where(
			entlog.UserID(userID),
			entlog.Type("monthly_card_reward"),
			entlog.CreatedAtGTE(today),
			entlog.CreatedAtLT(tomorrow),
		).
		First(ctx)
	if err == nil && existing != nil {
		c.JSON(http.StatusConflict, gin.H{"code": 409, "message": "already claimed today", "data": gin.H{
			"claimedAt": existing.CreatedAt.UTC().Format(time.RFC3339),
		}})
		return
	}

	// Record reward claim.
	if _, err := h.db.ActivityLog.Create().
		SetUserID(userID).
		SetType("monthly_card_reward").
		SetContent("月卡每日钻石奖励").
		SetMeta(map[string]interface{}{
			"diamonds": monthlyCardDailyDiamonds,
		}).
		Save(ctx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "record reward failed", "data": nil})
		return
	}

	// Grant diamonds.
	u, err = h.db.User.UpdateOneID(userID).
		AddDiamonds(monthlyCardDailyDiamonds).
		Save(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "grant diamonds failed", "data": nil})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": gin.H{
		"diamonds":    monthlyCardDailyDiamonds,
		"newDiamonds": u.Diamonds,
	}})
}

// ListPerks handles GET /api/v1/monthly-card/perks
// Returns the list of monthly card benefits.
func (h *MonthlyCardHandler) ListPerks(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": monthlyCardPerks})
}
