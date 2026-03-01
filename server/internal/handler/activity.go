package handler

import (
	"fmt"
	"net/http"
	"time"

	"entgo.io/ent/dialect/sql"
	"github.com/gin-gonic/gin"

	"github.com/liukai/farmer/server/ent"
	"github.com/liukai/farmer/server/ent/activitylog"
)

// ActivityHandler groups activity-related route handlers.
type ActivityHandler struct {
	db *ent.Client
}

// NewActivityHandler constructs an ActivityHandler.
func NewActivityHandler(db *ent.Client) *ActivityHandler { return &ActivityHandler{db: db} }

// OfflineSummary handles GET /api/v1/activity/offline-summary
//
// Query params:
//
//	since (optional) — RFC3339 timestamp; activities after this time are returned.
//	                   Defaults to 8 hours ago when missing or unparseable.
func (h *ActivityHandler) OfflineSummary(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "unauthorized", "data": nil})
		return
	}

	// Parse ?since= param; fall back to 8 hours ago.
	since := time.Now().Add(-8 * time.Hour)
	if raw := c.Query("since"); raw != "" {
		if t, err := time.Parse(time.RFC3339, raw); err == nil {
			since = t
		}
	}

	ctx := c.Request.Context()

	// Fetch all activity logs since the given time, ordered chronologically.
	logs, err := h.db.ActivityLog.Query().
		Where(
			activitylog.UserID(userID),
			activitylog.CreatedAtGT(since),
		).
		Order(activitylog.ByCreatedAt(sql.OrderAsc())).
		All(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "query failed", "data": nil})
		return
	}

	// Compute summary stats from all results.
	totalEvents := len(logs)
	harvests := 0
	coins := int64(0)

	for _, l := range logs {
		if l.Type == "harvest" {
			harvests++
			if v, ok := l.Meta["coinReward"]; ok {
				switch n := v.(type) {
				case float64:
					coins += int64(n)
				case int64:
					coins += n
				case int:
					coins += int64(n)
				}
			}
		}
	}

	// Build summary string.
	var summary string
	if harvests == 0 {
		summary = "你离开期间，暂时没有收获作物。"
	} else {
		summary = fmt.Sprintf("你离开期间，共收获了 %d 次作物，获得 %d 金币。", harvests, coins)
	}

	// Cap event list to the 20 most recent entries.
	// Logs are ordered ASC, so take the tail if there are more than 20.
	displayLogs := logs
	if len(displayLogs) > 20 {
		displayLogs = displayLogs[len(displayLogs)-20:]
	}

	events := make([]map[string]any, len(displayLogs))
	for i, l := range displayLogs {
		events[i] = map[string]any{
			"type":    l.Type,
			"content": l.Content,
			"at":      l.CreatedAt.UTC().Format(time.RFC3339),
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "ok",
		"data": gin.H{
			"totalEvents": totalEvents,
			"coins":       coins,
			"harvests":    harvests,
			"events":      events,
			"summary":     summary,
		},
	})
}
