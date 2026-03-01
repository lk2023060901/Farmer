package tick

import (
	"context"
	"log"
	"time"

	"github.com/liukai/farmer/server/ent"
	entinv "github.com/liukai/farmer/server/ent/inventoryitem"
	entrole "github.com/liukai/farmer/server/ent/role"
	entto "github.com/liukai/farmer/server/ent/tradeorder"
	entvm "github.com/liukai/farmer/server/ent/villagemember"
	"github.com/liukai/farmer/server/internal/service"
)

const (
	maxDailyTrades    = 5
	minSurplusToTrade = 5   // must have at least this many to auto-list
	autoListQty       = 3   // how many units to list per auto-trade
)

// runAgentTrades processes auto-trade listings for all agents in one tick pass.
// Only agents with strategy_trade >= 4 participate.
func (e *Engine) runAgentTrades(ctx context.Context) {
	agents, err := e.db.Role.Query().
		Where(entrole.StrategyTradeGTE(4)).
		All(ctx)
	if err != nil {
		log.Printf("[tick/trade] query agents: %v", err)
		return
	}
	for _, ag := range agents {
		e.maybeCreateTradeOrder(ctx, ag)
	}
}

// maybeCreateTradeOrder creates a trade listing if the agent has surplus crops
// and has not yet hit the daily trade limit.
func (e *Engine) maybeCreateTradeOrder(ctx context.Context, ag *ent.Role) {
	today := time.Now().Truncate(24 * time.Hour)

	// Count orders already listed today by this user.
	todayCount, err := e.db.TradeOrder.Query().
		Where(
			entto.SellerID(ag.UserID),
			entto.ListedAtGTE(today),
		).
		Count(ctx)
	if err != nil || todayCount >= maxDailyTrades {
		return
	}

	// Find the village this user belongs to (village market only).
	mem, err := e.db.VillageMember.Query().
		Where(entvm.UserID(ag.UserID)).
		Only(ctx)
	if err != nil {
		return // not in a village
	}

	// Find surplus crop inventory items (quantity >= minSurplusToTrade).
	items, err := e.db.InventoryItem.Query().
		Where(
			entinv.UserID(ag.UserID),
			entinv.ItemType("crop"),
			entinv.QuantityGTE(minSurplusToTrade),
		).
		Limit(1).
		All(ctx)
	if err != nil || len(items) == 0 {
		return
	}

	item := items[0]
	crop := service.GetCrop(item.ItemID)
	if crop == nil {
		return
	}

	// Price = 80% of the coin reward value (slightly below market to encourage sales).
	price := int64(float64(crop.CoinReward) * 0.8)
	if price < 1 {
		price = 1
	}

	// Deduct inventory first.
	if err := e.db.InventoryItem.UpdateOne(item).
		AddQuantity(-autoListQty).
		Exec(ctx); err != nil {
		return
	}

	// Create trade order.
	_, err = e.db.TradeOrder.Create().
		SetSellerID(ag.UserID).
		SetVillageID(mem.VillageID).
		SetScope("village").
		SetItemID(item.ItemID).
		SetItemType("crop").
		SetQuantity(int64(autoListQty)).
		SetQuantityLeft(int64(autoListQty)).
		SetPriceEach(price).
		SetFeeRate(0).
		SetStatus("active").
		SetListedAt(time.Now()).
		SetExpiresAt(time.Now().Add(24 * time.Hour)).
		Save(ctx)
	if err != nil {
		// Rollback inventory deduction on failure.
		_ = e.db.InventoryItem.UpdateOne(item).AddQuantity(autoListQty).Exec(ctx)
		log.Printf("[tick/trade] create order for agent %s: %v", ag.Name, err)
		return
	}

	log.Printf("[tick/trade] %s listed %d×%s at %d coins each", ag.Name, autoListQty, item.ItemID, price)
}
