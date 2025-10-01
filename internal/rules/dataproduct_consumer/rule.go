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

	// Get consumer sections from YAML
	consumerRanges := r.findConsumerSections(filePath, fileContent)
	if len(consumerRanges) == 0 {
		return []shared.LineRange{}
	}

	return consumerRanges
}

// analyzeFile analyzes a file to determine if consumer rule applies
func (r *DataProductConsumerRule) analyzeFile(filePath string, fileContent string, lineRanges []shared.LineRange) *ConsumerContext {
	context := &ConsumerContext{
		FilePath:       filePath,
		Environment:    r.extractEnvironmentFromPath(filePath),
		HasConsumers:   false,
		IsConsumerOnly: false,
		RequiresReview: false,
	}

	// Check if changes are consumer-related
	context.HasConsumers = r.hasConsumerChanges(fileContent, lineRanges)
	if !context.HasConsumers {
		return context
	}

	// Check if ONLY consumer fields are being modified
	context.IsConsumerOnly = r.areChangesConsumerOnly(lineRanges, fileContent)

	return context
}

// hasConsumerChanges checks if the file has consumer-related changes
func (r *DataProductConsumerRule) hasConsumerChanges(fileContent string, lineRanges []shared.LineRange) bool {
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

// findConsumerSections finds all consumer sections in the YAML file
func (r *DataProductConsumerRule) findConsumerSections(filePath string, fileContent string) []shared.LineRange {
	var ranges []shared.LineRange
	lines := strings.Split(fileContent, "\n")

	inConsumers := false
	startLine := 0
	indent := 0

	for i, line := range lines {
		lineNum := i + 1
		trimmed := strings.TrimSpace(line)

		// Find start of consumers section
		if strings.HasPrefix(trimmed, "consumers:") {
			inConsumers = true
			startLine = lineNum
			indent = len(line) - len(strings.TrimLeft(line, " "))
			continue
		}

		// If in consumers section, check for end
		if inConsumers {
			currentIndent := len(line) - len(strings.TrimLeft(line, " "))

			// End of consumers section if we hit a line with same or less indentation
			if trimmed != "" && currentIndent <= indent && !strings.HasPrefix(trimmed, "-") && !strings.HasPrefix(trimmed, "name:") && !strings.HasPrefix(trimmed, "kind:") {
				ranges = append(ranges, shared.LineRange{
					StartLine: startLine,
					EndLine:   lineNum - 1,
					FilePath:  filePath,
				})
				inConsumers = false
			}
		}
	}

	// Handle case where consumers section goes to end of file
	if inConsumers {
		ranges = append(ranges, shared.LineRange{
			StartLine: startLine,
			EndLine:   len(lines),
			FilePath:  filePath,
		})
	}

	return ranges
}

// isEnvironmentAllowed checks if the environment allows consumer access
func (r *DataProductConsumerRule) isEnvironmentAllowed(environment string) bool {
	if environment == "" {
		return false
	}

	for _, allowedEnv := range r.config.AllowedEnvironments {
		if r.config.CaseSensitive {
			if environment == allowedEnv {
				return true
			}
		} else {
			if strings.EqualFold(environment, allowedEnv) {
				return true
			}
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
