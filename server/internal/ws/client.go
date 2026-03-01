package ws

import (
	"encoding/json"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

const (
	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = 50 * time.Second // must be < pongWait
	maxMsgSize = 4 * 1024         // 4 KB inbound limit
	sendBuf    = 256              // outbound message buffer
)

// Client is a single WebSocket connection.
type Client struct {
	hub       *Hub
	conn      *websocket.Conn
	send      chan []byte
	UserID    uuid.UUID
	VillageID uuid.UUID // zero if not in a village

	// OnMoveTo is called when the client sends a "move_to" message.
	// Set by the WSHandler after NewClient returns.
	OnMoveTo func(tileX, tileY int)
}

// NewClient creates a client and starts its pump goroutines.
func NewClient(hub *Hub, conn *websocket.Conn, userID, villageID uuid.UUID) *Client {
	c := &Client{
		hub:       hub,
		conn:      conn,
		send:      make(chan []byte, sendBuf),
		UserID:    userID,
		VillageID: villageID,
	}
	go c.writePump()
	go c.readPump()
	return c
}

// readPump reads incoming messages (ping/pong and client→server messages).
// Blocks until the connection closes, then unregisters the client.
func (c *Client) readPump() {
	defer func() {
		c.hub.Unregister(c)
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMsgSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, raw, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err,
				websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				log.Printf("[ws/client] read error user=%s: %v", c.UserID, err)
			}
			break
		}

		var incoming struct {
			Type  string `json:"type"`
			TileX int    `json:"tileX"`
			TileY int    `json:"tileY"`
		}
		if json.Unmarshal(raw, &incoming) != nil {
			continue
		}
		switch incoming.Type {
		case "ping":
			pong, _ := json.Marshal(&Message{Type: EventPong})
			select {
			case c.send <- pong:
			default:
			}
		case "move_to":
			if c.OnMoveTo != nil {
				c.OnMoveTo(incoming.TileX, incoming.TileY)
			}
		}
	}
}

// writePump drains the send channel and writes frames to the connection.
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// hub closed the channel
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				log.Printf("[ws/client] write error user=%s: %v", c.UserID, err)
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
