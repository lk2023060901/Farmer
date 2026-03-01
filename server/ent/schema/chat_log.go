package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// ChatLog holds the schema definition for the ChatLog entity.
// user_a_id < user_b_id (UUID lexicographic order) to canonicalize the conversation pair.
type ChatLog struct {
	ent.Schema
}

func (ChatLog) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New),
		field.UUID("user_a_id", uuid.UUID{}).
			Comment("会话双方（小ID）"),
		field.UUID("user_b_id", uuid.UUID{}).
			Comment("会话双方（大ID）"),
		field.UUID("speaker_user_id", uuid.UUID{}).
			Comment("本条发言方"),
		field.String("scene").
			MaxLen(32).
			Comment("visit/trade/help/gift 等"),
		field.Text("content").
			Comment("对话内容"),
		field.Bool("is_llm_generated").
			Default(false),
		field.Time("created_at").
			Default(time.Now).
			Immutable(),
	}
}

func (ChatLog) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user_a", User.Type).
			Ref("chat_logs_as_a").
			Field("user_a_id").
			Required().
			Unique(),
		edge.From("user_b", User.Type).
			Ref("chat_logs_as_b").
			Field("user_b_id").
			Required().
			Unique(),
		edge.From("speaker", User.Type).
			Ref("chat_logs_spoken").
			Field("speaker_user_id").
			Required().
			Unique(),
	}
}

func (ChatLog) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_a_id", "user_b_id", "created_at").
			StorageKey("idx_chat_pair_time"),
	}
}
