package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/liukai/farmer/server/ent"
	entinv "github.com/liukai/farmer/server/ent/inventoryitem"
	entschema "github.com/liukai/farmer/server/ent/schema"
	entuser "github.com/liukai/farmer/server/ent/user"
	entvillage "github.com/liukai/farmer/server/ent/village"
	entvp "github.com/liukai/farmer/server/ent/villageproject"
	entvm "github.com/liukai/farmer/server/ent/villagemember"
)

// villageLevelThresholds defines the cumulative contribution required to reach each village level.
// Index 0 = level 1 threshold, index 4 = level 5 threshold.
var villageLevelThresholds = [5]int64{500, 2000, 5000, 15000, 50000}

// VillageHandler groups village route handlers.
type VillageHandler struct {
	db *ent.Client
}

// NewVillageHandler constructs a VillageHandler.
func NewVillageHandler(db *ent.Client) *VillageHandler { return &VillageHandler{db: db} }

// villageDTO maps a village to a response object.
func villageDTO(v *ent.Village) map[string]any {
	return map[string]any{
		"id":           v.ID,
		"name":         v.Name,
		"level":        v.Level,
		"contribution": v.Contribution,
		"memberCount":  v.MemberCount,
		"maxMembers":   v.MaxMembers,
		"specialty":    v.Specialty,
	}
}

// Mine handles GET /api/v1/villages/mine — returns caller's village (null if not in one).
func (h *VillageHandler) Mine(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "unauthorized", "data": nil})
		return
	}

	mem, err := h.db.VillageMember.Query().
		Where(entvm.UserID(userID)).
		Only(c.Request.Context())
	if err != nil {
		// Not in any village — return null data with success code
		c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": nil})
		return
	}

	village, err := h.db.Village.Get(c.Request.Context(), mem.VillageID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "village not found", "data": nil})
		return
	}

	result := villageDTO(village)
	result["myRole"] = mem.Role
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": result})
}

// List handles GET /api/v1/villages — returns open villages ordered by contribution.
func (h *VillageHandler) List(c *gin.Context) {
	villages, err := h.db.Village.Query().
		Where(entvillage.MemberCountLT(20)).
		Order(ent.Desc(entvillage.FieldContribution)).
		Limit(20).
		All(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "query failed", "data": nil})
		return
	}
	result := make([]map[string]any, len(villages))
	for i, v := range villages {
		result[i] = villageDTO(v)
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": result})
}

// createVillageReq is the request body for POST /api/v1/villages
type createVillageReq struct {
	Name string `json:"name" binding:"required,min=2,max=64"`
}

// Create handles POST /api/v1/villages — caller becomes chief.
func (h *VillageHandler) Create(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "unauthorized", "data": nil})
		return
	}

	var req createVillageReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error(), "data": nil})
		return
	}

	exists, _ := h.db.VillageMember.Query().Where(entvm.UserID(userID)).Exist(c.Request.Context())
	if exists {
		c.JSON(http.StatusConflict, gin.H{"code": 409, "message": "already in a village", "data": nil})
		return
	}

	village, err := h.db.Village.Create().
		SetName(req.Name).
		SetMemberCount(1).
		Save(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "create failed", "data": nil})
		return
	}

	_, err = h.db.VillageMember.Create().
		SetVillageID(village.ID).
		SetUserID(userID).
		SetRole("chief").
		Save(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "member create failed", "data": nil})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": villageDTO(village)})
}

// GetByID handles GET /api/v1/villages/:id
func (h *VillageHandler) GetByID(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid id", "data": nil})
		return
	}
	village, err := h.db.Village.Get(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "village not found", "data": nil})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": villageDTO(village)})
}

// Join handles POST /api/v1/villages/:id/join
func (h *VillageHandler) Join(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "unauthorized", "data": nil})
		return
	}

	villageID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid id", "data": nil})
		return
	}

	already, _ := h.db.VillageMember.Query().Where(entvm.UserID(userID)).Exist(c.Request.Context())
	if already {
		c.JSON(http.StatusConflict, gin.H{"code": 409, "message": "already in a village", "data": nil})
		return
	}

	village, err := h.db.Village.Get(c.Request.Context(), villageID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "village not found", "data": nil})
		return
	}
	if village.MemberCount >= village.MaxMembers {
		c.JSON(http.StatusConflict, gin.H{"code": 409, "message": "village is full", "data": nil})
		return
	}

	_, err = h.db.VillageMember.Create().
		SetVillageID(villageID).
		SetUserID(userID).
		SetRole("member").
		Save(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "join failed", "data": nil})
		return
	}

	_, _ = h.db.Village.UpdateOne(village).AddMemberCount(1).Save(c.Request.Context())
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "joined", "data": gin.H{
		"villageId": villageID, "role": "member",
	}})
}

// Leave handles POST /api/v1/villages/:id/leave
func (h *VillageHandler) Leave(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "unauthorized", "data": nil})
		return
	}

	villageID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid id", "data": nil})
		return
	}

	mem, err := h.db.VillageMember.Query().
		Where(entvm.UserID(userID), entvm.VillageID(villageID)).
		Only(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "not a member", "data": nil})
		return
	}

	if err := h.db.VillageMember.DeleteOne(mem).Exec(c.Request.Context()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "leave failed", "data": nil})
		return
	}

	village, _ := h.db.Village.Get(c.Request.Context(), villageID)
	if village != nil {
		_, _ = h.db.Village.UpdateOne(village).AddMemberCount(-1).Save(c.Request.Context())
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "left", "data": nil})
}

// ListProjects handles GET /api/v1/villages/:id/projects
func (h *VillageHandler) ListProjects(c *gin.Context) {
	villageID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid id", "data": nil})
		return
	}

	projects, err := h.db.VillageProject.Query().
		Where(entvp.VillageID(villageID)).
		All(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "query failed", "data": nil})
		return
	}

	type dto struct {
		ID           uuid.UUID                    `json:"id"`
		Name         string                       `json:"name"`
		Type         string                       `json:"type"`
		Requirements []entschema.RequirementItem  `json:"requirements"`
		Status       string                       `json:"status"`
	}
	result := make([]dto, len(projects))
	for i, p := range projects {
		result[i] = dto{ID: p.ID, Name: p.Name, Type: p.Type, Requirements: p.Requirements, Status: p.Status}
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": result})
}

// createProjectReq is the request body for POST /api/v1/villages/:id/projects
type createProjectReq struct {
	Name     string `json:"name" binding:"required,min=2,max=64"`
	Type     string `json:"type" binding:"required"`
	GoalCoins int64 `json:"goalCoins" binding:"min=100"`
}

// CreateProject handles POST /api/v1/villages/:id/projects (chief/elder only).
func (h *VillageHandler) CreateProject(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "unauthorized", "data": nil})
		return
	}

	villageID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid id", "data": nil})
		return
	}

	mem, err := h.db.VillageMember.Query().
		Where(entvm.UserID(userID), entvm.VillageID(villageID)).
		Only(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"code": 403, "message": "not a member", "data": nil})
		return
	}
	if mem.Role != "chief" && mem.Role != "elder" {
		c.JSON(http.StatusForbidden, gin.H{"code": 403, "message": "only chief/elder can create projects", "data": nil})
		return
	}

	var req createProjectReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error(), "data": nil})
		return
	}

	requirements := []entschema.RequirementItem{
		{Type: "coins", Required: req.GoalCoins, Current: 0},
	}

	project, err := h.db.VillageProject.Create().
		SetVillageID(villageID).
		SetName(req.Name).
		SetType(req.Type).
		SetRequirements(requirements).
		Save(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "create failed", "data": nil})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": gin.H{
		"id": project.ID, "name": project.Name, "type": project.Type,
	}})
}

// contributeReq is the request body for POST .../projects/:projectID/contribute
type contributeReq struct {
	Coins int64 `json:"coins" binding:"min=1"`
}

// ContributeProject handles POST /api/v1/villages/:id/projects/:projectID/contribute
func (h *VillageHandler) ContributeProject(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "unauthorized", "data": nil})
		return
	}

	villageID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid village id", "data": nil})
		return
	}
	projectID, err := uuid.Parse(c.Param("projectID"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid project id", "data": nil})
		return
	}

	var req contributeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error(), "data": nil})
		return
	}

	// Verify membership
	mem, err := h.db.VillageMember.Query().
		Where(entvm.UserID(userID), entvm.VillageID(villageID)).
		Only(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"code": 403, "message": "not a member", "data": nil})
		return
	}

	// Verify user coins
	u, err := h.db.User.Query().Where(entuser.ID(userID)).Only(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "user query failed", "data": nil})
		return
	}
	if u.Coins < req.Coins {
		c.JSON(http.StatusConflict, gin.H{"code": 409, "message": "not enough coins", "data": nil})
		return
	}

	project, err := h.db.VillageProject.Get(c.Request.Context(), projectID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "project not found", "data": nil})
		return
	}
	if project.Status != "active" {
		c.JSON(http.StatusConflict, gin.H{"code": 409, "message": "project is not active", "data": nil})
		return
	}

	// Update requirements[0].Current for coins requirement
	reqs := project.Requirements
	contributed := req.Coins
	for i := range reqs {
		if reqs[i].Type == "coins" {
			reqs[i].Current += contributed
			if reqs[i].Current > reqs[i].Required {
				contributed = reqs[i].Required - (reqs[i].Current - contributed)
				reqs[i].Current = reqs[i].Required
			}
			break
		}
	}

	// Check if all requirements are met
	allMet := true
	for _, r := range reqs {
		if r.Current < r.Required {
			allMet = false
			break
		}
	}
	status := "active"
	if allMet {
		status = "completed"
	}

	_, err = h.db.VillageProject.UpdateOne(project).
		SetRequirements(reqs).
		SetStatus(status).
		Save(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "project update failed", "data": nil})
		return
	}

	// Deduct coins from user
	_, _ = h.db.User.UpdateOne(u).AddCoins(-contributed).Save(c.Request.Context())

	// Increment member + village contribution
	_, _ = h.db.VillageMember.UpdateOne(mem).AddContribution(contributed).Save(c.Request.Context())
	village, _ := h.db.Village.Get(c.Request.Context(), villageID)
	if village != nil {
		_, _ = h.db.Village.UpdateOne(village).AddContribution(contributed).Save(c.Request.Context())
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "contributed", "data": gin.H{
		"coins":  contributed,
		"status": status,
	}})
}

// ─────────────────────────────────────────────────────────────────────────────
// T-065: 村庄等级与共建任务
// ─────────────────────────────────────────────────────────────────────────────

// projectDTO converts a VillageProject to a response map including progress.
func projectDTO(p *ent.VillageProject) map[string]any {
	// Compute aggregate progress: sum of Current values across all requirements.
	var progress, target int64
	for _, r := range p.Requirements {
		progress += r.Current
		target += r.Required
	}
	dto := map[string]any{
		"id":           p.ID,
		"villageId":    p.VillageID,
		"name":         p.Name,
		"type":         p.Type,
		"requirements": p.Requirements,
		"status":       p.Status,
		"progress":     progress,
		"target":       target,
		"startedAt":    p.StartedAt.Format(time.RFC3339),
	}
	if p.CompletedAt != nil {
		dto["completedAt"] = p.CompletedAt.Format(time.RFC3339)
	}
	return dto
}

// MyProjects handles GET /api/v1/villages/projects
// Returns all VillageProjects for the caller's village with progress info.
func (h *VillageHandler) MyProjects(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "unauthorized", "data": nil})
		return
	}

	mem, err := h.db.VillageMember.Query().
		Where(entvm.UserID(userID)).
		Only(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"code": 409, "message": "not in a village", "data": nil})
		return
	}

	projects, err := h.db.VillageProject.Query().
		Where(entvp.VillageID(mem.VillageID)).
		All(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "query failed", "data": nil})
		return
	}

	result := make([]map[string]any, len(projects))
	for i, p := range projects {
		result[i] = projectDTO(p)
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": result})
}

// contributeResourceReq is the request body for POST /api/v1/villages/projects/:id/contribute
type contributeResourceReq struct {
	Resource string `json:"resource" binding:"required"`
	Amount   int64  `json:"amount"   binding:"min=1"`
}

// ContributeResource handles POST /api/v1/villages/projects/:id/contribute
// Deducts materials from inventory, updates project progress, and handles
// project completion + village level-up when all requirements are met.
func (h *VillageHandler) ContributeResource(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "unauthorized", "data": nil})
		return
	}

	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid project id", "data": nil})
		return
	}

	var req contributeResourceReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error(), "data": nil})
		return
	}

	ctx := c.Request.Context()

	// Verify membership and get villageID
	mem, err := h.db.VillageMember.Query().
		Where(entvm.UserID(userID)).
		Only(ctx)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"code": 403, "message": "not in a village", "data": nil})
		return
	}

	// Verify the project belongs to the caller's village
	project, err := h.db.VillageProject.Get(ctx, projectID)
	if err != nil {
		if ent.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "project not found", "data": nil})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "project query failed", "data": nil})
		return
	}
	if project.VillageID != mem.VillageID {
		c.JSON(http.StatusForbidden, gin.H{"code": 403, "message": "project does not belong to your village", "data": nil})
		return
	}
	if project.Status != "active" {
		c.JSON(http.StatusConflict, gin.H{"code": 409, "message": "project is not active", "data": nil})
		return
	}

	// Check that the caller has sufficient materials in inventory
	inv, err := h.db.InventoryItem.Query().
		Where(
			entinv.UserID(userID),
			entinv.ItemID(req.Resource),
			entinv.ItemType("material"),
		).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "material not in inventory", "data": nil})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "inventory query failed", "data": nil})
		return
	}
	if inv.Quantity < req.Amount {
		c.JSON(http.StatusConflict, gin.H{"code": 409, "message": "not enough materials", "data": nil})
		return
	}

	// Update project requirements progress.
	// Look for a material requirement matching req.Resource; if none found, use
	// the first generic "material" requirement slot (or append progress as raw
	// contribution to the first requirement that still has capacity).
	reqs := project.Requirements
	actualContributed := req.Amount
	matched := false
	for i := range reqs {
		if reqs[i].Type == "material" && (reqs[i].ItemID == req.Resource || reqs[i].ItemID == "") {
			remaining := reqs[i].Required - reqs[i].Current
			if remaining <= 0 {
				continue
			}
			if actualContributed > remaining {
				actualContributed = remaining
			}
			reqs[i].Current += actualContributed
			matched = true
			break
		}
	}
	if !matched {
		// No matching slot: still record the contribution but cap at remaining
		// budget of the first incomplete requirement of any type.
		for i := range reqs {
			remaining := reqs[i].Required - reqs[i].Current
			if remaining > 0 {
				if actualContributed > remaining {
					actualContributed = remaining
				}
				reqs[i].Current += actualContributed
				matched = true
				break
			}
		}
	}
	if !matched {
		c.JSON(http.StatusConflict, gin.H{"code": 409, "message": "all project requirements already fulfilled", "data": nil})
		return
	}

	// Check if all requirements are now met
	allMet := true
	for _, r := range reqs {
		if r.Current < r.Required {
			allMet = false
			break
		}
	}
	newStatus := "active"
	var completedAt *time.Time
	if allMet {
		newStatus = "completed"
		now := time.Now()
		completedAt = &now
	}

	// Persist project update
	projectUpd := h.db.VillageProject.UpdateOne(project).
		SetRequirements(reqs).
		SetStatus(newStatus)
	if completedAt != nil {
		projectUpd = projectUpd.SetCompletedAt(*completedAt)
	}
	if _, err := projectUpd.Save(ctx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "project update failed", "data": nil})
		return
	}

	// Deduct materials from inventory
	if _, err := h.db.InventoryItem.UpdateOne(inv).AddQuantity(-actualContributed).Save(ctx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "inventory update failed", "data": nil})
		return
	}

	// Record contribution entry
	h.db.VillageProjectContribution.Create(). //nolint
						SetProjectID(project.ID).
						SetUserID(userID).
						SetResourceType("material").
						SetItemID(req.Resource).
						SetQuantity(actualContributed).
						Exec(ctx)

	// Update member and village contribution counters
	_, _ = h.db.VillageMember.UpdateOne(mem).AddContribution(actualContributed).Save(ctx)

	village, err := h.db.Village.Get(ctx, mem.VillageID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "village query failed", "data": nil})
		return
	}
	newVillageContribution := village.Contribution + actualContributed
	newLevel := village.Level

	// Check level-up thresholds (levels 1-5).
	// villageLevelThresholds[i] is the cumulative contribution required to reach level i+2:
	//   [0]=500  → reach level 2
	//   [1]=2000 → reach level 3
	//   [2]=5000 → reach level 4
	//   [3]=15000→ reach level 5
	//   [4]=50000→ (unused; village cap is 5)
	// The threshold to advance FROM level L to L+1 lives at index L-1.
	for newLevel < 5 {
		if newVillageContribution >= villageLevelThresholds[newLevel-1] {
			newLevel++
		} else {
			break
		}
	}

	_, _ = h.db.Village.UpdateOne(village).
		AddContribution(actualContributed).
		SetLevel(newLevel).
		Save(ctx)

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "contributed", "data": gin.H{
		"resource":      req.Resource,
		"contributed":   actualContributed,
		"projectStatus": newStatus,
		"villageLevel":  newLevel,
	}})
}

// callerMembershipVillageID is a helper used by T-065 handlers to get the
// user's village membership in a single call.
func (h *VillageHandler) callerMembershipVillageID(c *gin.Context, userID uuid.UUID) (uuid.UUID, error) {
	mem, err := h.db.VillageMember.Query().
		Where(entvm.UserID(userID)).
		Only(c.Request.Context())
	if err != nil {
		return uuid.UUID{}, err
	}
	return mem.VillageID, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// T-080: 跨村社交与探索
// ─────────────────────────────────────────────────────────────────────────────

// specialtyFromName derives a deterministic specialty label from the village name
// by summing character rune values and indexing into a fixed list.
func specialtyFromName(name string) string {
	specialties := []string{"粮食", "畜牧", "园艺", "香料"}
	var sum int
	for _, r := range name {
		sum += int(r)
	}
	return specialties[sum%len(specialties)]
}

// Explore handles GET /api/v1/villages/explore
// Lists up to 20 villages (excluding caller's own village) ordered by level desc,
// member_count desc, enriched with a computed specialty field.
func (h *VillageHandler) Explore(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "unauthorized", "data": nil})
		return
	}

	ctx := c.Request.Context()

	// Look up caller's village (may not be in one — that's OK, exclude nothing in that case).
	var excludeID *uuid.UUID
	mem, err := h.db.VillageMember.Query().
		Where(entvm.UserID(userID)).
		Only(ctx)
	if err == nil {
		excludeID = &mem.VillageID
	}

	q := h.db.Village.Query().
		Order(
			ent.Desc(entvillage.FieldLevel),
			ent.Desc(entvillage.FieldMemberCount),
		).
		Limit(20)
	if excludeID != nil {
		q = q.Where(entvillage.IDNEQ(*excludeID))
	}

	villages, err := q.All(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "query failed", "data": nil})
		return
	}

	result := make([]map[string]any, len(villages))
	for i, v := range villages {
		result[i] = map[string]any{
			"id":          v.ID,
			"name":        v.Name,
			"level":       v.Level,
			"memberCount": v.MemberCount,
			"description": v.Name + "村", // descriptive fallback
			"specialty":   specialtyFromName(v.Name),
		}
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": result})
}

// ListMembers handles GET /api/v1/villages/:id/members
// Returns up to 50 members of the given village with basic user info.
func (h *VillageHandler) ListMembers(c *gin.Context) {
	villageID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid id", "data": nil})
		return
	}

	ctx := c.Request.Context()

	// Verify the village exists.
	_, err = h.db.Village.Get(ctx, villageID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "village not found", "data": nil})
		return
	}

	// Fetch membership rows for this village.
	members, err := h.db.VillageMember.Query().
		Where(entvm.VillageID(villageID)).
		Limit(50).
		All(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "query failed", "data": nil})
		return
	}

	// Collect user IDs then fetch user records in one query.
	userIDs := make([]uuid.UUID, len(members))
	for i, m := range members {
		userIDs[i] = m.UserID
	}

	users, err := h.db.User.Query().
		Where(entuser.IDIn(userIDs...)).
		All(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "user query failed", "data": nil})
		return
	}

	result := make([]map[string]any, len(users))
	for i, u := range users {
		result[i] = map[string]any{
			"id":           u.ID,
			"nickname":     u.Nickname,
			"avatarUrl":    u.Avatar,
			"onlineStatus": false, // mock: always offline
		}
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": result})
}

