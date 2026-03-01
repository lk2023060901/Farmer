// Package handler provides health check and readiness probes for T-095 disaster recovery.
package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/liukai/farmer/server/ent"
)

// HealthHandler provides liveness and readiness probes.
type HealthHandler struct {
	db    *ent.Client
	start time.Time
}

// NewHealthHandler constructs a HealthHandler.
func NewHealthHandler(db *ent.Client) *HealthHandler {
	return &HealthHandler{db: db, start: time.Now()}
}

// Liveness handles GET /healthz
// Returns 200 if the process is alive (always true if responding).
func (h *HealthHandler) Liveness(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"uptime": time.Since(h.start).String(),
	})
}

// Readiness handles GET /readyz
// Returns 200 only when the DB is reachable (used by load balancer health checks).
func (h *HealthHandler) Readiness(c *gin.Context) {
	ctx := c.Request.Context()
	if err := h.db.Schema.Create(ctx); err != nil {
		// If schema.Create fails (e.g. DB unreachable), mark as not ready.
		// In production, use a simpler ping: h.db.User.Query().Count(ctx)
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "unavailable",
			"error":  "database unreachable",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ready"})
}
