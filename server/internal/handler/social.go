package handler

import (
	"net/http"
	"strconv"
	"time"

	"entgo.io/ent/dialect/sql"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/liukai/farmer/server/ent"
	"github.com/liukai/farmer/server/ent/activitylog"
	entlike "github.com/liukai/farmer/server/ent/activitylike"
	entgift "github.com/liukai/farmer/server/ent/gift"
	enthr "github.com/liukai/farmer/server/ent/helprequest"
	entinv "github.com/liukai/farmer/server/ent/inventoryitem"
	"github.com/liukai/farmer/server/internal/service"
)

// SocialHandler groups social / activity-feed route handlers.
type SocialHandler struct {
	db *ent.Client
}

// NewSocialHandler constructs a SocialHandler.
func NewSocialHandler(db *ent.Client) *SocialHandler { return &SocialHandler{db: db} }

// relationshipDTO maps a relationship to a response object from the caller's perspective.
func relationshipDTO(rel *ent.Relationship, callerID uuid.UUID) map[string]any {
	otherID := rel.UserBID
	if rel.UserBID == callerID {
		otherID = rel.UserAID
	}
	return map[string]any{
		"id":            rel.ID,
		"otherUserId":   otherID,
		"affinity":      rel.Affinity,
		"level":         rel.Level,
		"lastInteractAt": rel.LastInteractAt.Format(time.RFC3339),
	}
}

// GetRelationships handles GET /api/v1/social/relationships
// Returns all relationships for the current user ordered by affinity desc.
func (h *SocialHandler) GetRelationships(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "unauthorized", "data": nil})
		return
	}

	rels, err := service.ListRelationships(c.Request.Context(), h.db, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "query failed", "data": nil})
		return
	}

	result := make([]map[string]any, len(rels))
	for i, r := range rels {
		result[i] = relationshipDTO(r, userID)
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": result})
}

// GetFeed handles GET /api/v1/social/feed
// Query params:
//
//	userId (optional) — filter to a specific user's activities; defaults to caller
//	limit  (optional) — max records, default 20, capped at 50
//	offset (optional) — pagination offset
func (h *SocialHandler) GetFeed(c *gin.Context) {
	callerID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "unauthorized", "data": nil})
		return
	}

	// Resolve target user (default: caller's own feed)
	targetID := callerID
	if raw := c.Query("userId"); raw != "" {
		if id, err := uuid.Parse(raw); err == nil {
			targetID = id
		}
	}

	limit := 20
	if l, err := strconv.Atoi(c.DefaultQuery("limit", "20")); err == nil && l > 0 {
		if l > 50 {
			l = 50
		}
		limit = l
	}
	offset := 0
	if o, err := strconv.Atoi(c.DefaultQuery("offset", "0")); err == nil && o >= 0 {
		offset = o
	}

	ctx := c.Request.Context()
	logs, err := h.db.ActivityLog.Query().
		Where(activitylog.UserID(targetID)).
		Order(activitylog.ByCreatedAt(sql.OrderDesc())).
		WithAuthor().
		Limit(limit).
		Offset(offset).
		All(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "query failed", "data": nil})
		return
	}

	result := make([]map[string]any, len(logs))
	for i, log := range logs {
		entry := map[string]any{
			"id":        log.ID,
			"type":      log.Type,
			"content":   log.Content,
			"meta":      log.Meta,
			"createdAt": log.CreatedAt.Format(time.RFC3339),
		}
		if log.Edges.Author != nil {
			entry["author"] = map[string]any{
				"id":       log.Edges.Author.ID,
				"nickname": log.Edges.Author.Nickname,
				"avatar":   log.Edges.Author.Avatar,
				"level":    log.Edges.Author.Level,
			}
		}
		result[i] = entry
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": result})
}

// LikeActivity handles POST /api/v1/social/activities/:id/like
func (h *SocialHandler) LikeActivity(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "unauthorized", "data": nil})
		return
	}
	activityID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid activity id", "data": nil})
		return
	}
	ctx := c.Request.Context()

	activity, err := h.db.ActivityLog.Get(ctx, activityID)
	if err != nil {
		if ent.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "activity not found", "data": nil})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "query failed", "data": nil})
		}
		return
	}

	// Check if already liked.
	_, err = h.db.ActivityLike.Query().
		Where(entlike.ActivityID(activityID), entlike.UserID(userID)).
		Only(ctx)
	if err == nil {
		// Already liked.
		c.JSON(http.StatusOK, gin.H{"code": 0, "message": "already liked", "data": gin.H{
			"liked":     true,
			"likeCount": activity.LikeCount,
		}})
		return
	}
	if !ent.IsNotFound(err) {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "query failed", "data": nil})
		return
	}

	// Create the like record.
	_, err = h.db.ActivityLike.Create().
		SetActivityID(activityID).
		SetUserID(userID).
		Save(ctx)
	if err != nil {
		if ent.IsConstraintError(err) {
			// Race condition: like was created concurrently.
			c.JSON(http.StatusOK, gin.H{"code": 0, "message": "already liked", "data": gin.H{
				"liked":     true,
				"likeCount": activity.LikeCount,
			}})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "create failed", "data": nil})
		}
		return
	}

	// Increment like_count.
	if err := h.db.ActivityLog.UpdateOneID(activityID).AddLikeCount(1).Exec(ctx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "update failed", "data": nil})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": gin.H{
		"liked":     true,
		"likeCount": activity.LikeCount + 1,
	}})
}

type commentReq struct {
	Content string `json:"content" binding:"required,max=200"`
}

// CommentActivity handles POST /api/v1/social/activities/:id/comment
func (h *SocialHandler) CommentActivity(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "unauthorized", "data": nil})
		return
	}
	activityID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid activity id", "data": nil})
		return
	}
	var req commentReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error(), "data": nil})
		return
	}
	ctx := c.Request.Context()

	// Verify the activity exists.
	_, err = h.db.ActivityLog.Get(ctx, activityID)
	if err != nil {
		if ent.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "activity not found", "data": nil})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "query failed", "data": nil})
		}
		return
	}

	// Create the comment.
	comment, err := h.db.ActivityComment.Create().
		SetActivityID(activityID).
		SetAuthorID(userID).
		SetContent(req.Content).
		Save(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "create failed", "data": nil})
		return
	}

	// Increment comment_count.
	if err := h.db.ActivityLog.UpdateOneID(activityID).AddCommentCount(1).Exec(ctx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "update failed", "data": nil})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": gin.H{
		"id":        comment.ID,
		"content":   comment.Content,
		"userId":    userID,
		"createdAt": comment.CreatedAt.Format(time.RFC3339),
	}})
}

// ── T-060: 赠礼系统 ──────────────────────────────────────────────────────────

// giftAffinityGain computes affinity delta based on item rarity.
// Animal products give +12, default items give +10.
func giftAffinityGain(itemID string) int {
	animalProducts := map[string]bool{"egg": true, "milk": true, "wool": true, "honey": true, "rabbit_fur": true}
	if animalProducts[itemID] {
		return 12
	}
	return 10
}

type sendGiftReq struct {
	TargetUserID string `json:"targetUserId" binding:"required"`
	ItemID       string `json:"itemId" binding:"required"`
	Quantity     int    `json:"quantity" binding:"required,min=1"`
	Message      string `json:"message"`
}

// SendGift handles POST /api/v1/social/gift
// Transfers items from sender to receiver and increases affinity.
func (h *SocialHandler) SendGift(c *gin.Context) {
	senderID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "unauthorized", "data": nil})
		return
	}
	var req sendGiftReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error(), "data": nil})
		return
	}
	receiverID, err := uuid.Parse(req.TargetUserID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid targetUserId", "data": nil})
		return
	}
	if senderID == receiverID {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "cannot gift yourself", "data": nil})
		return
	}

	ctx := c.Request.Context()

	// Deduct from sender's inventory.
	senderItem, err := h.db.InventoryItem.Query().
		Where(entinv.UserID(senderID), entinv.ItemID(req.ItemID)).
		Only(ctx)
	if err != nil || senderItem.Quantity < int64(req.Quantity) {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "insufficient items", "data": nil})
		return
	}
	if err := h.db.InventoryItem.UpdateOneID(senderItem.ID).AddQuantity(-int64(req.Quantity)).Exec(ctx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "deduct failed", "data": nil})
		return
	}

	// Add to receiver's inventory.
	recvItem, err := h.db.InventoryItem.Query().
		Where(entinv.UserID(receiverID), entinv.ItemID(req.ItemID)).
		Only(ctx)
	if ent.IsNotFound(err) {
		_, err = h.db.InventoryItem.Create().
			SetUserID(receiverID).
			SetItemID(req.ItemID).
			SetItemType(senderItem.ItemType).
			SetQuantity(int64(req.Quantity)).
			Save(ctx)
	} else if err == nil {
		err = h.db.InventoryItem.UpdateOneID(recvItem.ID).AddQuantity(int64(req.Quantity)).Exec(ctx)
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "credit failed", "data": nil})
		return
	}

	// Record the gift.
	affinityDelta := giftAffinityGain(req.ItemID)
	mut := h.db.Gift.Create().
		SetSenderID(senderID).
		SetReceiverID(receiverID).
		SetItemID(req.ItemID).
		SetQuantity(req.Quantity).
		SetAffinityGained(affinityDelta).
		SetIsAgentAction(false)
	if req.Message != "" {
		mut = mut.SetMessage(req.Message)
	}
	_, _ = mut.Save(ctx)

	// Update affinity.
	rel, _ := service.AddAffinity(ctx, h.db, senderID, receiverID, affinityDelta)

	newAffinity := 0
	if rel != nil {
		newAffinity = rel.Affinity
	}
	c.JSON(http.StatusOK, gin.H{
		"code": 0, "message": "ok",
		"data": gin.H{"affinityGained": affinityDelta, "newAffinity": newAffinity},
	})
}

// ListGifts handles GET /api/v1/social/gifts
// Returns gifts received by the current user (most recent first, limit 20).
func (h *SocialHandler) ListGifts(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "unauthorized", "data": nil})
		return
	}
	gifts, err := h.db.Gift.Query().
		Where(entgift.ReceiverID(userID)).
		Order(ent.Desc(entgift.FieldCreatedAt)).
		Limit(20).
		All(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "query failed", "data": nil})
		return
	}
	result := make([]map[string]any, len(gifts))
	for i, g := range gifts {
		result[i] = map[string]any{
			"id": g.ID, "senderId": g.SenderID, "itemId": g.ItemID,
			"quantity": g.Quantity, "message": g.Message,
			"affinityGained": g.AffinityGained, "createdAt": g.CreatedAt.Format(time.RFC3339),
		}
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": result})
}

// ── T-061: 求助系统 ──────────────────────────────────────────────────────────

type helpRequestReq struct {
	HelperUserID string  `json:"helperUserId" binding:"required"`
	ResourceType string  `json:"resourceType" binding:"required"` // seed/material/coins
	ResourceID   *string `json:"resourceId"`
	Quantity     int     `json:"quantity" binding:"required,min=1"`
	Message      string  `json:"message"`
}

// CreateHelpRequest handles POST /api/v1/social/help-request
func (h *SocialHandler) CreateHelpRequest(c *gin.Context) {
	requesterID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "unauthorized", "data": nil})
		return
	}
	var req helpRequestReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error(), "data": nil})
		return
	}
	helperID, err := uuid.Parse(req.HelperUserID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid helperUserId", "data": nil})
		return
	}
	ctx := c.Request.Context()
	mut := h.db.HelpRequest.Create().
		SetRequesterID(requesterID).
		SetHelperID(helperID).
		SetResourceType(req.ResourceType).
		SetQuantity(req.Quantity)
	if req.ResourceID != nil {
		mut = mut.SetResourceID(*req.ResourceID)
	}
	if req.Message != "" {
		mut = mut.SetMessage(req.Message)
	}
	hr, err := mut.Save(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "create failed", "data": nil})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": gin.H{"id": hr.ID, "status": hr.Status}})
}

type helpRespondReq struct {
	RequestID string `json:"requestId" binding:"required"`
	Accept    bool   `json:"accept"`
}

// RespondHelpRequest handles POST /api/v1/social/help-respond
// If accepted, transfers the requested resource and grants +15 affinity to the helper.
func (h *SocialHandler) RespondHelpRequest(c *gin.Context) {
	helperID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "unauthorized", "data": nil})
		return
	}
	var req helpRespondReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error(), "data": nil})
		return
	}
	hrID, err := uuid.Parse(req.RequestID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid requestId", "data": nil})
		return
	}
	ctx := c.Request.Context()
	hr, err := h.db.HelpRequest.Get(ctx, hrID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "request not found", "data": nil})
		return
	}
	if hr.HelperID != helperID {
		c.JSON(http.StatusForbidden, gin.H{"code": 403, "message": "forbidden", "data": nil})
		return
	}
	if hr.Status != "pending" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "request already handled", "data": nil})
		return
	}

	now := time.Now()
	status := "rejected"
	if req.Accept {
		status = "accepted"
		// Transfer resource: coins or item.
		if hr.ResourceType == "coins" {
			helperUser, err := h.db.User.Get(ctx, helperID)
			if err == nil && helperUser.Coins >= int64(hr.Quantity) {
				h.db.User.UpdateOneID(helperID).AddCoins(-int64(hr.Quantity)).Exec(ctx)
				h.db.User.UpdateOneID(hr.RequesterID).AddCoins(int64(hr.Quantity)).Exec(ctx)
			}
		} else if hr.ResourceID != nil {
			// Transfer inventory item from helper to requester.
			helperItem, err := h.db.InventoryItem.Query().
				Where(entinv.UserID(helperID), entinv.ItemID(*hr.ResourceID)).
				Only(ctx)
			if err == nil && helperItem.Quantity >= int64(hr.Quantity) {
				h.db.InventoryItem.UpdateOneID(helperItem.ID).AddQuantity(-int64(hr.Quantity)).Exec(ctx)
				// Upsert requester inventory.
				reqItem, err := h.db.InventoryItem.Query().
					Where(entinv.UserID(hr.RequesterID), entinv.ItemID(*hr.ResourceID)).
					Only(ctx)
				if ent.IsNotFound(err) {
					h.db.InventoryItem.Create().
						SetUserID(hr.RequesterID).
						SetItemID(*hr.ResourceID).
						SetItemType(helperItem.ItemType).
						SetQuantity(int64(hr.Quantity)).
						Save(ctx)
				} else if err == nil {
					h.db.InventoryItem.UpdateOneID(reqItem.ID).AddQuantity(int64(hr.Quantity)).Exec(ctx)
				}
			}
		}
		// Helper gains +15 affinity (reward for helping).
		service.AddAffinity(ctx, h.db, helperID, hr.RequesterID, 15)
	}

	h.db.HelpRequest.UpdateOneID(hrID).
		SetStatus(status).
		SetRespondedAt(now).
		Exec(ctx)

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": gin.H{"status": status}})
}

// ListHelpRequests handles GET /api/v1/social/help-requests
// Returns pending help requests addressed to the current user.
func (h *SocialHandler) ListHelpRequests(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "unauthorized", "data": nil})
		return
	}
	hrs, err := h.db.HelpRequest.Query().
		Where(enthr.HelperID(userID), enthr.Status("pending")).
		Order(ent.Desc(enthr.FieldCreatedAt)).
		Limit(20).
		All(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "query failed", "data": nil})
		return
	}
	result := make([]map[string]any, len(hrs))
	for i, hr := range hrs {
		result[i] = map[string]any{
			"id": hr.ID, "requesterId": hr.RequesterID,
			"resourceType": hr.ResourceType, "resourceId": hr.ResourceID,
			"quantity": hr.Quantity, "message": hr.Message,
			"status": hr.Status, "createdAt": hr.CreatedAt.Format(time.RFC3339),
		}
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": result})
}
