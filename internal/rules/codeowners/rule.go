package codeowners

import (
	"sort"
	"strings"

	"github.com/redhat-data-and-ai/naysayer/internal/gitlab"
	"github.com/redhat-data-and-ai/naysayer/internal/logging"
	"github.com/redhat-data-and-ai/naysayer/internal/rules/common"
	"github.com/redhat-data-and-ai/naysayer/internal/rules/shared"
	"gopkg.in/yaml.v3"
)

// CODEOWNERSSyncRule auto-approves CODEOWNERS changes when they match
// corresponding developers.yaml or groups/*.yaml changes
type CODEOWNERSSyncRule struct {
	*common.BaseRule
	*common.ValidationHelper
	client gitlab.GitLabClient
}

// NewCODEOWNERSSyncRule creates a new CODEOWNERS sync rule instance
func NewCODEOWNERSSyncRule(client gitlab.GitLabClient) *CODEOWNERSSyncRule {
	return &CODEOWNERSSyncRule{
		BaseRule:         common.NewBaseRule("codeowners_sync_rule", "Auto-approves CODEOWNERS changes that match developers.yaml or groups/*.yaml changes"),
		ValidationHelper: common.NewValidationHelper(),
		client:           client,
	}
}

// ValidateLines validates lines for CODEOWNERS sync
func (r *CODEOWNERSSyncRule) ValidateLines(filePath string, fileContent string, lineRanges []shared.LineRange) (shared.DecisionType, string) {
	if !r.isCODEOWNERSFile(filePath) {
		return r.CreateApprovalResult("Not a CODEOWNERS file - rule does not apply")
	}

	mrCtx := r.GetMRContext()
	if mrCtx == nil {
		return r.CreateManualReviewResult("MR context not available")
	}

	// Get all YAML changes in this MR
	yamlChanges := r.getYAMLChanges(mrCtx)
	if len(yamlChanges) == 0 {
		return r.CreateManualReviewResult("CODEOWNERS changed without corresponding YAML changes")
	}

	// Check for new data product (requires manual review)
	for _, change := range yamlChanges {
		if change.FileType == "developers" && change.IsNewFile {
			return r.CreateManualReviewResult("New data product detected - manual review required")
		}
		if change.FileType == "group" && change.IsNewFile {
			if !r.dataProductExists(mrCtx, change.DataProduct) {
				return r.CreateManualReviewResult("New group in new data product - manual review required")
			}
		}
	}

	// Validate CODEOWNERS changes match YAML changes
	addedEntries := r.parseAddedCODEOWNERSEntries(mrCtx, filePath)
	expectedEntries := r.buildExpectedEntries(yamlChanges)

	if reason := r.validateEntriesMatch(expectedEntries, addedEntries); reason != "" {
		return r.CreateManualReviewResult(reason)
	}

	return r.CreateApprovalResult("Auto-approved: CODEOWNERS changes match YAML changes")
}

// GetCoveredLines returns line ranges this rule covers
func (r *CODEOWNERSSyncRule) GetCoveredLines(filePath string, fileContent string) []shared.LineRange {
	if !r.isCODEOWNERSFile(filePath) {
		return []shared.LineRange{}
	}
	return r.GetFullFileCoverage(filePath, fileContent)
}

// isCODEOWNERSFile checks if the file is a CODEOWNERS file
func (r *CODEOWNERSSyncRule) isCODEOWNERSFile(filePath string) bool {
	return strings.HasSuffix(strings.ToLower(filePath), "codeowners")
}

// getYAMLChanges extracts information about changed YAML files from the MR
func (r *CODEOWNERSSyncRule) getYAMLChanges(mrCtx *shared.MRContext) []YAMLChangeInfo {
	var changes []YAMLChangeInfo

	for _, change := range mrCtx.Changes {
		if change.NewPath == "" {
			continue
		}

		if info := r.parseDevelopersYAML(mrCtx, change); info != nil {
			changes = append(changes, *info)
		} else if info := r.parseGroupYAML(mrCtx, change); info != nil {
			changes = append(changes, *info)
		}
	}
	return changes
}

// parseDevelopersYAML parses a developers.yaml change
func (r *CODEOWNERSSyncRule) parseDevelopersYAML(mrCtx *shared.MRContext, change gitlab.FileChange) *YAMLChangeInfo {
	if !strings.HasSuffix(change.NewPath, "developers.yaml") && !strings.HasSuffix(change.NewPath, "developers.yml") {
		return nil
	}

	dp := r.extractDataProductInfo(change.NewPath)
	if dp == nil {
		return nil
	}

	owners := r.fetchOwners(mrCtx, change.NewPath)
	if owners == nil {
		return nil
	}

	return &YAMLChangeInfo{
		FilePath:        change.NewPath,
		FileType:        "developers",
		DataProduct:     *dp,
		OwnersApprovers: owners,
		IsNewFile:       change.NewFile,
	}
}

// parseGroupYAML parses a groups/*.yaml change
func (r *CODEOWNERSSyncRule) parseGroupYAML(mrCtx *shared.MRContext, change gitlab.FileChange) *YAMLChangeInfo {
	if !strings.Contains(change.NewPath, "/groups/") ||
		!strings.HasPrefix(change.NewPath, "dataproducts/") ||
		(!strings.HasSuffix(change.NewPath, ".yaml") && !strings.HasSuffix(change.NewPath, ".yml")) {
		return nil
	}

	dp := r.extractDataProductInfo(change.NewPath)
	if dp == nil {
		return nil
	}

	groupInfo := r.fetchGroupInfo(mrCtx, change.NewPath)
	if groupInfo == nil {
		return nil
	}

	return &YAMLChangeInfo{
		FilePath:        change.NewPath,
		FileType:        "group",
		DataProduct:     *dp,
		OwnersApprovers: groupInfo.Approvers,
		IsNewFile:       change.NewFile,
		GroupName:       groupInfo.GroupName,
	}
}

// extractDataProductInfo extracts data product info from file path
func (r *CODEOWNERSSyncRule) extractDataProductInfo(filePath string) *DataProductInfo {
	parts := strings.Split(filePath, "/")
	if len(parts) < 3 || parts[0] != "dataproducts" {
		return nil
	}

	dpType := parts[1]
	if dpType != "aggregate" && dpType != "source" && dpType != "platform" {
		return nil
	}

	return &DataProductInfo{
		Type: dpType,
		Name: parts[2],
		Path: strings.Join(parts[:3], "/"),
	}
}

// fetchOwners fetches owners from developers.yaml
func (r *CODEOWNERSSyncRule) fetchOwners(mrCtx *shared.MRContext, filePath string) []string {
	if r.client == nil || mrCtx.MRInfo == nil {
		return nil
	}

	content, err := r.client.FetchFileContent(mrCtx.ProjectID, filePath, mrCtx.MRInfo.SourceBranch)
	if err != nil {
		logging.Warn("Failed to fetch developers.yaml: %v", err)
		return nil
	}

	var data DevelopersYAML
	if err := yaml.Unmarshal([]byte(content.Content), &data); err != nil {
		logging.Warn("Failed to parse developers.yaml: %v", err)
		return nil
	}
	return data.Group.Owners
}

// fetchGroupInfo fetches group info from groups/*.yaml
func (r *CODEOWNERSSyncRule) fetchGroupInfo(mrCtx *shared.MRContext, filePath string) *GroupYAML {
	if r.client == nil || mrCtx.MRInfo == nil {
		return nil
	}

	content, err := r.client.FetchFileContent(mrCtx.ProjectID, filePath, mrCtx.MRInfo.SourceBranch)
	if err != nil {
		logging.Warn("Failed to fetch group YAML: %v", err)
		return nil
	}

	var data GroupYAML
	if err := yaml.Unmarshal([]byte(content.Content), &data); err != nil {
		logging.Warn("Failed to parse group YAML: %v", err)
		return nil
	}
	return &data
}

// dataProductExists checks if data product exists in base branch
func (r *CODEOWNERSSyncRule) dataProductExists(mrCtx *shared.MRContext, dp DataProductInfo) bool {
	if r.client == nil || mrCtx.MRInfo == nil {
		return false
	}
	_, err := r.client.FetchFileContent(mrCtx.ProjectID, dp.Path+"/developers.yaml", mrCtx.MRInfo.TargetBranch)
	return err == nil
}

// parseAddedCODEOWNERSEntries parses added entries from CODEOWNERS diff
func (r *CODEOWNERSSyncRule) parseAddedCODEOWNERSEntries(mrCtx *shared.MRContext, filePath string) []CODEOWNERSEntry {
	var entries []CODEOWNERSEntry

	for _, change := range mrCtx.Changes {
		if change.NewPath != filePath {
			continue
		}

		for _, line := range strings.Split(change.Diff, "\n") {
			if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "++") {
				if entry := r.parseCODEOWNERSLine(strings.TrimPrefix(line, "+")); entry != nil {
					entries = append(entries, *entry)
				}
			}
		}
		break
	}
	return entries
}

// parseCODEOWNERSLine parses a single CODEOWNERS line
func (r *CODEOWNERSSyncRule) parseCODEOWNERSLine(line string) *CODEOWNERSEntry {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "[") {
		return nil
	}

	parts := strings.Fields(line)
	if len(parts) < 2 {
		return nil
	}

	var owners []string
	for _, part := range parts[1:] {
		if strings.HasPrefix(part, "@") {
			owner := strings.TrimPrefix(part, "@")
			// Skip GitLab group references (e.g., @dataverse/dataverse-groups/...)
			// These are added by update-codeowners.py script automatically
			// We only validate individual user owners from YAML files
			if !strings.Contains(owner, "/") {
				owners = append(owners, owner)
			}
		}
	}

	if len(owners) == 0 {
		return nil
	}
	return &CODEOWNERSEntry{Path: parts[0], Owners: owners}
}

// buildExpectedEntries builds expected CODEOWNERS entries from YAML changes
func (r *CODEOWNERSSyncRule) buildExpectedEntries(yamlChanges []YAMLChangeInfo) []CODEOWNERSEntry {
	var entries []CODEOWNERSEntry

	for _, change := range yamlChanges {
		switch change.FileType {
		case "developers":
			entries = append(entries, CODEOWNERSEntry{
				Path:   "/" + change.DataProduct.Path + "/",
				Owners: change.OwnersApprovers,
			})
		case "group":
			entries = append(entries, CODEOWNERSEntry{
				Path:   "/" + change.FilePath,
				Owners: change.OwnersApprovers,
			})
			if change.GroupName != "" {
				entries = append(entries, CODEOWNERSEntry{
					Path:   "/" + change.DataProduct.Path + "/access-requests/groups/" + change.GroupName + "/",
					Owners: change.OwnersApprovers,
				})
			}
		}
	}
	return entries
}

// validateEntriesMatch validates that expected entries match added entries
func (r *CODEOWNERSSyncRule) validateEntriesMatch(expected, added []CODEOWNERSEntry) string {
	matched := make(map[int]bool)

	// Check each expected entry exists in added
	for _, exp := range expected {
		found := false
		for i, add := range added {
			if r.entriesMatch(exp, add) {
				found = true
				matched[i] = true
				break
			}
		}
		if !found {
			return "Missing CODEOWNERS entry for: " + exp.Path
		}
	}

	// Check for extra data product entries
	for i, add := range added {
		if !matched[i] && strings.HasPrefix(add.Path, "/dataproducts/") {
			return "Unexpected CODEOWNERS entry: " + add.Path
		}
	}

	return ""
}

// entriesMatch checks if two CODEOWNERS entries match
func (r *CODEOWNERSSyncRule) entriesMatch(expected, actual CODEOWNERSEntry) bool {
	if expected.Path != actual.Path {
		return false
	}

	if len(expected.Owners) != len(actual.Owners) {
		return false
	}

	expSorted := make([]string, len(expected.Owners))
	actSorted := make([]string, len(actual.Owners))
	copy(expSorted, expected.Owners)
	copy(actSorted, actual.Owners)
	sort.Strings(expSorted)
	sort.Strings(actSorted)

	for i := range expSorted {
		if expSorted[i] != actSorted[i] {
			return false
		}
	}
	return true
}
