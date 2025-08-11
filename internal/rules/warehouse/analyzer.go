package warehouse

import (
	"fmt"
	"strings"

	"github.com/redhat-data-and-ai/naysayer/internal/gitlab"
	"github.com/redhat-data-and-ai/naysayer/internal/rules/shared"
	"gopkg.in/yaml.v3"
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

// GitLabClientInterface defines the interface for GitLab API operations needed by the analyzer
type GitLabClientInterface interface {
	GetMRTargetBranch(projectID, mrIID int) (string, error)
	FetchFileContent(projectID int, filePath, ref string) (*gitlab.FileContent, error)
	GetMRDetails(projectID, mrIID int) (*gitlab.MRDetails, error)
}

// AnalyzerInterface defines the interface for warehouse analyzers
type AnalyzerInterface interface {
	AnalyzeChanges(projectID, mrIID int, changes []gitlab.FileChange) ([]WarehouseChange, error)
}

// Analyzer analyzes YAML files for warehouse changes
type Analyzer struct {
	gitlabClient GitLabClientInterface
}

// NewAnalyzer creates a new warehouse analyzer
func NewAnalyzer(gitlabClient GitLabClientInterface) *Analyzer {
	return &Analyzer{
		gitlabClient: gitlabClient,
	}
}

// AnalyzeChanges analyzes GitLab MR changes for warehouse modifications using proper YAML parsing
func (a *Analyzer) AnalyzeChanges(projectID, mrIID int, changes []gitlab.FileChange) ([]WarehouseChange, error) {
	warehouseChanges := make([]WarehouseChange, 0)

	for _, change := range changes {
		// Skip deleted files
		if change.DeletedFile {
			continue
		}

		// Only analyze dataproduct YAML files
		if !shared.IsDataProductFile(change.NewPath) {
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

// analyzeFileChange fetches complete file content and compares YAML structures
func (a *Analyzer) analyzeFileChange(projectID, mrIID int, filePath string) (*[]WarehouseChange, error) {
	// Get target branch
	targetBranch, err := a.gitlabClient.GetMRTargetBranch(projectID, mrIID)
	if err != nil {
		return nil, fmt.Errorf("failed to get target branch: %v", err)
	}

	// Get the MR details to find source branch
	mrDetails, err := a.gitlabClient.GetMRDetails(projectID, mrIID)
	if err != nil {
		return nil, fmt.Errorf("failed to get MR details: %v", err)
	}

	// Determine project IDs for target and source branches
	targetProjectID := projectID // Always use the target project ID for target branch
	sourceProjectID := projectID // Default to target project ID
	
	// For cross-fork MRs, use the source project ID for source branch
	if mrDetails.SourceProjectID != 0 && mrDetails.SourceProjectID != targetProjectID {
		sourceProjectID = mrDetails.SourceProjectID
	}

	// Fetch file content from target branch (before changes)
	oldContent, err := a.gitlabClient.FetchFileContent(targetProjectID, filePath, targetBranch)
	if err != nil && strings.Contains(err.Error(), "file not found") {
		// File is new - doesn't exist in target branch
		// Try to fetch from source branch to analyze the new file
		newContent, err := a.gitlabClient.FetchFileContent(sourceProjectID, filePath, mrDetails.SourceBranch)
		if err != nil {
			if strings.Contains(err.Error(), "file not found") {
				// File doesn't exist in either branch - this shouldn't happen for non-deleted files
				return &[]WarehouseChange{}, nil
			}
			return nil, fmt.Errorf("failed to fetch new file content from source project %d, branch %s: %v", sourceProjectID, mrDetails.SourceBranch, err)
		}
		
		// New file - compare empty state with new content
		oldDP := &DataProduct{Warehouses: []Warehouse{}}
		newDP, err := a.parseDataProduct(newContent.Content)
		if err != nil {
			return nil, fmt.Errorf("failed to parse new YAML file: %v", err)
		}
		changes := a.compareWarehouses(filePath, oldDP, newDP)
		return &changes, nil
	} else if err != nil {
		return nil, fmt.Errorf("failed to fetch old file content from target project %d, branch %s: %v", targetProjectID, targetBranch, err)
	}

	// Fetch file content from source branch (after changes)
	newContent, err := a.gitlabClient.FetchFileContent(sourceProjectID, filePath, mrDetails.SourceBranch)
	if err != nil {
		// File might be deleted in source branch
		if strings.Contains(err.Error(), "file not found") {
			// File was deleted in source branch - compare old content with empty state
			newDP := &DataProduct{Warehouses: []Warehouse{}}
			oldDP, err := a.parseDataProduct(oldContent.Content)
			if err != nil {
				return nil, fmt.Errorf("failed to parse old YAML for deleted file: %v", err)
			}
			changes := a.compareWarehouses(filePath, oldDP, newDP)
			return &changes, nil
		}
		return nil, fmt.Errorf("failed to fetch new file content from source project %d, branch %s: %v", sourceProjectID, mrDetails.SourceBranch, err)
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
	changes := make([]WarehouseChange, 0)

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
