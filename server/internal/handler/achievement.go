package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/liukai/farmer/server/ent"
	entlog "github.com/liukai/farmer/server/ent/activitylog"
	entuser "github.com/liukai/farmer/server/ent/user"
)

// AchievementHandler groups achievement-system endpoints.
type AchievementHandler struct {
	db *ent.Client
}

// NewAchievementHandler constructs an AchievementHandler.
func NewAchievementHandler(db *ent.Client) *AchievementHandler { return &AchievementHandler{db: db} }

// achievementDef defines a single achievement.
type achievementDef struct {
	ID          string `json:"id"`
	Category    string `json:"category"` // operation/social/collection/season/special
	Name        string `json:"name"`
	Description string `json:"description"`
	Target      int    `json:"target"` // required count to unlock
	Reward      string `json:"reward"`
}

// achievements is the static achievement catalogue.
var achievements = []achievementDef{
	// 经营
	{ID: "first_harvest", Category: "operation", Name: "初次丰收", Description: "完成第一次收获", Target: 1, Reward: "金币×50"},
	{ID: "harvest_100", Category: "operation", Name: "百亩良田", Description: "累计收获100次", Target: 100, Reward: "金币×500"},
	{ID: "harvest_1000", Category: "operation", Name: "万亩农场主", Description: "累计收获1000次", Target: 1000, Reward: "金币×5000"},
	// 社交
	{ID: "first_visit", Category: "social", Name: "串门初体验", Description: "第一次拜访邻居", Target: 1, Reward: "体力×10"},
	{ID: "visit_50", Category: "social", Name: "邻里热心肠", Description: "累计拜访50次", Target: 50, Reward: "金币×300"},
	{ID: "gift_10", Category: "social", Name: "慷慨的邻居", Description: "累计赠礼10次", Target: 10, Reward: "钻石×5"},
	// 收藏
	{ID: "collect_all_crops", Category: "collection", Name: "农业收藏家", Description: "收获过所有8种作物", Target: 8, Reward: "钻石×10"},
	{ID: "level_10", Category: "special", Name: "农场老手", Description: "达到10级", Target: 10, Reward: "专属称号"},
	{ID: "level_20", Category: "special", Name: "农场大师", Description: "达到20级", Target: 20, Reward: "专属装饰"},
	// 赛季
	{ID: "season_top10", Category: "season", Name: "赛季精英", Description: "赛季排名进入前10", Target: 10, Reward: "钻石×50"},
}

// ListAchievements handles GET /api/v1/achievements
// Returns achievement list with progress and unlock status for the current user.
func (h *AchievementHandler) ListAchievements(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "unauthorized", "data": nil})
		return
	}
	ctx := c.Request.Context()

	// Count harvests from activity log
	harvestCount, _ := h.db.ActivityLog.Query().
		Where(entlog.UserID(userID), entlog.Type("harvest")).
		Count(ctx)

	// Count visits from activity log
	visitCount, _ := h.db.ActivityLog.Query().
		Where(entlog.UserID(userID), entlog.Type("visit")).
		Count(ctx)

	// Count gifts from activity log
	giftCount, _ := h.db.ActivityLog.Query().
		Where(entlog.UserID(userID), entlog.Type("gift")).
		Count(ctx)

	// Count distinct crops harvested (approximate via ActivityLog meta — use harvestCount proxy)
	cropVarietyCount, _ := h.db.ActivityLog.Query().
		Where(entlog.UserID(userID), entlog.Type("harvest")).
		Count(ctx)
	if cropVarietyCount > 8 {
		cropVarietyCount = 8
	}

	// Get user level
	u, err := h.db.User.Query().Where(entuser.ID(userID)).Only(ctx)
	_ = err

	// Count unlocked achievements from activity log
	unlockedLogs, _ := h.db.ActivityLog.Query().
		Where(entlog.UserID(userID), entlog.Type("achievement_unlock")).
		All(ctx)
	unlockedIDs := map[string]bool{}
	for _, al := range unlockedLogs {
		if id, ok := al.Meta["achievementId"].(string); ok {
			unlockedIDs[id] = true
		}
	}

	progressMap := map[string]int{
		"first_harvest":    harvestCount,
		"harvest_100":      harvestCount,
		"harvest_1000":     harvestCount,
		"first_visit":      visitCount,
		"visit_50":         visitCount,
		"gift_10":          giftCount,
		"collect_all_crops": cropVarietyCount,
		"season_top10":    0,
	}
	if u != nil {
		progressMap["level_10"] = u.Level
		progressMap["level_20"] = u.Level
	}

	type achievementResult struct {
		achievementDef
		Progress int  `json:"progress"`
		Unlocked bool `json:"unlocked"`
	}

	results := make([]achievementResult, 0, len(achievements))
	for _, a := range achievements {
		progress := progressMap[a.ID]
		unlocked := unlockedIDs[a.ID] || progress >= a.Target

		// Auto-unlock: record in DB if newly unlocked
		if unlocked && !unlockedIDs[a.ID] {
			_ = h.db.ActivityLog.Create().
				SetUserID(userID).
				SetType("achievement_unlock").
				SetContent("解锁成就: " + a.Name).
				SetMeta(map[string]any{"achievementId": a.ID, "reward": a.Reward}).
				Exec(ctx)
		}

		results = append(results, achievementResult{
			achievementDef: a,
			Progress:       min(progress, a.Target),
			Unlocked:       unlocked,
		})
	}

	total := len(achievements)
	unlocked := 0
	for _, r := range results {
		if r.Unlocked {
			unlocked++
		}
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": gin.H{
		"achievements": results,
		"total":        total,
		"unlocked":     unlocked,
	}})
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
