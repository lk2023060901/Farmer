package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// Animal holds the schema definition for the Animal entity.
type Animal struct {
	ent.Schema
}

func (Animal) Mixin() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
	}
}

func (Animal) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New),
		field.UUID("farm_id", uuid.UUID{}),
		field.String("type").
			MaxLen(16).
			Comment("chicken/cow/sheep/bee/rabbit"),
		field.String("name").
			MaxLen(32).
			Optional().
			Nillable().
			Comment("玩家自定义名字"),
		field.Int("mood").
			Default(50).
			Min(0).
			Max(100),
		field.Int("health").
			Default(100).
			Min(0).
			Max(100),
		field.Time("last_fed_at").
			Optional().
			Nillable(),
		field.Time("last_cleaned_at").
			Optional().
			Nillable(),
		field.Time("last_product_at").
			Optional().
			Nillable().
			Comment("最后产出时间"),
	}
}

func (Animal) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("farm", Farm.Type).
			Ref("animals").
			Field("farm_id").
			Required().
			Unique(),
	}
}

func (Animal) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("farm_id").StorageKey("idx_animals_farm_id"),
	}
}
