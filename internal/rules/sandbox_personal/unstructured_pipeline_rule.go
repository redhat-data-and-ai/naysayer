package sandbox_personal

import (
	"fmt"
	"strings"

	"github.com/redhat-data-and-ai/naysayer/internal/gitlab"
	"github.com/redhat-data-and-ai/naysayer/internal/logging"
	"github.com/redhat-data-and-ai/naysayer/internal/rules/common"
	"github.com/redhat-data-and-ai/naysayer/internal/rules/shared"
	"gopkg.in/yaml.v3"
)

// UnstructuredPipelineRule validates unstructured-data-pipeline.yaml files
// in sandbox environments for UnstructuredDataProducts.
// Validates that the file exists in correct path and s3 prefixes match the product name.
type UnstructuredPipelineRule struct {
	*common.BaseRule
	*common.ValidationHelper
	client    gitlab.GitLabClient
	mrContext *shared.MRContext
}

// PipelineConfig represents the structure of unstructured-data-pipeline.yaml
type PipelineConfig struct {
	SourceCrawlerConfig      SourceCrawlerConfig      `yaml:"source_crawler_config"`
	DestinationSyncerConfig DestinationSyncerConfig `yaml:"destination_syncer_config"`
}

// SourceCrawlerConfig represents the source crawler configuration
type SourceCrawlerConfig struct {
	Type              string            `yaml:"type"`
	S3Config          S3Config          `yaml:"s3Config,omitempty"`
	GoogleDriveConfig GoogleDriveConfig `yaml:"googleDriveConfig,omitempty"`
}

// DestinationSyncerConfig represents the destination syncer configuration
type DestinationSyncerConfig struct {
	Type                string                  `yaml:"type"`
	S3DestinationConfig S3DestinationConfigType `yaml:"s3DestinationConfig"`
}

// S3Config represents s3 source configuration
type S3Config struct {
	Bucket string `yaml:"bucket"`
	Prefix string `yaml:"prefix"`
}

// GoogleDriveConfig represents google drive source configuration
type GoogleDriveConfig struct {
	FolderIDs []FolderID `yaml:"folder_ids"`
}

// FolderID represents a single folder ID entry
type FolderID struct {
	ID string `yaml:"id"`
}

// S3DestinationConfigType represents s3 destination configuration
type S3DestinationConfigType struct {
	Bucket string `yaml:"bucket"`
	Prefix string `yaml:"prefix"`
}

// NewUnstructuredPipelineRule creates a new unstructured pipeline rule instance
func NewUnstructuredPipelineRule(client gitlab.GitLabClient) *UnstructuredPipelineRule {
	return &UnstructuredPipelineRule{
		BaseRule:         common.NewBaseRule("sandbox_unstructured_pipeline_rule", "Validates sandbox unstructured-data-pipeline.yaml exists in correct path with matching prefixes"),
		ValidationHelper: common.NewValidationHelper(),
		client:           client,
	}
}

// SetMRContext implements the ContextAwareRule interface
func (r *UnstructuredPipelineRule) SetMRContext(mrCtx *shared.MRContext) {
	r.mrContext = mrCtx
}

// ValidateLines validates the unstructured pipeline file
func (r *UnstructuredPipelineRule) ValidateLines(filePath string, fileContent string, lineRanges []shared.LineRange) (shared.DecisionType, string) {
	if r.mrContext == nil {
		return r.CreateApprovalResult("Auto-approved: No MR context")
	}

	// Only apply when the associated product is a sandbox UnstructuredDataProduct with aif-* name
	isAIFProduct, err := IsSandboxPersonalProductForFile(r.mrContext, r.client, filePath)
	if err != nil {
		// Fail-closed: if we can't verify the product type, require manual review
		logging.Error("[%s] Failed to check product type: %v", r.Name(), err)
		return r.CreateManualReviewResult(fmt.Sprintf("Manual review required: Failed to verify product type: %v", err))
	}
	if !isAIFProduct {
		return r.CreateApprovalResult("Auto-approved: Not a sandbox UnstructuredDataProduct with aif-* name")
	}

	// Validate file is in correct path: dataproducts/unstructured/{product_name}/sandbox/
	if !strings.Contains(filePath, "dataproducts/unstructured/") || !strings.Contains(filePath, "/sandbox/unstructured-data-pipeline.yaml") {
		reason := "Manual review required: unstructured-data-pipeline.yaml must be in dataproducts/unstructured/{product_name}/sandbox/"
		logging.Warn("[%s] %s", r.Name(), reason)
		return r.CreateManualReviewResult(reason)
	}

	// Extract product name from file path
	productName, err := r.extractProductName(filePath)
	if err != nil {
		logging.Error("[%s] Failed to extract product name: %v", r.Name(), err)
		return r.CreateManualReviewResult(fmt.Sprintf("Manual review required: Failed to extract product name: %v", err))
	}

	// Parse the pipeline YAML content
	var pipelineConfig PipelineConfig
	err = yaml.Unmarshal([]byte(fileContent), &pipelineConfig)
	if err != nil {
		logging.Error("[%s] Failed to parse unstructured-data-pipeline.yaml: %v", r.Name(), err)
		return r.CreateManualReviewResult(fmt.Sprintf("Manual review required: Failed to parse unstructured-data-pipeline.yaml: %v", err))
	}

	// Validate source_crawler_config based on type
	sourceType := pipelineConfig.SourceCrawlerConfig.Type
	switch sourceType {
	case "s3":
		// Validate source prefix matches product name
		expectedSourcePrefix := productName + "/source/"
		if !strings.HasPrefix(pipelineConfig.SourceCrawlerConfig.S3Config.Prefix, expectedSourcePrefix) {
			reason := fmt.Sprintf("Manual review required: source_crawler_config.s3Config.prefix '%s' must start with '%s'",
				pipelineConfig.SourceCrawlerConfig.S3Config.Prefix, expectedSourcePrefix)
			logging.Warn("[%s] %s", r.Name(), reason)
			return r.CreateManualReviewResult(reason)
		}
		logging.Info("[%s] S3 source validation passed: prefix matches product name '%s'", r.Name(), productName)

	case "google_drive":
		// Validate that folder_ids exists and has at least one entry
		if len(pipelineConfig.SourceCrawlerConfig.GoogleDriveConfig.FolderIDs) == 0 {
			reason := "Manual review required: source_crawler_config.googleDriveConfig.folder_ids must have at least one folder ID"
			logging.Warn("[%s] %s", r.Name(), reason)
			return r.CreateManualReviewResult(reason)
		}
		// Validate that at least one folder ID is non-empty
		hasValidID := false
		for _, folder := range pipelineConfig.SourceCrawlerConfig.GoogleDriveConfig.FolderIDs {
			if strings.TrimSpace(folder.ID) != "" {
				hasValidID = true
				break
			}
		}
		if !hasValidID {
			reason := "Manual review required: source_crawler_config.googleDriveConfig.folder_ids must contain at least one non-empty folder ID"
			logging.Warn("[%s] %s", r.Name(), reason)
			return r.CreateManualReviewResult(reason)
		}
		logging.Info("[%s] Google Drive source validation passed: folder_ids present", r.Name())

	default:
		// Unsupported type - require manual review
		reason := fmt.Sprintf("Manual review required: source_crawler_config.type '%s' is not supported (only 's3' and 'google_drive' are allowed)", sourceType)
		logging.Warn("[%s] %s", r.Name(), reason)
		return r.CreateManualReviewResult(reason)
	}

	// Validate destination_syncer_config - must always be s3
	if pipelineConfig.DestinationSyncerConfig.Type != "s3" {
		reason := fmt.Sprintf("Manual review required: destination_syncer_config.type must be 's3', found '%s'",
			pipelineConfig.DestinationSyncerConfig.Type)
		logging.Warn("[%s] %s", r.Name(), reason)
		return r.CreateManualReviewResult(reason)
	}

	// Validate destination prefix matches product name
	expectedDestPrefix := productName + "/destination/"
	if !strings.HasPrefix(pipelineConfig.DestinationSyncerConfig.S3DestinationConfig.Prefix, expectedDestPrefix) {
		reason := fmt.Sprintf("Manual review required: destination_syncer_config.s3DestinationConfig.prefix '%s' must start with '%s'",
			pipelineConfig.DestinationSyncerConfig.S3DestinationConfig.Prefix, expectedDestPrefix)
		logging.Warn("[%s] %s", r.Name(), reason)
		return r.CreateManualReviewResult(reason)
	}

	logging.Info("[%s] Validation passed: file in correct path, configuration valid for product '%s' with source type '%s'", r.Name(), productName, sourceType)
	return r.CreateApprovalResult(fmt.Sprintf("Auto-approved: Unstructured data pipeline configuration valid for product '%s' (source: %s)", productName, sourceType))
}

// GetCoveredLines returns the full file coverage
func (r *UnstructuredPipelineRule) GetCoveredLines(filePath string, fileContent string) []shared.LineRange {
	// Cover the entire file
	return r.GetFullFileCoverage(filePath, fileContent)
}

// extractProductName extracts the product name from file path
// Example: dataproducts/unstructured/aif-test/sandbox/unstructured-data-pipeline.yaml -> aif-test
func (r *UnstructuredPipelineRule) extractProductName(filePath string) (string, error) {
	parts := strings.Split(filePath, "/")
	for i, part := range parts {
		if part == "dataproducts" && i+2 < len(parts) && parts[i+1] == "unstructured" {
			return parts[i+2], nil
		}
	}
	return "", fmt.Errorf("could not extract product name from path: %s", filePath)
}
