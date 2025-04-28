package app

import (
	"github.com/gofiber/fiber/v2"
)

func SetupApp(handlerRegistry map[string][]Handler) *fiber.App {
	app := fiber.New()

	// Attach the handler registry to the Fiber app instance
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("handlerRegistry", handlerRegistry)
		return c.Next()
	})

	// Register routes
	SetupRoutes(app)

	return app
}
