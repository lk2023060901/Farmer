package handler

import (
	"context"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"github.com/liukai/farmer/server/ent"
	entrole "github.com/liukai/farmer/server/ent/role"
	entvm "github.com/liukai/farmer/server/ent/villagemember"
	"github.com/liukai/farmer/server/internal/middleware"
	ws "github.com/liukai/farmer/server/internal/ws"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// Allow all origins for now; tighten in production via cfg.Server.Mode check
	CheckOrigin: func(r *http.Request) bool { return true },
}

// WSHandler upgrades HTTP connections to WebSocket.
type WSHandler struct {
	hub       *ws.Hub
	db        *ent.Client
	jwtSecret string
}

// NewWSHandler constructs a WSHandler.
func NewWSHandler(hub *ws.Hub, db *ent.Client, jwtSecret string) *WSHandler {
	return &WSHandler{hub: hub, db: db, jwtSecret: jwtSecret}
}

// Connect handles GET /api/v1/ws
//
// Authentication: JWT passed as query param ?token=<jwt>
// (headers are not reliably supported by WebSocket clients in mini-programs).
//
// After upgrade the server:
//  1. Looks up which village the user belongs to.
//  2. Registers the client in the appropriate room.
//  3. Starts read/write pumps.
func (h *WSHandler) Connect(c *gin.Context) {
	tokenStr := c.Query("token")
	if tokenStr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "missing token"})
		return
	}

	claims := &middleware.Claims{}
	_, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
		return []byte(h.jwtSecret), nil
	})
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "invalid token"})
		return
	}

	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "bad user id"})
		return
	}

	// Determine village membership
	var villageID uuid.UUID
	mem, err := h.db.VillageMember.Query().
		Where(entvm.UserID(userID)).
		Only(c.Request.Context())
	if err == nil {
		villageID = mem.VillageID
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		// Upgrade writes its own error response
		return
	}

	client := ws.NewClient(h.hub, conn, userID, villageID)

	// Inject move_to handler: updates DB and broadcasts role_move to all clients.
	client.OnMoveTo = func(tileX, tileY int) {
		if tileX < 0 { tileX = 0 }
		if tileX > 127 { tileX = 127 }
		if tileY < 0 { tileY = 0 }
		if tileY > 127 { tileY = 127 }

		ctx := context.Background()
		ag, err := h.db.Role.Query().Where(entrole.UserID(userID)).Only(ctx)
		if err != nil {
			log.Printf("[ws/move_to] role not found user=%s: %v", userID, err)
			return
		}
		if _, err := h.db.Role.UpdateOne(ag).SetTileX(tileX).SetTileY(tileY).Save(ctx); err != nil {
			log.Printf("[ws/move_to] update role %s: %v", ag.ID, err)
			return
		}
		h.hub.BroadcastAll(&ws.Message{
			Type: ws.EventRoleMove,
			Payload: ws.RoleMovePayload{
				RoleID: ag.ID.String(),
				Name:   ag.Name,
				Avatar: ag.Avatar,
				MapID:  "world",
				TileX:  tileX,
				TileY:  tileY,
			},
		})
	}

	h.hub.Register(client)
}
