package shared

import (
	"strings"
	"time"

	"github.com/redhat-data-and-ai/naysayer/internal/gitlab"
)

// Decision represents a binary approval decision
type DecisionType string

const (
	Approve      DecisionType = "approve"       // Auto-approve the MR
	ManualReview DecisionType = "manual_review" // Require manual approval
)

// Decision represents a simplified approval decision for a merge request
type Decision struct {
	Type    DecisionType `json:"type"`
	Reason  string       `json:"reason"`
	Summary string       `json:"summary"`
	Details string       `json:"details,omitempty"`
}

// RuleResult represents the result of a rule evaluation
type RuleResult struct {
	Decision      Decision       `json:"decision"`
	RuleName      string         `json:"rule_name"`
	Confidence    float64        `json:"confidence"` // 0.0-1.0
	Metadata      map[string]any `json:"metadata,omitempty"`
	ExecutionTime time.Duration  `json:"execution_time"`
}

// MRContext contains all information needed for rule evaluation
type MRContext struct {
	ProjectID   int                 `json:"project_id"`
	MRIID       int                 `json:"mr_iid"`
	Changes     []gitlab.FileChange `json:"changes"`
	MRInfo      *gitlab.MRInfo      `json:"mr_info"`
	Environment string              `json:"environment,omitempty"`
	Labels      []string            `json:"labels,omitempty"`
	Metadata    map[string]any      `json:"metadata,omitempty"`
}

// Rule defines a simplified interface for all rules
type Rule interface {
	// Name returns the unique identifier for this rule
	Name() string

	// Description returns a human-readable description
	Description() string

	// Returns which line ranges this rule validates in a file
	GetCoveredLines(filePath string, fileContent string) []LineRange

	// Validates only the specified line ranges
	ValidateLines(filePath string, fileContent string, lineRanges []LineRange) (DecisionType, string)
}

// RuleManager manages and executes rules with simple logic
type RuleManager interface {
	// AddRule registers a rule
	AddRule(rule Rule)

	// EvaluateAll runs all applicable rules and returns a final decision
	EvaluateAll(mrCtx *MRContext) *RuleEvaluation
}

// LineRange represents a range of lines in a file
type LineRange struct {
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
	FilePath  string `json:"file_path"`
}

// LineValidationResult represents validation result for specific lines
type LineValidationResult struct {
	RuleName   string       `json:"rule_name"`
	LineRanges []LineRange  `json:"line_ranges"`
	Decision   DecisionType `json:"decision"`
	Reason     string       `json:"reason"`
}

// FileValidationSummary shows validation results for a single file
type FileValidationSummary struct {
	FilePath       string                 `json:"file_path"`
	TotalLines     int                    `json:"total_lines"`
	CoveredLines   []LineRange            `json:"covered_lines"`
	UncoveredLines []LineRange            `json:"uncovered_lines"`
	RuleResults    []LineValidationResult `json:"rule_results"`
	FileDecision   DecisionType           `json:"file_decision"`
}

// RuleEvaluation contains the results of evaluating all rules
type RuleEvaluation struct {
	FinalDecision   Decision                          `json:"final_decision"`
	FileValidations map[string]*FileValidationSummary `json:"file_validations"` // filePath -> summary
	ExecutionTime   time.Duration                     `json:"execution_time"`

	// Summary statistics
	TotalFiles     int `json:"total_files"`
	ApprovedFiles  int `json:"approved_files"`
	ReviewFiles    int `json:"review_files"`
	UncoveredFiles int `json:"uncovered_files"`

}

// Common helper functions for rule evaluation

// IsDraftMR returns true if the MR is a draft/WIP
func IsDraftMR(mrCtx *MRContext) bool {
	if mrCtx.MRInfo == nil {
		return false
	}

	title := strings.ToLower(mrCtx.MRInfo.Title)
	return strings.Contains(title, "draft") ||
		strings.Contains(title, "wip") ||
		strings.HasPrefix(title, "draft:") ||
		strings.HasPrefix(title, "wip:")
}

// IsAutomatedUser returns true if the MR author is a bot or automated user
func IsAutomatedUser(mrCtx *MRContext) bool {
	if mrCtx.MRInfo == nil {
		return false
	}

	author := strings.ToLower(mrCtx.MRInfo.Author)
	automatedUsers := []string{"dependabot", "renovate", "greenkeeper", "snyk-bot"}

	for _, botUser := range automatedUsers {
		if strings.Contains(author, botUser) {
			return true
		}
	}

	return false
}

// Helper functions for line range operations

// ContainsLine checks if a line number is within any of the given line ranges
func ContainsLine(lineRanges []LineRange, lineNumber int) bool {
	for _, lr := range lineRanges {
		if lineNumber >= lr.StartLine && lineNumber <= lr.EndLine {
			return true
		}
	}
	return false
}

// MergeLineRanges combines overlapping or adjacent line ranges
func MergeLineRanges(ranges []LineRange) []LineRange {
	if len(ranges) <= 1 {
		return ranges
	}

	// Sort by start line
	var merged []LineRange
	for _, r := range ranges {
		if len(merged) == 0 {
			merged = append(merged, r)
			continue
		}

		last := &merged[len(merged)-1]
		if r.StartLine <= last.EndLine+1 {
			// Overlapping or adjacent - merge
			if r.EndLine > last.EndLine {
				last.EndLine = r.EndLine
			}
		} else {
			// Non-overlapping - add new range
			merged = append(merged, r)
		}
	}

	return merged
}

// GetUncoveredLines returns line ranges that are not covered by any of the given ranges
func GetUncoveredLines(totalLines int, coveredRanges []LineRange) []LineRange {
	if totalLines == 0 {
		return nil
	}

	merged := MergeLineRanges(coveredRanges)
	var uncovered []LineRange

	currentLine := 1
	for _, covered := range merged {
		if currentLine < covered.StartLine {
			// Gap before this covered range
			uncovered = append(uncovered, LineRange{
				StartLine: currentLine,
				EndLine:   covered.StartLine - 1,
			})
		}
		currentLine = covered.EndLine + 1
	}

	// Check for gap after last covered range
	if currentLine <= totalLines {
		uncovered = append(uncovered, LineRange{
			StartLine: currentLine,
			EndLine:   totalLines,
		})
	}

	return uncovered
}

// CountLines counts the number of lines in a string
func CountLines(content string) int {
	if content == "" {
		return 0
	}
	return strings.Count(content, "\n") + 1
}

// YAML Section-Aware Validation Types

// YAMLField represents a single field within a YAML section
type YAMLField struct {
	Name      string `json:"name"`
	Value     string `json:"value"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
}

// YAMLSection represents a logical section of a YAML file with line tracking
type YAMLSection struct {
	Name       string      `json:"name"`
	StartLine  int         `json:"start_line"`
	EndLine    int         `json:"end_line"`
	Content    string      `json:"content"`
	Fields     []YAMLField `json:"fields"`
	FilePath   string      `json:"file_path"`
}

// YAMLParseResult contains the parsed YAML structure with line mappings
type YAMLParseResult struct {
	Sections    []YAMLSection `json:"sections"`
	TotalLines  int           `json:"total_lines"`
	FilePath    string        `json:"file_path"`
	ParseErrors []string      `json:"parse_errors,omitempty"`
}

// ParseYAMLWithLineNumbers parses YAML content and tracks line numbers for each section
func ParseYAMLWithLineNumbers(filePath, content string) *YAMLParseResult {
	if content == "" {
		return &YAMLParseResult{
			Sections:   []YAMLSection{},
			TotalLines: 0,
			FilePath:   filePath,
		}
	}

	lines := strings.Split(content, "\n")
	totalLines := len(lines)
	
	result := &YAMLParseResult{
		Sections:   []YAMLSection{},
		TotalLines: totalLines,
		FilePath:   filePath,
	}

	var currentSection *YAMLSection
	var currentIndentLevel int = -1

	for i, line := range lines {
		lineNum := i + 1
		trimmedLine := strings.TrimSpace(line)
		
		// Skip empty lines and comments
		if trimmedLine == "" || strings.HasPrefix(trimmedLine, "#") || strings.HasPrefix(trimmedLine, "---") {
			continue
		}

		// Calculate indentation level
		indent := len(line) - len(strings.TrimLeft(line, " \t"))
		
		// Check if this is a top-level key (new section)
		if strings.Contains(trimmedLine, ":") && (indent == 0 || currentIndentLevel == -1) {
			// Close previous section if it exists
			if currentSection != nil {
				currentSection.EndLine = lineNum - 1
				result.Sections = append(result.Sections, *currentSection)
			}
			
			// Start new section
			keyName := strings.Split(trimmedLine, ":")[0]
			currentSection = &YAMLSection{
				Name:      strings.TrimSpace(keyName),
				StartLine: lineNum,
				EndLine:   lineNum, // Will be updated when section ends
				Content:   "",
				Fields:    []YAMLField{},
				FilePath:  filePath,
			}
			currentIndentLevel = indent
			
			// Add the field for this line
			value := ""
			if parts := strings.SplitN(trimmedLine, ":", 2); len(parts) > 1 {
				value = strings.TrimSpace(parts[1])
			}
			
			currentSection.Fields = append(currentSection.Fields, YAMLField{
				Name:      keyName,
				Value:     value,
				StartLine: lineNum,
				EndLine:   lineNum,
			})
		} else if currentSection != nil {
			// This line belongs to the current section
			currentSection.EndLine = lineNum
			
			// Parse nested fields
			if strings.Contains(trimmedLine, ":") {
				keyName := strings.Split(trimmedLine, ":")[0]
				value := ""
				if parts := strings.SplitN(trimmedLine, ":", 2); len(parts) > 1 {
					value = strings.TrimSpace(parts[1])
				}
				
				currentSection.Fields = append(currentSection.Fields, YAMLField{
					Name:      strings.TrimSpace(keyName),
					Value:     value,
					StartLine: lineNum,
					EndLine:   lineNum,
				})
			}
		}
		
		// Add line to current section content
		if currentSection != nil {
			if currentSection.Content != "" {
				currentSection.Content += "\n"
			}
			currentSection.Content += line
		}
	}
	
	// Close the last section
	if currentSection != nil {
		currentSection.EndLine = totalLines
		result.Sections = append(result.Sections, *currentSection)
	}

	return result
}

// GetSectionByName returns a YAML section by its name
func (ypr *YAMLParseResult) GetSectionByName(name string) *YAMLSection {
	for i := range ypr.Sections {
		if ypr.Sections[i].Name == name {
			return &ypr.Sections[i]
		}
	}
	return nil
}

// GetSectionsWithName returns all YAML sections matching the given name (for arrays)
func (ypr *YAMLParseResult) GetSectionsWithName(name string) []YAMLSection {
	var matching []YAMLSection
	for _, section := range ypr.Sections {
		if section.Name == name {
			matching = append(matching, section)
		}
	}
	return matching
}

// HasField checks if a section contains a specific field
func (ys *YAMLSection) HasField(fieldName string) bool {
	for _, field := range ys.Fields {
		if field.Name == fieldName {
			return true
		}
	}
	return false
}

// GetField returns a field by name from the section
func (ys *YAMLSection) GetField(fieldName string) *YAMLField {
	for i := range ys.Fields {
		if ys.Fields[i].Name == fieldName {
			return &ys.Fields[i]
		}
	}
	return nil
}

// ToLineRange converts a YAML section to a LineRange
func (ys *YAMLSection) ToLineRange() LineRange {
	return LineRange{
		StartLine: ys.StartLine,
		EndLine:   ys.EndLine,
		FilePath:  ys.FilePath,
	}
}

// ToLineRange converts a YAML field to a LineRange
func (yf *YAMLField) ToLineRange(filePath string) LineRange {
	return LineRange{
		StartLine: yf.StartLine,
		EndLine:   yf.EndLine,
		FilePath:  filePath,
	}
}
