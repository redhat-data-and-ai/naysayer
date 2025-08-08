package warehouse

// WarehouseChange represents a detected warehouse size change
type WarehouseChange struct {
	FilePath   string
	FromSize   string
	ToSize     string
	IsDecrease bool
}

// ValidationResult represents warehouse validation outcome
type ValidationResult struct {
	IsValid      bool
	Issues       []string
	RequiresTOC  bool
	RequiresPlatform bool
}

// WarehouseSizes maps warehouse size names to numeric values for comparison
// Complete Snowflake warehouse size hierarchy
var WarehouseSizes = map[string]int{
	"XSMALL":  1,  // X-Small
	"SMALL":   2,  // Small
	"MEDIUM":  3,  // Medium
	"LARGE":   4,  // Large
	"XLARGE":  5,  // X-Large
	"XXLARGE": 6,  // 2X-Large
	"X3LARGE": 7,  // 3X-Large
	"X4LARGE": 8,  // 4X-Large
	"X5LARGE": 9,  // 5X-Large
	"X6LARGE": 10, // 6X-Large
}
