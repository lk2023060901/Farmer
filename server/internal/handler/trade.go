package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/liukai/farmer/server/ent"
	entinv "github.com/liukai/farmer/server/ent/inventoryitem"
	entto "github.com/liukai/farmer/server/ent/tradeorder"
	entuser "github.com/liukai/farmer/server/ent/user"
	entvm "github.com/liukai/farmer/server/ent/villagemember"
	"github.com/liukai/farmer/server/internal/service"
	"github.com/liukai/farmer/server/internal/ws"
)

// npcItem represents a single NPC merchant listing.
type npcItem struct {
	ItemID      string `json:"itemId"`
	ItemType    string `json:"itemType"`
	Name        string `json:"name"`
	Description string `json:"description"`
	PriceEach   int64  `json:"priceEach"`
	Quantity    int    `json:"quantity"`
}

// npcInventory is the full pool of NPC items rotated by day-of-week.
// 7 groups of 3 items — one group per weekday (Sunday=0 … Saturday=6).
var npcInventory = [7][3]npcItem{
	// Sunday
	{
		{ItemID: "rainbow_seed", ItemType: "seed", Name: "彩虹种子", Description: "传说中的神奇种子，可长出七彩蔬菜", PriceEach: 800, Quantity: 5},
		{ItemID: "moon_dew", ItemType: "material", Name: "月露", Description: "月夜收集的神秘露水，用于高级合成", PriceEach: 450, Quantity: 10},
		{ItemID: "starfruit_seed", ItemType: "seed", Name: "星果种子", Description: "产出珍贵星果的种子", PriceEach: 600, Quantity: 8},
	},
	// Monday
	{
		{ItemID: "golden_wheat_seed", ItemType: "seed", Name: "黄金小麦种子", Description: "产量是普通小麦3倍的优良品种", PriceEach: 350, Quantity: 15},
		{ItemID: "ancient_fertilizer", ItemType: "material", Name: "远古肥料", Description: "使用后作物品质大幅提升", PriceEach: 500, Quantity: 6},
		{ItemID: "crystal_water", ItemType: "material", Name: "晶莹活泉水", Description: "灌溉后作物成长速度翻倍", PriceEach: 400, Quantity: 10},
	},
	// Tuesday
	{
		{ItemID: "dragon_fruit_seed", ItemType: "seed", Name: "火龙果种子", Description: "稀有热带水果种子", PriceEach: 700, Quantity: 6},
		{ItemID: "lightning_seed", ItemType: "seed", Name: "雷霆种子", Description: "成熟速度极快的神奇种子", PriceEach: 550, Quantity: 8},
		{ItemID: "mystic_compost", ItemType: "material", Name: "秘境堆肥", Description: "来自神秘森林的超级堆肥", PriceEach: 380, Quantity: 12},
	},
	// Wednesday
	{
		{ItemID: "blue_rose_seed", ItemType: "seed", Name: "蓝玫瑰种子", Description: "极其罕见的蓝色玫瑰种子", PriceEach: 900, Quantity: 4},
		{ItemID: "phoenix_feather", ItemType: "material", Name: "凤凰羽毛", Description: "传说凤凰脱落的羽毛，可用于稀有配方", PriceEach: 1200, Quantity: 2},
		{ItemID: "jade_soil", ItemType: "material", Name: "翡翠土", Description: "矿化土壤，种植后产出品质更高", PriceEach: 600, Quantity: 8},
	},
	// Thursday
	{
		{ItemID: "black_truffle_seed", ItemType: "seed", Name: "黑松露种子", Description: "价值连城的稀有菌类种子", PriceEach: 1500, Quantity: 3},
		{ItemID: "bamboo_essence", ItemType: "material", Name: "竹精", Description: "提取自千年翠竹的精华", PriceEach: 700, Quantity: 5},
		{ItemID: "saffron_seed", ItemType: "seed", Name: "藏红花种子", Description: "世界上最贵的香料种子之一", PriceEach: 800, Quantity: 5},
	},
	// Friday
	{
		{ItemID: "spirit_mushroom_seed", ItemType: "seed", Name: "灵芝孢子", Description: "能种出珍贵灵芝的孢子", PriceEach: 650, Quantity: 7},
		{ItemID: "glacier_water", ItemType: "material", Name: "冰川水", Description: "来自遥远冰川的纯净水，浇灌效果极佳", PriceEach: 480, Quantity: 10},
		{ItemID: "aurora_seed", ItemType: "seed", Name: "极光种子", Description: "散发极光色光芒的神奇种子", PriceEach: 750, Quantity: 6},
	},
	// Saturday
	{
		{ItemID: "lava_ore", ItemType: "material", Name: "火山矿石", Description: "从火山口采集的稀有矿石", PriceEach: 850, Quantity: 5},
		{ItemID: "dragon_herb_seed", ItemType: "seed", Name: "龙草种子", Description: "神话中龙最爱吃的草的种子", PriceEach: 950, Quantity: 4},
		{ItemID: "pearl_dew", ItemType: "material", Name: "珍珠露", Description: "由珍珠研磨而成的滋养露水", PriceEach: 560, Quantity: 9},
	},
}

// TradeHandler groups marketplace trade route handlers.
type TradeHandler struct {
	db  *ent.Client
	hub *ws.Hub
}

// NewTradeHandler constructs a TradeHandler.
func NewTradeHandler(db *ent.Client, hub *ws.Hub) *TradeHandler {
	return &TradeHandler{db: db, hub: hub}
}

// orderDTO converts a TradeOrder to a response map.
func orderDTO(o *ent.TradeOrder) map[string]any {
	return map[string]any{
		"id":           o.ID,
		"sellerId":     o.SellerID,
		"itemId":       o.ItemID,
		"itemType":     o.ItemType,
		"quantity":     o.Quantity,
		"quantityLeft": o.QuantityLeft,
		"priceEach":    o.PriceEach,
		"feeRate":      o.FeeRate,
		"scope":        o.Scope,
		"status":       o.Status,
		"listedAt":     o.ListedAt.Format(time.RFC3339),
		"expiresAt":    o.ExpiresAt.Format(time.RFC3339),
	}
}

// ListOrders handles GET /api/v1/trade/orders
// Returns active orders scoped by market type.
//
// Query params:
//   - market_type: "village" (default, same-village only) or "cross_village" (all villages)
//   - mine=true: limit to caller's own listings
func (h *TradeHandler) ListOrders(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "unauthorized", "data": nil})
		return
	}

	marketType := c.DefaultQuery("market_type", "village")
	if marketType != "village" && marketType != "cross_village" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "market_type must be 'village' or 'cross_village'", "data": nil})
		return
	}

	ctx := c.Request.Context()

	var q *ent.TradeOrderQuery
	if marketType == "cross_village" {
		// Cross-village: return all active cross_village-scoped listings
		q = h.db.TradeOrder.Query().
			Where(
				entto.Scope("cross_village"),
				entto.Status("active"),
			).
			Order(ent.Desc(entto.FieldListedAt)).
			Limit(50)
	} else {
		// Village: return active listings belonging to the caller's village
		villageID, err := h.callerVillageID(c, userID)
		if err != nil {
			c.JSON(http.StatusConflict, gin.H{"code": 409, "message": "not in a village", "data": nil})
			return
		}
		q = h.db.TradeOrder.Query().
			Where(
				entto.VillageID(villageID),
				entto.Status("active"),
			).
			Order(ent.Desc(entto.FieldListedAt)).
			Limit(50)
	}

	if c.Query("mine") == "true" {
		q = q.Where(entto.SellerID(userID))
	}

	orders, err := q.All(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "query failed", "data": nil})
		return
	}

	// Build supply-demand hint per item_id.
	// Count active orders by item_id across all fetched orders (same scope).
	itemCounts := make(map[string]int, len(orders))
	for _, o := range orders {
		itemCounts[o.ItemID]++
	}

	result := make([]map[string]any, len(orders))
	for i, o := range orders {
		dto := orderDTO(o)
		cnt := itemCounts[o.ItemID]
		switch {
		case cnt >= 5:
			dto["priceHint"] = "oversupply"
		case cnt <= 1:
			dto["priceHint"] = "scarce"
		default:
			dto["priceHint"] = "normal"
		}
		result[i] = dto
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": result})
}

// createOrderReq is the request body for POST /api/v1/trade/orders
type createOrderReq struct {
	ItemID     string `json:"itemId"      binding:"required"`
	Quantity   int64  `json:"quantity"    binding:"min=1"`
	Price      int64  `json:"price"       binding:"min=1"`
	MarketType string `json:"marketType"` // "village" (default) or "cross_village"
}

// crossVillageFeeRate is the seller fee for cross-village listings (5%).
const crossVillageFeeRate float32 = 0.05

// CreateOrder handles POST /api/v1/trade/orders
// Deducts inventory immediately (escrow) and creates an active listing.
//
// marketType "village" (default): no fee, listing is village-scoped.
// marketType "cross_village": 5% seller fee, listing is visible to all villages.
func (h *TradeHandler) CreateOrder(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "unauthorized", "data": nil})
		return
	}

	var req createOrderReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error(), "data": nil})
		return
	}

	// Normalise and validate market type
	if req.MarketType == "" {
		req.MarketType = "village"
	}
	if req.MarketType != "village" && req.MarketType != "cross_village" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "marketType must be 'village' or 'cross_village'", "data": nil})
		return
	}

	villageID, err := h.callerVillageID(c, userID)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"code": 409, "message": "not in a village", "data": nil})
		return
	}

	ctx := c.Request.Context()

	// Check inventory
	inv, err := h.db.InventoryItem.Query().
		Where(entinv.UserID(userID), entinv.ItemID(req.ItemID)).
		Only(ctx)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "item not in inventory", "data": nil})
		return
	}
	if inv.Quantity < req.Quantity {
		c.JSON(http.StatusConflict, gin.H{"code": 409, "message": "not enough items", "data": nil})
		return
	}

	// Deduct from inventory (escrow)
	if _, err := h.db.InventoryItem.UpdateOne(inv).AddQuantity(-req.Quantity).Save(ctx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "inventory update failed", "data": nil})
		return
	}

	// Determine fee rate and scope
	var feeRate float32
	scope := req.MarketType
	if req.MarketType == "cross_village" {
		feeRate = crossVillageFeeRate
	}

	expiresAt := time.Now().Add(7 * 24 * time.Hour)

	// For cross-village orders, village_id is nil (visible globally).
	// For village orders, we set the caller's village_id.
	orderCreate := h.db.TradeOrder.Create().
		SetSellerID(userID).
		SetScope(scope).
		SetItemID(req.ItemID).
		SetItemType(inv.ItemType).
		SetQuantity(req.Quantity).
		SetQuantityLeft(req.Quantity).
		SetPriceEach(req.Price).
		SetFeeRate(feeRate).
		SetExpiresAt(expiresAt)

	if req.MarketType == "village" {
		orderCreate = orderCreate.SetVillageID(villageID)
	}

	order, err := orderCreate.Save(ctx)
	if err != nil {
		// Rollback inventory deduction
		h.db.InventoryItem.UpdateOne(inv).AddQuantity(req.Quantity).Exec(ctx) //nolint
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "create order failed", "data": nil})
		return
	}

	// WebSocket: notify village members of new listing
	if h.hub != nil {
		h.hub.BroadcastVillage(villageID, &ws.Message{
			Type:      ws.EventTradeNotify,
			VillageID: villageID,
			Payload: &ws.TradeNotifyPayload{
				OrderID:  order.ID.String(),
				Action:   "listed",
				ItemName: req.ItemID,
				Quantity: req.Quantity,
				Price:    req.Price,
			},
		})
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": orderDTO(order)})
}

// buyOrderReq is the request body for POST /api/v1/trade/orders/:id/buy
type buyOrderReq struct {
	Quantity int64 `json:"quantity" binding:"min=1"`
}

// BuyOrder handles POST /api/v1/trade/orders/:id/buy
// Deducts buyer coins, transfers items, records transaction, updates affinity.
func (h *TradeHandler) BuyOrder(c *gin.Context) {
	buyerID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "unauthorized", "data": nil})
		return
	}

	orderID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid id", "data": nil})
		return
	}

	var req buyOrderReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error(), "data": nil})
		return
	}

	ctx := c.Request.Context()

	order, err := h.db.TradeOrder.Get(ctx, orderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "order not found", "data": nil})
		return
	}
	if order.Status != "active" {
		c.JSON(http.StatusConflict, gin.H{"code": 409, "message": "order is not active", "data": nil})
		return
	}
	if order.SellerID == buyerID {
		c.JSON(http.StatusConflict, gin.H{"code": 409, "message": "cannot buy own listing", "data": nil})
		return
	}
	if req.Quantity > order.QuantityLeft {
		c.JSON(http.StatusConflict, gin.H{"code": 409, "message": "not enough stock", "data": nil})
		return
	}

	totalCost := order.PriceEach * req.Quantity

	buyer, err := h.db.User.Query().Where(entuser.ID(buyerID)).Only(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "user query failed", "data": nil})
		return
	}
	if buyer.Coins < totalCost {
		c.JSON(http.StatusConflict, gin.H{"code": 409, "message": "not enough coins", "data": nil})
		return
	}

	// TODO: use DB transaction for atomic check-and-buy
	// The status/quantityLeft check above and the updates below are separate DB
	// round-trips. A concurrent buyer could pass the quantityLeft guard and both
	// purchase the last unit, overselling the listing. Wrapping steps 1-4 in a
	// single DB transaction with a SELECT FOR UPDATE on the order row would
	// eliminate this TOCTOU race condition.

	// ── Atomic updates ────────────────────────────────────────────────────

	// 1. Deduct buyer coins
	if _, err := h.db.User.UpdateOne(buyer).AddCoins(-totalCost).Save(ctx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "coin deduction failed", "data": nil})
		return
	}

	// 2. Credit seller coins (minus fee for cross-village listings)
	seller, _ := h.db.User.Query().Where(entuser.ID(order.SellerID)).Only(ctx)
	if seller != nil {
		fee := int64(float32(totalCost) * order.FeeRate)
		sellerProceeds := totalCost - fee
		h.db.User.UpdateOne(seller).AddCoins(sellerProceeds).Exec(ctx) //nolint
	}

	// 3. Transfer items to buyer's inventory
	if err := service.AddToInventory(ctx, h.db, buyerID, order.ItemID, order.ItemType, req.Quantity); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "inventory transfer failed", "data": nil})
		return
	}

	// 4. Decrement quantity_left; mark sold if exhausted
	newLeft := order.QuantityLeft - req.Quantity
	newStatus := order.Status
	if newLeft == 0 {
		newStatus = "sold"
	}
	if _, err := h.db.TradeOrder.UpdateOne(order).
		SetQuantityLeft(newLeft).
		SetStatus(newStatus).
		Save(ctx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "order update failed", "data": nil})
		return
	}

	// 5. Record transaction (fee = feeRate * totalCost)
	fee := int64(float32(totalCost) * order.FeeRate)
	h.db.TradeTransaction.Create(). //nolint
					SetOrderID(order.ID).
					SetBuyerID(buyerID).
					SetSellerID(order.SellerID).
					SetItemID(order.ItemID).
					SetQuantity(req.Quantity).
					SetPriceEach(order.PriceEach).
					SetFee(fee).
					Exec(ctx)

	// 6. Affinity +5 for both parties (trade bonus)
	service.AddAffinity(ctx, h.db, buyerID, order.SellerID, service.AffinityTrade) //nolint

	// 7. WebSocket: notify seller of sale
	if h.hub != nil {
		h.hub.Send(order.SellerID, &ws.Message{
			Type:    ws.EventTradeNotify,
			UserID:  order.SellerID,
			Payload: &ws.TradeNotifyPayload{
				OrderID:  order.ID.String(),
				Action:   "bought",
				ItemName: order.ItemID,
				Quantity: req.Quantity,
				Price:    order.PriceEach,
			},
		})
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": gin.H{
		"totalCost":    totalCost,
		"quantityLeft": newLeft,
		"status":       newStatus,
	}})
}

// NPCListings handles GET /api/v1/trade/npc-listings
// Returns 3 hardcoded NPC merchant items that rotate based on day-of-week.
// No database interaction is required.
func (h *TradeHandler) NPCListings(c *gin.Context) {
	_, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "unauthorized", "data": nil})
		return
	}

	// Use the day-of-week (Sunday=0 … Saturday=6) to select the item group.
	weekday := int(time.Now().Weekday()) // 0-6
	items := npcInventory[weekday]

	result := make([]npcItem, len(items))
	copy(result, items[:])

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": result})
}

// callerVillageID returns the village the caller belongs to, or an error.
func (h *TradeHandler) callerVillageID(c *gin.Context, userID uuid.UUID) (uuid.UUID, error) {
	mem, err := h.db.VillageMember.Query().
		Where(entvm.UserID(userID)).
		Only(c.Request.Context())
	if err != nil {
		return uuid.UUID{}, err
	}
	return mem.VillageID, nil
}
