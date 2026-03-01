package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// SeasonScore holds the schema definition for the SeasonScore entity.
type SeasonScore struct {
	ent.Schema
}

func (SeasonScore) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New),
		field.UUID("season_id", uuid.UUID{}),
		field.UUID("user_id", uuid.UUID{}),
		field.Int64("score_output").
			Default(0).
			Min(0).
			Comment("产值分 40%"),
		field.Int64("score_social").
			Default(0).
			Min(0).
			Comment("社交分 30%"),
		field.Int64("score_collection").
			Default(0).
			Min(0).
			Comment("收藏分 20%"),
		field.Int64("score_quality").
			Default(0).
			Min(0).
			Comment("品质分 10%"),
		field.Int64("score_total").
			Default(0).
			Min(0).
			Comment("综合分，由业务层计算并写入"),
		field.Int("final_rank").
			Optional().
			Nillable().
			Comment("赛季结束后写入"),
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now),
	}
}

func (SeasonScore) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("season", Season.Type).
			Ref("scores").
			Field("season_id").
			Required().
			Unique(),
		edge.From("user", User.Type).
			Ref("season_scores").
			Field("user_id").
			Required().
			Unique(),
	}
}

func (SeasonScore) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("season_id", "user_id").
			Unique().
			StorageKey("uidx_ss_season_user"),
		index.Fields("season_id", "score_total").
			StorageKey("idx_ss_season_total"), // 排行榜降序查询
	}
}
