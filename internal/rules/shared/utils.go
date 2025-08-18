package shared

import (
	"strings"
)

// IsDataProductFile checks if a file is a dataproduct configuration file
func IsDataProductFile(path string) bool {
	if path == "" {
		return false
	}

	lowerPath := strings.ToLower(path)
	return strings.HasSuffix(lowerPath, "product.yaml") || strings.HasSuffix(lowerPath, "product.yml")
}

// IsMigrationFile checks if a file is a migration file
func IsMigrationFile(path string) bool {
	if path == "" {
		return false
	}

	lowerPath := strings.ToLower(path)
	return strings.Contains(lowerPath, "/migrations/") &&
		(strings.HasSuffix(lowerPath, ".sql") || strings.HasSuffix(lowerPath, ".yaml") || strings.HasSuffix(lowerPath, ".yml"))
}

// GetEnvironmentFromPath extracts environment from file path
func GetEnvironmentFromPath(filePath string) string {
	// Extract from path like: dataproducts/source/product-name/env/product.yaml
	parts := strings.Split(filePath, "/")
	if len(parts) >= 4 {
		return parts[3] // env position
	}
	return "unknown"
}

// GetDataProductFromPath extracts data product name from file path
func GetDataProductFromPath(filePath string) string {
	parts := strings.Split(filePath, "/")
	if len(parts) >= 3 {
		return parts[2] // product name position
	}
	return "unknown"
}

// Constants for file validation
const (
	// File extensions
	YAMLExt = ".yaml"
	YMLExt  = ".yml"
	SQLExt  = ".sql"

	// Path patterns
	ProductFileName = "product.yaml"
	MigrationsPath  = "/migrations/"
)
