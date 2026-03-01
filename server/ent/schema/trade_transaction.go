package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// TradeTransaction holds the schema definition for the TradeTransaction entity.
type TradeTransaction struct {
	ent.Schema
}

func (TradeTransaction) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New),
		field.UUID("order_id", uuid.UUID{}),
		field.UUID("buyer_id", uuid.UUID{}),
		field.UUID("seller_id", uuid.UUID{}),
		field.String("item_id").
			MaxLen(64),
		field.Int64("quantity").
			Min(1),
		field.Int64("price_each").
			Min(0),
		field.Int64("fee").
			Default(0).
			Min(0).
			Comment("手续费（金币）"),
		field.Time("created_at").
			Default(time.Now).
			Immutable(),
	}
}

func (TradeTransaction) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("order", TradeOrder.Type).
			Ref("transactions").
			Field("order_id").
			Required().
			Unique(),
		edge.From("buyer", User.Type).
			Ref("purchases").
			Field("buyer_id").
			Required().
			Unique(),
		edge.From("seller", User.Type).
			Ref("sales").
			Field("seller_id").
			Required().
			Unique(),
	}
}

func (TradeTransaction) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("order_id").StorageKey("idx_tt_order"),
		index.Fields("buyer_id").StorageKey("idx_tt_buyer"),
		index.Fields("seller_id").StorageKey("idx_tt_seller"),
	}
}
