package main

import (
	"log"
	"math/rand"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/redhat-data-and-ai/naysayer/api/routes"
	"github.com/redhat-data-and-ai/naysayer/pkg/config"
)

func main() {
	// Load configuration
	cfg := config.LoadConfig()
	
	// Seed random for naysayer responses
	rand.Seed(time.Now().UnixNano())
	
	// Create Fiber app with configuration
	app := fiber.New(fiber.Config{
		AppName: "NAYSAYER v1.0.0-phase1",
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
			}
			log.Printf("Error: %v", err)
			return c.Status(code).JSON(fiber.Map{
				"error": err.Error(),
			})
		},
	})

	// Middleware
	app.Use(recover.New())
	app.Use(logger.New(logger.Config{
		Format: "${time} ${status} - ${method} ${path} - ${latency}\n",
	}))
	app.Use(cors.New())

	// Naysayer responses (legacy endpoint)
	nayList := []string{
		"no",
		"nope",
		"nah",
		"not",
		"nay",
		"not at all",
		"nada",
		"don't even think about it",
		"nothing doing",
		"actually... no",
		"platform approval required 🚫",
		"TOC approval needed 📋",

		"warehouse increase detected 📊",
		"warehouse decrease only = auto-merge ✅",
		"mixed changes = platform approval 🚫",
		"separate your warehouse decreases! 📦",
	}

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString(nayList[rand.Intn(len(nayList))])
	})

	// Health check at root level
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":      "healthy",
			"service":     "naysayer",
			"version":     "1.0.0-phase1",
			"description": "Self-service approval bot for dataproduct-config",
			"config": fiber.Map{
				"gitlab_base_url":          cfg.GitLabBaseURL,
				"dataproduct_repo":         cfg.DataProductRepo,
				"gitlab_api_enabled":       cfg.EnableActualGitLabAPI,
				"log_level":                cfg.LogLevel,
				
			},
			"features": []string{
				"Warehouse decrease auto-merge policy",
				"Warehouse size change detection (XSMALL→XXLARGE)",
				"Mock diff analysis (Phase 1)",
			},
		})
	})

	// Data Product Config routes
	dataProductConfig := app.Group("/dataproductconfig")
	routes.DataProductConfigRouterWithConfig(dataProductConfig, cfg)

	// Also create backward compatible routes
	routes.DataProductConfigRouter(dataProductConfig)

	log.Printf("🚀 NAYSAYER starting on port %s", cfg.Port)
	log.Printf("📊 Monitoring repository: %s", cfg.DataProductRepo)
	log.Printf("🔧 GitLab API enabled: %t", cfg.EnableActualGitLabAPI)
	log.Printf("📝 Log level: %s", cfg.LogLevel)
	log.Printf("📦 Focus: Warehouse field changes only")
	
	log.Fatal(app.Listen(":" + cfg.Port))
}
