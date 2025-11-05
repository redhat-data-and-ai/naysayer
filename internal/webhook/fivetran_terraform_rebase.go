package webhook

import (
	fiber "github.com/gofiber/fiber/v2"
	"github.com/redhat-data-and-ai/naysayer/internal/config"
)

// FivetranTerraformRebaseHandler handles Fivetran Terraform rebase requests
type FivetranTerraformRebaseHandler struct {
	config *config.Config
}

// NewFivetranTerraformRebaseHandler creates a new Fivetran Terraform rebase handler
func NewFivetranTerraformRebaseHandler(cfg *config.Config) *FivetranTerraformRebaseHandler {
	return &FivetranTerraformRebaseHandler{config: cfg}
}

// HandleWebhook handles Fivetran Terraform rebase requests
func (h *FivetranTerraformRebaseHandler) HandleWebhook(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"message": "Fivetran Terraform rebase request received",
	})
}
