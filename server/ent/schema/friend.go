package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// Friend holds the schema definition for the Friend entity (好友关系).
// user_a_id is always lexicographically smaller than user_b_id.
type Friend struct {
	ent.Schema
}

func (Friend) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New),
		field.UUID("user_a_id", uuid.UUID{}).
			Comment("ID 较小的一方"),
		field.UUID("user_b_id", uuid.UUID{}),
		field.Time("created_at").
			Default(time.Now).
			Immutable(),
	}
}

func (Friend) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user_a", User.Type).
			Ref("friends_as_a").
			Field("user_a_id").
			Required().
			Unique(),
		edge.From("user_b", User.Type).
			Ref("friends_as_b").
			Field("user_b_id").
			Required().
			Unique(),
	}
}

func (Friend) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_a_id", "user_b_id").
			Unique().
			StorageKey("uidx_friend_a_b"),
		index.Fields("user_a_id").StorageKey("idx_friend_a"),
		index.Fields("user_b_id").StorageKey("idx_friend_b"),
	}
}
