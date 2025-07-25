package main

import (
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"

	"github.com/redhat-data-and-ai/naysayer/internal/config"
	"github.com/redhat-data-and-ai/naysayer/internal/webhook"
)

func main() {
	// Load configuration
	cfg := config.Load()

	if !cfg.HasGitLabToken() {
		log.Printf("⚠️  Warning: GITLAB_TOKEN not set - file analysis will be limited")
	}

	// Create handlers
	webhookHandler := webhook.NewWebhookHandler(cfg)
	healthHandler := webhook.NewHealthHandler(cfg)
	managementHandler := webhook.NewManagementHandler(cfg)

	// Create Fiber app
	app := fiber.New(fiber.Config{
		AppName: "NAYSAYER Webhook v1.0.0",
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			log.Printf("Error: %v", err)
			return c.Status(500).JSON(fiber.Map{
				"error": "Internal server error",
			})
		},
	})

	// Core middleware
	app.Use(recover.New())
	app.Use(logger.New(logger.Config{
		Format: "${time} ${status} - ${method} ${path} - ${latency}\n",
	}))
	app.Use(cors.New())

	// Health and monitoring routes
	app.Get("/health", healthHandler.HandleHealth)
	app.Get("/ready", healthHandler.HandleReady)

	// Management API routes
	app.Get("/api/system", managementHandler.HandleSystemInfo)
	app.Get("/api/rules", managementHandler.HandleRules)
	app.Get("/api/rules/enabled", managementHandler.HandleRulesEnabled)
	app.Get("/api/rules/category/:category", managementHandler.HandleRulesByCategory)

	// Webhook routes
	app.Post("/dataverse-product-config-review", webhookHandler.HandleWebhook)

	log.Printf("🚀 NAYSAYER Webhook starting on port %s", cfg.Server.Port)
	log.Printf("📁 Analysis mode: %s", cfg.AnalysisMode())
	log.Printf("🔒 Webhook security: %s", cfg.WebhookSecurityMode())
	log.Fatal(app.Listen(":" + cfg.Server.Port))
}
