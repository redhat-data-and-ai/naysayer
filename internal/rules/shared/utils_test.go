package shared

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsDataProductFile_Extended(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		// Additional edge cases not covered in types_test.go
		{"whitespace path", "   ", false},
		{"path with special characters", "dataproducts/test-env/product.yaml", true},
		{"unicode in path", "dataproducts/测试/product.yaml", true},
		{"very long path", "dataproducts/" + string(make([]byte, 100)) + "/product.yaml", true},
		{"path with null bytes", "dataproducts/test\x00/product.yaml", true},
		{"case variations", "DATAPRODUCTS/TEST/PRODUCT.YAML", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsDataProductFile(tt.path)
			assert.Equal(t, tt.expected, result, "IsDataProductFile(%q) = %v, want %v", tt.path, result, tt.expected)
		})
	}
}

func TestIsMigrationFile(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		// Valid migration files
		{"SQL migration", "dataproducts/analytics/migrations/001_create_table.sql", true},
		{"YAML migration", "dataproducts/source/platform/migrations/002_update_schema.yaml", true},
		{"YML migration", "dataproducts/test/migrations/003_add_column.yml", true},
		{"uppercase SQL", "dataproducts/test/migrations/004_DROP_TABLE.SQL", true},
		{"uppercase YAML", "dataproducts/test/migrations/005_CONFIG.YAML", true},
		{"nested migrations", "dataproducts/source/deep/nested/migrations/006_complex.sql", true},
		{"migrations in root", "migrations/007_init.sql", false}, // The function requires /migrations/ not just migrations/

		// Invalid migration files
		{"not in migrations directory", "dataproducts/analytics/schema.sql", false},
		{"wrong extension", "dataproducts/test/migrations/008_script.sh", false},
		{"migrations as filename not directory", "dataproducts/test/migrations.sql", false},
		{"partial migrations path", "dataproducts/migration/script.sql", false},
		{"README in migrations", "dataproducts/test/migrations/README.md", false},
		{"config file", "config/database.yaml", false},
		{"product file", "dataproducts/test/product.yaml", false},

		// Edge cases
		{"empty path", "", false},
		{"whitespace path", "   ", false},
		{"path with spaces", "data products/test/migrations/001 create table.sql", true},
		{"multiple migrations dirs", "dataproducts/test/migrations/sub/migrations/script.sql", true},
		{"migrations with special chars", "dataproducts/test-env/migrations/001_test-script.sql", true},
		{"unicode in migration path", "dataproducts/测试/migrations/001_创建表.sql", true},
		{"case sensitive migrations", "dataproducts/test/Migrations/001_script.sql", true}, // Function is case insensitive
		{"migrations at end", "dataproducts/test/db_migrations/001_script.sql", false},     // Must be exactly "/migrations/" not "migrations" at end
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsMigrationFile(tt.path)
			assert.Equal(t, tt.expected, result, "IsMigrationFile(%q) = %v, want %v", tt.path, result, tt.expected)
		})
	}
}

func TestUtilsFunctions_EdgeCasesAndPerformance(t *testing.T) {
	// Test with very long paths
	longPath := "dataproducts/" + string(make([]byte, 1000)) + "/product.yaml"
	assert.True(t, IsDataProductFile(longPath))

	// Test with paths containing null bytes (should handle gracefully)
	pathWithNull := "dataproducts/test\x00/product.yaml"
	assert.True(t, IsDataProductFile(pathWithNull))

	// Test multiple calls to ensure no side effects
	testPath := "dataproducts/analytics/product.yaml"
	for i := 0; i < 100; i++ {
		assert.True(t, IsDataProductFile(testPath))
	}

	// Test migration file with multiple extensions
	migrationPath := "dataproducts/test/migrations/script.sql.backup"
	assert.False(t, IsMigrationFile(migrationPath)) // Function checks suffix, not contains

	// Test case sensitivity thoroughly
	assert.True(t, IsDataProductFile("dataproducts/test/product.yaml"))
	assert.True(t, IsDataProductFile("dataproducts/test/PRODUCT.YAML"))
	assert.True(t, IsDataProductFile("DATAPRODUCTS/TEST/PRODUCT.YAML"))

	assert.True(t, IsMigrationFile("dataproducts/test/migrations/script.sql"))
	assert.True(t, IsMigrationFile("dataproducts/test/migrations/SCRIPT.SQL"))
	assert.True(t, IsMigrationFile("dataproducts/test/MIGRATIONS/script.sql")) // Function is case insensitive
}

func TestUtilsFunctions_RealWorldPaths(t *testing.T) {
	// Real-world paths that should be recognized
	realWorldProductPaths := []string{
		"dataproducts/aggregate/bookingsmaster/prod/product.yaml",
		"dataproducts/source/marketo/sandbox/product.yaml",
		"dataproducts/aggregate/costops/dev/product.yml",
		"dataproducts/source/fivetranplatform/sandbox/product.yaml",
	}

	for _, path := range realWorldProductPaths {
		t.Run("real_world_product_"+path, func(t *testing.T) {
			assert.True(t, IsDataProductFile(path))
		})
	}

	// Real-world migration paths
	realWorldMigrationPaths := []string{
		"dataproducts/aggregate/bookingsmaster/migrations/001_initial_schema.sql",
		"dataproducts/source/marketo/migrations/002_add_indexes.yaml",
		"dataproducts/aggregate/costops/migrations/003_update_views.yml",
	}

	for _, path := range realWorldMigrationPaths {
		t.Run("real_world_migration_"+path, func(t *testing.T) {
			assert.True(t, IsMigrationFile(path))
		})
	}

	// Paths that should NOT be recognized
	nonProductPaths := []string{
		"README.md",
		"dataproducts/aggregate/bookingsmaster/developers.yaml",
		"dataproducts/source/marketo/promotion_checklist.md",
		"config/deployment.yaml",
		"scripts/deploy.sh",
	}

	for _, path := range nonProductPaths {
		t.Run("non_product_"+path, func(t *testing.T) {
			assert.False(t, IsDataProductFile(path))
		})
	}
}
