package handler

import (
	"bytes"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/liukai/farmer/server/ent"
	entfriend "github.com/liukai/farmer/server/ent/friend"
	entfr "github.com/liukai/farmer/server/ent/friendrequest"
	entuser "github.com/liukai/farmer/server/ent/user"
)

// FriendHandler groups friend-management route handlers.
type FriendHandler struct {
	db *ent.Client
}

// NewFriendHandler constructs a FriendHandler.
func NewFriendHandler(db *ent.Client) *FriendHandler { return &FriendHandler{db: db} }

// normalizeFriendIDs returns (a, b) with a ≤ b (byte-lexicographic) for the Friend table.
func normalizeFriendIDs(x, y uuid.UUID) (uuid.UUID, uuid.UUID) {
	if bytes.Compare(x[:], y[:]) <= 0 {
		return x, y
	}
	return y, x
}

// ListFriends handles GET /api/v1/friends
// Returns all accepted friends of the current user with basic profile info.
func (h *FriendHandler) ListFriends(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "unauthorized", "data": nil})
		return
	}
	ctx := c.Request.Context()
	friends, err := h.db.Friend.Query().
		Where(entfriend.Or(entfriend.UserAID(userID), entfriend.UserBID(userID))).
		WithUserA().
		WithUserB().
		Order(ent.Desc(entfriend.FieldCreatedAt)).
		All(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "query failed", "data": nil})
		return
	}

	result := make([]map[string]any, 0, len(friends))
	for _, f := range friends {
		// Resolve the "other" user in the pair.
		var other *ent.User
		if f.Edges.UserA != nil && f.Edges.UserA.ID != userID {
			other = f.Edges.UserA
		} else if f.Edges.UserB != nil {
			other = f.Edges.UserB
		}
		if other == nil {
			continue
		}
		result = append(result, map[string]any{
			"friendId":  f.ID,
			"userId":    other.ID,
			"nickname":  other.Nickname,
			"avatar":    other.Avatar,
			"level":     other.Level,
			"createdAt": f.CreatedAt.Format(time.RFC3339),
		})
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": result})
}

// ListRequests handles GET /api/v1/friends/requests
// Returns pending friend requests sent to the current user.
func (h *FriendHandler) ListRequests(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "unauthorized", "data": nil})
		return
	}
	reqs, err := h.db.FriendRequest.Query().
		Where(entfr.ToUserID(userID), entfr.Status("pending")).
		WithFromUser().
		Order(ent.Desc(entfr.FieldCreatedAt)).
		All(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "query failed", "data": nil})
		return
	}
	result := make([]map[string]any, len(reqs))
	for i, r := range reqs {
		entry := map[string]any{
			"id":      r.ID,
			"message": r.Message,
			"status":  r.Status,
		}
		if r.Edges.FromUser != nil {
			entry["from"] = map[string]any{
				"id": r.Edges.FromUser.ID, "nickname": r.Edges.FromUser.Nickname,
				"avatar": r.Edges.FromUser.Avatar, "level": r.Edges.FromUser.Level,
			}
		}
		result[i] = entry
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": result})
}

type addFriendReq struct {
	TargetUserID string  `json:"targetUserId" binding:"required"`
	Message      *string `json:"message"`
}

// AddFriend handles POST /api/v1/friends/requests
// Sends a friend request; idempotent (returns existing pending request if exists).
func (h *FriendHandler) AddFriend(c *gin.Context) {
	fromID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "unauthorized", "data": nil})
		return
	}
	var req addFriendReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error(), "data": nil})
		return
	}
	toID, err := uuid.Parse(req.TargetUserID)
	if err != nil || fromID == toID {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid targetUserId", "data": nil})
		return
	}
	ctx := c.Request.Context()

	// Check target user exists.
	if _, err := h.db.User.Query().Where(entuser.ID(toID)).Only(ctx); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "user not found", "data": nil})
		return
	}

	// Check not already friends.
	aID, bID := normalizeFriendIDs(fromID, toID)
	existing, _ := h.db.Friend.Query().
		Where(entfriend.UserAID(aID), entfriend.UserBID(bID)).
		Only(ctx)
	if existing != nil {
		c.JSON(http.StatusOK, gin.H{"code": 0, "message": "already friends", "data": nil})
		return
	}

	// Check for existing pending request.
	existingReq, _ := h.db.FriendRequest.Query().
		Where(entfr.FromUserID(fromID), entfr.ToUserID(toID), entfr.Status("pending")).
		Only(ctx)
	if existingReq != nil {
		c.JSON(http.StatusOK, gin.H{"code": 0, "message": "request already sent", "data": gin.H{"id": existingReq.ID}})
		return
	}

	mut := h.db.FriendRequest.Create().SetFromUserID(fromID).SetToUserID(toID)
	if req.Message != nil {
		mut = mut.SetMessage(*req.Message)
	}
	fr, err := mut.Save(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "create failed", "data": nil})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": gin.H{"id": fr.ID, "status": fr.Status}})
}

// AcceptRequest handles POST /api/v1/friends/requests/:id/accept
// Creates the Friend relationship and marks the request as accepted.
func (h *FriendHandler) AcceptRequest(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "unauthorized", "data": nil})
		return
	}
	reqID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid id", "data": nil})
		return
	}
	ctx := c.Request.Context()
	fr, err := h.db.FriendRequest.Get(ctx, reqID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "request not found", "data": nil})
		return
	}
	if fr.ToUserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"code": 403, "message": "forbidden", "data": nil})
		return
	}
	if fr.Status != "pending" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "request already handled", "data": nil})
		return
	}

	// Create Friend entry (normalise IDs).
	aID, bID := normalizeFriendIDs(fr.FromUserID, fr.ToUserID)
	_, err = h.db.Friend.Create().SetUserAID(aID).SetUserBID(bID).Save(ctx)
	if err != nil && !ent.IsConstraintError(err) {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "create friend failed", "data": nil})
		return
	}

	// Mark request accepted.
	h.db.FriendRequest.UpdateOneID(reqID).SetStatus("accepted").Exec(ctx)

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": gin.H{"status": "accepted"}})
}

// RejectRequest handles POST /api/v1/friends/requests/:id/reject
func (h *FriendHandler) RejectRequest(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "unauthorized", "data": nil})
		return
	}
	reqID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid id", "data": nil})
		return
	}
	ctx := c.Request.Context()
	fr, err := h.db.FriendRequest.Get(ctx, reqID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "request not found", "data": nil})
		return
	}
	if fr.ToUserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"code": 403, "message": "forbidden", "data": nil})
		return
	}
	h.db.FriendRequest.UpdateOneID(reqID).SetStatus("rejected").Exec(ctx)
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": gin.H{"status": "rejected"}})
}
