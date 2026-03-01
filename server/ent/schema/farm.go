package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

// Farm holds the schema definition for the Farm entity.
type Farm struct {
	ent.Schema
}

func (Farm) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New),
		field.UUID("owner_id", uuid.UUID{}).
			Comment("农场主 user_id"),
		field.String("name").
			MaxLen(64).
			NotEmpty(),
		field.Int("level").
			Default(1).
			Min(1),
		field.JSON("specialty", []string{}).
			Default([]string{}).
			Comment("村庄特产作物 ID 列表"),
		field.JSON("plots", []PlotState{}).
			Default([]PlotState{}).
			Comment("8×8 地块状态数组"),
		field.Time("last_tick_at").
			Default(time.Now).
			Comment("最后一次 Tick 时间"),
		field.Time("created_at").
			Default(time.Now).
			Immutable(),
	}
}

func (Farm) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("owner", User.Type).
			Ref("farm").
			Field("owner_id").
			Required().
			Unique(),
		edge.To("buildings", Building.Type),
		edge.To("animals", Animal.Type),
	}
}
