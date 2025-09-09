package shared

import (
	"github.com/redhat-data-and-ai/naysayer/internal/config"
)

// SectionType represents the type of content in a section
type SectionType string

const (
	YAMLSection     SectionType = "yaml"
	JSONSection     SectionType = "json"
	TextSection     SectionType = "text"
	MarkdownSection SectionType = "markdown"
)

// Section represents a logical section within a file
type Section struct {
	Name        string                 `json:"name"`         // e.g., "warehouse", "consumers", "serviceaccount"
	StartLine   int                    `json:"start_line"`   // Section start line (1-based)
	EndLine     int                    `json:"end_line"`     // Section end line (1-based)
	Content     string                 `json:"content"`      // Raw section content
	Type        SectionType            `json:"type"`         // Section content type
	Fields      map[string]interface{} `json:"fields"`       // Parsed fields for this section
	FilePath    string                 `json:"file_path"`    // Parent file path
	YAMLPath    string                 `json:"yaml_path"`    // YAML path (e.g., "spec.warehouse")
	Required    bool                   `json:"required"`     // Is this section required?
	RuleNames   []string               `json:"rule_names"`   // Rules that apply to this section
	AutoApprove bool                   `json:"auto_approve"` // Auto-approve this section if rules pass
}

// SectionValidationResult represents validation result for a specific section
type SectionValidationResult struct {
	Section      *Section               `json:"section"`
	AppliedRules []string               `json:"applied_rules"`
	Decision     DecisionType           `json:"decision"`
	Reason       string                 `json:"reason"`
	Violations   []SectionViolation     `json:"violations"`
	RuleResults  []LineValidationResult `json:"rule_results"`
}

// SectionViolation represents a validation issue within a section
type SectionViolation struct {
	Type        ViolationType `json:"type"`
	Severity    Severity      `json:"severity"`
	LineNumber  int           `json:"line_number"`
	FieldPath   string        `json:"field_path"` // YAML field path (e.g., "spec.warehouse.size")
	Description string        `json:"description"`
	Suggestion  string        `json:"suggestion"`
	RuleName    string        `json:"rule_name"`
}

// ViolationType categorizes different types of validation violations
type ViolationType string

const (
	MissingField    ViolationType = "missing_field"
	InvalidValue    ViolationType = "invalid_value"
	FormatError     ViolationType = "format_error"
	SecurityIssue   ViolationType = "security_issue"
	PolicyViolation ViolationType = "policy_violation"
	SyntaxError     ViolationType = "syntax_error"
)

// Severity indicates the importance of a violation
type Severity string

const (
	Critical Severity = "critical" // Blocks deployment
	High     Severity = "high"     // Requires immediate attention
	Medium   Severity = "medium"   // Should be fixed soon
	Low      Severity = "low"      // Nice to have
)

// SectionParser defines the interface for parsing file sections
type SectionParser interface {
	// ParseSections extracts sections from file content
	ParseSections(filePath string, content string) ([]Section, error)

	// GetSectionAtLine returns the section that contains the given line number
	GetSectionAtLine(sections []Section, lineNumber int) *Section

	// ValidateSection validates a section using the specified rules
	ValidateSection(section *Section, rules []Rule) *SectionValidationResult

	// GetSectionDefinitions returns the section definitions for this parser
	GetSectionDefinitions() map[string]config.SectionDefinition
}

// SectionFileValidationSummary extends FileValidationSummary with section information
type SectionFileValidationSummary struct {
	*FileValidationSummary
	Sections          []Section                 `json:"sections"`
	SectionResults    []SectionValidationResult `json:"section_results"`
	UncoveredSections []Section                 `json:"uncovered_sections"` // Sections without applicable rules
}

// LineToSectionMap helps map line numbers to sections efficiently
type LineToSectionMap struct {
	sections []Section
	lineMap  map[int]*Section // line number -> section
}

// NewLineToSectionMap creates a new line-to-section mapping
func NewLineToSectionMap(sections []Section) *LineToSectionMap {
	lineMap := make(map[int]*Section)

	for i := range sections {
		section := &sections[i]
		for line := section.StartLine; line <= section.EndLine; line++ {
			lineMap[line] = section
		}
	}

	return &LineToSectionMap{
		sections: sections,
		lineMap:  lineMap,
	}
}

// GetSectionAtLine returns the section containing the specified line
func (lsm *LineToSectionMap) GetSectionAtLine(lineNumber int) *Section {
	return lsm.lineMap[lineNumber]
}

// GetAllSections returns all sections
func (lsm *LineToSectionMap) GetAllSections() []Section {
	return lsm.sections
}
