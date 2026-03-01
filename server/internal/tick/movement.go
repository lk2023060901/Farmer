package tick

import (
	"context"
	"log"
	"math/rand"
	"time"

	"github.com/google/uuid"
	entrole "github.com/liukai/farmer/server/ent/role"
	"github.com/liukai/farmer/server/internal/pathfinding"
	"github.com/liukai/farmer/server/internal/ws"
)

const (
	moveInterval = 5 * time.Second
	worldMapW    = 128
	worldMapH    = 128
	wanderRadius = 15 // max tile offset from current position per wander goal
)

// wanderState holds an in-progress A* path for one role.
// It lives only in the movement goroutine — no mutex needed.
type wanderState struct {
	path  []pathfinding.Point
	index int // index of the next step to take
}

// StartMovement launches the autonomous role movement loop in a background goroutine.
// Roles on the "world" map take one step every moveInterval toward a random wander goal.
func (e *Engine) StartMovement(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(moveInterval)
		defer ticker.Stop()
		states := make(map[uuid.UUID]*wanderState)
		log.Println("[movement] engine started, interval=", moveInterval)
		for {
			select {
			case <-ctx.Done():
				log.Println("[movement] engine stopped")
				return
			case <-ticker.C:
				e.tickMovement(ctx, states)
			}
		}
	}()
}

// tickMovement advances each role on the world map by one tile along its wander path.
func (e *Engine) tickMovement(ctx context.Context, states map[uuid.UUID]*wanderState) {
	roles, err := e.db.Role.Query().
		Where(entrole.MapID("world")).
		All(ctx)
	if err != nil {
		log.Printf("[movement] query roles: %v", err)
		return
	}

	// roleUserMap: roleID → userID，用于 AOI 反查在线玩家
	roleUserMap := make(map[uuid.UUID]uuid.UUID, len(roles))
	for _, r := range roles {
		roleUserMap[r.ID] = r.UserID
	}

	for _, r := range roles {
		st := states[r.ID]

		// Assign a fresh wander path when the current one is exhausted.
		if st == nil || st.index >= len(st.path) {
			goal := randomWanderPoint(r.TileX, r.TileY, worldMapW, worldMapH)
			path := pathfinding.FindPath(
				pathfinding.Point{X: r.TileX, Y: r.TileY},
				goal,
				worldMapW, worldMapH,
				func(x, y int) bool { return true }, // open world — no server-side obstacles
			)
			if len(path) < 2 {
				continue // already at goal or zero-length path
			}
			st = &wanderState{path: path, index: 1}
			states[r.ID] = st
		}

		next := st.path[st.index]
		st.index++

		if _, err := e.db.Role.UpdateOne(r).
			SetTileX(next.X).
			SetTileY(next.Y).
			Save(ctx); err != nil {
			log.Printf("[movement] update role %s: %v", r.ID, err)
			continue
		}

		// Keep the AOI spatial grid in sync with the new position.
		e.Grid.Move(r.ID, r.TileX, r.TileY, next.X, next.Y)

		// AOI 广播：只向九宫格范围内玩家发送 role_move，避免全服广播
		msg := &ws.Message{
			Type: ws.EventRoleMove,
			Payload: ws.RoleMovePayload{
				RoleID: r.ID.String(),
				Name:   r.Name,
				Avatar: r.Avatar,
				MapID:  "world",
				TileX:  next.X,
				TileY:  next.Y,
			},
		}
		for _, nearRoleID := range e.Grid.NineGrid(next.X, next.Y) {
			if userID, ok := roleUserMap[nearRoleID]; ok {
				e.hub.Send(userID, msg)
			}
		}
	}
}

// randomWanderPoint picks a random tile within wanderRadius tiles of (fromX, fromY),
// clamped to the map boundary.
func randomWanderPoint(fromX, fromY, mapW, mapH int) pathfinding.Point {
	dx := rand.Intn(wanderRadius*2+1) - wanderRadius
	dy := rand.Intn(wanderRadius*2+1) - wanderRadius
	return pathfinding.Point{
		X: clampInt(fromX+dx, 0, mapW-1),
		Y: clampInt(fromY+dy, 0, mapH-1),
	}
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
