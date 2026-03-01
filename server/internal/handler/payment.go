package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/liukai/farmer/server/ent"
)

// PaymentHandler groups payment / billing route handlers.
type PaymentHandler struct {
	db *ent.Client
}

// NewPaymentHandler constructs a PaymentHandler.
func NewPaymentHandler(db *ent.Client) *PaymentHandler { return &PaymentHandler{db: db} }

// diamondPackage describes a recharge tier.
type diamondPackage struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	AmountCents  int    `json:"amountCents"`  // 价格（分）
	Diamonds     int    `json:"diamonds"`     // 发放钻石数
	BonusDiamonds int   `json:"bonusDiamonds"` // 赠送钻石（首充加成在回调时单独处理）
}

// 5 recharge tiers (PRD §8.6)
var diamondPackages = []diamondPackage{
	{ID: "pkg_6yuan",   Name: "小袋钻石",   AmountCents: 600,   Diamonds: 60,   BonusDiamonds: 0},
	{ID: "pkg_18yuan",  Name: "中袋钻石",   AmountCents: 1800,  Diamonds: 188,  BonusDiamonds: 10},
	{ID: "pkg_30yuan",  Name: "大袋钻石",   AmountCents: 3000,  Diamonds: 328,  BonusDiamonds: 20},
	{ID: "pkg_68yuan",  Name: "豪华钻石",   AmountCents: 6800,  Diamonds: 780,  BonusDiamonds: 50},
	{ID: "pkg_128yuan", Name: "至尊钻石",   AmountCents: 12800, Diamonds: 1580, BonusDiamonds: 100},
}

// firstChargeBonus is the extra diamond grant for the very first purchase.
const firstChargeBonus = 100

type createPaymentOrderReq struct {
	PackageID string `json:"packageId" binding:"required"`
}

// CreateOrder handles POST /api/v1/payment/orders
// Authenticated: creates a PaymentOrder and returns a (mocked) prepay_id.
// In production, call the WeChat Pay unified-order API here.
func (h *PaymentHandler) CreateOrder(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "unauthorized", "data": nil})
		return
	}
	var req createPaymentOrderReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid request", "data": nil})
		return
	}

	var pkg *diamondPackage
	for i := range diamondPackages {
		if diamondPackages[i].ID == req.PackageID {
			pkg = &diamondPackages[i]
			break
		}
	}
	if pkg == nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "package not found", "data": nil})
		return
	}

	ctx := c.Request.Context()
	order, err := h.db.PaymentOrder.Create().
		SetUserID(userID).
		SetPackageID(pkg.ID).
		SetAmountCents(pkg.AmountCents).
		SetDiamondsToGrant(pkg.Diamonds).
		SetStatus("pending").
		Save(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "create order failed", "data": nil})
		return
	}

	// In production: call wx.requestPayment with real prepay_id from WeChat Pay API.
	// Here we generate a mock prepay_id for development/testing.
	mockPrepayID := "mock_prepay_" + order.ID.String()

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": gin.H{
		"orderId":     order.ID,
		"packageId":   pkg.ID,
		"amountCents": pkg.AmountCents,
		"diamonds":    pkg.Diamonds,
		"prepayId":    mockPrepayID,
	}})
}

// ListPackages handles GET /api/v1/payment/packages
// Returns all available recharge tiers.
func (h *PaymentHandler) ListPackages(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": diamondPackages})
}

type wxCallbackBody struct {
	// In production: parse WeChat Pay XML notification fields.
	// For the stub, we accept a JSON body with order_id + transaction_id.
	OrderID       string `json:"orderId"`
	TransactionID string `json:"transactionId"`
	ResultCode    string `json:"resultCode"` // "SUCCESS" | "FAIL"
}

// WxCallback handles POST /api/v1/payment/wx-callback
// Public endpoint: called by the WeChat Pay gateway (or during testing).
// Grants diamonds to the user upon successful payment.
func (h *PaymentHandler) WxCallback(c *gin.Context) {
	var body wxCallbackBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid body", "data": nil})
		return
	}

	if body.ResultCode != "SUCCESS" {
		c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ignored", "data": nil})
		return
	}

	orderID, err := uuid.Parse(body.OrderID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid orderId", "data": nil})
		return
	}

	ctx := c.Request.Context()
	order, err := h.db.PaymentOrder.Get(ctx, orderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "order not found", "data": nil})
		return
	}
	if order.Status != "pending" {
		// Idempotent: already processed.
		c.JSON(http.StatusOK, gin.H{"code": 0, "message": "already processed", "data": nil})
		return
	}

	// Mark order paid.
	now := time.Now()
	if err := h.db.PaymentOrder.UpdateOneID(orderID).
		SetStatus("paid").
		SetWxTransactionID(body.TransactionID).
		SetPaidAt(now).
		Exec(ctx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "update order failed", "data": nil})
		return
	}

	// Grant diamonds; check first-charge bonus.
	user, err := h.db.User.Get(ctx, order.UserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "user not found", "data": nil})
		return
	}

	diamondsGranted := order.DiamondsToGrant
	upd := h.db.User.UpdateOneID(order.UserID).AddDiamonds(diamondsGranted)

	// T-074: first-charge bonus — extra 100 diamonds + set flag
	if !user.IsFirstCharge {
		diamondsGranted += firstChargeBonus
		upd = upd.AddDiamonds(firstChargeBonus).SetIsFirstCharge(true)
	}

	if err := upd.Exec(ctx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "grant diamonds failed", "data": nil})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": gin.H{
		"orderId":         orderID,
		"diamondsGranted": diamondsGranted,
	}})
}
