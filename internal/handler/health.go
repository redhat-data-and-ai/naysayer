package handler

import (
	"github.com/gofiber/fiber/v2"
	"github.com/redhat-data-and-ai/naysayer/internal/config"
)

// HealthHandler handles health check requests
type HealthHandler struct {
	config *config.Config
}

// NewHealthHandler creates a new health handler
func NewHealthHandler(cfg *config.Config) *HealthHandler {
	return &HealthHandler{config: cfg}
}

// HandleHealth returns health status
func (h *HealthHandler) HandleHealth(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"status":         "healthy",
		"service":        "naysayer-dataproduct-config",
		"version":        "yaml-analysis",
		"analysis_mode":  h.config.AnalysisMode(),
		"gitlab_token":   h.config.HasGitLabToken(),
	})
}