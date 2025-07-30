package analyzer

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
	"github.com/redhat-data-and-ai/naysayer/internal/decision"
	"github.com/redhat-data-and-ai/naysayer/internal/gitlab"
)

// DataProduct represents the structure of a dataproduct YAML
type DataProduct struct {
	Name       string      `yaml:"name"`
	Kind       string      `yaml:"kind,omitempty"`
	RoverGroup string      `yaml:"rover_group"`
	Warehouses []Warehouse `yaml:"warehouses"`
	Tags       Tags        `yaml:"tags"`
}

// Warehouse represents a warehouse configuration
type Warehouse struct {
	Type string `yaml:"type"`
	Size string `yaml:"size"`
}

// Tags represents the tags section
type Tags struct {
	DataProduct string `yaml:"data_product"`
}

// YAMLAnalyzer analyzes YAML files for warehouse changes using proper YAML parsing
type YAMLAnalyzer struct {
	gitlabClient *gitlab.Client
}

// NewYAMLAnalyzer creates a new YAML analyzer
func NewYAMLAnalyzer(gitlabClient *gitlab.Client) *YAMLAnalyzer {
	return &YAMLAnalyzer{
		gitlabClient: gitlabClient,
	}
}

// AnalyzeChanges analyzes GitLab MR changes for warehouse modifications using proper YAML parsing
func (a *YAMLAnalyzer) AnalyzeChanges(projectID, mrIID int, changes *gitlab.MRChanges) ([]decision.WarehouseChange, error) {
	var warehouseChanges []decision.WarehouseChange

	for _, change := range changes.Changes {
		// Skip deleted files
		if change.DeletedFile {
			continue
		}

		// Only analyze product.yaml files
		if !a.isProductYAML(change.NewPath) {
			continue
		}

		// Get the complete file content for before and after
		fileChanges, err := a.analyzeFileChange(projectID, mrIID, change.NewPath)
		if err != nil {
			return nil, fmt.Errorf("failed to analyze file %s: %v", change.NewPath, err)
		}

		if fileChanges != nil {
			warehouseChanges = append(warehouseChanges, *fileChanges...)
		}
	}

	return warehouseChanges, nil
}


// isProductYAML checks if the file is a product.yaml file
func (a *YAMLAnalyzer) isProductYAML(path string) bool {
	path = strings.ToLower(path)
	return strings.HasSuffix(path, "product.yaml") || strings.HasSuffix(path, "product.yml")
}

// analyzeFileChange fetches complete file content and compares YAML structures
func (a *YAMLAnalyzer) analyzeFileChange(projectID, mrIID int, filePath string) (*[]decision.WarehouseChange, error) {
	// Get target branch (usually 'main' or 'master')
	targetBranch, err := a.gitlabClient.GetMRTargetBranch(projectID, mrIID)
	if err != nil {
		return nil, fmt.Errorf("failed to get target branch: %v", err)
	}

	// Fetch file content from target branch (before changes)
	oldContent, err := a.gitlabClient.FetchFileContent(projectID, filePath, targetBranch)
	if err != nil {
		// File might be new, skip comparison
		if strings.Contains(err.Error(), "file not found") {
			return &[]decision.WarehouseChange{}, nil
		}
		return nil, fmt.Errorf("failed to fetch old file content: %v", err)
	}

	// Get the MR details to find source branch
	mrDetails, err := a.gitlabClient.GetMRDetails(projectID, mrIID)
	if err != nil {
		return nil, fmt.Errorf("failed to get MR details: %v", err)
	}

	// Fetch file content from source branch (after changes)
	newContent, err := a.gitlabClient.FetchFileContent(projectID, filePath, mrDetails.SourceBranch)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch new file content: %v", err)
	}

	// Parse both YAML files
	oldDP, err := a.parseDataProductFromContent(oldContent.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse old YAML: %v", err)
	}

	newDP, err := a.parseDataProductFromContent(newContent.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse new YAML: %v", err)
	}

	// Compare warehouse configurations
	changes := a.compareWarehouses(filePath, oldDP, newDP)
	return &changes, nil
}

// parseDataProductFromContent parses YAML content from string
func (a *YAMLAnalyzer) parseDataProductFromContent(content string) (*DataProduct, error) {
	var dp DataProduct
	err := yaml.Unmarshal([]byte(content), &dp)
	if err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %v", err)
	}
	return &dp, nil
}

// compareWarehouses compares warehouse configurations between old and new
func (a *YAMLAnalyzer) compareWarehouses(filePath string, oldDP, newDP *DataProduct) []decision.WarehouseChange {
	var changes []decision.WarehouseChange

	// Create maps for easier comparison
	oldWarehouses := make(map[string]string) // type -> size
	newWarehouses := make(map[string]string) // type -> size

	for _, wh := range oldDP.Warehouses {
		oldWarehouses[wh.Type] = wh.Size
	}

	for _, wh := range newDP.Warehouses {
		newWarehouses[wh.Type] = wh.Size
	}

	// Check for changes
	for whType, newSize := range newWarehouses {
		if oldSize, exists := oldWarehouses[whType]; exists {
			if oldSize != newSize {
				// Warehouse size changed
				oldValue, oldExists := decision.WarehouseSizes[oldSize]
				newValue, newExists := decision.WarehouseSizes[newSize]

				if oldExists && newExists {
					changes = append(changes, decision.WarehouseChange{
						FilePath:   fmt.Sprintf("%s (type: %s)", filePath, whType),
						FromSize:   oldSize,
						ToSize:     newSize,
						IsDecrease: oldValue > newValue,
					})
				}
			}
		}
	}

	return changes
}

