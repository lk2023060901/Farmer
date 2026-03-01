package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/liukai/farmer/server/ent"
	entinv "github.com/liukai/farmer/server/ent/inventoryitem"
)

// AddToInventory upserts a quantity of an item for the given user.
// If the item already exists the quantity is incremented; otherwise a new row is created.
func AddToInventory(ctx context.Context, db *ent.Client, userID uuid.UUID, itemID, itemType string, qty int64) error {
	existing, err := db.InventoryItem.Query().
		Where(entinv.UserID(userID), entinv.ItemID(itemID)).
		Only(ctx)
	if err == nil {
		return db.InventoryItem.UpdateOne(existing).AddQuantity(qty).Exec(ctx)
	}
	return db.InventoryItem.Create().
		SetUserID(userID).
		SetItemID(itemID).
		SetItemType(itemType).
		SetQuantity(qty).
		Exec(ctx)
}
