package app

import (
	"fmt"
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/redhat-data-and-ai/gitlab-bot-backend/library"
)

// SetupRoutes registers all the routes for the application
func SetupRoutes(app *fiber.App) {
	// route that can handle gitlab webhook
	app.Post("/gitlab-webhook", func(c *fiber.Ctx) error {

		var event MergeRequestEvent
		if err := c.BodyParser(&event); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid payload",
			})
		}

		fmt.Println("Received event:", event)

		gitLabClient := library.NewGitLabClient(os.Getenv("GITLAB_BASE_URL"), os.Getenv("GITLAB_TOKEN"))

		handlerRegistry := c.Locals("handlerRegistry").(map[string][]Handler)
		for _, handler := range handlerRegistry[event.ObjectKind] {
			if err := handler.Handle(event, *gitLabClient); err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error": "Failed to process event",
				})
			}
		}

		return c.JSON(fiber.Map{
			"message": "Event not handled",
		})
	})
}
