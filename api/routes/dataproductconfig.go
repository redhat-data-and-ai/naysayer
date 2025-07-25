package routes

import (
	"github.com/gofiber/fiber/v2"
	"github.com/redhat-data-and-ai/naysayer/api/handlers"
	"github.com/redhat-data-and-ai/naysayer/pkg/config"
)

func DataProductConfigRouter(app fiber.Router) {
	// Health check endpoint
	app.Get("/health", handlers.HealthCheck())
	
	// Webhook endpoint for GitLab MR events
	app.Post("/review-mr", handlers.ReviewMR())
}

// DataProductConfigRouterWithConfig creates routes with custom config
func DataProductConfigRouterWithConfig(app fiber.Router, cfg *config.Config) {
	// Create webhook handler with config
	webhookHandler := handlers.NewWebhookHandler(cfg)
	
	// Health check endpoint
	app.Get("/health", handlers.HealthCheck())
	
	// Webhook endpoint for GitLab MR events
	app.Post("/review-mr", webhookHandler.ReviewMR())
}
