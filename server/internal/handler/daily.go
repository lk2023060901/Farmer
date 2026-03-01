package handler

import (
	"net/http"
	"time"

	"entgo.io/ent/dialect/sql"
	"github.com/gin-gonic/gin"

	"github.com/liukai/farmer/server/ent"
	entcheckin "github.com/liukai/farmer/server/ent/dailycheckin"
	entuser "github.com/liukai/farmer/server/ent/user"
)

// DailyHandler groups daily check-in route handlers.
type DailyHandler struct {
	db *ent.Client
}

// NewDailyHandler constructs a DailyHandler.
func NewDailyHandler(db *ent.Client) *DailyHandler { return &DailyHandler{db: db} }

// dailyReward describes the reward for one day in the 7-day cycle.
type dailyReward struct {
	Type     string `json:"type"`     // "coins" | "diamonds" | "stamina"
	Quantity int    `json:"quantity"`
}

// checkinRewards defines the 7-day rotating reward cycle (index 0 = Day 1).
var checkinRewards = [7]dailyReward{
	{Type: "coins", Quantity: 100},
	{Type: "coins", Quantity: 200},
	{Type: "coins", Quantity: 300},
	{Type: "stamina", Quantity: 30},
	{Type: "coins", Quantity: 500},
	{Type: "coins", Quantity: 800},
	{Type: "diamonds", Quantity: 50},
}

// todayUTC returns today's date as a UTC time at 00:00:00.
func todayUTC() time.Time {
	now := time.Now().UTC()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
}

// CheckIn handles POST /api/v1/daily/checkin
// Idempotent: returns the existing record if already checked in today.
func (h *DailyHandler) CheckIn(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "unauthorized", "data": nil})
		return
	}
	ctx := c.Request.Context()
	today := todayUTC()

	// Already checked in today?
	existing, err := h.db.DailyCheckin.Query().
		Where(
			entcheckin.UserID(userID),
			entcheckin.CheckinDateGTE(today),
			entcheckin.CheckinDateLT(today.Add(24*time.Hour)),
		).
		Only(ctx)
	if err == nil {
		// Already checked in — return existing record (idempotent)
		c.JSON(http.StatusOK, gin.H{
			"code": 0, "message": "already checked in",
			"data": checkinDTO(existing, false),
		})
		return
	}

	// Determine streak: find yesterday's checkin
	yesterday := today.Add(-24 * time.Hour)
	prev, err := h.db.DailyCheckin.Query().
		Where(
			entcheckin.UserID(userID),
			entcheckin.CheckinDateGTE(yesterday),
			entcheckin.CheckinDateLT(today),
		).
		Only(ctx)

	streak := 1
	if err == nil {
		// Consecutive
		streak = prev.ConsecutiveDays + 1
	}

	// 7-day cycle: Day 1–7 reward, then resets
	cycleDay := (streak - 1) % 7
	reward := checkinRewards[cycleDay]

	// Apply reward to user
	u, err := h.db.User.Query().Where(entuser.ID(userID)).Only(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "user not found", "data": nil})
		return
	}

	upd := h.db.User.UpdateOne(u)
	switch reward.Type {
	case "coins":
		upd = upd.AddCoins(int64(reward.Quantity))
	case "diamonds":
		upd = upd.AddDiamonds(reward.Quantity)
	case "stamina":
		newStamina := u.Stamina + reward.Quantity
		if newStamina > u.MaxStamina {
			newStamina = u.MaxStamina
		}
		upd = upd.SetStamina(newStamina).SetStaminaUpdatedAt(time.Now())
	}
	if _, err := upd.Save(ctx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "reward failed", "data": nil})
		return
	}

	// Record checkin
	record, err := h.db.DailyCheckin.Create().
		SetUserID(userID).
		SetCheckinDate(today).
		SetConsecutiveDays(streak).
		SetRewardType(reward.Type).
		SetRewardQuantity(reward.Quantity).
		Save(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "save failed", "data": nil})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0, "message": "ok",
		"data": checkinDTO(record, true),
	})
}

// GetStreak handles GET /api/v1/daily/streak
// Returns the current consecutive check-in streak and today's status.
func (h *DailyHandler) GetStreak(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "unauthorized", "data": nil})
		return
	}
	ctx := c.Request.Context()
	today := todayUTC()

	// Check today's record
	todayRecord, todayErr := h.db.DailyCheckin.Query().
		Where(
			entcheckin.UserID(userID),
			entcheckin.CheckinDateGTE(today),
			entcheckin.CheckinDateLT(today.Add(24*time.Hour)),
		).
		Only(ctx)

	// Find most recent record for streak count
	last, err := h.db.DailyCheckin.Query().
		Where(entcheckin.UserID(userID)).
		Order(entcheckin.ByCheckinDate(sql.OrderDesc())).
		First(ctx)

	streak := 0
	if err == nil && last != nil {
		streak = last.ConsecutiveDays
	}

	checkedToday := todayErr == nil
	cycleDay := (streak - 1) % 7
	if cycleDay < 0 {
		cycleDay = 0
	}

	// Build next 7 rewards for UI display
	rewards := make([]map[string]any, 7)
	for i := 0; i < 7; i++ {
		r := checkinRewards[i]
		rewards[i] = map[string]any{
			"day":      i + 1,
			"type":     r.Type,
			"quantity": r.Quantity,
		}
	}

	resp := map[string]any{
		"streak":       streak,
		"checkedToday": checkedToday,
		"cycleDay":     cycleDay + 1, // 1-indexed for UI
		"rewards":      rewards,
	}
	if checkedToday && todayRecord != nil {
		resp["todayReward"] = map[string]any{
			"type":     todayRecord.RewardType,
			"quantity": todayRecord.RewardQuantity,
		}
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": resp})
}

// checkinDTO converts a DailyCheckin record to a response map.
func checkinDTO(r *ent.DailyCheckin, isNew bool) map[string]any {
	return map[string]any{
		"isNew":           isNew,
		"streak":          r.ConsecutiveDays,
		"rewardType":      r.RewardType,
		"rewardQuantity":  r.RewardQuantity,
		"checkinDate":     r.CheckinDate.Format("2006-01-02"),
	}
}
