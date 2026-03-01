package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

// Village holds the schema definition for the Village entity.
type Village struct {
	ent.Schema
}

func (Village) Mixin() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
	}
}

func (Village) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New),
		field.String("name").
			MaxLen(64).
			NotEmpty(),
		field.Int("level").
			Default(1).
			Min(1).
			Max(5),
		field.Int64("contribution").
			Default(0).
			Min(0).
			Comment("总贡献值"),
		field.Int("member_count").
			Default(0).
			Min(0).
			Comment("冗余成员数，加速查询"),
		field.Int("max_members").
			Default(20),
		field.JSON("specialty", []string{}).
			Default([]string{}).
			Comment("特产作物 ID 列表"),
	}
}

func (Village) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("members", VillageMember.Type),
		edge.To("projects", VillageProject.Type),
	}
}
