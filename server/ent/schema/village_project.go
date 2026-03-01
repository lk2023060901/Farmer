package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// VillageProject holds the schema definition for the VillageProject entity.
type VillageProject struct {
	ent.Schema
}

func (VillageProject) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New),
		field.UUID("village_id", uuid.UUID{}),
		field.String("type").
			MaxLen(32).
			Comment("market/clocktower/road 等"),
		field.String("name").
			MaxLen(64),
		field.JSON("requirements", []RequirementItem{}).
			Comment("所需资源配置（含当前进度）"),
		field.String("status").
			MaxLen(16).
			Default("active").
			Comment("active/completed/cancelled"),
		field.Time("started_at").
			Default(time.Now),
		field.Time("completed_at").
			Optional().
			Nillable(),
	}
}

func (VillageProject) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("village", Village.Type).
			Ref("projects").
			Field("village_id").
			Required().
			Unique(),
		edge.To("contributions", VillageProjectContribution.Type),
	}
}

func (VillageProject) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("village_id", "status").StorageKey("idx_vp_village_status"),
	}
}
