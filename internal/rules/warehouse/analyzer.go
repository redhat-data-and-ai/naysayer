package warehouse

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
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

// Analyzer analyzes YAML files for warehouse changes
type Analyzer struct {
	gitlabClient *gitlab.Client
}

// NewAnalyzer creates a new warehouse analyzer
func NewAnalyzer(gitlabClient *gitlab.Client) *Analyzer {
	return &Analyzer{
		gitlabClient: gitlabClient,
	}
}

// AnalyzeChanges analyzes GitLab MR changes for warehouse modifications using proper YAML parsing
func (a *Analyzer) AnalyzeChanges(projectID, mrIID int, changes []gitlab.FileChange) ([]WarehouseChange, error) {
	var warehouseChanges []WarehouseChange

	for _, change := range changes {
		// Skip deleted files
		if change.DeletedFile {
			continue
		}

		// Only analyze dataproduct YAML files
		if !a.isDataProductFile(change.NewPath) {
			continue
		}

		// Analyze this specific file for warehouse changes
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

// isDataProductFile checks if a file is a dataproduct configuration file
func (a *Analyzer) isDataProductFile(path string) bool {
	if path == "" {
		return false
	}
	
	lowerPath := strings.ToLower(path)
	return strings.HasSuffix(lowerPath, "product.yaml") || strings.HasSuffix(lowerPath, "product.yml")
}

// analyzeFileChange fetches complete file content and compares YAML structures
func (a *Analyzer) analyzeFileChange(projectID, mrIID int, filePath string) (*[]WarehouseChange, error) {
	// Get target branch
	targetBranch, err := a.gitlabClient.GetMRTargetBranch(projectID, mrIID)
	if err != nil {
		return nil, fmt.Errorf("failed to get target branch: %v", err)
	}

	// Fetch file content from target branch (before changes)
	oldContent, err := a.gitlabClient.FetchFileContent(projectID, filePath, targetBranch)
	if err != nil {
		// File might be new, skip comparison
		if strings.Contains(err.Error(), "file not found") {
			return &[]WarehouseChange{}, nil
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

	// Parse both YAML contents
	oldDP, err := a.parseDataProduct(oldContent.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse old YAML: %v", err)
	}

	newDP, err := a.parseDataProduct(newContent.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse new YAML: %v", err)
	}

	// Compare warehouse configurations
	changes := a.compareWarehouses(filePath, oldDP, newDP)
	return &changes, nil
}

// parseDataProduct parses YAML content into DataProduct struct
func (a *Analyzer) parseDataProduct(content string) (*DataProduct, error) {
	var dp DataProduct
	err := yaml.Unmarshal([]byte(content), &dp)
	if err != nil {
		return nil, fmt.Errorf("YAML parsing error: %v", err)
	}
	return &dp, nil
}

// compareWarehouses compares warehouse configurations between old and new
func (a *Analyzer) compareWarehouses(filePath string, oldDP, newDP *DataProduct) []WarehouseChange {
	var changes []WarehouseChange

	// Create maps for easier comparison
	oldWarehouses := make(map[string]string) // type -> size
	newWarehouses := make(map[string]string) // type -> size

	for _, wh := range oldDP.Warehouses {
		oldWarehouses[wh.Type] = wh.Size
	}

	for _, wh := range newDP.Warehouses {
		newWarehouses[wh.Type] = wh.Size
	}

	// Check for warehouse size changes
	for whType, newSize := range newWarehouses {
		if oldSize, exists := oldWarehouses[whType]; exists {
			if oldSize != newSize {
				// Warehouse size changed
				oldValue, oldExists := WarehouseSizes[oldSize]
				newValue, newExists := WarehouseSizes[newSize]

				if oldExists && newExists {
					changes = append(changes, WarehouseChange{
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