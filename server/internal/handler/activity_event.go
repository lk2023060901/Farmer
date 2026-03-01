package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/liukai/farmer/server/ent"
)

// EventActivityHandler handles festival/game-event endpoints.
type EventActivityHandler struct {
	db *ent.Client
}

// NewEventActivityHandler constructs an EventActivityHandler.
func NewEventActivityHandler(db *ent.Client) *EventActivityHandler {
	return &EventActivityHandler{db: db}
}

// gameEvent describes a recurring in-game festival or special event.
type gameEvent struct {
	Type        string `json:"type"`
	Name        string `json:"name"`
	Active      bool   `json:"active"`
	Description string `json:"description"`
	Reward      string `json:"reward"`
	NextAt      string `json:"nextAt"`
}

// isVillageFeastActive returns true on Sundays (weekday == 0).
func isVillageFeastActive(t time.Time) bool {
	return t.Weekday() == time.Sunday
}

// isMarketDayActive returns true when day-of-month % 3 == 0.
func isMarketDayActive(t time.Time) bool {
	return t.Day()%3 == 0
}

// isFishingContestActive returns true on Saturdays (weekday == 6).
func isFishingContestActive(t time.Time) bool {
	return t.Weekday() == time.Saturday
}

// isHarvestFestivalActive returns true in the last week of September (month==9, day>=22).
func isHarvestFestivalActive(t time.Time) bool {
	return t.Month() == time.September && t.Day() >= 22
}

// nextVillageFeast returns the next Sunday (or today if it is Sunday).
func nextVillageFeast(t time.Time) time.Time {
	if t.Weekday() == time.Sunday {
		return startOfDay(t)
	}
	daysUntil := (7 - int(t.Weekday())) % 7
	if daysUntil == 0 {
		daysUntil = 7
	}
	return startOfDay(t.AddDate(0, 0, daysUntil))
}

// nextMarketDay returns the next day where day%3==0 (or today if already qualifies).
func nextMarketDay(t time.Time) time.Time {
	d := t
	for i := 0; i <= 31; i++ {
		if d.Day()%3 == 0 {
			return startOfDay(d)
		}
		d = d.AddDate(0, 0, 1)
	}
	return startOfDay(t)
}

// nextFishingContest returns the next Saturday (or today if it is Saturday).
func nextFishingContest(t time.Time) time.Time {
	if t.Weekday() == time.Saturday {
		return startOfDay(t)
	}
	daysUntil := (int(time.Saturday) - int(t.Weekday()) + 7) % 7
	if daysUntil == 0 {
		daysUntil = 7
	}
	return startOfDay(t.AddDate(0, 0, daysUntil))
}

// nextHarvestFestival returns the start of the next harvest festival window.
// The window is September 22–30; if currently in range, return today.
func nextHarvestFestival(t time.Time) time.Time {
	if t.Month() == time.September && t.Day() >= 22 {
		return startOfDay(t)
	}
	// Find next September 22
	year := t.Year()
	candidate := time.Date(year, time.September, 22, 0, 0, 0, 0, time.UTC)
	if candidate.Before(t) {
		// This year's festival already passed — use next year
		candidate = time.Date(year+1, time.September, 22, 0, 0, 0, 0, time.UTC)
	}
	return candidate
}

// startOfDay truncates a time to midnight UTC.
func startOfDay(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// buildEvents computes the current status for all four game events relative to now.
func buildEvents(now time.Time) []gameEvent {
	now = now.UTC()

	villageFeastActive := isVillageFeastActive(now)
	marketDayActive := isMarketDayActive(now)
	fishingContestActive := isFishingContestActive(now)
	harvestFestivalActive := isHarvestFestivalActive(now)

	villageFeastNext := nextVillageFeast(now)
	marketDayNext := nextMarketDay(now)
	fishingContestNext := nextFishingContest(now)
	harvestFestivalNext := nextHarvestFestival(now)

	return []gameEvent{
		{
			Type:        "village_feast",
			Name:        "村庄聚餐",
			Active:      villageFeastActive,
			Description: "每周日，全村村民齐聚一堂，共享丰盛的聚餐。社交互动获得双倍积分！",
			Reward:      "社交分×2",
			NextAt:      villageFeastNext.Format(time.RFC3339),
		},
		{
			Type:        "market_day",
			Name:        "赶集日",
			Active:      marketDayActive,
			Description: "每三天举办一次的热闹集市，商人们争相摆摊。今日所有交易手续费减半！",
			Reward:      "交易手续费减半",
			NextAt:      marketDayNext.Format(time.RFC3339),
		},
		{
			Type:        "fishing_contest",
			Name:        "钓鱼比赛",
			Active:      fishingContestActive,
			Description: "每周六在村旁小河举行钓鱼比赛，钓到稀有鱼类可赢取丰厚奖励！",
			Reward:      "钓鱼金币×3",
			NextAt:      fishingContestNext.Format(time.RFC3339),
		},
		{
			Type:        "harvest_festival",
			Name:        "丰收祭",
			Active:      harvestFestivalActive,
			Description: "九月下旬全村举行盛大丰收祭，全体村民收获量提升30%！",
			Reward:      "全员收获量+30%",
			NextAt:      harvestFestivalNext.Format(time.RFC3339),
		},
	}
}

// ListEvents handles GET /api/v1/events
// Returns upcoming and active game events based on the current real-world calendar.
func (h *EventActivityHandler) ListEvents(c *gin.Context) {
	_, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "unauthorized", "data": nil})
		return
	}

	events := buildEvents(time.Now())

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "ok",
		"data": gin.H{
			"events": events,
		},
	})
}

// GetEventStatus handles GET /api/v1/events/:type/status
// Returns whether the named event is currently active and the time until its next occurrence.
func (h *EventActivityHandler) GetEventStatus(c *gin.Context) {
	_, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "unauthorized", "data": nil})
		return
	}

	eventType := c.Param("type")
	now := time.Now().UTC()
	events := buildEvents(now)

	for _, ev := range events {
		if ev.Type == eventType {
			var secondsUntilNext int64
			if nextT, err := time.Parse(time.RFC3339, ev.NextAt); err == nil {
				secondsUntilNext = int64(time.Until(nextT).Seconds())
				if secondsUntilNext < 0 {
					secondsUntilNext = 0
				}
			}
			c.JSON(http.StatusOK, gin.H{
				"code":    0,
				"message": "ok",
				"data": gin.H{
					"type":             ev.Type,
					"name":             ev.Name,
					"active":           ev.Active,
					"description":      ev.Description,
					"reward":           ev.Reward,
					"nextAt":           ev.NextAt,
					"secondsUntilNext": secondsUntilNext,
				},
			})
			return
		}
	}

	c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "event type not found", "data": nil})
}
