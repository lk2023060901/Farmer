package handler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/liukai/farmer/server/ent"
	"github.com/liukai/farmer/server/ent/user"
	entschema "github.com/liukai/farmer/server/ent/schema"
	"github.com/liukai/farmer/server/internal/gameconfig"
	"github.com/liukai/farmer/server/internal/middleware"
	"github.com/liukai/farmer/server/internal/service"
)

// AuthHandler groups authentication-related route handlers.
type AuthHandler struct {
	db           *ent.Client
	jwtSecret    string
	expiresHours int
	gameCfg      *gameconfig.Config
}

// NewAuthHandler constructs an AuthHandler.
func NewAuthHandler(db *ent.Client, jwtSecret string, expiresHours int, gameCfg *gameconfig.Config) *AuthHandler {
	return &AuthHandler{db: db, jwtSecret: jwtSecret, expiresHours: expiresHours, gameCfg: gameCfg}
}

// genNickname 生成不可预测的昵称，格式：农场主_xxxxxxxx（8位随机十六进制）
func genNickname() string {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		// 极少发生，fallback 到时间戳
		return fmt.Sprintf("农场主_%d", time.Now().UnixNano()%100000000)
	}
	return "农场主_" + hex.EncodeToString(b)
}

// signToken creates a signed JWT for the given user ID.
func (h *AuthHandler) signToken(userID uuid.UUID) (string, error) {
	claims := middleware.Claims{
		UserID: userID.String(),
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(h.expiresHours) * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(h.jwtSecret))
}

// upsertDevUser finds or creates the deterministic dev user, ensuring they have
// a farm and a role record.
func (h *AuthHandler) upsertDevUser(ctx context.Context) (*ent.User, error) {
	const devNickname = "Dev User"
	devID := uuid.MustParse("00000000-0000-0000-0000-000000000001")

	// Try to find existing user
	u, err := h.db.User.Query().
		Where(user.ID(devID)).
		WithFarm().
		WithRole().
		Only(ctx)
	if err == nil {
		// User exists — but may be missing role/farm if created before those were added.
		if u.Edges.Role == nil {
			if err2 := h.ensureDevRole(ctx, u); err2 != nil {
				return nil, fmt.Errorf("ensure dev role: %w", err2)
			}
		}
		return u, nil
	}
	if !ent.IsNotFound(err) {
		return nil, fmt.Errorf("query dev user: %w", err)
	}

	// Create user
	u, err = h.db.User.Create().
		SetID(devID).
		SetNickname(devNickname).
		SetAvatar("").
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("create dev user: %w", err)
	}

	// Create initial 8×8 farm (all empty plots)
	plots := make([]entschema.PlotState, 0, 64)
	for row := 0; row < 8; row++ {
		for col := 0; col < 8; col++ {
			plots = append(plots, entschema.PlotState{
				X: col, Y: row, Type: "empty",
			})
		}
	}
	_, err = h.db.Farm.Create().
		SetOwner(u).
		SetName(devNickname + "的农场").
		SetPlots(plots).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("create dev farm: %w", err)
	}

	// Create role — role comes from roles.json via global.json role_id
	devRole := h.gameCfg.GetRole(h.gameCfg.Global.RoleID)
	devRoleCreate := h.db.Role.Create().
		SetUser(u).
		SetLastActiveAt(time.Now())
	if devRole != nil {
		devRoleCreate = devRoleCreate.
			SetName(devRole.Name).
			SetAvatar(devRole.Avatar).
			SetExtroversion(devRole.Extroversion).
			SetGenerosity(devRole.Generosity).
			SetAdventure(devRole.Adventure).
			SetStrategyManagement(devRole.StrategyManagement).
			SetStrategyPlanting(devRole.StrategyPlanting).
			SetStrategySocial(devRole.StrategySocial).
			SetStrategyTrade(devRole.StrategyTrade).
			SetStrategyResource(devRole.StrategyResource)
	}
	_, err = devRoleCreate.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("create dev agent: %w", err)
	}

	// Reload with edges
	return h.db.User.Query().
		Where(user.ID(devID)).
		WithFarm().
		WithRole().
		Only(ctx)
}

// ensureDevRole creates a Role for the dev user if one doesn't exist yet.
// Called when the dev user account pre-dates role creation.
func (h *AuthHandler) ensureDevRole(ctx context.Context, u *ent.User) error {
	devRole := h.gameCfg.GetRole(h.gameCfg.Global.RoleID)
	rc := h.db.Role.Create().
		SetUser(u).
		SetLastActiveAt(time.Now())
	if devRole != nil {
		rc = rc.
			SetName(devRole.Name).
			SetAvatar(devRole.Avatar).
			SetExtroversion(devRole.Extroversion).
			SetGenerosity(devRole.Generosity).
			SetAdventure(devRole.Adventure).
			SetStrategyManagement(devRole.StrategyManagement).
			SetStrategyPlanting(devRole.StrategyPlanting).
			SetStrategySocial(devRole.StrategySocial).
			SetStrategyTrade(devRole.StrategyTrade).
			SetStrategyResource(devRole.StrategyResource)
	}
	_, err := rc.Save(ctx)
	return err
}

// DevLogin handles POST /api/v1/auth/dev-login
// H5 development only: finds or creates the dev user and returns a real JWT.
func (h *AuthHandler) DevLogin(c *gin.Context) {
	u, err := h.upsertDevUser(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 500, "message": err.Error(), "data": nil,
		})
		return
	}

	token, err := h.signToken(u.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 500, "message": "failed to sign token", "data": nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "ok",
		"data": gin.H{
			"token":    token,
			"userId":   u.ID.String(),
			"nickname": u.Nickname,
			"avatar":   u.Avatar,
		},
	})
}

// WxLogin handles POST /api/v1/auth/wx-login
func (h *AuthHandler) WxLogin(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": nil})
}

// Refresh handles POST /api/v1/auth/refresh
func (h *AuthHandler) Refresh(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": nil})
}

// Register handles POST /api/v1/auth/register
// Body: { "phone": "...", "password": "...", "nickname": "..." }
func (h *AuthHandler) Register(c *gin.Context) {
	var req struct {
		Phone    string `json:"phone"    binding:"required"`
		Password string `json:"password" binding:"required,min=6"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error(), "data": nil})
		return
	}

	ctx := c.Request.Context()

	// 检查手机号是否已注册
	exists, err := h.db.User.Query().Where(user.PhoneEQ(req.Phone)).Exist(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error(), "data": nil})
		return
	}
	if exists {
		c.JSON(http.StatusConflict, gin.H{"code": 409, "message": "该手机号已注册", "data": nil})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "密码加密失败", "data": nil})
		return
	}
	hashStr := string(hash)
	nickname := genNickname()
	global := h.gameCfg.Global
	role := h.gameCfg.GetRole(global.RoleID)
	if role == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "invalid default role_id in global.json", "data": nil})
		return
	}

	// 创建用户，金币 / 钻石来自 global.json
	u, err := h.db.User.Create().
		SetPhone(req.Phone).
		SetPasswordHash(hashStr).
		SetNickname(nickname).
		SetAvatar("").
		SetCoins(global.Coins).
		SetDiamonds(global.Diamonds).
		Save(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error(), "data": nil})
		return
	}

	// 发放初始道具
	for _, di := range global.Items {
		itemDef := h.gameCfg.GetItem(di.ItemID)
		itemType := "special"
		if itemDef != nil {
			itemType = itemDef.Category
		}
		if err = service.AddToInventory(ctx, h.db, u.ID, di.ItemID, itemType, int64(di.Quantity)); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "inventory init failed", "data": nil})
			return
		}
	}

	// 初始化 8×8 农场
	plots := make([]entschema.PlotState, 0, 64)
	for row := 0; row < 8; row++ {
		for col := 0; col < 8; col++ {
			plots = append(plots, entschema.PlotState{X: col, Y: row, Type: "empty"})
		}
	}
	if _, err = h.db.Farm.Create().SetOwner(u).SetName(nickname + "的农场").SetPlots(plots).Save(ctx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error(), "data": nil})
		return
	}

	// 初始化 Role，名字、外观、人格来自 roles.json，位置来自 maps.json 出生点
	spawn := h.gameCfg.Spawn("world")
	if _, err = h.db.Role.Create().
		SetUser(u).
		SetName(role.Name).
		SetAvatar(role.Avatar).
		SetExtroversion(role.Extroversion).
		SetGenerosity(role.Generosity).
		SetAdventure(role.Adventure).
		SetStrategyManagement(role.StrategyManagement).
		SetStrategyPlanting(role.StrategyPlanting).
		SetStrategySocial(role.StrategySocial).
		SetStrategyTrade(role.StrategyTrade).
		SetStrategyResource(role.StrategyResource).
		SetMapID("world").
		SetTileX(spawn.TileX).
		SetTileY(spawn.TileY).
		SetLastActiveAt(time.Now()).
		Save(ctx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error(), "data": nil})
		return
	}

	token, err := h.signToken(u.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "token 签发失败", "data": nil})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0, "message": "ok",
		"data": gin.H{"token": token, "userId": u.ID.String(), "nickname": u.Nickname, "avatar": u.Avatar},
	})
}

// Login handles POST /api/v1/auth/login
// Body: { "phone": "...", "password": "..." }
func (h *AuthHandler) Login(c *gin.Context) {
	var req struct {
		Phone    string `json:"phone"    binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error(), "data": nil})
		return
	}

	ctx := c.Request.Context()

	u, err := h.db.User.Query().Where(user.PhoneEQ(req.Phone)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "手机号或密码错误", "data": nil})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error(), "data": nil})
		}
		return
	}

	if u.PasswordHash == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "该账号未设置密码，请使用其他方式登录", "data": nil})
		return
	}

	if err = bcrypt.CompareHashAndPassword([]byte(*u.PasswordHash), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "手机号或密码错误", "data": nil})
		return
	}

	token, err := h.signToken(u.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "token 签发失败", "data": nil})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0, "message": "ok",
		"data": gin.H{"token": token, "userId": u.ID.String(), "nickname": u.Nickname, "avatar": u.Avatar},
	})
}
