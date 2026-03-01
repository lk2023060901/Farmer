package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// FriendRequest holds the schema definition for the FriendRequest entity.
type FriendRequest struct {
	ent.Schema
}

func (FriendRequest) Mixin() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
	}
}

func (FriendRequest) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New),
		field.UUID("from_user_id", uuid.UUID{}),
		field.UUID("to_user_id", uuid.UUID{}),
		field.String("message").
			MaxLen(200).
			Optional().
			Nillable(),
		field.String("status").
			MaxLen(16).
			Default("pending").
			Comment("pending/accepted/rejected"),
	}
}

func (FriendRequest) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("from_user", User.Type).
			Ref("sent_friend_requests").
			Field("from_user_id").
			Required().
			Unique(),
		edge.From("to_user", User.Type).
			Ref("received_friend_requests").
			Field("to_user_id").
			Required().
			Unique(),
	}
}

func (FriendRequest) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("to_user_id", "status").StorageKey("idx_fr_to_user_status"),
	}
}
