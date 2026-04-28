package rules

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/redhat-data-and-ai/naysayer/internal/config"
	"github.com/redhat-data-and-ai/naysayer/internal/gitlab"
	"github.com/redhat-data-and-ai/naysayer/internal/rules/shared"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// forkMRTestGitLabClient simulates a fork MR: source branch exists only on SourceProjectID,
// while target branch files live on TargetProjectID (same as webhook project_id).
type forkMRTestGitLabClient struct {
	targetProjectID int
	sourceProjectID int
	targetBranch    string
	sourceBranch    string
	beforeYAML      string
	afterYAML       string

	FetchFileContentCalls []struct {
		ProjectID int
		FilePath  string
		Ref       string
	}
}

var _ gitlab.GitLabClient = (*forkMRTestGitLabClient)(nil)

func (m *forkMRTestGitLabClient) FetchFileContent(projectID int, filePath, ref string) (*gitlab.FileContent, error) {
	m.FetchFileContentCalls = append(m.FetchFileContentCalls, struct {
		ProjectID int
		FilePath  string
		Ref       string
	}{projectID, filePath, ref})

	switch {
	case projectID == m.targetProjectID && ref == m.targetBranch:
		return &gitlab.FileContent{Content: m.beforeYAML, FilePath: filePath}, nil
	case projectID == m.sourceProjectID && ref == m.sourceBranch:
		return &gitlab.FileContent{Content: m.afterYAML, FilePath: filePath}, nil
	case projectID == m.targetProjectID && ref == m.sourceBranch:
		// Simulates GitLab: source branch ref does not exist on target project
		return nil, fmt.Errorf("file not found: %s", filePath)
	default:
		return nil, fmt.Errorf("file not found: %s", filePath)
	}
}

func (m *forkMRTestGitLabClient) GetMRTargetBranch(projectID, mrIID int) (string, error) {
	return m.targetBranch, nil
}

func (m *forkMRTestGitLabClient) GetMRDetails(projectID, mrIID int) (*gitlab.MRDetails, error) {
	return &gitlab.MRDetails{
		IID:             mrIID,
		ProjectID:       projectID,
		SourceProjectID: m.sourceProjectID,
		TargetBranch:    m.targetBranch,
		SourceBranch:    m.sourceBranch,
	}, nil
}

func (m *forkMRTestGitLabClient) FetchMRChanges(projectID, mrIID int) ([]gitlab.FileChange, error) {
	return []gitlab.FileChange{{
		NewPath: "dataproducts/marketing/prod/product.yaml",
		Diff: `@@ -7,7 +7,7 @@
-  size: SMALL
+  size: MEDIUM`,
	}}, nil
}

func (m *forkMRTestGitLabClient) AddMRComment(projectID, mrIID int, comment string) error { return nil }
func (m *forkMRTestGitLabClient) AddOrUpdateMRComment(projectID, mrIID int, commentBody, commentType string) error {
	return nil
}
func (m *forkMRTestGitLabClient) ListMRComments(projectID, mrIID int) ([]gitlab.MRComment, error) {
	return nil, nil
}
func (m *forkMRTestGitLabClient) UpdateMRComment(projectID, mrIID, commentID int, newBody string) error {
	return nil
}
func (m *forkMRTestGitLabClient) FindLatestNaysayerComment(projectID, mrIID int, commentType ...string) (*gitlab.MRComment, error) {
	return nil, nil
}
func (m *forkMRTestGitLabClient) ApproveMR(projectID, mrIID int) error { return nil }
func (m *forkMRTestGitLabClient) ApproveMRWithMessage(projectID, mrIID int, message string) error {
	return nil
}
func (m *forkMRTestGitLabClient) ResetNaysayerApproval(projectID, mrIID int) error { return nil }
func (m *forkMRTestGitLabClient) GetCurrentBotUsername() (string, error) {
	return "naysayer-bot", nil
}
func (m *forkMRTestGitLabClient) IsNaysayerBotAuthor(author map[string]interface{}) bool {
	return false
}
func (m *forkMRTestGitLabClient) CompareBranches(sourceProjectID int, sourceBranch string, targetProjectID int, targetBranch string) (*gitlab.CompareResult, error) {
	return &gitlab.CompareResult{}, nil
}
func (m *forkMRTestGitLabClient) GetBranchCommit(projectID int, branch string) (string, error) {
	return "abc123", nil
}
func (m *forkMRTestGitLabClient) CompareCommits(projectID int, fromSHA, toSHA string) (*gitlab.CompareResult, error) {
	return &gitlab.CompareResult{}, nil
}
func (m *forkMRTestGitLabClient) RebaseMR(projectID, mrIID int) (bool, error) { return false, nil }
func (m *forkMRTestGitLabClient) ListOpenMRs(projectID int) ([]int, error)    { return nil, nil }
func (m *forkMRTestGitLabClient) ListOpenMRsWithDetails(projectID int) ([]gitlab.MRDetails, error) {
	return nil, nil
}
func (m *forkMRTestGitLabClient) ListAllOpenMRsWithDetails(projectID int) ([]gitlab.MRDetails, error) {
	return nil, nil
}
func (m *forkMRTestGitLabClient) CloseMR(projectID, mrIID int) error { return nil }
func (m *forkMRTestGitLabClient) FindCommentByPattern(projectID, mrIID int, pattern string) (bool, error) {
	return false, nil
}
func (m *forkMRTestGitLabClient) GetPipelineJobs(projectID, pipelineID int) ([]gitlab.PipelineJob, error) {
	return nil, nil
}
func (m *forkMRTestGitLabClient) GetJobTrace(projectID, jobID int) (string, error) { return "", nil }
func (m *forkMRTestGitLabClient) FindLatestAtlantisComment(projectID, mrIID int) (*gitlab.MRComment, error) {
	return nil, nil
}
func (m *forkMRTestGitLabClient) AreAllPipelineJobsSucceeded(projectID, pipelineID int) (bool, error) {
	return true, nil
}
func (m *forkMRTestGitLabClient) CheckAtlantisCommentForPlanFailures(projectID, mrIID int) (bool, string) {
	return false, ""
}

func TestSourceProjectIDForMR_Fork(t *testing.T) {
	ruleConfig := &config.GlobalRuleConfig{Enabled: true, Files: []config.FileRuleConfig{}}
	client := &forkMRTestGitLabClient{targetProjectID: 106670, sourceProjectID: 9999}
	mgr := NewSectionRuleManager(ruleConfig, client)

	mrCtx := &shared.MRContext{ProjectID: 106670, MRIID: 7309}
	assert.Equal(t, 9999, mgr.sourceProjectIDForMR(mrCtx))
}

func TestSourceProjectIDForMR_SameRepo(t *testing.T) {
	ruleConfig := &config.GlobalRuleConfig{Enabled: true, Files: []config.FileRuleConfig{}}
	client := &forkMRTestGitLabClient{targetProjectID: 106670, sourceProjectID: 106670}
	mgr := NewSectionRuleManager(ruleConfig, client)

	mrCtx := &shared.MRContext{ProjectID: 106670, MRIID: 1}
	assert.Equal(t, 106670, mgr.sourceProjectIDForMR(mrCtx))
}

func TestEvaluateAll_ForkMR_WarehouseIncreaseRequiresManualReview(t *testing.T) {
	tempDir := t.TempDir()
	rulesPath := filepath.Join(tempDir, "rules.yaml")
	rulesYAML := `enabled: true
files:
  - name: "product_configs"
    path: "dataproducts/**/"
    filename: "product.{yaml,yml}"
    parser_type: yaml
    enabled: true
    sections:
      - name: warehouses
        yaml_path: warehouses
        rule_configs:
          - name: warehouse_rule
            enabled: true
        auto_approve: false
`
	require.NoError(t, os.WriteFile(rulesPath, []byte(rulesYAML), 0600))

	client := &forkMRTestGitLabClient{
		targetProjectID: 106670,
		sourceProjectID: 9999,
		targetBranch:    "main",
		sourceBranch:    "feature/warehouse-scale-up",
		beforeYAML: `---
name: marketing
kind: aggregated
rover_group: dataverse-aggregate-marketing
warehouses:
- type: user
  size: SMALL
- type: service_account
  size: XSMALL
service_account:
  dbt: true
tags:
  data_product: marketing
data_product_db:
- database: marketing_db
  presentation_schemas:
  - name: marts
    consumers: []
`,
		afterYAML: `---
name: marketing
kind: aggregated
rover_group: dataverse-aggregate-marketing
warehouses:
- type: user
  size: MEDIUM
- type: service_account
  size: XSMALL
service_account:
  dbt: true
tags:
  data_product: marketing
data_product_db:
- database: marketing_db
  presentation_schemas:
  - name: marts
    consumers: []
`,
	}

	registry := GetGlobalRegistry()
	manager, err := registry.CreateSectionBasedRuleManager(client, rulesPath)
	require.NoError(t, err)

	mrCtx := &shared.MRContext{
		ProjectID: 106670,
		MRIID:     7309,
		MRInfo: &gitlab.MRInfo{
			SourceBranch: "feature/warehouse-scale-up",
			TargetBranch: "main",
		},
		Changes: []gitlab.FileChange{{
			NewPath: "dataproducts/marketing/prod/product.yaml",
			Diff: `@@ -7,7 +7,7 @@
-  size: SMALL
+  size: MEDIUM`,
		}},
	}

	result := manager.EvaluateAll(mrCtx)
	require.Equal(t, shared.ManualReview, result.FinalDecision.Type)
	assert.Contains(t, result.FinalDecision.Reason, "manual review")

	fv := result.FileValidations["dataproducts/marketing/prod/product.yaml"]
	require.NotNil(t, fv)
	var warehouseManual bool
	for _, rr := range fv.RuleResults {
		if rr.RuleName == "warehouse_rule" && rr.Decision == shared.ManualReview {
			warehouseManual = true
			assert.Contains(t, rr.Reason, "Warehouse")
			break
		}
	}
	assert.True(t, warehouseManual, "warehouse_rule should require manual review for fork MR with size increase")

	// Section manager must load source file from fork project, not target
	var sawSourceFetchOnFork bool
	for _, c := range client.FetchFileContentCalls {
		if c.ProjectID == 9999 && c.Ref == "feature/warehouse-scale-up" &&
			c.FilePath == "dataproducts/marketing/prod/product.yaml" {
			sawSourceFetchOnFork = true
			break
		}
	}
	assert.True(t, sawSourceFetchOnFork, "expected FetchFileContent on fork project for source branch")
}
