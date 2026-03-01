package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/liukai/farmer/server/ent"
	entinv "github.com/liukai/farmer/server/ent/inventoryitem"
)

// ShopHandler groups in-game shop route handlers.
type ShopHandler struct {
	db *ent.Client
}

// NewShopHandler constructs a ShopHandler.
func NewShopHandler(db *ent.Client) *ShopHandler { return &ShopHandler{db: db} }

// ── General seed shop (static catalog) ───────────────────────────────────────

type seedShopItem struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Cost    int64  `json:"cost"` // coins
	ItemID  string `json:"itemId"`
	Stock   int    `json:"stock"` // -1 = unlimited
}

var seedCatalog = []seedShopItem{
	{ID: "seed_wheat",    Name: "小麦种子",   Cost: 10,  ItemID: "seed_wheat",    Stock: -1},
	{ID: "seed_carrot",   Name: "胡萝卜种子", Cost: 15,  ItemID: "seed_carrot",   Stock: -1},
	{ID: "seed_potato",   Name: "土豆种子",   Cost: 20,  ItemID: "seed_potato",   Stock: -1},
	{ID: "seed_tomato",   Name: "番茄种子",   Cost: 25,  ItemID: "seed_tomato",   Stock: -1},
	{ID: "seed_corn",     Name: "玉米种子",   Cost: 30,  ItemID: "seed_corn",     Stock: -1},
	{ID: "seed_pumpkin",  Name: "南瓜种子",   Cost: 40,  ItemID: "seed_pumpkin",  Stock: -1},
	{ID: "seed_sunflower",Name: "向日葵种子", Cost: 50,  ItemID: "seed_sunflower",Stock: -1},
	{ID: "seed_strawberry",Name: "草莓种子",  Cost: 80,  ItemID: "seed_strawberry",Stock: 10},
}

// ListItems handles GET /api/v1/shop/items
func (h *ShopHandler) ListItems(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": seedCatalog})
}

type buyItemReq struct {
	Quantity int `json:"quantity" binding:"required,min=1"`
}

// BuyItem handles POST /api/v1/shop/items/:id/buy
func (h *ShopHandler) BuyItem(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "unauthorized", "data": nil})
		return
	}
	itemID := c.Param("id")
	var item *seedShopItem
	for i := range seedCatalog {
		if seedCatalog[i].ID == itemID {
			item = &seedCatalog[i]
			break
		}
	}
	if item == nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "item not found", "data": nil})
		return
	}

	var req buyItemReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid request", "data": nil})
		return
	}

	ctx := c.Request.Context()
	total := item.Cost * int64(req.Quantity)
	u, err := h.db.User.Get(ctx, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "user not found", "data": nil})
		return
	}
	if u.Coins < total {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "insufficient coins", "data": nil})
		return
	}
	if err := h.db.User.UpdateOneID(userID).AddCoins(-total).Exec(ctx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "deduct failed", "data": nil})
		return
	}

	// Upsert inventory.
	existing, err := h.db.InventoryItem.Query().
		Where(entinv.UserID(userID), entinv.ItemID(item.ItemID)).
		Only(ctx)
	if ent.IsNotFound(err) {
		_, err = h.db.InventoryItem.Create().
			SetUserID(userID).
			SetItemID(item.ItemID).
			SetItemType("seed").
			SetQuantity(int64(req.Quantity)).
			Save(ctx)
	} else if err == nil {
		err = h.db.InventoryItem.UpdateOneID(existing.ID).AddQuantity(int64(req.Quantity)).Exec(ctx)
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "inventory update failed", "data": nil})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": gin.H{
		"itemId":   item.ItemID,
		"quantity": req.Quantity,
		"cost":     total,
	}})
}

// ── T-062: Friendship shop ────────────────────────────────────────────────────

// friendshipItem is an item in the friendship exchange shop.
type friendshipItem struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Cost        int    `json:"cost"`   // friendship_points
	ItemID      string `json:"itemId"` // inventory item_id granted
	ItemType    string `json:"itemType"`
	Stock       int    `json:"stock"` // restock period in days
}

// friendshipCatalog is the static friendship shop catalog.
// Refreshes weekly (tracked by cycle = floor(unixDay / 7)).
var friendshipCatalog = []friendshipItem{
	{ID: "fs_seed_strawberry", Name: "草莓种子 x5",    Description: "稀有种子，限量供应", Cost: 50,  ItemID: "seed_strawberry", ItemType: "seed",    Stock: 5},
	{ID: "fs_seed_blueberry",  Name: "蓝莓种子 x3",    Description: "高价值浆果",         Cost: 80,  ItemID: "seed_blueberry",  ItemType: "seed",    Stock: 3},
	{ID: "fs_deco_windmill",   Name: "风车装饰",        Description: "农场装饰物",         Cost: 100, ItemID: "deco_windmill",   ItemType: "decoration", Stock: 1},
	{ID: "fs_deco_fountain",   Name: "喷泉装饰",        Description: "让农场更美丽",       Cost: 150, ItemID: "deco_fountain",   ItemType: "decoration", Stock: 1},
	{ID: "fs_pet_rabbit",      Name: "宠物兔子优惠券",  Description: "购买兔子享折扣",     Cost: 200, ItemID: "coupon_rabbit",   ItemType: "coupon",  Stock: 2},
}

// currentFriendshipCycle returns the current weekly cycle number.
func currentFriendshipCycle() int {
	return int(time.Now().UTC().Unix()) / (7 * 24 * 3600)
}

// ListFriendshipShop handles GET /api/v1/shop/friendship
func (h *ShopHandler) ListFriendshipShop(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "unauthorized", "data": nil})
		return
	}
	u, err := h.db.User.Get(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "user not found", "data": nil})
		return
	}
	cycle := currentFriendshipCycle()
	c.JSON(http.StatusOK, gin.H{
		"code": 0, "message": "ok",
		"data": gin.H{
			"balance": u.FriendshipPoints,
			"cycle":   cycle,
			"items":   friendshipCatalog,
		},
	})
}

type exchangeReq struct {
	ItemID   string `json:"itemId" binding:"required"`
	Quantity int    `json:"quantity" binding:"required,min=1"`
}

// ExchangeFriendship handles POST /api/v1/shop/friendship/exchange
func (h *ShopHandler) ExchangeFriendship(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "unauthorized", "data": nil})
		return
	}
	var req exchangeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid request", "data": nil})
		return
	}

	var item *friendshipItem
	for i := range friendshipCatalog {
		if friendshipCatalog[i].ID == req.ItemID {
			item = &friendshipCatalog[i]
			break
		}
	}
	if item == nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "item not found", "data": nil})
		return
	}
	if req.Quantity > item.Stock {
		req.Quantity = item.Stock
	}

	ctx := c.Request.Context()
	total := item.Cost * req.Quantity
	u, err := h.db.User.Get(ctx, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "user not found", "data": nil})
		return
	}
	if u.FriendshipPoints < total {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "insufficient friendship points", "data": nil})
		return
	}
	if err := h.db.User.UpdateOneID(userID).AddFriendshipPoints(-total).Exec(ctx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "deduct failed", "data": nil})
		return
	}

	// Upsert inventory.
	existing, err := h.db.InventoryItem.Query().
		Where(entinv.UserID(userID), entinv.ItemID(item.ItemID)).
		Only(ctx)
	if ent.IsNotFound(err) {
		_, err = h.db.InventoryItem.Create().
			SetUserID(userID).
			SetItemID(item.ItemID).
			SetItemType(item.ItemType).
			SetQuantity(int64(req.Quantity)).
			Save(ctx)
	} else if err == nil {
		err = h.db.InventoryItem.UpdateOneID(existing.ID).AddQuantity(int64(req.Quantity)).Exec(ctx)
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "inventory update failed", "data": nil})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": gin.H{
		"itemId":    item.ItemID,
		"quantity":  req.Quantity,
		"cost":      total,
		"remaining": u.FriendshipPoints - total,
	}})
}
