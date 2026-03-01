package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// ActivityLog holds the schema definition for the ActivityLog entity (动态流).
type ActivityLog struct {
	ent.Schema
}

func (ActivityLog) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New),
		field.UUID("user_id", uuid.UUID{}).
			Comment("动态发布者"),
		field.UUID("village_id", uuid.UUID{}).
			Optional().
			Nillable().
			Comment("所属村庄，用于村庄动态流"),
		field.String("type").
			MaxLen(32).
			Comment("social_visit/harvest_milestone/level_up 等"),
		field.Text("content").
			Comment("展示文字"),
		field.JSON("meta", map[string]interface{}{}).
			Default(map[string]interface{}{}).
			Comment("扩展数据，如关联用户ID、物品ID"),
		field.Int("like_count").
			Default(0).
			Min(0).
			Comment("冗余点赞数，原子更新"),
		field.Int("comment_count").
			Default(0).
			Min(0).
			Comment("冗余评论数，原子更新"),
		field.Time("created_at").
			Default(time.Now).
			Immutable(),
	}
}

func (ActivityLog) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("author", User.Type).
			Ref("activity_logs").
			Field("user_id").
			Required().
			Unique(),
		edge.To("likes", ActivityLike.Type),
		edge.To("comments", ActivityComment.Type),
	}
}

func (ActivityLog) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("village_id", "created_at").StorageKey("idx_al_village_time"),
		index.Fields("user_id", "created_at").StorageKey("idx_al_user_time"),
	}
}
