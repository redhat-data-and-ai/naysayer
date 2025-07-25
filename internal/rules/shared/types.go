package shared

import (
	"fmt"
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

	// Applies checks if this rule should be evaluated for the given MR
	Applies(mrCtx *MRContext) bool

	// ShouldApprove executes the rule logic and returns a binary decision
	ShouldApprove(mrCtx *MRContext) (DecisionType, string)
}

// RuleManager manages and executes rules with simple logic
type RuleManager interface {
	// AddRule registers a rule
	AddRule(rule Rule)

	// EvaluateAll runs all applicable rules and returns a final decision
	EvaluateAll(mrCtx *MRContext) *RuleEvaluation
}

// RuleEvaluation contains the results of evaluating all rules
type RuleEvaluation struct {
	FinalDecision Decision      `json:"final_decision"`
	RuleResults   []RuleResult  `json:"rule_results"`
	ExecutionTime time.Duration `json:"execution_time"`
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

// DataverseFileType represents different types of dataverse configuration files
type DataverseFileType string

const (
	WarehouseFile     DataverseFileType = "warehouse"
	SourceBindingFile DataverseFileType = "sourcebinding"
	// Future file types can be added here: ConfigFile, SchemaFile, etc.
)

// IsDataverseFile checks if a file is a dataverse configuration file and returns its type
func IsDataverseFile(path string) (bool, DataverseFileType) {
	if path == "" {
		return false, ""
	}

	lowerPath := strings.ToLower(path)

	// Check for warehouse files (product.yaml)
	if strings.HasSuffix(lowerPath, "product.yaml") || strings.HasSuffix(lowerPath, "product.yml") {
		return true, WarehouseFile
	}

	// Check for sourcebinding files
	if strings.HasSuffix(lowerPath, "sourcebinding.yaml") || strings.HasSuffix(lowerPath, "sourcebinding.yml") {
		return true, SourceBindingFile
	}

	return false, ""
}

// AnalyzeDataverseChanges analyzes file changes and returns counts by dataverse file type
func AnalyzeDataverseChanges(changes []gitlab.FileChange) map[DataverseFileType]int {
	fileTypes := make(map[DataverseFileType]int)

	for _, change := range changes {
		// Check both old and new paths for file type detection
		paths := []string{change.NewPath, change.OldPath}

		for _, path := range paths {
			if isDataverse, fileType := IsDataverseFile(path); isDataverse {
				fileTypes[fileType]++
				break // Don't double count if both paths are same type
			}
		}
	}

	return fileTypes
}

// AreAllDataverseSafe checks if all file changes are dataverse-safe files
func AreAllDataverseSafe(changes []gitlab.FileChange) bool {
	for _, change := range changes {
		// Check if at least one of the paths (new or old) is a dataverse file
		newIsDataverse, _ := IsDataverseFile(change.NewPath)
		oldIsDataverse, _ := IsDataverseFile(change.OldPath)

		if !newIsDataverse && !oldIsDataverse {
			return false
		}
	}

	return true
}

// BuildDataverseApprovalMessage creates a dynamic approval message based on file types
func BuildDataverseApprovalMessage(fileTypes map[DataverseFileType]int) string {
	if len(fileTypes) == 0 {
		return "Auto-approving MR with no dataverse file changes"
	}

	var parts []string

	// Add parts in a consistent order
	if count := fileTypes[WarehouseFile]; count > 0 {
		if count == 1 {
			parts = append(parts, "1 warehouse")
		} else {
			parts = append(parts, fmt.Sprintf("%d warehouse", count))
		}
	}

	if count := fileTypes[SourceBindingFile]; count > 0 {
		if count == 1 {
			parts = append(parts, "1 sourcebinding")
		} else {
			parts = append(parts, fmt.Sprintf("%d sourcebinding", count))
		}
	}

	// Join parts with proper grammar
	var changeDesc string
	switch len(parts) {
	case 0:
		changeDesc = "no changes"
	case 1:
		changeDesc = parts[0] + " changes"
	case 2:
		changeDesc = parts[0] + " and " + parts[1] + " changes"
	default:
		// For future expansion with more file types
		changeDesc = strings.Join(parts[:len(parts)-1], ", ") + " and " + parts[len(parts)-1] + " changes"
	}

	return fmt.Sprintf("Auto-approving MR with only %s", changeDesc)
}
