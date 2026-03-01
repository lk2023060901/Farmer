package handler

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"time"

	"entgo.io/ent/dialect/sql"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/liukai/farmer/server/ent"
	entseason "github.com/liukai/farmer/server/ent/season"
	entss "github.com/liukai/farmer/server/ent/seasonscore"
	entstp "github.com/liukai/farmer/server/ent/seasontaskprogress"
	entvillage "github.com/liukai/farmer/server/ent/village"
	entvm "github.com/liukai/farmer/server/ent/villagemember"
)

// SeasonHandler groups season / competitive-event route handlers.
type SeasonHandler struct {
	db *ent.Client
}

// NewSeasonHandler constructs a SeasonHandler.
func NewSeasonHandler(db *ent.Client) *SeasonHandler { return &SeasonHandler{db: db} }

// defaultTasksConfig is the standard task list seeded on season creation.
var defaultTasksConfig = []interface{}{
	map[string]interface{}{
		"id": "harvest_50", "title": "收获50次", "type": "harvest", "target": 50,
		"reward": map[string]interface{}{"type": "coins", "amount": 200},
	},
	map[string]interface{}{
		"id": "visit_20", "title": "串门20次", "type": "visit", "target": 20,
		"reward": map[string]interface{}{"type": "coins", "amount": 150},
	},
	map[string]interface{}{
		"id": "trade_10", "title": "完成10次交易", "type": "trade", "target": 10,
		"reward": map[string]interface{}{"type": "diamonds", "amount": 10},
	},
	map[string]interface{}{
		"id": "gift_5", "title": "赠礼5次", "type": "gift", "target": 5,
		"reward": map[string]interface{}{"type": "coins", "amount": 100},
	},
	map[string]interface{}{
		"id": "contribute_village", "title": "参与3次共建", "type": "contribute", "target": 3,
		"reward": map[string]interface{}{"type": "diamonds", "amount": 5},
	},
}

// ensureCurrentSeason guarantees an active season exists, creating Season 1 if needed.
func ensureCurrentSeason(ctx context.Context, db *ent.Client) (*ent.Season, error) {
	s, err := db.Season.Query().
		Where(entseason.Status("active")).
		First(ctx)
	if err == nil {
		return s, nil
	}
	if !ent.IsNotFound(err) {
		return nil, fmt.Errorf("query active season: %w", err)
	}

	// No active season — create Season 1.
	now := time.Now()
	s, err = db.Season.Create().
		SetNumber(1).
		SetName("第一赛季").
		SetStartAt(now).
		SetEndAt(now.Add(28 * 24 * time.Hour)).
		SetStatus("active").
		SetTasksConfig(defaultTasksConfig).
		SetRewardsConfig(map[string]interface{}{}).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("create season: %w", err)
	}
	return s, nil
}

// computeTotal calculates the weighted total score.
func computeTotal(output, social, collection, quality int64) float64 {
	return float64(output)*0.4 + float64(social)*0.3 + float64(collection)*0.2 + float64(quality)*0.1
}

// GetCurrent handles GET /api/v1/seasons/current
func (h *SeasonHandler) GetCurrent(c *gin.Context) {
	ctx := c.Request.Context()

	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "unauthorized", "data": nil})
		return
	}

	season, err := ensureCurrentSeason(ctx, h.db)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error(), "data": nil})
		return
	}

	daysLeft := int(time.Until(season.EndAt).Hours() / 24)
	if daysLeft < 0 {
		daysLeft = 0
	}

	seasonData := map[string]interface{}{
		"id":       season.ID.String(),
		"number":   season.Number,
		"name":     season.Name,
		"startAt":  season.StartAt.Format(time.RFC3339),
		"endAt":    season.EndAt.Format(time.RFC3339),
		"status":   season.Status,
		"daysLeft": daysLeft,
	}

	// Get or create SeasonScore for this user+season.
	ss, err := h.db.SeasonScore.Query().
		Where(
			entss.SeasonID(season.ID),
			entss.UserID(userID),
		).
		First(ctx)
	if err != nil && !ent.IsNotFound(err) {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "query score failed", "data": nil})
		return
	}
	if ent.IsNotFound(err) {
		ss, err = h.db.SeasonScore.Create().
			SetSeasonID(season.ID).
			SetUserID(userID).
			SetScoreOutput(0).
			SetScoreSocial(0).
			SetScoreCollection(0).
			SetScoreQuality(0).
			SetScoreTotal(0).
			Save(ctx)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "create score failed", "data": nil})
			return
		}
	}

	total := computeTotal(ss.ScoreOutput, ss.ScoreSocial, ss.ScoreCollection, ss.ScoreQuality)

	// Compute rank: count of scores with a higher score_total + 1.
	higher, err := h.db.SeasonScore.Query().
		Where(
			entss.SeasonID(season.ID),
			entss.ScoreTotalGT(ss.ScoreTotal),
		).
		Count(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "compute rank failed", "data": nil})
		return
	}
	rank := higher + 1

	userScore := map[string]interface{}{
		"scoreOutput":     ss.ScoreOutput,
		"scoreSocial":     ss.ScoreSocial,
		"scoreCollection": ss.ScoreCollection,
		"scoreQuality":    ss.ScoreQuality,
		"totalScore":      total,
		"rank":            rank,
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "ok",
		"data": map[string]interface{}{
			"season":    seasonData,
			"userScore": userScore,
		},
	})
}

// GetLeaderboard handles GET /api/v1/seasons/current/leaderboard?limit=50
func (h *SeasonHandler) GetLeaderboard(c *gin.Context) {
	ctx := c.Request.Context()

	if _, ok := currentUserID(c); !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "unauthorized", "data": nil})
		return
	}

	season, err := ensureCurrentSeason(ctx, h.db)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error(), "data": nil})
		return
	}

	limitStr := c.DefaultQuery("limit", "50")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 || limit > 100 {
		limit = 50
	}

	scores, err := h.db.SeasonScore.Query().
		Where(entss.SeasonID(season.ID)).
		Order(entss.ByScoreTotal(sql.OrderDesc())).
		Limit(limit).
		WithUser().
		All(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "query leaderboard failed", "data": nil})
		return
	}

	items := make([]map[string]interface{}, 0, len(scores))
	for i, s := range scores {
		total := computeTotal(s.ScoreOutput, s.ScoreSocial, s.ScoreCollection, s.ScoreQuality)
		entry := map[string]interface{}{
			"rank":            i + 1,
			"userId":          s.UserID.String(),
			"totalScore":      total,
			"scoreOutput":     s.ScoreOutput,
			"scoreSocial":     s.ScoreSocial,
			"scoreCollection": s.ScoreCollection,
			"scoreQuality":    s.ScoreQuality,
		}
		if s.Edges.User != nil {
			u := s.Edges.User
			entry["nickname"] = u.Nickname
			entry["avatar"] = u.Avatar
			entry["level"] = u.Level
		} else {
			entry["nickname"] = "未知用户"
			entry["avatar"] = ""
			entry["level"] = 1
		}
		items = append(items, entry)
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": items})
}

// GetTasks handles GET /api/v1/seasons/current/tasks
func (h *SeasonHandler) GetTasks(c *gin.Context) {
	ctx := c.Request.Context()

	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "unauthorized", "data": nil})
		return
	}

	season, err := ensureCurrentSeason(ctx, h.db)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error(), "data": nil})
		return
	}

	// Load task progress rows for this user.
	progresses, err := h.db.SeasonTaskProgress.Query().
		Where(
			entstp.SeasonID(season.ID),
			entstp.UserID(userID),
		).
		All(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "query task progress failed", "data": nil})
		return
	}

	// Index existing progress by task_id.
	progressMap := make(map[string]*ent.SeasonTaskProgress, len(progresses))
	for _, p := range progresses {
		progressMap[p.TaskID] = p
	}

	// Build response from season tasks_config (source of truth for title, type, reward).
	tasksConfig := season.TasksConfig
	result := make([]map[string]interface{}, 0, len(tasksConfig))

	for _, raw := range tasksConfig {
		task, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		taskID, _ := task["id"].(string)
		title, _ := task["title"].(string)
		taskType, _ := task["type"].(string)
		reward := task["reward"]

		var targetInt int64
		switch v := task["target"].(type) {
		case float64:
			targetInt = int64(v)
		case int:
			targetInt = int64(v)
		case int64:
			targetInt = v
		}

		var current int64
		var completed bool
		if p, found := progressMap[taskID]; found {
			current = p.Progress
			completed = p.Completed
		}

		result = append(result, map[string]interface{}{
			"taskId":    taskID,
			"title":     title,
			"type":      taskType,
			"target":    targetInt,
			"current":   current,
			"completed": completed,
			"reward":    reward,
		})
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": result})
}

// GetVillageLeaderboard handles GET /api/v1/seasons/current/village-leaderboard
func (h *SeasonHandler) GetVillageLeaderboard(c *gin.Context) {
	ctx := c.Request.Context()

	season, err := ensureCurrentSeason(ctx, h.db)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error(), "data": nil})
		return
	}

	// Fetch all season scores for this season.
	allScores, err := h.db.SeasonScore.Query().
		Where(entss.SeasonID(season.ID)).
		All(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "query scores failed", "data": nil})
		return
	}

	if len(allScores) == 0 {
		c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": []interface{}{}})
		return
	}

	// Collect all user IDs.
	userIDs := make([]uuid.UUID, 0, len(allScores))
	for _, s := range allScores {
		userIDs = append(userIDs, s.UserID)
	}

	// Look up village memberships for all users.
	members, err := h.db.VillageMember.Query().
		Where(entvm.UserIDIn(userIDs...)).
		All(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "query village members failed", "data": nil})
		return
	}

	// Build user -> village map.
	userVillage := make(map[uuid.UUID]uuid.UUID, len(members))
	for _, m := range members {
		userVillage[m.UserID] = m.VillageID
	}

	// Aggregate scores per village.
	type villageAgg struct {
		villageID  uuid.UUID
		totalScore float64
	}
	villageScores := make(map[uuid.UUID]float64)
	for _, s := range allScores {
		vID, found := userVillage[s.UserID]
		if !found {
			continue
		}
		villageScores[vID] += computeTotal(s.ScoreOutput, s.ScoreSocial, s.ScoreCollection, s.ScoreQuality)
	}

	aggs := make([]villageAgg, 0, len(villageScores))
	for vID, score := range villageScores {
		aggs = append(aggs, villageAgg{villageID: vID, totalScore: score})
	}
	sort.Slice(aggs, func(i, j int) bool {
		return aggs[i].totalScore > aggs[j].totalScore
	})
	if len(aggs) > 20 {
		aggs = aggs[:20]
	}

	// Fetch village names.
	villageIDs := make([]uuid.UUID, 0, len(aggs))
	for _, a := range aggs {
		villageIDs = append(villageIDs, a.villageID)
	}

	villageNameMap := make(map[uuid.UUID]string)
	if len(villageIDs) > 0 {
		vs, qErr := h.db.Village.Query().
			Where(entvillage.IDIn(villageIDs...)).
			All(ctx)
		if qErr == nil {
			for _, v := range vs {
				villageNameMap[v.ID] = v.Name
			}
		}
	}

	items := make([]map[string]interface{}, 0, len(aggs))
	for i, a := range aggs {
		name := villageNameMap[a.villageID]
		if name == "" {
			name = a.villageID.String()
		}
		items = append(items, map[string]interface{}{
			"rank":        i + 1,
			"villageId":   a.villageID.String(),
			"villageName": name,
			"totalScore":  a.totalScore,
		})
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": items})
}
