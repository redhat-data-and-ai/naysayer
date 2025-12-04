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
		FilePath:       filePath,
		Environment:    r.extractEnvironmentFromPath(filePath),
		HasConsumers:   false,
		IsConsumerOnly: false,
	}

	// Check if file contains consumers section
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

// findConsumerSections finds all consumer sections in the YAML file using yaml.Node API
func (r *DataProductConsumerRule) findConsumerSections(filePath string, fileContent string) []shared.LineRange {
	var ranges []shared.LineRange

	// Parse YAML into Node structure for accurate line tracking
	var node yaml.Node
	if err := yaml.Unmarshal([]byte(fileContent), &node); err != nil {
		return ranges
	}

	// Find all "consumers" nodes in the YAML tree
	r.findConsumersNodes(&node, filePath, &ranges)

	return ranges
}

// findConsumersNodes recursively traverses the YAML node tree to find all "consumers" keys
func (r *DataProductConsumerRule) findConsumersNodes(node *yaml.Node, filePath string, ranges *[]shared.LineRange) {
	if node == nil {
		return
	}

	// Process mapping nodes (key-value pairs)
	if node.Kind == yaml.MappingNode {
		// Mapping nodes have content in pairs: [key1, value1, key2, value2, ...]
		for i := 0; i < len(node.Content); i += 2 {
			keyNode := node.Content[i]
			valueNode := node.Content[i+1]

			// Check if this is a "consumers" key
			if keyNode.Value == "consumers" && valueNode.Kind == yaml.SequenceNode {
				// Found a consumers section - extract line range
				startLine := keyNode.Line
				endLine := valueNode.Line

				// For sequences, find the last line
				if len(valueNode.Content) > 0 {
					lastItem := valueNode.Content[len(valueNode.Content)-1]
					endLine = r.getNodeEndLine(lastItem)
				}

				*ranges = append(*ranges, shared.LineRange{
					StartLine: startLine,
					EndLine:   endLine,
					FilePath:  filePath,
				})
			}

			// Recursively search in value node
			r.findConsumersNodes(valueNode, filePath, ranges)
		}
	}

	// Process sequence nodes (arrays)
	if node.Kind == yaml.SequenceNode {
		for _, item := range node.Content {
			r.findConsumersNodes(item, filePath, ranges)
		}
	}

	// Document nodes contain the root content
	if node.Kind == yaml.DocumentNode {
		for _, item := range node.Content {
			r.findConsumersNodes(item, filePath, ranges)
		}
	}
}

// getNodeEndLine recursively finds the last line number in a YAML node
func (r *DataProductConsumerRule) getNodeEndLine(node *yaml.Node) int {
	if node == nil {
		return 0
	}

	endLine := node.Line

	// Check all content nodes and find the maximum line number
	for _, child := range node.Content {
		childEndLine := r.getNodeEndLine(child)
		if childEndLine > endLine {
			endLine = childEndLine
		}
	}

	return endLine
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
