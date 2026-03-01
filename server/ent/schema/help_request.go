package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// HelpRequest holds the schema definition for the HelpRequest entity.
type HelpRequest struct {
	ent.Schema
}

func (HelpRequest) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New),
		field.UUID("requester_id", uuid.UUID{}).
			Comment("求助方"),
		field.UUID("helper_id", uuid.UUID{}).
			Comment("被求助方"),
		field.String("resource_type").
			MaxLen(16).
			Comment("seed/material/coins 等"),
		field.String("resource_id").
			MaxLen(64).
			Optional().
			Nillable().
			Comment("物品ID，coins 类型为 null"),
		field.Int("quantity").
			Min(1),
		field.String("message").
			MaxLen(200).
			Optional().
			Nillable(),
		field.String("status").
			MaxLen(16).
			Default("pending").
			Comment("pending/accepted/rejected/expired"),
		field.Time("created_at").
			Default(time.Now).
			Immutable(),
		field.Time("responded_at").
			Optional().
			Nillable(),
	}
}

func (HelpRequest) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("requester", User.Type).
			Ref("help_requests_sent").
			Field("requester_id").
			Required().
			Unique(),
		edge.From("helper", User.Type).
			Ref("help_requests_received").
			Field("helper_id").
			Required().
			Unique(),
	}
}

func (HelpRequest) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("helper_id", "status").StorageKey("idx_hr_helper_status"),
	}
}
