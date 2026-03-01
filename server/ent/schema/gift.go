package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// Gift holds the schema definition for the Gift entity.
type Gift struct {
	ent.Schema
}

func (Gift) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New),
		field.UUID("sender_id", uuid.UUID{}),
		field.UUID("receiver_id", uuid.UUID{}),
		field.String("item_id").
			MaxLen(64),
		field.Int("quantity").
			Min(1),
		field.String("message").
			MaxLen(200).
			Optional().
			Nillable(),
		field.Int("affinity_gained").
			Comment("好感度变化量"),
		field.Bool("is_agent_action").
			Default(false).
			Comment("是否 Agent 自动赠礼"),
		field.Time("created_at").
			Default(time.Now).
			Immutable(),
	}
}

func (Gift) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("sender", User.Type).
			Ref("gifts_sent").
			Field("sender_id").
			Required().
			Unique(),
		edge.From("receiver", User.Type).
			Ref("gifts_received").
			Field("receiver_id").
			Required().
			Unique(),
	}
}

func (Gift) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("receiver_id", "created_at").StorageKey("idx_gifts_receiver"),
	}
}
