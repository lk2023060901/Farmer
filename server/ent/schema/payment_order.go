package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// PaymentOrder holds the schema definition for the PaymentOrder entity (充值订单).
type PaymentOrder struct {
	ent.Schema
}

func (PaymentOrder) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New),
		field.UUID("user_id", uuid.UUID{}),
		field.String("package_id").
			MaxLen(32).
			Comment("pkg_6yuan 等"),
		field.Int("amount_cents").
			Min(1).
			Comment("价格（分）"),
		field.Int("diamonds_to_grant").
			Min(0).
			Comment("应发钻石数"),
		field.String("status").
			MaxLen(16).
			Default("pending").
			Comment("pending/paid/refunded/failed"),
		field.String("wx_prepay_id").
			MaxLen(128).
			Optional().
			Nillable(),
		field.String("wx_transaction_id").
			MaxLen(64).
			Optional().
			Nillable().
			Unique().
			Comment("微信流水号"),
		field.Time("paid_at").
			Optional().
			Nillable(),
		field.Time("created_at").
			Default(time.Now).
			Immutable(),
	}
}

func (PaymentOrder) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("payment_orders").
			Field("user_id").
			Required().
			Unique(),
	}
}

func (PaymentOrder) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "created_at").StorageKey("idx_po_user"),
	}
}
