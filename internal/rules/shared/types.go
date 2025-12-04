package shared

import (
	"sort"
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

// ContextAwareRule is an optional interface that rules can implement to access MR context
type ContextAwareRule interface {
	Rule

	// SetMRContext provides the full MR context to the rule for advanced analysis
	SetMRContext(mrCtx *MRContext)
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
	RuleName     string       `json:"rule_name"`
	LineRanges   []LineRange  `json:"line_ranges"`
	Decision     DecisionType `json:"decision"`
	Reason       string       `json:"reason"`
	WasEvaluated bool         `json:"was_evaluated"` // true if rule actually executed (vs skipped)
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
	// First, copy the ranges to avoid modifying the input
	sorted := make([]LineRange, len(ranges))
	copy(sorted, ranges)

	// Sort by StartLine using standard library
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].StartLine < sorted[j].StartLine
	})

	var merged []LineRange
	for _, r := range sorted {
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
