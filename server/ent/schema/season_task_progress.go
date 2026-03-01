package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// SeasonTaskProgress holds the schema definition for the SeasonTaskProgress entity.
type SeasonTaskProgress struct {
	ent.Schema
}

func (SeasonTaskProgress) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New),
		field.UUID("season_id", uuid.UUID{}),
		field.UUID("user_id", uuid.UUID{}),
		field.String("task_id").
			MaxLen(64),
		field.Int64("progress").
			Default(0).
			Min(0).
			Comment("当前进度"),
		field.Int64("target").
			Min(1).
			Comment("目标值"),
		field.Bool("completed").
			Default(false),
		field.Bool("claimed").
			Default(false).
			Comment("奖励是否已领取"),
		field.Time("completed_at").
			Optional().
			Nillable(),
	}
}

func (SeasonTaskProgress) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("season", Season.Type).
			Ref("task_progresses").
			Field("season_id").
			Required().
			Unique(),
		edge.From("user", User.Type).
			Ref("season_task_progresses").
			Field("user_id").
			Required().
			Unique(),
	}
}

func (SeasonTaskProgress) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("season_id", "user_id", "task_id").
			Unique().
			StorageKey("uidx_stp_season_user_task"),
	}
}
