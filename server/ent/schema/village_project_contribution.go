package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// VillageProjectContribution holds the schema definition for the VillageProjectContribution entity.
type VillageProjectContribution struct {
	ent.Schema
}

func (VillageProjectContribution) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New),
		field.UUID("project_id", uuid.UUID{}),
		field.UUID("user_id", uuid.UUID{}),
		field.String("resource_type").
			MaxLen(16).
			Comment("material/coins"),
		field.String("item_id").
			MaxLen(64).
			Optional().
			Nillable(),
		field.Int64("quantity").
			Min(1),
		field.Time("created_at").
			Default(time.Now).
			Immutable(),
	}
}

func (VillageProjectContribution) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("project", VillageProject.Type).
			Ref("contributions").
			Field("project_id").
			Required().
			Unique(),
		edge.From("contributor", User.Type).
			Ref("contributions").
			Field("user_id").
			Required().
			Unique(),
	}
}

func (VillageProjectContribution) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("project_id", "user_id").StorageKey("idx_vpc_project_user"),
	}
}
