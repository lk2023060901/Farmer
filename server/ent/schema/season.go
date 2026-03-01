package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

// Season holds the schema definition for the Season entity (赛季配置).
type Season struct {
	ent.Schema
}

func (Season) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New),
		field.Int("number").
			Unique().
			Min(1).
			Comment("赛季序号 1, 2, 3..."),
		field.String("name").
			MaxLen(64),
		field.Time("start_at"),
		field.Time("end_at"),
		field.String("status").
			MaxLen(16).
			Default("upcoming").
			Comment("upcoming/active/settling/completed"),
		field.JSON("tasks_config", []interface{}{}).
			Default([]interface{}{}).
			Comment("赛季任务配置"),
		field.JSON("rewards_config", map[string]interface{}{}).
			Default(map[string]interface{}{}).
			Comment("排名奖励配置"),
	}
}

func (Season) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("scores", SeasonScore.Type),
		edge.To("task_progresses", SeasonTaskProgress.Type),
	}
}
