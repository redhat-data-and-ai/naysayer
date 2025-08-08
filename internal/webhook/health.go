package webhook

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/redhat-data-and-ai/naysayer/internal/config"
)

// HealthHandler handles health check requests
type HealthHandler struct {
	config    *config.Config
	startTime time.Time
}

// NewHealthHandler creates a new health handler
func NewHealthHandler(cfg *config.Config) *HealthHandler {
	return &HealthHandler{
		config:    cfg,
		startTime: time.Now(),
	}
}

// HandleHealth returns comprehensive health status
func (h *HealthHandler) HandleHealth(c *fiber.Ctx) error {
	uptime := time.Since(h.startTime)

	health := fiber.Map{
		"status":         "healthy",
		"service":        "naysayer-webhook",
		"version":        "v1.0.0",
		"uptime_seconds": int64(uptime.Seconds()),
		"timestamp":      time.Now().UTC().Format(time.RFC3339),
		"analysis_mode":  h.config.AnalysisMode(),
		"security_mode":  h.config.WebhookSecurityMode(),
		"gitlab_token":   h.config.HasGitLabToken(),
		"webhook_secret": h.config.HasWebhookSecret(),
	}

	return c.JSON(health)
}

// HandleReady returns readiness status for Kubernetes
func (h *HealthHandler) HandleReady(c *fiber.Ctx) error {
	// Check if service is ready to accept traffic
	ready := fiber.Map{
		"ready":          true,
		"service":        "naysayer-webhook",
		"timestamp":      time.Now().UTC().Format(time.RFC3339),
		"gitlab_token":   h.config.HasGitLabToken(),
		"webhook_secret": h.config.HasWebhookSecret(),
	}

	// Check GitLab token
	if !h.config.HasGitLabToken() {
		ready["ready"] = false
		ready["reason"] = "GitLab token not configured"
		return c.Status(503).JSON(ready)
	}

	return c.JSON(ready)
}
