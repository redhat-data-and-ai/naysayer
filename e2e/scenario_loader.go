package e2e

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/redhat-data-and-ai/naysayer/internal/rules/shared"
	"gopkg.in/yaml.v3"
)

// ScenarioConfig represents a complete test scenario
type ScenarioConfig struct {
	Name        string
	Description string
	BeforeDir   string
	AfterDir    string
	Expected    ExpectedResults
	MRMetadata  MRMetadata
}

// ExpectedResults defines what to expect from the scenario
type ExpectedResults struct {
	Decision         shared.DecisionType
	Reason           string
	Approved         bool
	RulesEvaluated   []ExpectedRule
	CommentContains  []string
	CommentFile      string // Path to expected_comment.txt
}

// ExpectedRule defines expected rule evaluation
type ExpectedRule struct {
	Name     string
	Section  string
	Decision shared.DecisionType
	Reason   string
}

// MRMetadata contains MR-specific metadata
type MRMetadata struct {
	Title        string
	Author       string
	SourceBranch string
	TargetBranch string
}

// ScenarioYAML represents the YAML format for scenario.yaml
type ScenarioYAML struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Expected    struct {
		Decision        string `yaml:"decision"`
		Reason          string `yaml:"reason"`
		Approved        bool   `yaml:"approved"`
		RulesEvaluated  []struct {
			Name     string `yaml:"name"`
			Section  string `yaml:"section"`
			Decision string `yaml:"decision"`
			Reason   string `yaml:"reason"`
		} `yaml:"rules_evaluated"`
		CommentContains []string `yaml:"comment_contains"`
	} `yaml:"expected"`
	MRMetadata struct {
		Title        string `yaml:"title"`
		Author       string `yaml:"author"`
		SourceBranch string `yaml:"source_branch"`
		TargetBranch string `yaml:"target_branch"`
	} `yaml:"mr_metadata"`
}

// LoadScenarios discovers and loads all scenarios from testdata/scenarios/
func LoadScenarios(testdataPath string) ([]ScenarioConfig, error) {
	scenariosPath := filepath.Join(testdataPath, "scenarios")

	// Check if scenarios directory exists
	if _, err := os.Stat(scenariosPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("scenarios directory not found: %s", scenariosPath)
	}

	var scenarios []ScenarioConfig

	// Walk through all subdirectories
	err := filepath.Walk(scenariosPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Look for scenario.yaml files
		if !info.IsDir() && (info.Name() == "scenario.yaml" || info.Name() == "scenario.yml") {
			scenarioDir := filepath.Dir(path)
			scenario, err := LoadScenario(scenarioDir)
			if err != nil {
				return fmt.Errorf("failed to load scenario from %s: %w", scenarioDir, err)
			}
			scenarios = append(scenarios, *scenario)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return scenarios, nil
}

// LoadScenario loads a specific scenario from a directory
func LoadScenario(scenarioDir string) (*ScenarioConfig, error) {
	// Load scenario.yaml
	scenarioYAML, err := loadScenarioYAML(scenarioDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load scenario.yaml: %w", err)
	}

	// Build scenario config
	scenario := &ScenarioConfig{
		Name:        scenarioYAML.Name,
		Description: scenarioYAML.Description,
		BeforeDir:   filepath.Join(scenarioDir, "before"),
		AfterDir:    filepath.Join(scenarioDir, "after"),
		Expected: ExpectedResults{
			Decision:        parseDecisionType(scenarioYAML.Expected.Decision),
			Reason:          scenarioYAML.Expected.Reason,
			Approved:        scenarioYAML.Expected.Approved,
			CommentContains: scenarioYAML.Expected.CommentContains,
			CommentFile:     filepath.Join(scenarioDir, "expected_comment.txt"),
		},
		MRMetadata: MRMetadata{
			Title:        scenarioYAML.MRMetadata.Title,
			Author:       scenarioYAML.MRMetadata.Author,
			SourceBranch: scenarioYAML.MRMetadata.SourceBranch,
			TargetBranch: scenarioYAML.MRMetadata.TargetBranch,
		},
	}

	// Set defaults for MR metadata
	if scenario.MRMetadata.Author == "" {
		scenario.MRMetadata.Author = "testuser"
	}
	if scenario.MRMetadata.SourceBranch == "" {
		scenario.MRMetadata.SourceBranch = "feature/test"
	}
	if scenario.MRMetadata.TargetBranch == "" {
		scenario.MRMetadata.TargetBranch = "main"
	}
	if scenario.MRMetadata.Title == "" {
		scenario.MRMetadata.Title = scenario.Name
	}

	// Parse expected rules
	for _, ruleYAML := range scenarioYAML.Expected.RulesEvaluated {
		scenario.Expected.RulesEvaluated = append(scenario.Expected.RulesEvaluated, ExpectedRule{
			Name:     ruleYAML.Name,
			Section:  ruleYAML.Section,
			Decision: parseDecisionType(ruleYAML.Decision),
			Reason:   ruleYAML.Reason,
		})
	}

	// Validate scenario structure
	if err := validateScenario(scenario); err != nil {
		return nil, fmt.Errorf("scenario validation failed: %w", err)
	}

	return scenario, nil
}

// loadScenarioYAML loads and parses scenario.yaml
func loadScenarioYAML(scenarioDir string) (*ScenarioYAML, error) {
	// Try scenario.yaml first, then scenario.yml
	paths := []string{
		filepath.Join(scenarioDir, "scenario.yaml"),
		filepath.Join(scenarioDir, "scenario.yml"),
	}

	var content []byte
	var err error

	for _, path := range paths {
		content, err = os.ReadFile(path)
		if err == nil {
			break
		}
	}

	if err != nil {
		return nil, fmt.Errorf("scenario.yaml not found in %s", scenarioDir)
	}

	var scenarioYAML ScenarioYAML
	if err := yaml.Unmarshal(content, &scenarioYAML); err != nil {
		return nil, fmt.Errorf("failed to parse scenario.yaml: %w", err)
	}

	return &scenarioYAML, nil
}

// LoadExpectedComment loads the expected comment from expected_comment.txt
func LoadExpectedComment(commentPath string) (string, error) {
	// Check if file exists
	if _, err := os.Stat(commentPath); os.IsNotExist(err) {
		// expected_comment.txt is optional
		return "", nil
	}

	content, err := os.ReadFile(commentPath)
	if err != nil {
		return "", fmt.Errorf("failed to read expected_comment.txt: %w", err)
	}

	return strings.TrimSpace(string(content)), nil
}

// parseDecisionType converts string to DecisionType
func parseDecisionType(decision string) shared.DecisionType {
	decision = strings.ToLower(strings.TrimSpace(decision))
	switch decision {
	case "approve", "approved", "auto-approve":
		return shared.Approve
	case "manualreview", "manual_review", "manual-review", "manual review":
		return shared.ManualReview
	default:
		return shared.ManualReview // Default to safe option
	}
}

// validateScenario validates that a scenario has required structure
func validateScenario(scenario *ScenarioConfig) error {
	if scenario.Name == "" {
		return fmt.Errorf("scenario name is required")
	}

	// Check that before/ directory exists
	if _, err := os.Stat(scenario.BeforeDir); os.IsNotExist(err) {
		return fmt.Errorf("before/ directory not found: %s", scenario.BeforeDir)
	}

	// Check that after/ directory exists
	if _, err := os.Stat(scenario.AfterDir); os.IsNotExist(err) {
		return fmt.Errorf("after/ directory not found: %s", scenario.AfterDir)
	}

	return nil
}

// FilterScenariosByTag filters scenarios by tags in their names or descriptions
func FilterScenariosByTag(scenarios []ScenarioConfig, tags []string) []ScenarioConfig {
	if len(tags) == 0 {
		return scenarios
	}

	var filtered []ScenarioConfig
	for _, scenario := range scenarios {
		for _, tag := range tags {
			if strings.Contains(strings.ToLower(scenario.Name), strings.ToLower(tag)) ||
				strings.Contains(strings.ToLower(scenario.Description), strings.ToLower(tag)) {
				filtered = append(filtered, scenario)
				break
			}
		}
	}

	return filtered
}

// GetScenarioByName finds a scenario by name
func GetScenarioByName(scenarios []ScenarioConfig, name string) (*ScenarioConfig, error) {
	for _, scenario := range scenarios {
		if scenario.Name == name {
			return &scenario, nil
		}
	}
	return nil, fmt.Errorf("scenario not found: %s", name)
}

// CountScenariosByDecision counts scenarios by expected decision
func CountScenariosByDecision(scenarios []ScenarioConfig) (approve, manualReview int) {
	for _, scenario := range scenarios {
		if scenario.Expected.Decision == shared.Approve {
			approve++
		} else {
			manualReview++
		}
	}
	return
}
