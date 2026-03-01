package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

// Role holds the schema definition for the Role entity (AI 人格体).
type Role struct {
	ent.Schema
}

func (Role) Mixin() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
	}
}

func (Role) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New),
		field.UUID("user_id", uuid.UUID{}),
		field.String("name").
			MaxLen(32).
			NotEmpty().
			Comment("Agent 名字"),
		field.String("avatar").
			MaxLen(64).
			Comment("sprite 帧名"),
		// 人格维度 1-10
		field.Int("extroversion").
			Default(5).
			Min(1).
			Max(10).
			Comment("外向度"),
		field.Int("generosity").
			Default(5).
			Min(1).
			Max(10).
			Comment("慷慨度"),
		field.Int("adventure").
			Default(5).
			Min(1).
			Max(10).
			Comment("冒险度"),
		// 策略维度 1-5
		field.Int("strategy_management").
			Default(3).
			Min(1).
			Max(5).
			Comment("经营风格"),
		field.Int("strategy_planting").
			Default(3).
			Min(1).
			Max(5).
			Comment("种植偏好"),
		field.Int("strategy_social").
			Default(3).
			Min(1).
			Max(5).
			Comment("社交倾向"),
		field.Int("strategy_trade").
			Default(3).
			Min(1).
			Max(5).
			Comment("交易策略"),
		field.Int("strategy_resource").
			Default(3).
			Min(1).
			Max(5).
			Comment("资源分配"),
		field.Int("daily_social_count").
			Default(0).
			Min(0).
			Comment("当日社交次数"),
		field.Time("daily_social_date").
			Optional().
			Nillable().
			Comment("当日社交次数重置日期"),
		field.Time("last_active_at"),

		// 地图位置
		field.String("map_id").
			Default("world").
			Comment("所在地图 ID"),
		field.Int("tile_x").
			Default(10).
			Comment("地图 tile X 坐标"),
		field.Int("tile_y").
			Default(10).
			Comment("地图 tile Y 坐标"),
	}
}

func (Role) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("role").
			Field("user_id").
			Required().
			Unique(),
	}
}
