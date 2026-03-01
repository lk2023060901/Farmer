package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

// Subscription holds the schema definition for the Subscription entity (月卡订阅).
type Subscription struct {
	ent.Schema
}

func (Subscription) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New),
		field.UUID("user_id", uuid.UUID{}).
			Unique().
			Comment("每个用户最多一条有效订阅"),
		field.String("plan_id").
			MaxLen(16).
			Comment("monthly/yearly"),
		field.Time("started_at"),
		field.Time("expires_at"),
		field.String("status").
			MaxLen(16).
			Comment("active/expired/cancelled"),
		field.Bool("auto_renew").
			Default(true),
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now),
	}
}

func (Subscription) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("subscription").
			Field("user_id").
			Required().
			Unique(),
	}
}
