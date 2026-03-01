package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// Notification holds the schema definition for the Notification entity.
type Notification struct {
	ent.Schema
}

func (Notification) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New),
		field.UUID("user_id", uuid.UUID{}),
		field.String("type").
			MaxLen(32).
			Comment("social/trade/village/system/event"),
		field.String("title").
			MaxLen(64),
		field.Text("content"),
		field.Bool("is_read").
			Default(false),
		field.String("action_type").
			MaxLen(32).
			Optional().
			Nillable().
			Comment("goto_trade/goto_farm 等"),
		field.JSON("action_data", map[string]interface{}{}).
			Optional().
			Comment("跳转所需参数"),
		field.Time("created_at").
			Default(time.Now).
			Immutable(),
	}
}

func (Notification) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("notifications").
			Field("user_id").
			Required().
			Unique(),
	}
}

func (Notification) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "is_read", "created_at").StorageKey("idx_notif_user_read"),
	}
}
