package main

import (
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"

	"github.com/redhat-data-and-ai/naysayer/internal/config"
	"github.com/redhat-data-and-ai/naysayer/internal/handler"
)

func main() {
	// Load configuration
	cfg := config.Load()

	if !cfg.HasGitLabToken() {
		log.Printf("⚠️  Warning: GITLAB_TOKEN not set - file analysis will be limited")
	}

	// Create handlers
	webhookHandler := handler.NewWebhookHandler(cfg)
	healthHandler := handler.NewHealthHandler(cfg)

	// Create Fiber app
	app := fiber.New(fiber.Config{
		AppName: "NAYSAYER Dataproduct Config",
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			log.Printf("Error: %v", err)
			return c.Status(500).JSON(fiber.Map{
				"error": "Internal server error",
			})
		},
	})

	// Basic middleware
	app.Use(logger.New())
	app.Use(cors.New())

	// Routes
	app.Get("/health", healthHandler.HandleHealth)
	app.Post("/webhook", webhookHandler.HandleWebhook)

	log.Printf("🚀 NAYSAYER Dataproduct Config starting on port %s", cfg.Server.Port)
	log.Printf("📁 Analysis mode: %s", cfg.AnalysisMode())
	log.Fatal(app.Listen(":" + cfg.Server.Port))
}