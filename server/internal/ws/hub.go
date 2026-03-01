package ws

import (
	"encoding/json"
	"log"
	"sync"

	"github.com/google/uuid"
)

// Hub manages all active WebSocket connections grouped by village room.
//
// A single Hub goroutine owns the rooms map; no mutex is needed for that map.
// External goroutines (tick engine, handlers) send messages via the exported
// Broadcast / BroadcastVillage / Send methods which are goroutine-safe.
type Hub struct {
	// rooms maps villageID → set of connected clients in that room.
	rooms map[uuid.UUID]map[*Client]struct{}
	// noVillage holds authenticated clients not yet in any room.
	noVillage map[*Client]struct{}

	mu sync.RWMutex // protects rooms + noVillage

	register   chan *Client
	unregister chan *Client
	broadcast  chan broadcastReq
}

type broadcastReq struct {
	villageID uuid.UUID // zero → broadcast to all
	userID    uuid.UUID // zero → all users in room
	msg       []byte
}

// NewHub creates and starts a Hub.
func NewHub() *Hub {
	h := &Hub{
		rooms:      make(map[uuid.UUID]map[*Client]struct{}),
		noVillage:  make(map[*Client]struct{}),
		register:   make(chan *Client, 64),
		unregister: make(chan *Client, 64),
		broadcast:  make(chan broadcastReq, 256),
	}
	go h.run()
	return h
}

// run is the single goroutine that owns the rooms map.
func (h *Hub) run() {
	for {
		select {
		case c := <-h.register:
			h.mu.Lock()
			if c.VillageID == (uuid.UUID{}) {
				h.noVillage[c] = struct{}{}
			} else {
				if h.rooms[c.VillageID] == nil {
					h.rooms[c.VillageID] = make(map[*Client]struct{})
				}
				h.rooms[c.VillageID][c] = struct{}{}
			}
			h.mu.Unlock()
			log.Printf("[ws/hub] registered user=%s village=%s", c.UserID, c.VillageID)

		case c := <-h.unregister:
			h.mu.Lock()
			delete(h.noVillage, c)
			if room, ok := h.rooms[c.VillageID]; ok {
				delete(room, c)
				if len(room) == 0 {
					delete(h.rooms, c.VillageID)
				}
			}
			h.mu.Unlock()
			close(c.send)
			log.Printf("[ws/hub] unregistered user=%s", c.UserID)

		case req := <-h.broadcast:
			h.mu.RLock()
			if req.villageID == (uuid.UUID{}) {
				// Global broadcast — include clients not yet in any village room
				for c := range h.noVillage {
					h.trySend(c, req)
				}
				for _, room := range h.rooms {
					for c := range room {
						h.trySend(c, req)
					}
				}
			} else {
				// Village-scoped
				for c := range h.rooms[req.villageID] {
					if req.userID == (uuid.UUID{}) || c.UserID == req.userID {
						h.trySend(c, req)
					}
				}
			}
			h.mu.RUnlock()
		}
	}
}

func (h *Hub) trySend(c *Client, req broadcastReq) {
	select {
	case c.send <- req.msg:
	default:
		// Slow client: drop message rather than block the hub
		log.Printf("[ws/hub] drop msg for slow client user=%s", c.UserID)
	}
}

// BroadcastVillage sends msg to all clients in the given village.
func (h *Hub) BroadcastVillage(villageID uuid.UUID, msg *Message) {
	b, err := json.Marshal(msg)
	if err != nil {
		log.Printf("[ws/hub] marshal: %v", err)
		return
	}
	h.broadcast <- broadcastReq{villageID: villageID, msg: b}
}

// Send delivers msg to a single user (searches noVillage + all rooms).
func (h *Hub) Send(userID uuid.UUID, msg *Message) {
	b, err := json.Marshal(msg)
	if err != nil {
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.noVillage {
		if c.UserID == userID {
			select {
			case c.send <- b:
			default:
			}
			return
		}
	}
	for _, room := range h.rooms {
		for c := range room {
			if c.UserID == userID {
				select {
				case c.send <- b:
				default:
				}
				return
			}
		}
	}
}

// BroadcastAll sends msg to every connected client.
func (h *Hub) BroadcastAll(msg *Message) {
	b, err := json.Marshal(msg)
	if err != nil {
		return
	}
	h.broadcast <- broadcastReq{msg: b}
}

// Register adds a client to the hub.
func (h *Hub) Register(c *Client) { h.register <- c }

// Unregister removes a client from the hub.
func (h *Hub) Unregister(c *Client) { h.unregister <- c }

// IsOnline reports whether a user currently has an active WebSocket connection.
func (h *Hub) IsOnline(userID uuid.UUID) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.noVillage {
		if c.UserID == userID {
			return true
		}
	}
	for _, room := range h.rooms {
		for c := range room {
			if c.UserID == userID {
				return true
			}
		}
	}
	return false
}
