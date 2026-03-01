package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// TutorialProgress holds the schema definition for the TutorialProgress entity.
type TutorialProgress struct {
	ent.Schema
}

func (TutorialProgress) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New),
		field.UUID("user_id", uuid.UUID{}),
		field.String("step").MaxLen(64).Comment("tutorial step key"),
		field.Time("completed_at").Default(time.Now).Immutable(),
	}
}

func (TutorialProgress) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("tutorial_progresses").
			Field("user_id").
			Required().
			Unique(),
	}
}

func (TutorialProgress) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "step").Unique().StorageKey("uidx_tutorial_user_step"),
	}
}
