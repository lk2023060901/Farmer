// Package ws implements the WebSocket hub for real-time village communication.
package ws

import "github.com/google/uuid"

// EventType enumerates outbound event kinds.
type EventType string

const (
	EventFarmUpdate     EventType = "farm_update"     // crop stage changed
	EventSocialEvent    EventType = "social_event"    // agent visit / chat
	EventTradeNotify    EventType = "trade_notify"    // new order or purchase
	EventSystemAnnounce EventType = "system_announce" // server-wide message
	EventPong           EventType = "pong"            // heartbeat reply
	EventNotification   EventType = "notification"    // new notification for user
	EventRoleMove       EventType = "role_move"       // role position updated
)

// Message is the canonical wire format for all server→client messages.
type Message struct {
	Type      EventType `json:"type"`
	VillageID uuid.UUID `json:"villageId,omitempty"`
	UserID    uuid.UUID `json:"userId,omitempty"`
	Payload   any       `json:"payload,omitempty"`
}

// FarmUpdatePayload is the payload for EventFarmUpdate.
type FarmUpdatePayload struct {
	FarmID string `json:"farmId"`
	X      int    `json:"x"`
	Y      int    `json:"y"`
	Stage  string `json:"stage"`
}

// SocialEventPayload is the payload for EventSocialEvent.
type SocialEventPayload struct {
	CallerName string `json:"callerName"`
	TargetName string `json:"targetName"`
	Scene      string `json:"scene"`
	Lines      []struct {
		Role string `json:"role"`
		Text string `json:"text"`
	} `json:"lines"`
}

// NotificationPayload is the payload for EventNotification.
type NotificationPayload struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Title   string `json:"title"`
	Content string `json:"content"`
}

// RoleMovePayload is the payload for EventRoleMove.
// Avatar and Name are included so the client can render a previously-unseen role
// on first encounter without making an extra API call.
type RoleMovePayload struct {
	RoleID string `json:"roleId"`
	Name   string `json:"name"`
	Avatar string `json:"avatar"`
	MapID  string `json:"mapId"`
	TileX  int    `json:"tileX"`
	TileY  int    `json:"tileY"`
}

// TradeNotifyPayload is the payload for EventTradeNotify.
type TradeNotifyPayload struct {
	OrderID  string `json:"orderId"`
	Action   string `json:"action"` // "listed" | "bought"
	ItemName string `json:"itemName"`
	Quantity int64  `json:"quantity"`
	Price    int64  `json:"price"`
}
