package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/liukai/farmer/server/ent"
	entchatlog "github.com/liukai/farmer/server/ent/chatlog"
	entfarm "github.com/liukai/farmer/server/ent/farm"
	entrole "github.com/liukai/farmer/server/ent/role"
	entuser "github.com/liukai/farmer/server/ent/user"
	"github.com/liukai/farmer/server/internal/gameconfig"
	"github.com/liukai/farmer/server/internal/llm"
	"github.com/liukai/farmer/server/internal/role"
)

// RoleHandler groups AI farming-agent route handlers.
type RoleHandler struct {
	db      *ent.Client
	llmSvc  *llm.Service
	gameCfg *gameconfig.Config
}

// NewRoleHandler constructs a RoleHandler.
func NewRoleHandler(db *ent.Client, llmSvc *llm.Service, gameCfg *gameconfig.Config) *RoleHandler {
	return &RoleHandler{db: db, llmSvc: llmSvc, gameCfg: gameCfg}
}

// GetAgent handles GET /api/v1/agent
// Returns the authenticated user's AI agent configuration.
func (h *RoleHandler) GetAgent(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "unauthorized", "data": nil})
		return
	}

	ctx := c.Request.Context()
	ag, err := h.db.Role.Query().Where(entrole.UserID(userID)).Only(ctx)
	if ent.IsNotFound(err) {
		// 老账号没有 role 记录，自动用游戏配置默认值创建一条
		ag, err = h.autoCreateRole(ctx, userID)
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "agent error", "data": nil})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0, "message": "ok",
		"data": roleDTO(ag),
	})
}

// autoCreateRole creates a Role for accounts that pre-date role creation.
func (h *RoleHandler) autoCreateRole(ctx context.Context, userID uuid.UUID) (*ent.Role, error) {
	name, avatar := "农场主", ""
	if r := h.gameCfg.GetRole(h.gameCfg.Global.RoleID); r != nil {
		name, avatar = r.Name, r.Avatar
	}
	spawn := h.gameCfg.Spawn("world")
	return h.db.Role.Create().
		SetUserID(userID).
		SetName(name).
		SetAvatar(avatar).
		SetMapID("world").
		SetTileX(spawn.TileX).
		SetTileY(spawn.TileY).
		SetLastActiveAt(time.Now()).
		Save(ctx)
}

// updateAgentReq holds editable agent fields.
type updateAgentReq struct {
	Name               *string `json:"name"`
	Avatar             *string `json:"avatar"`
	StrategyManagement *int    `json:"strategyManagement"`
	StrategyPlanting   *int    `json:"strategyPlanting"`
	StrategySocial     *int    `json:"strategySocial"`
	StrategyTrade      *int    `json:"strategyTrade"`
	StrategyResource   *int    `json:"strategyResource"`
}

// UpdateAgent handles PUT /api/v1/agent
// Updates agent name, avatar, and/or strategy sliders (1-5 each).
func (h *RoleHandler) UpdateAgent(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "unauthorized", "data": nil})
		return
	}

	var req updateAgentReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error(), "data": nil})
		return
	}

	ag, err := h.db.Role.Query().Where(entrole.UserID(userID)).Only(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "agent not found", "data": nil})
		return
	}

	upd := h.db.Role.UpdateOne(ag)
	if req.Name != nil {
		upd = upd.SetName(*req.Name)
	}
	if req.Avatar != nil {
		upd = upd.SetAvatar(*req.Avatar)
	}
	if req.StrategyManagement != nil {
		upd = upd.SetStrategyManagement(*req.StrategyManagement)
	}
	if req.StrategyPlanting != nil {
		upd = upd.SetStrategyPlanting(*req.StrategyPlanting)
	}
	if req.StrategySocial != nil {
		upd = upd.SetStrategySocial(*req.StrategySocial)
	}
	if req.StrategyTrade != nil {
		upd = upd.SetStrategyTrade(*req.StrategyTrade)
	}
	if req.StrategyResource != nil {
		upd = upd.SetStrategyResource(*req.StrategyResource)
	}

	ag, err = upd.Save(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "save failed", "data": nil})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": roleDTO(ag)})
}

// chatReq is the request body for POST /api/v1/agent/chat.
type chatReq struct {
	TargetUserID string `json:"targetUserId" binding:"required"` // other agent's user UUID
	Scene        string `json:"scene"`                           // visit/trade/help/gift/general
	Affinity     int    `json:"affinity"`                        // current affinity score (0-100)
}

// Chat handles POST /api/v1/agent/chat
// Generates a two-line exchange between the caller's agent and the target's agent.
// Uses LLM when quota allows, falls back to template dialog automatically.
func (h *RoleHandler) Chat(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "unauthorized", "data": nil})
		return
	}

	var req chatReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error(), "data": nil})
		return
	}

	targetID, err := uuid.Parse(req.TargetUserID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid targetUserId", "data": nil})
		return
	}

	ctx := c.Request.Context()

	// Load both agents.
	agA, err := h.db.Role.Query().Where(entrole.UserID(userID)).Only(ctx)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "your agent not found", "data": nil})
		return
	}
	agB, err := h.db.Role.Query().Where(entrole.UserID(targetID)).Only(ctx)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "target agent not found", "data": nil})
		return
	}

	// Canonicalise pair order for chat log lookup (smaller UUID first).
	var uidA, uidB uuid.UUID
	if userID.String() < targetID.String() {
		uidA, uidB = userID, targetID
	} else {
		uidA, uidB = targetID, userID
	}

	// Fetch last 5 chat lines for context.
	recentLogs, _ := h.db.ChatLog.Query().
		Where(entchatlog.UserAID(uidA), entchatlog.UserBID(uidB)).
		Order(ent.Desc(entchatlog.FieldCreatedAt)).
		Limit(5).
		All(ctx)
	var recentLines []string
	for i := len(recentLogs) - 1; i >= 0; i-- { // chronological order
		recentLines = append(recentLines, recentLogs[i].Content)
	}

	scene := llm.Scene(req.Scene)
	if scene == "" {
		scene = llm.SceneGeneral
	}
	relLevel := llm.AffinityToLevel(req.Affinity)

	dialogReq := llm.DialogRequest{
		UserID: userID.String(),
		AgentA: llm.AgentPersonality{
			Name:         agA.Name,
			Extroversion: agA.Extroversion,
			Generosity:   agA.Generosity,
			Adventure:    agA.Adventure,
		},
		AgentB: llm.AgentPersonality{
			Name:         agB.Name,
			Extroversion: agB.Extroversion,
			Generosity:   agB.Generosity,
			Adventure:    agB.Adventure,
		},
		RelationLvl: relLevel,
		Scene:       scene,
		RecentLines: recentLines,
	}

	result, err := h.llmSvc.Generate(ctx, dialogReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "dialog generation failed", "data": nil})
		return
	}

	// Persist both lines to ChatLog.
	combined := agA.Name + "：" + result.LineA + "\n" + agB.Name + "：" + result.LineB
	h.db.ChatLog.Create().
		SetUserAID(uidA).
		SetUserBID(uidB).
		SetSpeakerUserID(userID).
		SetScene(string(scene)).
		SetContent(combined).
		SetIsLlmGenerated(result.IsLLM).
		Save(ctx) //nolint:errcheck — best-effort log

	c.JSON(http.StatusOK, gin.H{
		"code": 0, "message": "ok",
		"data": gin.H{
			"lineA":       result.LineA,
			"lineB":       result.LineB,
			"isLlm":       result.IsLLM,
			"fingerprint": result.Fingerprint,
		},
	})
}

// GetStrategy handles GET /api/v1/agent/strategy
// Returns the agent's current farming plan based on the rules engine.
func (h *RoleHandler) GetStrategy(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "unauthorized", "data": nil})
		return
	}

	ag, err := h.db.Role.Query().Where(entrole.UserID(userID)).Only(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "agent not found", "data": nil})
		return
	}

	u, err := h.db.User.Query().Where(entuser.ID(userID)).Only(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "user not found", "data": nil})
		return
	}

	farm, err := h.db.Farm.Query().Where(entfarm.OwnerID(userID)).Only(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "farm not found", "data": nil})
		return
	}

	params := role.Params{
		StrategyManagement: ag.StrategyManagement,
		StrategyPlanting:   ag.StrategyPlanting,
		StrategyTrade:      ag.StrategyTrade,
		StrategyResource:   ag.StrategyResource,
		Extroversion:       ag.Extroversion,
		Generosity:         ag.Generosity,
		Adventure:          ag.Adventure,
		UserLevel:          u.Level,
		Stamina:            u.Stamina,
	}

	actions := role.Decide(farm.Plots, params)
	explanation := role.Explain(farm.Plots, params)

	c.JSON(http.StatusOK, gin.H{
		"code": 0, "message": "ok",
		"data": gin.H{
			"actions":     actions,
			"explanation": explanation,
		},
	})
}

// roleDTO maps an ent.Role to a JSON-friendly map.
func roleDTO(ag *ent.Role) map[string]any {
	return map[string]any{
		"id":                 ag.ID,
		"name":               ag.Name,
		"avatar":             ag.Avatar,
		"map_id":             ag.MapID,
		"tile_x":             ag.TileX,
		"tile_y":             ag.TileY,
		"extroversion":       ag.Extroversion,
		"generosity":         ag.Generosity,
		"adventure":          ag.Adventure,
		"strategyManagement": ag.StrategyManagement,
		"strategyPlanting":   ag.StrategyPlanting,
		"strategySocial":     ag.StrategySocial,
		"strategyTrade":      ag.StrategyTrade,
		"strategyResource":   ag.StrategyResource,
	}
}
