package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// InventoryItem holds the schema definition for the InventoryItem entity.
type InventoryItem struct {
	ent.Schema
}

func (InventoryItem) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New),
		field.UUID("user_id", uuid.UUID{}),
		field.String("item_id").
			MaxLen(64).
			Comment("物品 ID，如 tomato / wheat_seed"),
		field.String("item_type").
			MaxLen(32).
			Comment("crop/seed/animal_product/material/recipe/tool/special/decoration"),
		field.Int64("quantity").
			Default(0).
			Min(0),
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now),
	}
}

func (InventoryItem) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("owner", User.Type).
			Ref("inventory_items").
			Field("user_id").
			Required().
			Unique(),
	}
}

func (InventoryItem) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "item_id").
			Unique().
			StorageKey("uidx_inventory_user_item"),
		index.Fields("user_id", "item_type").
			StorageKey("idx_inventory_user_type"),
	}
}
