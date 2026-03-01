package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// DailyCheckin holds the schema definition for the DailyCheckin entity.
type DailyCheckin struct {
	ent.Schema
}

func (DailyCheckin) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New),
		field.UUID("user_id", uuid.UUID{}),
		field.Time("checkin_date").
			Comment("签到日期，存为当天 00:00:00 UTC"),
		field.Int("consecutive_days").
			Min(1).
			Comment("连续签到天数"),
		field.String("reward_type").
			MaxLen(32).
			Comment("coins/seed/potion/diamonds 等"),
		field.Int("reward_quantity").
			Min(1),
		field.Time("created_at").
			Default(time.Now).
			Immutable(),
	}
}

func (DailyCheckin) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("daily_checkins").
			Field("user_id").
			Required().
			Unique(),
	}
}

func (DailyCheckin) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "checkin_date").
			Unique().
			StorageKey("uidx_checkin_user_date"),
	}
}
