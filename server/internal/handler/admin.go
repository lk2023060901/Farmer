package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/liukai/farmer/server/ent"
	entcheckin "github.com/liukai/farmer/server/ent/dailycheckin"
	entpo "github.com/liukai/farmer/server/ent/paymentorder"
	entuser "github.com/liukai/farmer/server/ent/user"
	entvillage "github.com/liukai/farmer/server/ent/village"
)

// AdminHandler groups internal / operational metric endpoints.
type AdminHandler struct {
	db *ent.Client
}

// NewAdminHandler constructs an AdminHandler.
func NewAdminHandler(db *ent.Client) *AdminHandler { return &AdminHandler{db: db} }

// Metrics handles GET /api/v1/admin/metrics
// Returns core operational KPIs for the ops dashboard.
// NOTE: In production, protect this endpoint with an admin secret header
// (e.g. X-Admin-Key) or move it behind a VPN. For now it is JWT-authed only.
func (h *AdminHandler) Metrics(c *gin.Context) {
	ctx := c.Request.Context()
	now := time.Now().UTC()
	todayStart := now.Truncate(24 * time.Hour)
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	weekAgo := now.AddDate(0, 0, -7)
	day7Start := weekAgo.Truncate(24 * time.Hour)

	// ── Users ────────────────────────────────────────────────────────────────
	totalUsers, _ := h.db.User.Query().Count(ctx)

	// New users today (registered today)
	newUsersToday, _ := h.db.User.Query().
		Where(entuser.CreatedAtGTE(todayStart)).
		Count(ctx)

	// MAU approximation: users with a checkin in the last 30 days
	mauUsers, _ := h.db.DailyCheckin.Query().
		Where(entcheckin.CreatedAtGTE(monthStart)).
		Unique(true).
		Count(ctx)

	// DAU: users with a checkin today
	dauUsers, _ := h.db.DailyCheckin.Query().
		Where(entcheckin.CreatedAtGTE(todayStart)).
		Unique(true).
		Count(ctx)

	// 7-day new user retention: users registered in the last 7 days
	newUsersWeek, _ := h.db.User.Query().
		Where(entuser.CreatedAtGTE(day7Start)).
		Count(ctx)

	// ── Revenue ──────────────────────────────────────────────────────────────
	// Total paid orders (all time)
	totalPaidOrders, _ := h.db.PaymentOrder.Query().
		Where(entpo.Status("paid")).
		Count(ctx)

	// Total revenue today (cents)
	todayOrders, _ := h.db.PaymentOrder.Query().
		Where(entpo.Status("paid"), entpo.CreatedAtGTE(todayStart)).
		All(ctx)
	var revenueToday int64
	for _, o := range todayOrders {
		revenueToday += int64(o.AmountCents)
	}

	// Total revenue this month (cents)
	monthOrders, _ := h.db.PaymentOrder.Query().
		Where(entpo.Status("paid"), entpo.CreatedAtGTE(monthStart)).
		All(ctx)
	var revenueMonth int64
	for _, o := range monthOrders {
		revenueMonth += int64(o.AmountCents)
	}

	// Paying users (users with at least one paid order)
	payingUsers, _ := h.db.PaymentOrder.Query().
		Where(entpo.Status("paid")).
		Unique(true).
		Count(ctx)

	var payRate float64
	if totalUsers > 0 {
		payRate = float64(payingUsers) / float64(totalUsers) * 100
	}

	var arpu float64
	if payingUsers > 0 {
		// Sum all time revenue / paying users
		allPaid, _ := h.db.PaymentOrder.Query().
			Where(entpo.Status("paid")).
			All(ctx)
		var total int64
		for _, o := range allPaid {
			total += int64(o.AmountCents)
		}
		arpu = float64(total) / float64(payingUsers) / 100.0 // yuan
	}

	// ── Funnel ───────────────────────────────────────────────────────────────
	// Funnel steps: registered → has_checkin → has_trade → has_social → 7day
	usersWithCheckin, _ := h.db.DailyCheckin.Query().Unique(true).Count(ctx)

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": gin.H{
		"users": gin.H{
			"total":        totalUsers,
			"newToday":     newUsersToday,
			"newThisWeek":  newUsersWeek,
			"dau":          dauUsers,
			"mau":          mauUsers,
		},
		"revenue": gin.H{
			"todayCents":   revenueToday,
			"monthCents":   revenueMonth,
			"paidOrders":   totalPaidOrders,
			"payingUsers":  payingUsers,
			"payRatePct":   payRate,
			"arpuYuan":     arpu,
		},
		"funnel": gin.H{
			"registered":       totalUsers,
			"checkinAtLeastOnce": usersWithCheckin,
		},
		"generatedAt": now.Format(time.RFC3339),
	}})
}

// ListUsers handles GET /api/v1/admin/users
// Returns paginated user list for admin management.
func (h *AdminHandler) ListUsers(c *gin.Context) {
	ctx := c.Request.Context()
	users, err := h.db.User.Query().
		Order(entuser.ByCreatedAt()).
		Limit(100).
		All(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "query failed", "data": nil})
		return
	}
	type userRow struct {
		ID        interface{} `json:"id"`
		Nickname  string      `json:"nickname"`
		Level     int         `json:"level"`
		Coins     int64       `json:"coins"`
		Diamonds  int         `json:"diamonds"`
		CreatedAt interface{} `json:"createdAt"`
	}
	rows := make([]userRow, 0, len(users))
	for _, u := range users {
		rows = append(rows, userRow{
			ID: u.ID, Nickname: u.Nickname, Level: u.Level,
			Coins: u.Coins, Diamonds: u.Diamonds, CreatedAt: u.CreatedAt,
		})
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": gin.H{"users": rows, "total": len(rows)}})
}

// ListVillages handles GET /api/v1/admin/villages
// Returns all villages for admin management.
func (h *AdminHandler) ListVillages(c *gin.Context) {
	ctx := c.Request.Context()
	villages, err := h.db.Village.Query().
		Order(entvillage.ByLevel()).
		All(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "query failed", "data": nil})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": gin.H{"villages": villages, "total": len(villages)}})
}
