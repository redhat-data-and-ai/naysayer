package dataproduct_consumer

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/redhat-data-and-ai/naysayer/internal/gitlab"
	"github.com/redhat-data-and-ai/naysayer/internal/logging"
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
	client gitlab.GitLabClient
}

// NewDataProductConsumerRule creates a new data product consumer rule instance
func NewDataProductConsumerRule(allowedEnvs []string, client gitlab.GitLabClient) *DataProductConsumerRule {
	config := DefaultDataProductConsumerConfig()
	if allowedEnvs != nil {
		config.AllowedEnvironments = allowedEnvs
	}

	return &DataProductConsumerRule{
		BaseRule:         common.NewBaseRule("dataproduct_consumer_rule", "Auto-approves consumer access changes to data products in allowed environments (preprod/prod)"),
		FileTypeMatcher:  common.NewFileTypeMatcher(),
		ValidationHelper: common.NewValidationHelper(),
		config:           config,
		client:           client,
	}
}

// ValidateLines validates lines for consumer access changes
func (r *DataProductConsumerRule) ValidateLines(filePath string, fileContent string, lineRanges []shared.LineRange) (shared.DecisionType, string) {
	if !r.IsProductFile(filePath) {
		return r.CreateApprovalResult("Not a product.yaml file - consumer rule does not apply")
	}

	yamlContent, err := readYaml(fileContent)
	if err != nil {
		return shared.ManualReview, fmt.Sprintf("Failed to parse YAML content: %v", err)
	}

	context := r.analyzeFile(filePath, yamlContent, fileContent, lineRanges)

	// Validate consumer_group files exist regardless of consumer-only status.
	// Missing groups files should block approval even when mixed with other changes.
	if context.HasConsumers {
		groupNames := r.extractConsumerGroupNames(yamlContent)
		if len(groupNames) > 0 {
			allExist, missingGroups := r.validateConsumerGroupFiles(filePath, groupNames)
			if !allExist {
				return shared.ManualReview, fmt.Sprintf(
					"Consumer group file(s) not found in repository: %s",
					strings.Join(missingGroups, ", "),
				)
			}
		}
	}

	if context.HasConsumers && context.IsConsumerOnly {
		if context.Environment != "" {
			return r.CreateApprovalResult("Consumer access changes in " + context.Environment + " environment - data product owner approval sufficient (no TOC approval required)")
		}
		return r.CreateApprovalResult("Consumer access changes - data product owner approval sufficient (no TOC approval required)")
	}

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

// analyzeFile analyzes a file to determine if consumer rule applies.
// yamlContent is the pre-parsed YAML; fileContent is the raw text needed for line-based checks.
func (r *DataProductConsumerRule) analyzeFile(filePath string, yamlContent interface{}, fileContent string, lineRanges []shared.LineRange) *ConsumerContext {
	context := &ConsumerContext{
		FilePath:       filePath,
		Environment:    r.extractEnvironmentFromPath(filePath),
		HasConsumers:   false,
		IsConsumerOnly: false,
	}

	context.HasConsumers = r.containsConsumersKey(yamlContent)
	if !context.HasConsumers {
		return context
	}

	context.IsConsumerOnly = r.areChangesConsumerOnly(lineRanges, fileContent)

	return context
}

// containsConsumersKey recursively searches for a "consumers" key in parsed YAML.
func (r *DataProductConsumerRule) containsConsumersKey(data interface{}) bool {
	switch v := data.(type) {
	case map[string]interface{}:
		if _, hasConsumers := v["consumers"]; hasConsumers {
			return true
		}
		for _, val := range v {
			if r.containsConsumersKey(val) {
				return true
			}
		}
	case []interface{}:
		for _, item := range v {
			if r.containsConsumersKey(item) {
				return true
			}
		}
	}
	return false
}

// areChangesConsumerOnly checks if only consumer-related lines are being modified.
// Returns true only if at least one consumer-related line was found and no
// non-consumer lines exist in the range.
func (r *DataProductConsumerRule) areChangesConsumerOnly(lineRanges []shared.LineRange, fileContent string) bool {
	if len(lineRanges) == 0 {
		return false
	}

	lines := strings.Split(fileContent, "\n")
	foundConsumerLine := false

	for _, lr := range lineRanges {
		for lineNum := lr.StartLine; lineNum <= lr.EndLine && lineNum <= len(lines); lineNum++ {
			line := strings.TrimSpace(lines[lineNum-1])

			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}

			if !r.isConsumerRelatedLine(line) {
				return false
			}
			foundConsumerLine = true
		}
	}

	return foundConsumerLine
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

// extractConsumerGroupNames returns names of all consumers with kind: consumer_group
// from pre-parsed YAML content.
func (r *DataProductConsumerRule) extractConsumerGroupNames(yamlContent interface{}) []string {
	var groupNames []string
	r.collectConsumerGroups(yamlContent, &groupNames)
	return groupNames
}

func readYaml(fileContent string) (interface{}, error) {
	var yamlContent interface{}
	err := yaml.Unmarshal([]byte(fileContent), &yamlContent)
	return yamlContent, err
}

// collectConsumerGroups recursively traverses parsed YAML to find consumer entries
// with kind: consumer_group and collects their names.
func (r *DataProductConsumerRule) collectConsumerGroups(data interface{}, groupNames *[]string) {
	switch v := data.(type) {
	case map[string]interface{}:
		kind, hasKind := v["kind"].(string)
		name, hasName := v["name"].(string)
		if hasKind && hasName && kind == "consumer_group" && name != "" {
			*groupNames = append(*groupNames, name)
		}
		for _, val := range v {
			r.collectConsumerGroups(val, groupNames)
		}
	case []interface{}:
		for _, item := range v {
			r.collectConsumerGroups(item, groupNames)
		}
	}
}

// listFilesOnBranch lists filenames in a directory on the given branch.
// Returns an empty set if the client is nil, branch is empty, or the directory doesn't exist.
func (r *DataProductConsumerRule) listFilesOnBranch(projectID int, dirPath string, branch string) map[string]bool {
	if r.client == nil || branch == "" {
		return map[string]bool{}
	}
	files, err := r.client.ListDirectoryFiles(projectID, dirPath, branch)
	if err != nil {
		logging.Warn("Error listing directory %s on branch %s: %v", dirPath, branch, err)
		return map[string]bool{}
	}
	fileSet := make(map[string]bool, len(files))
	for _, f := range files {
		fileSet[f] = true
	}
	return fileSet
}

// validateConsumerGroupFiles checks that each consumer group name has a corresponding
// YAML file in the product's groups/ folder. Lists the directory once per branch
// instead of checking each file individually.
func (r *DataProductConsumerRule) validateConsumerGroupFiles(productFilePath string, groupNames []string) (bool, []string) {
	mrCtx := r.GetMRContext()
	if mrCtx == nil || mrCtx.MRInfo == nil {
		logging.Warn("No MR context available for consumer group file validation")
		return false, groupNames
	}

	productDir := filepath.Dir(filepath.Dir(productFilePath))
	groupsFolderPath := filepath.Join(productDir, "groups")

	sourceGroupFiles := r.listFilesOnBranch(mrCtx.ProjectID, groupsFolderPath, mrCtx.MRInfo.SourceBranch)
	targetGroupFiles := r.listFilesOnBranch(mrCtx.ProjectID, groupsFolderPath, mrCtx.MRInfo.TargetBranch)

	var missingGroups []string
	for _, name := range groupNames {
		expectedFile := name + ".yaml"
		if sourceGroupFiles[expectedFile] || targetGroupFiles[expectedFile] {
			continue
		}

		expectedPath := filepath.Join(groupsFolderPath, expectedFile)
		logging.Warn("Consumer group file not found: %s (checked branches: %s, %s)",
			expectedPath, mrCtx.MRInfo.SourceBranch, mrCtx.MRInfo.TargetBranch)
		missingGroups = append(missingGroups, name)
	}

	return len(missingGroups) == 0, missingGroups
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
