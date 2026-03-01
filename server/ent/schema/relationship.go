package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// Relationship tracks the affinity between two agents/users.
// user_a_id is always lexicographically smaller than user_b_id to avoid duplicates.
type Relationship struct {
	ent.Schema
}

func (Relationship) Mixin() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
	}
}

func (Relationship) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New),
		field.UUID("user_a_id", uuid.UUID{}).
			Comment("永远是 UUID 字典序较小的一方"),
		field.UUID("user_b_id", uuid.UUID{}),
		field.Int("affinity").
			Default(0).
			Min(0).
			Max(100).
			Comment("好感度 0-100"),
		field.String("level").
			MaxLen(20).
			Default("stranger").
			Comment("stranger/acquaintance/friend/close_friend/best_friend"),
		field.Time("last_interact_at").
			Default(time.Now),
	}
}

func (Relationship) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user_a", User.Type).
			Ref("relationships_as_a").
			Field("user_a_id").
			Required().
			Unique(),
		edge.From("user_b", User.Type).
			Ref("relationships_as_b").
			Field("user_b_id").
			Required().
			Unique(),
	}
}

func (Relationship) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_a_id", "user_b_id").
			Unique().
			StorageKey("uidx_rel_a_b"),
		index.Fields("user_a_id").StorageKey("idx_rel_user_a"),
		index.Fields("user_b_id").StorageKey("idx_rel_user_b"),
	}
}
