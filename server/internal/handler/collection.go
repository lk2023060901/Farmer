package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/liukai/farmer/server/ent"
	entinv "github.com/liukai/farmer/server/ent/inventoryitem"
)

// CollectionHandler handles encyclopedia/collection endpoints.
type CollectionHandler struct {
	db *ent.Client
}

// NewCollectionHandler constructs a CollectionHandler.
func NewCollectionHandler(db *ent.Client) *CollectionHandler { return &CollectionHandler{db: db} }

// collectionEntry represents a single item in the collection encyclopedia.
type collectionEntry struct {
	ItemID      string `json:"itemId"`
	Name        string `json:"name"`
	Category    string `json:"category"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
	Obtained    bool   `json:"obtained"`
}

// collectionCatalog is the master list of all collectable items grouped by
// category.  Adding an entry here makes it automatically appear in the
// encyclopedia; obtaining it in-game (i.e. having >0 in inventory) marks it
// as collected.
var collectionCatalog = []collectionEntry{
	// ── Crops (作物) ──────────────────────────────────────────────────────
	{ItemID: "turnip", Name: "萝卜", Category: "crop", Description: "常见的根茎蔬菜，春季作物", Icon: "🌱"},
	{ItemID: "potato", Name: "土豆", Category: "crop", Description: "营养丰富的块茎植物", Icon: "🥔"},
	{ItemID: "wheat", Name: "小麦", Category: "crop", Description: "面包和饲料的重要原料", Icon: "🌾"},
	{ItemID: "carrot", Name: "胡萝卜", Category: "crop", Description: "富含维生素的橙色蔬菜", Icon: "🥕"},
	{ItemID: "tomato", Name: "番茄", Category: "crop", Description: "红彤彤的果实，酸甜可口", Icon: "🍅"},
	{ItemID: "corn", Name: "玉米", Category: "crop", Description: "金黄饱满的夏日作物", Icon: "🌽"},
	{ItemID: "strawberry", Name: "草莓", Category: "crop", Description: "鲜红欲滴的浆果，香气四溢", Icon: "🍓"},
	{ItemID: "pumpkin", Name: "南瓜", Category: "crop", Description: "圆润饱满的秋季作物", Icon: "🎃"},

	// ── Animal products (动物) ────────────────────────────────────────────
	{ItemID: "milk", Name: "牛奶", Category: "animal", Description: "新鲜的农场牛奶", Icon: "🥛"},
	{ItemID: "egg", Name: "鸡蛋", Category: "animal", Description: "母鸡每天产的新鲜鸡蛋", Icon: "🥚"},
	{ItemID: "wool", Name: "羊毛", Category: "animal", Description: "柔软温暖的绵羊毛", Icon: "🧶"},
	{ItemID: "honey", Name: "蜂蜜", Category: "animal", Description: "蜜蜂酿造的香甜蜂蜜", Icon: "🍯"},

	// ── Processed products (料理) ─────────────────────────────────────────
	{ItemID: "bread", Name: "面包", Category: "product", Description: "用小麦烘焙的香软面包", Icon: "🍞"},
	{ItemID: "cheese", Name: "奶酪", Category: "product", Description: "醇厚浓郁的手工奶酪", Icon: "🧀"},
	{ItemID: "jam", Name: "草莓酱", Category: "product", Description: "香甜浓稠的草莓果酱", Icon: "🫙"},
	{ItemID: "flower_bouquet", Name: "花束", Category: "product", Description: "精心扎制的精美花束", Icon: "💐"},
	{ItemID: "cake", Name: "蛋糕", Category: "product", Description: "层层叠叠的精致奶油蛋糕", Icon: "🎂"},

	// ── Decorations (装饰) ────────────────────────────────────────────────
	{ItemID: "skin_spring", Name: "农场皮肤·春日", Category: "decoration", Description: "让农场焕发春天气息的皮肤", Icon: "🌸"},
	{ItemID: "skin_harvest", Name: "农场皮肤·丰收", Category: "decoration", Description: "金秋丰收的温暖色调皮肤", Icon: "🍂"},
	{ItemID: "agent_panda", Name: "Agent外观·熊猫", Category: "decoration", Description: "可爱熊猫管家外观", Icon: "🐼"},
	{ItemID: "agent_rabbit", Name: "Agent外观·兔子", Category: "decoration", Description: "活泼兔子管家外观", Icon: "🐰"},
	{ItemID: "skin_windmill", Name: "风车建筑皮肤", Category: "decoration", Description: "荷兰风格风车造型皮肤", Icon: "🌬️"},
	{ItemID: "effect_fountain", Name: "喷泉特效", Category: "decoration", Description: "庭院喷泉水花动画特效", Icon: "⛲"},
}

// validCategories lists the accepted category query values.
var validCategories = map[string]bool{
	"crop":       true,
	"animal":     true,
	"product":    true,
	"decoration": true,
}

// ListCollection handles GET /api/v1/collection
// Query param: ?category=crop|animal|product|decoration (optional, omit for all)
// Returns all items in the category with obtained/not-obtained status.
func (h *CollectionHandler) ListCollection(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "unauthorized", "data": nil})
		return
	}

	category := c.Query("category")
	if category != "" && !validCategories[category] {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 400, "message": "invalid category; use crop|animal|product|decoration", "data": nil,
		})
		return
	}

	ctx := c.Request.Context()

	// Collect the item IDs we need to check in inventory.
	wantIDs := make([]string, 0, len(collectionCatalog))
	for _, e := range collectionCatalog {
		if category == "" || e.Category == category {
			wantIDs = append(wantIDs, e.ItemID)
		}
	}

	// Query inventory in one shot.
	items, err := h.db.InventoryItem.Query().
		Where(entinv.UserID(userID), entinv.ItemIDIn(wantIDs...)).
		All(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "inventory query failed", "data": nil})
		return
	}
	obtained := make(map[string]bool, len(items))
	for _, it := range items {
		if it.Quantity > 0 {
			obtained[it.ItemID] = true
		}
	}

	// Build response.
	result := make([]collectionEntry, 0, len(wantIDs))
	for _, e := range collectionCatalog {
		if category != "" && e.Category != category {
			continue
		}
		entry := e // copy
		entry.Obtained = obtained[e.ItemID]
		result = append(result, entry)
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": result})
}

// GetProgress handles GET /api/v1/collection/progress
// Returns completion percentage per category and total.
func (h *CollectionHandler) GetProgress(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "unauthorized", "data": nil})
		return
	}

	ctx := c.Request.Context()

	// All item IDs we care about.
	allIDs := make([]string, 0, len(collectionCatalog))
	for _, e := range collectionCatalog {
		allIDs = append(allIDs, e.ItemID)
	}

	items, err := h.db.InventoryItem.Query().
		Where(entinv.UserID(userID), entinv.ItemIDIn(allIDs...)).
		All(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "inventory query failed", "data": nil})
		return
	}
	obtained := make(map[string]bool, len(items))
	for _, it := range items {
		if it.Quantity > 0 {
			obtained[it.ItemID] = true
		}
	}

	// Tally per-category counts.
	type catStats struct {
		Total    int     `json:"total"`
		Obtained int     `json:"obtained"`
		Percent  float64 `json:"percent"`
	}
	byCategory := make(map[string]*catStats)
	for _, e := range collectionCatalog {
		if _, ok := byCategory[e.Category]; !ok {
			byCategory[e.Category] = &catStats{}
		}
		byCategory[e.Category].Total++
		if obtained[e.ItemID] {
			byCategory[e.Category].Obtained++
		}
	}
	for _, s := range byCategory {
		if s.Total > 0 {
			s.Percent = float64(s.Obtained) / float64(s.Total) * 100
		}
	}

	// Overall totals.
	totalItems := len(collectionCatalog)
	obtainedItems := len(obtained)
	overallPercent := 0.0
	if totalItems > 0 {
		overallPercent = float64(obtainedItems) / float64(totalItems) * 100
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": gin.H{
		"total":      totalItems,
		"obtained":   obtainedItems,
		"percent":    overallPercent,
		"categories": byCategory,
	}})
}
