package handler

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/liukai/farmer/server/ent"
	"github.com/liukai/farmer/server/ent/activitylog"
	entuser "github.com/liukai/farmer/server/ent/user"
	"github.com/liukai/farmer/server/internal/service"
)

// ShareHandler handles WeChat sharing and invite reward endpoints.
type ShareHandler struct {
	db *ent.Client
}

// NewShareHandler constructs a ShareHandler.
func NewShareHandler(db *ent.Client) *ShareHandler {
	return &ShareHandler{db: db}
}

// recordShareRequest is the body for POST /api/v1/share/record.
type recordShareRequest struct {
	ShareType string `json:"shareType"` // "farm" | "achievement" | "ranking"
	TargetID  string `json:"targetId"`  // optional
}

// RecordShare handles POST /api/v1/share/record
// Records a share action and grants a daily first-share reward of Stamina +20.
func (h *ShareHandler) RecordShare(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "unauthorized", "data": nil})
		return
	}

	var req recordShareRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid request body", "data": nil})
		return
	}

	ctx := c.Request.Context()
	todayStart := startOfDay(time.Now())

	// Check if user already shared today.
	existingCount, err := h.db.ActivityLog.Query().
		Where(
			activitylog.UserID(userID),
			activitylog.Type("share"),
			activitylog.CreatedAtGTE(todayStart),
		).
		Count(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "query failed", "data": nil})
		return
	}

	firstShareToday := existingCount == 0
	staminaGranted := 0

	// Fetch the current user.
	u, err := h.db.User.Query().Where(entuser.ID(userID)).Only(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "user not found", "data": nil})
		return
	}

	if firstShareToday {
		// Grant +20 stamina (capped at max).
		newStamina := u.Stamina + 20
		if newStamina > u.MaxStamina {
			newStamina = u.MaxStamina
		}
		staminaGranted = newStamina - u.Stamina

		u, err = h.db.User.UpdateOne(u).
			SetStamina(newStamina).
			SetStaminaUpdatedAt(time.Now()).
			Save(ctx)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "reward failed", "data": nil})
			return
		}
	}

	// Record the share activity log regardless (for history).
	meta := map[string]any{
		"shareType": req.ShareType,
	}
	if req.TargetID != "" {
		meta["targetId"] = req.TargetID
	}
	content := fmt.Sprintf("分享了%s", shareTypeLabel(req.ShareType))
	if err := service.CreateActivityLog(ctx, h.db, userID, "share", content, meta); err != nil {
		// Non-fatal: reward already applied, just log the failure silently.
		_ = err
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "ok",
		"data": gin.H{
			"firstShareToday": firstShareToday,
			"staminaGranted":  staminaGranted,
			"totalStamina":    u.Stamina,
		},
	})
}

// shareTypeLabel returns a human-readable label for a share type.
func shareTypeLabel(t string) string {
	switch t {
	case "farm":
		return "农场"
	case "achievement":
		return "成就"
	case "ranking":
		return "排行榜"
	default:
		return "内容"
	}
}

// GetInviteCode handles GET /api/v1/share/invite-code
// Returns the user's invite code (their UUID in short hex form) and a deeplink.
func (h *ShareHandler) GetInviteCode(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "unauthorized", "data": nil})
		return
	}

	// Use the raw UUID string as the invite code (universally unique, easy to validate).
	inviteCode := userID.String()
	deeplink := fmt.Sprintf("farmer://invite?code=%s", inviteCode)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "ok",
		"data": gin.H{
			"inviteCode": inviteCode,
			"deeplink":   deeplink,
		},
	})
}

// claimInviteRequest is the body for POST /api/v1/share/invite-reward.
type claimInviteRequest struct {
	InviteCode string `json:"inviteCode"`
}

// ClaimInviteReward handles POST /api/v1/share/invite-reward
// Grants coins to both the inviter and the new user upon first-time invite claim.
func (h *ShareHandler) ClaimInviteReward(c *gin.Context) {
	newUserID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "unauthorized", "data": nil})
		return
	}

	var req claimInviteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid request body", "data": nil})
		return
	}

	// Parse inviter UUID from invite code.
	inviterID, err := uuid.Parse(req.InviteCode)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid invite code", "data": nil})
		return
	}

	// Self-invite is not allowed.
	if inviterID == newUserID {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "cannot use your own invite code", "data": nil})
		return
	}

	ctx := c.Request.Context()

	// Verify inviter exists.
	inviter, err := h.db.User.Query().Where(entuser.ID(inviterID)).Only(ctx)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "inviter not found", "data": nil})
		return
	}

	// Check if current user was already invited (has an "invited_by" activity log).
	alreadyInvitedCount, err := h.db.ActivityLog.Query().
		Where(
			activitylog.UserID(newUserID),
			activitylog.Type("invited_by"),
		).
		Count(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "query failed", "data": nil})
		return
	}

	if alreadyInvitedCount > 0 {
		c.JSON(http.StatusOK, gin.H{
			"code":    0,
			"message": "ok",
			"data": gin.H{
				"success": false,
				"message": "邀请奖励已领取过，每个用户只能使用一次邀请码",
			},
		})
		return
	}

	// Fetch current user (the new invitee).
	newUser, err := h.db.User.Query().Where(entuser.ID(newUserID)).Only(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "user not found", "data": nil})
		return
	}

	// Grant inviter +50 coins.
	if _, err := h.db.User.UpdateOne(inviter).AddCoins(50).Save(ctx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "reward failed", "data": nil})
		return
	}

	// Grant new user +30 coins.
	if _, err := h.db.User.UpdateOne(newUser).AddCoins(30).Save(ctx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "reward failed", "data": nil})
		return
	}

	// Log invite for new user.
	_ = service.CreateActivityLog(ctx, h.db, newUserID, "invited_by",
		"通过邀请码加入，获得30金币奖励",
		map[string]any{"inviterID": inviterID.String()},
	)

	// Log invite reward for inviter.
	_ = service.CreateActivityLog(ctx, h.db, inviterID, "invite_reward",
		"成功邀请好友加入，获得50金币奖励",
		map[string]any{"inviteeID": newUserID.String()},
	)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "ok",
		"data": gin.H{
			"success": true,
			"message": "邀请奖励已发放！你获得30金币，邀请人获得50金币",
		},
	})
}
