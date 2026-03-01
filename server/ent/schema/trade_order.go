package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// TradeOrder holds the schema definition for the TradeOrder entity (集市上架订单).
type TradeOrder struct {
	ent.Schema
}

func (TradeOrder) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New),
		field.UUID("seller_id", uuid.UUID{}),
		field.UUID("village_id", uuid.UUID{}).
			Optional().
			Nillable().
			Comment("村内交易=本村；跨村=null"),
		field.String("scope").
			MaxLen(16).
			Comment("village/cross_village"),
		field.String("item_id").
			MaxLen(64),
		field.String("item_type").
			MaxLen(32),
		field.String("quality").
			MaxLen(16).
			Optional().
			Nillable().
			Comment("normal/good/excellent"),
		field.Int64("quantity").
			Min(1).
			Comment("总上架数"),
		field.Int64("quantity_left").
			Min(0).
			Comment("剩余数量"),
		field.Int64("price_each").
			Min(1).
			Comment("单价（金币）"),
		field.Float32("fee_rate").
			Default(0).
			Comment("手续费率（0 或 0.05）"),
		field.String("status").
			MaxLen(16).
			Default("active").
			Comment("active/sold/expired/cancelled"),
		field.Time("listed_at").
			Default(time.Now),
		field.Time("expires_at").
			Comment("默认 7 天后"),
	}
}

func (TradeOrder) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("seller", User.Type).
			Ref("trade_orders").
			Field("seller_id").
			Required().
			Unique(),
		edge.To("transactions", TradeTransaction.Type),
	}
}

func (TradeOrder) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("seller_id").StorageKey("idx_to_seller"),
		index.Fields("village_id", "status", "listed_at").StorageKey("idx_to_village_status"),
		index.Fields("status", "expires_at").StorageKey("idx_to_status_expires"),
	}
}
