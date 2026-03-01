package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/liukai/farmer/server/ent"
	entinv "github.com/liukai/farmer/server/ent/inventoryitem"
	entuser "github.com/liukai/farmer/server/ent/user"
)

// InventoryHandler groups inventory route handlers.
type InventoryHandler struct {
	db *ent.Client
}

// NewInventoryHandler constructs an InventoryHandler.
func NewInventoryHandler(db *ent.Client) *InventoryHandler { return &InventoryHandler{db: db} }

// GetInventory handles GET /api/v1/inventory
func (h *InventoryHandler) GetInventory(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "unauthorized", "data": nil})
		return
	}

	items, err := h.db.InventoryItem.Query().
		Where(entinv.UserID(userID)).
		Order(ent.Asc(entinv.FieldItemType), ent.Asc(entinv.FieldItemID)).
		All(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "query failed", "data": nil})
		return
	}

	type itemDTO struct {
		ID       uuid.UUID `json:"id"`
		ItemID   string    `json:"itemId"`
		ItemType string    `json:"itemType"`
		Quantity int64     `json:"quantity"`
	}
	result := make([]itemDTO, 0, len(items))
	for _, it := range items {
		if it.Quantity > 0 {
			result = append(result, itemDTO{
				ID:       it.ID,
				ItemID:   it.ItemID,
				ItemType: it.ItemType,
				Quantity: it.Quantity,
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": result})
}

// sellReq is the request body for POST /api/v1/inventory/sell
type sellReq struct {
	ItemID   string `json:"itemId"   binding:"required"`
	Quantity int64  `json:"quantity" binding:"min=1"`
}

// SellItem handles POST /api/v1/inventory/sell
func (h *InventoryHandler) SellItem(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "unauthorized", "data": nil})
		return
	}

	var req sellReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error(), "data": nil})
		return
	}

	item, err := h.db.InventoryItem.Query().
		Where(entinv.UserID(userID), entinv.ItemID(req.ItemID)).
		Only(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "item not found", "data": nil})
		return
	}
	if item.Quantity < req.Quantity {
		c.JSON(http.StatusConflict, gin.H{
			"code": 409, "message": "not enough items",
			"data": gin.H{"have": item.Quantity, "want": req.Quantity},
		})
		return
	}

	totalCoins := int64(cropSellPrice(req.ItemID)) * req.Quantity

	_, err = h.db.InventoryItem.UpdateOne(item).AddQuantity(-req.Quantity).Save(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "inventory update failed", "data": nil})
		return
	}

	u, err := h.db.User.Query().Where(entuser.ID(userID)).Only(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "user lookup failed", "data": nil})
		return
	}
	u, err = h.db.User.UpdateOne(u).AddCoins(totalCoins).Save(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "coin grant failed", "data": nil})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0, "message": "sold",
		"data": gin.H{
			"itemId": req.ItemID, "quantity": req.Quantity,
			"totalCoins": totalCoins, "newCoins": u.Coins,
		},
	})
}

// cropSellPrice returns market price per unit. Falls back to 5.
func cropSellPrice(itemID string) int {
	prices := map[string]int{
		"turnip": 8, "potato": 14, "wheat": 20, "carrot": 24,
		"tomato": 40, "corn": 60, "strawberry": 85, "pumpkin": 150,
	}
	if p, ok := prices[itemID]; ok {
		return p
	}
	return 5
}

