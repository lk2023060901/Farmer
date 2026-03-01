package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// Building holds the schema definition for the Building entity.
type Building struct {
	ent.Schema
}

func (Building) Mixin() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
	}
}

func (Building) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New),
		field.UUID("farm_id", uuid.UUID{}),
		field.String("type").
			MaxLen(32).
			Comment("farmland/warehouse/coop/barn/pen/workshop/well/fence/mailbox"),
		field.Int("level").
			Default(1).
			Min(1).
			Max(3),
		field.Int("position_x"),
		field.Int("position_y"),
		field.String("state").
			MaxLen(16).
			Default("normal").
			Comment("normal/building/upgrading"),
		field.Time("finish_at").
			Optional().
			Nillable().
			Comment("建造/升级完成时间"),
	}
}

func (Building) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("farm", Farm.Type).
			Ref("buildings").
			Field("farm_id").
			Required().
			Unique(),
	}
}

func (Building) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("farm_id").StorageKey("idx_buildings_farm_id"),
	}
}
