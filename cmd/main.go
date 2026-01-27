package main

import (
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"

	"github.com/redhat-data-and-ai/naysayer/internal/config"
	"github.com/redhat-data-and-ai/naysayer/internal/logging"
	"github.com/redhat-data-and-ai/naysayer/internal/webhook"
)

func setupRoutes(app *fiber.App, cfg *config.Config) {
	// Core middleware
	app.Use(recover.New())
	app.Use(logger.New(logger.Config{
		Format: "${time} ${status} - ${method} ${path} - ${latency}\n",
	}))
	app.Use(cors.New())

	// Create handlers
	dataProductConfigMrReviewHandler := webhook.NewDataProductConfigMrReviewHandler(cfg)
	healthHandler := webhook.NewHealthHandler(cfg)
	autoRebaseHandler := webhook.NewAutoRebaseHandler(cfg)
	staleMRCleanupHandler := webhook.NewStaleMRCleanupHandler(cfg)

	// Health and monitoring routes
	app.Get("/health", healthHandler.HandleHealth)
	app.Get("/ready", healthHandler.HandleReady)

	// Webhook routes
	app.Post("/dataverse-product-config-review", dataProductConfigMrReviewHandler.HandleWebhook)

	// Auto-rebase route (generic, reusable)
	app.Post("/auto-rebase", autoRebaseHandler.HandleWebhook)

	// Stale MR cleanup routes
	app.Post("/stale-mr-cleanup", staleMRCleanupHandler.HandleWebhook)
}

func main() {
	// Initialize configuration
	cfg := config.Load()

	// Initialize logging
	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "info"
	}
	logging.InitLogger(logLevel, "NAYSAYER")

	// Validate GitLab configuration
	if !cfg.HasGitLabToken() {
		logging.Warn("GITLAB_TOKEN not set - file analysis will be limited")
	}

	// Create Fiber app
	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			logging.Error("Fiber error: %v", err)
			return c.Status(500).JSON(fiber.Map{
				"error": "Internal server error",
			})
		},
	})

	// Add routes
	setupRoutes(app, cfg)

	// Start server
	port := cfg.Server.Port
	logging.Info("NAYSAYER Webhook starting on port %s", port)
	logging.Info("Analysis mode: %s", cfg.AnalysisMode())
	logging.Info("Webhook security: %s", cfg.WebhookSecurityMode())

	if err := app.Listen(":" + port); err != nil {
		logging.Error("Failed to start server: %v", err)
		os.Exit(1)
	}
}
