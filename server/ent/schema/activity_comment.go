package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// ActivityComment holds the schema definition for the ActivityComment entity.
type ActivityComment struct {
	ent.Schema
}

func (ActivityComment) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New),
		field.UUID("activity_id", uuid.UUID{}),
		field.UUID("user_id", uuid.UUID{}),
		field.String("content").
			MaxLen(200).
			NotEmpty(),
		field.Time("created_at").
			Default(time.Now).
			Immutable(),
	}
}

func (ActivityComment) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("activity", ActivityLog.Type).
			Ref("comments").
			Field("activity_id").
			Required().
			Unique(),
		edge.From("author", User.Type).
			Ref("activity_comments").
			Field("user_id").
			Required().
			Unique(),
	}
}

func (ActivityComment) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("activity_id", "created_at").StorageKey("idx_ac_activity_time"),
	}
}
