package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// ActivityLike holds the schema definition for the ActivityLike entity.
type ActivityLike struct {
	ent.Schema
}

func (ActivityLike) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New),
		field.UUID("activity_id", uuid.UUID{}),
		field.UUID("user_id", uuid.UUID{}),
		field.Time("created_at").
			Default(time.Now).
			Immutable(),
	}
}

func (ActivityLike) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("activity", ActivityLog.Type).
			Ref("likes").
			Field("activity_id").
			Required().
			Unique(),
		edge.From("user", User.Type).
			Ref("activity_likes").
			Field("user_id").
			Required().
			Unique(),
	}
}

func (ActivityLike) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("activity_id", "user_id").
			Unique().
			StorageKey("uidx_like_activity_user"),
	}
}
