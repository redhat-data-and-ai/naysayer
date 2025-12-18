package rules

import (
	"fmt"
	"strings"

	"github.com/redhat-data-and-ai/naysayer/internal/config"
	"github.com/redhat-data-and-ai/naysayer/internal/logging"
	"github.com/redhat-data-and-ai/naysayer/internal/rules/shared"
	"gopkg.in/yaml.v3"
)

// YAMLSectionParser parses YAML files into logical sections
type YAMLSectionParser struct {
	sectionDefinitions map[string]config.SectionDefinition
	filePath           string
}

// NewYAMLSectionParser creates a new YAML section parser
func NewYAMLSectionParser(definitions map[string]config.SectionDefinition) *YAMLSectionParser {
	return &YAMLSectionParser{
		sectionDefinitions: definitions,
	}
}

// ParseSections extracts sections from YAML content based on definitions
func (p *YAMLSectionParser) ParseSections(filePath string, content string) ([]shared.Section, error) {
	p.filePath = filePath

	// Parse YAML with line number tracking
	var yamlNode yaml.Node
	if err := yaml.Unmarshal([]byte(content), &yamlNode); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	var sections []shared.Section
	contentLines := strings.Split(content, "\n")

	// Extract sections based on definitions
	for _, definition := range p.sectionDefinitions {

		section, err := p.extractSection(definition, &yamlNode, contentLines)
		if err != nil {
			if definition.Required {
				return nil, fmt.Errorf("required section %s not found: %w", definition.Name, err)
			}
			// Optional section not found - continue
			continue
		}

		if section != nil {
			sections = append(sections, *section)
		}
	}

	return sections, nil
}

// extractSection extracts a specific section from the YAML node
func (p *YAMLSectionParser) extractSection(definition config.SectionDefinition, rootNode *yaml.Node, contentLines []string) (*shared.Section, error) {
	// Navigate to the YAML path
	node, err := p.navigateYAMLPath(rootNode, definition.YAMLPath)
	if err != nil {
		return nil, err
	}

	if node == nil {
		return nil, fmt.Errorf("section not found at path: %s", definition.YAMLPath)
	}

	// Calculate line range for this section
	startLine, endLine := p.calculateSectionLines(node, contentLines, definition.YAMLPath)

	// Extract section content
	sectionContent := p.extractSectionContent(contentLines, startLine, endLine)

	// Parse fields from the node
	fields, err := p.parseNodeToMap(node)
	if err != nil {
		return nil, fmt.Errorf("failed to parse section fields: %w", err)
	}

	section := &shared.Section{
		Name:        definition.Name,
		StartLine:   startLine,
		EndLine:     endLine,
		Content:     sectionContent,
		Type:        shared.YAMLSection,
		Fields:      fields,
		FilePath:    p.filePath,
		YAMLPath:    definition.YAMLPath,
		Required:    definition.Required,
		RuleConfigs: definition.RuleConfigs,
		AutoApprove: definition.AutoApprove,
	}

	return section, nil
}

// navigateYAMLPath navigates to a specific path in the YAML node tree
func (p *YAMLSectionParser) navigateYAMLPath(rootNode *yaml.Node, yamlPath string) (*yaml.Node, error) {
	currentNode := rootNode

	// Navigate through document nodes to find the root mapping
	// This must happen BEFORE checking for "." path
	for currentNode.Kind == yaml.DocumentNode && len(currentNode.Content) > 0 {
		currentNode = currentNode.Content[0]
	}

	// If path is "." or empty, return the unwrapped root mapping
	if yamlPath == "" || yamlPath == "." {
		return currentNode, nil
	}

	pathParts := strings.Split(yamlPath, ".")

	for _, part := range pathParts {
		if part == "" {
			continue
		}

		nextNode, err := p.findChildNode(currentNode, part)
		if err != nil {
			return nil, err
		}
		if nextNode == nil {
			return nil, fmt.Errorf("path not found: %s", part)
		}
		currentNode = nextNode
	}

	return currentNode, nil
}

// findChildNode finds a child node by key in a mapping node
func (p *YAMLSectionParser) findChildNode(parent *yaml.Node, key string) (*yaml.Node, error) {
	if parent.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("expected mapping node, got %v", parent.Kind)
	}

	// YAML mapping nodes store key-value pairs as alternating content items
	for i := 0; i < len(parent.Content); i += 2 {
		keyNode := parent.Content[i]
		valueNode := parent.Content[i+1]

		if keyNode.Value == key {
			return valueNode, nil
		}
	}

	return nil, nil // Key not found
}

// calculateSectionLines determines the start and end lines for a section
func (p *YAMLSectionParser) calculateSectionLines(node *yaml.Node, contentLines []string, yamlPath string) (int, int) {
	startLine := node.Line
	endLine := node.Line

	// For complex nodes, calculate the end line by traversing all content
	if node.Kind == yaml.MappingNode || node.Kind == yaml.SequenceNode {
		endLine = p.calculateEndLine(node)
	}

	// Special handling for root path ("."): should cover entire file from line 1
	if yamlPath == "." {
		startLine = 1
		endLine = len(contentLines)
	}

	// Ensure we don't go beyond the file bounds
	if endLine > len(contentLines) {
		endLine = len(contentLines)
	}

	return startLine, endLine
}

// calculateEndLine recursively calculates the last line of a YAML node
func (p *YAMLSectionParser) calculateEndLine(node *yaml.Node) int {
	maxLine := node.Line

	// Also check the Column to get a more accurate end position
	// For scalar nodes, use the line they're on
	if node.Kind == yaml.ScalarNode {
		maxLine = node.Line
	}

	for _, child := range node.Content {
		childEndLine := p.calculateEndLine(child)
		if childEndLine > maxLine {
			maxLine = childEndLine
		}
	}

	return maxLine
}

// extractSectionContent extracts the text content for a section
func (p *YAMLSectionParser) extractSectionContent(contentLines []string, startLine, endLine int) string {
	if startLine < 1 || endLine < startLine || startLine > len(contentLines) {
		return ""
	}

	// Adjust for 0-based indexing
	start := startLine - 1
	end := endLine
	if end > len(contentLines) {
		end = len(contentLines)
	}

	return strings.Join(contentLines[start:end], "\n")
}

// parseNodeToMap converts a YAML node to a map[string]interface{}
func (p *YAMLSectionParser) parseNodeToMap(node *yaml.Node) (map[string]interface{}, error) {
	var result interface{}
	if err := node.Decode(&result); err != nil {
		return nil, err
	}

	// Convert to map if possible
	if mapResult, ok := result.(map[string]interface{}); ok {
		return mapResult, nil
	}

	// If it's not a map, wrap it in a map with a generic key
	return map[string]interface{}{
		"value": result,
	}, nil
}

// GetSectionAtLine returns the section that contains the given line number
func (p *YAMLSectionParser) GetSectionAtLine(sections []shared.Section, lineNumber int) *shared.Section {
	for i := range sections {
		section := &sections[i]
		if lineNumber >= section.StartLine && lineNumber <= section.EndLine {
			return section
		}
	}
	return nil
}

// ValidateSection validates a section using the specified rules
func (p *YAMLSectionParser) ValidateSection(section *shared.Section, rules []shared.Rule) *shared.SectionValidationResult {
	result := &shared.SectionValidationResult{
		Section:      section,
		AppliedRules: make([]string, 0),
		Decision:     shared.Approve,
		Reason:       "Section validation passed",
		Violations:   make([]shared.SectionViolation, 0),
		RuleResults:  make([]shared.LineValidationResult, 0),
	}

	// Create line ranges for this section
	lineRanges := []shared.LineRange{
		{
			StartLine: section.StartLine,
			EndLine:   section.EndLine,
			FilePath:  section.FilePath,
		},
	}

	// Step 1: Run any configured rules first
	hasRules := len(rules) > 0
	rulesPassed := true
	var lastRuleReason string

	if hasRules {
		// Apply each rule to the section
		for _, rule := range rules {
			// Check if this rule applies to this section
			coveredLines := rule.GetCoveredLines(section.FilePath, section.Content)
			if len(coveredLines) == 0 {
				continue // Rule doesn't apply
			}

			// Validate using the rule
			decision, reason := rule.ValidateLines(section.FilePath, section.Content, lineRanges)

			result.AppliedRules = append(result.AppliedRules, rule.Name())
			result.RuleResults = append(result.RuleResults, shared.LineValidationResult{
				RuleName:     rule.Name(),
				LineRanges:   lineRanges,
				Decision:     decision,
				Reason:       reason,
				WasEvaluated: true, // Mark that this rule actually executed
			})

			lastRuleReason = reason

			// If any rule requires manual review, rules failed
			if decision == shared.ManualReview {
				rulesPassed = false
				result.Decision = shared.ManualReview
				result.Reason = fmt.Sprintf("Rule validation failed: %s", reason)
				break // Stop on first rule failure
			}
		}
	}

	// Step 2: Apply decision logic - handle definitive cases first

	// Case 1: Rules failed - always manual review regardless of auto-approve setting
	if hasRules && !rulesPassed {
		result.Decision = shared.ManualReview
		// Reason already set above when rules failed
		if section.AutoApprove {
			logging.Info("AUTO_APPROVE_AUDIT: Section '%s' at %s:%d-%d failed auto-approve (rules failed: %s)",
				section.Name, section.FilePath, section.StartLine, section.EndLine, result.Reason)
		}
		return result
	}

	// Case 2: Auto-approve enabled - approve if rules passed or no rules
	if section.AutoApprove {
		result.Decision = shared.Approve

		if !hasRules {
			result.Reason = fmt.Sprintf("Auto-approved: %s (no validation required)", section.Name)
			logging.Info("AUTO_APPROVE_AUDIT: Section '%s' at %s:%d-%d auto-approved (no rules required)",
				section.Name, section.FilePath, section.StartLine, section.EndLine)
		} else if len(result.AppliedRules) == 0 {
			result.Reason = fmt.Sprintf("Auto-approved: %s (no applicable rules)", section.Name)
			logging.Info("AUTO_APPROVE_AUDIT: Section '%s' at %s:%d-%d auto-approved (no applicable rules)",
				section.Name, section.FilePath, section.StartLine, section.EndLine)
		} else {
			result.Reason = fmt.Sprintf("Auto-approved: %s (validation passed)", lastRuleReason)
			logging.Info("AUTO_APPROVE_AUDIT: Section '%s' at %s:%d-%d auto-approved (rules: %v passed)",
				section.Name, section.FilePath, section.StartLine, section.EndLine, result.AppliedRules)
		}
		return result
	}

	// Case 3: Not auto-approve - normal approval process
	if !hasRules || len(result.AppliedRules) == 0 {
		result.Decision = shared.ManualReview
		result.Reason = fmt.Sprintf("No validation rules configured for %s - manual review required", section.Name)
	} else if rulesPassed {
		result.Decision = shared.Approve
		result.Reason = lastRuleReason
	}

	return result
}

// GetSectionDefinitions returns the section definitions for this parser
func (p *YAMLSectionParser) GetSectionDefinitions() map[string]config.SectionDefinition {
	return p.sectionDefinitions
}
