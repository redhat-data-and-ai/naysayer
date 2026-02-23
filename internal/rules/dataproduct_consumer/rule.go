package dataproduct_consumer

import (
	"strings"

	"github.com/redhat-data-and-ai/naysayer/internal/rules/common"
	"github.com/redhat-data-and-ai/naysayer/internal/rules/shared"
	"gopkg.in/yaml.v3"
)

// DataProductConsumerRule validates consumer access changes to data products
// Consumers can be added without TOC approval as long as data product owner approves
// Consumer access can be granted across any environment (dev, sandbox, preprod, prod)
type DataProductConsumerRule struct {
	*common.BaseRule
	*common.FileTypeMatcher
	*common.ValidationHelper
	config *DataProductConsumerConfig
}

// NewDataProductConsumerRule creates a new data product consumer rule instance
func NewDataProductConsumerRule(allowedEnvs []string) *DataProductConsumerRule {
	config := DefaultDataProductConsumerConfig()
	if allowedEnvs != nil {
		config.AllowedEnvironments = allowedEnvs
	}

	return &DataProductConsumerRule{
		BaseRule:         common.NewBaseRule("dataproduct_consumer_rule", "Auto-approves consumer access changes to data products in allowed environments (preprod/prod)"),
		FileTypeMatcher:  common.NewFileTypeMatcher(),
		ValidationHelper: common.NewValidationHelper(),
		config:           config,
	}
}

// ValidateLines validates lines for consumer access changes
func (r *DataProductConsumerRule) ValidateLines(filePath string, fileContent string, lineRanges []shared.LineRange) (shared.DecisionType, string) {
	// Only apply to product.yaml files
	if !r.IsProductFile(filePath) {
		return r.CreateApprovalResult("Not a product.yaml file - consumer rule does not apply")
	}

	// Analyze the context for this file
	context := r.analyzeFile(filePath, fileContent, lineRanges)

	// Check for self-consumer configuration - this requires manual review
	if context.IsSelfConsumer {
		return shared.ManualReview, "Self-consumer detected: data product '" + context.SelfConsumerName + "' cannot be added as a consumer of itself - manual review required"
	}

	// Auto-approve consumer-only changes across all environments
	// Data product owner approval is sufficient, no TOC approval required
	if context.HasConsumers && context.IsConsumerOnly {
		if context.Environment != "" {
			return r.CreateApprovalResult("Consumer access changes in " + context.Environment + " environment - data product owner approval sufficient (no TOC approval required)")
		}
		return r.CreateApprovalResult("Consumer access changes - data product owner approval sufficient (no TOC approval required)")
	}

	// Not a consumer-only change, let other rules handle it
	return r.CreateApprovalResult("No consumer-only changes detected")
}

// GetCoveredLines returns line ranges this rule covers
func (r *DataProductConsumerRule) GetCoveredLines(filePath string, fileContent string) []shared.LineRange {
	// Only cover product.yaml files
	if !r.IsProductFile(filePath) {
		return []shared.LineRange{}
	}

	// Check if file has content
	if len(strings.TrimSpace(fileContent)) == 0 {
		return []shared.LineRange{}
	}

	// For section-based validation, we return a placeholder range to indicate
	// this rule wants to participate in validation. The actual section content
	// (data_product_db) will be provided by the section manager.
	return []shared.LineRange{
		{
			StartLine: 1,
			EndLine:   1,
		},
	}
}

// analyzeFile analyzes a file to determine if consumer rule applies
func (r *DataProductConsumerRule) analyzeFile(filePath string, fileContent string, lineRanges []shared.LineRange) *ConsumerContext {
	context := &ConsumerContext{
		FilePath:         filePath,
		Environment:      r.extractEnvironmentFromPath(filePath),
		HasConsumers:     false,
		IsConsumerOnly:   false,
		IsSelfConsumer:   false,
		SelfConsumerName: "",
	}

	// Check for self-consumer configuration FIRST (independent of HasConsumers)
	// This uses extractConsumersFromContent which handles both full file and section content
	isSelfConsumer, selfConsumerName := r.detectSelfConsumer(filePath, fileContent)
	context.IsSelfConsumer = isSelfConsumer
	context.SelfConsumerName = selfConsumerName

	// Check if file contains consumers section (for consumer-only change detection)
	context.HasConsumers = r.fileContainsConsumersSection(fileContent)
	if !context.HasConsumers {
		return context
	}

	// Check if ONLY consumer fields are being modified
	context.IsConsumerOnly = r.areChangesConsumerOnly(lineRanges, fileContent)

	return context
}

// fileContainsConsumersSection checks if the file has a consumers section
func (r *DataProductConsumerRule) fileContainsConsumersSection(fileContent string) bool {
	// Parse YAML to check for consumers field
	var yamlContent map[string]interface{}
	if err := yaml.Unmarshal([]byte(fileContent), &yamlContent); err != nil {
		return false
	}

	// Check for consumers in data_product_db.presentation_schemas
	if dataProductDB, ok := yamlContent["data_product_db"].([]interface{}); ok {
		for _, db := range dataProductDB {
			if dbMap, ok := db.(map[string]interface{}); ok {
				if schemas, ok := dbMap["presentation_schemas"].([]interface{}); ok {
					for _, schema := range schemas {
						if schemaMap, ok := schema.(map[string]interface{}); ok {
							if _, hasConsumers := schemaMap["consumers"]; hasConsumers {
								return true
							}
						}
					}
				}
			}
		}
	}

	return false
}

// detectSelfConsumer checks if the product is added as a consumer of itself
// Returns (isSelfConsumer, productName) - only checks consumers with kind: data_product
func (r *DataProductConsumerRule) detectSelfConsumer(filePath string, fileContent string) (bool, string) {
	// Try to get product name from YAML 'name' field first (full file content)
	var yamlContent map[string]interface{}
	productName := ""
	if err := yaml.Unmarshal([]byte(fileContent), &yamlContent); err == nil {
		if name, ok := yamlContent["name"].(string); ok {
			productName = name
		}
	}

	// Fall back to extracting product name from file path
	if productName == "" {
		productName = r.extractProductNameFromPath(filePath)
		if productName == "" {
			return false, ""
		}
	}

	// Check consumers for self-reference - handle both full file and section-only content
	consumers := r.extractConsumersFromContent(fileContent)
	for _, consumer := range consumers {
		if consumerMap, ok := consumer.(map[string]interface{}); ok {
			consumerName, _ := consumerMap["name"].(string)
			consumerKind, _ := consumerMap["kind"].(string)
			// Only flag as self-consumer if kind is data_product
			if consumerName == productName && consumerKind == "data_product" {
				return true, consumerName
			}
		}
	}

	return false, ""
}

// extractProductNameFromPath extracts the product name from the file path
// Path format: dataproducts/<type>/<productname>/<env>/product.yaml
func (r *DataProductConsumerRule) extractProductNameFromPath(filePath string) string {
	// Normalize path separators
	normalizedPath := strings.ReplaceAll(filePath, "\\", "/")
	parts := strings.Split(normalizedPath, "/")

	// Find "dataproducts" in the path and get the product name
	// Format: dataproducts/<type>/<productname>/<env>/product.yaml
	for i, part := range parts {
		if part == "dataproducts" && i+3 < len(parts) {
			// parts[i+1] = type (source, aggregate, platform)
			// parts[i+2] = product name
			return parts[i+2]
		}
	}

	return ""
}

// extractConsumersFromContent extracts consumers from YAML content string
// Handles both full file content and section-only content (data_product_db section)
func (r *DataProductConsumerRule) extractConsumersFromContent(fileContent string) []interface{} {
	var allConsumers []interface{}

	// First, try to parse as a map (full file content or map-style data_product_db)
	var yamlMap map[string]interface{}
	if err := yaml.Unmarshal([]byte(fileContent), &yamlMap); err == nil {
		// Check if data_product_db is an array (common format)
		if dataProductDB, ok := yamlMap["data_product_db"].([]interface{}); ok {
			allConsumers = r.extractConsumersFromDBArray(dataProductDB)
		}
		// Check if data_product_db is a map (alternative format)
		if len(allConsumers) == 0 {
			if dataProductDB, ok := yamlMap["data_product_db"].(map[string]interface{}); ok {
				allConsumers = r.extractConsumersFromDBMap(dataProductDB)
			}
		}
		// Check if this is section content that is a map with presentation_schemas directly
		if len(allConsumers) == 0 {
			allConsumers = r.extractConsumersFromDBMap(yamlMap)
		}
	}

	// If no consumers found, try parsing as a direct array (section content)
	// Section content format: [{ database: x, presentation_schemas: [{ consumers: [...] }] }]
	if len(allConsumers) == 0 {
		var yamlArray []interface{}
		if err := yaml.Unmarshal([]byte(fileContent), &yamlArray); err == nil {
			allConsumers = r.extractConsumersFromDBArray(yamlArray)
		}
	}

	return allConsumers
}

// extractConsumersFromDBArray extracts consumers from the data_product_db array structure
func (r *DataProductConsumerRule) extractConsumersFromDBArray(dbArray []interface{}) []interface{} {
	var allConsumers []interface{}

	for _, db := range dbArray {
		if dbMap, ok := db.(map[string]interface{}); ok {
			allConsumers = append(allConsumers, r.extractConsumersFromDBMap(dbMap)...)
		}
	}

	return allConsumers
}

// extractConsumersFromDBMap extracts consumers from a data_product_db map structure
func (r *DataProductConsumerRule) extractConsumersFromDBMap(dbMap map[string]interface{}) []interface{} {
	var allConsumers []interface{}

	if schemas, ok := dbMap["presentation_schemas"].([]interface{}); ok {
		for _, schema := range schemas {
			if schemaMap, ok := schema.(map[string]interface{}); ok {
				if consumers, ok := schemaMap["consumers"].([]interface{}); ok {
					allConsumers = append(allConsumers, consumers...)
				}
			}
		}
	}

	return allConsumers
}

// areChangesConsumerOnly checks if only consumer-related lines are being modified
func (r *DataProductConsumerRule) areChangesConsumerOnly(lineRanges []shared.LineRange, fileContent string) bool {
	if len(lineRanges) == 0 {
		return false
	}

	lines := strings.Split(fileContent, "\n")

	for _, lr := range lineRanges {
		for lineNum := lr.StartLine; lineNum <= lr.EndLine && lineNum <= len(lines); lineNum++ {
			line := strings.TrimSpace(lines[lineNum-1])

			// Skip empty lines and comments
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}

			// Check if line is consumer-related
			if !r.isConsumerRelatedLine(line) {
				// Found a non-consumer change
				return false
			}
		}
	}

	return true
}

// isConsumerRelatedLine checks if a line is related to consumer configuration
func (r *DataProductConsumerRule) isConsumerRelatedLine(line string) bool {
	line = strings.TrimSpace(line)

	// Consumer-related keywords
	consumerKeywords := []string{
		"consumers:",
		"- name:",
		"kind:",
	}

	for _, keyword := range consumerKeywords {
		if strings.Contains(line, keyword) {
			return true
		}
	}

	return false
}

// extractEnvironmentFromPath attempts to extract the environment name from the file path
func (r *DataProductConsumerRule) extractEnvironmentFromPath(filePath string) string {
	lowerPath := strings.ToLower(filePath)

	for _, env := range r.config.AllowedEnvironments {
		lowerEnv := strings.ToLower(env)
		if strings.Contains(lowerPath, "/"+lowerEnv+"/") ||
			strings.Contains(lowerPath, "/"+lowerEnv+"_") ||
			strings.Contains(lowerPath, "_"+lowerEnv+"/") ||
			strings.Contains(lowerPath, "_"+lowerEnv+"_") {
			return env
		}
	}

	// Check for other common environments (dev, sandbox) to detect non-allowed envs
	otherEnvs := []string{"dev", "sandbox", "platformtest"}
	for _, env := range otherEnvs {
		if strings.Contains(lowerPath, "/"+env+"/") ||
			strings.Contains(lowerPath, "/"+env+"_") ||
			strings.Contains(lowerPath, "_"+env+"/") ||
			strings.Contains(lowerPath, "_"+env+"_") {
			return env
		}
	}

	return ""
}
