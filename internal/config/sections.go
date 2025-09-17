package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// SectionDefinition defines how to identify and parse a section within a file
type SectionDefinition struct {
	Name        string   `yaml:"name"`         // Section identifier (e.g., "warehouse", "consumers")
	YAMLPath    string   `yaml:"yaml_path"`    // YAML path to section (e.g., "spec.warehouse")
	Required    bool     `yaml:"required"`     // Is this section required in the file?
	RuleNames   []string `yaml:"rule_names"`   // Rules that should validate this section
	AutoApprove bool     `yaml:"auto_approve"` // Auto-approve this section if rules pass (or no rules)
	Description string   `yaml:"description"`  // Human-readable description
}

// FileRuleConfig defines sections and rules for a specific file type
type FileRuleConfig struct {
	Name          string              `yaml:"name"`           // Unique identifier for this file type
	Path          string              `yaml:"path"`           // Directory path pattern (e.g., "**/" or "serviceaccounts/**/")
	Filename      string              `yaml:"filename"`       // Filename pattern (e.g., "product.{yaml,yml}")
	ParserType    string              `yaml:"parser_type"`    // Parser to use (yaml, json, etc.)
	Description   string              `yaml:"description"`    // Description of this file type
	Enabled       bool                `yaml:"enabled"`        // Enable/disable this file type
	DefaultAction string              `yaml:"default_action"` // Default action for unconfigured sections (manual_review, auto_approve)
	Sections      []SectionDefinition `yaml:"sections"`       // Sections within this file type
}

// RuleConfig holds the complete rule configuration for all file types
type RuleConfig struct {
	Enabled                 bool             `yaml:"enabled"`
	Files                   []FileRuleConfig `yaml:"files"`                      // Array of file configurations
	RequireFullCoverage     bool             `yaml:"require_full_coverage"`      // All sections must have rules
	ManualReviewOnUncovered bool             `yaml:"manual_review_on_uncovered"` // Manual review for uncovered sections
}

// RuleBasedConfig is the external YAML format for rule configuration
type RuleBasedConfig struct {
	Enabled                 bool             `yaml:"enabled"`
	Files                   []FileRuleConfig `yaml:"files"`                      // Array of file configurations
	RequireFullCoverage     bool             `yaml:"require_full_coverage"`      // All sections must have rules
	ManualReviewOnUncovered bool             `yaml:"manual_review_on_uncovered"` // Manual review for uncovered changes
}

// LoadRuleConfig loads rule-based validation configuration from YAML
// The YAML file must exist and be valid - no fallbacks or defaults
func LoadRuleConfig(configPath string) (*RuleConfig, error) {
	// If no config path provided, use default
	if configPath == "" {
		configPath = "rules.yaml"
	}

	// Check if YAML config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("rule config file not found: %s (create this file to define validation rules)", configPath)
	}

	// Read YAML config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read rule config file %s: %w", configPath, err)
	}

	// Parse YAML configuration
	var yamlConfig RuleBasedConfig
	if err := yaml.Unmarshal(data, &yamlConfig); err != nil {
		return nil, fmt.Errorf("failed to parse rule config YAML %s: %w", configPath, err)
	}

	// Convert YAML config to internal format
	config := &RuleConfig{
		Enabled:                 yamlConfig.Enabled,
		Files:                   yamlConfig.Files,
		RequireFullCoverage:     yamlConfig.RequireFullCoverage,
		ManualReviewOnUncovered: yamlConfig.ManualReviewOnUncovered,
	}

	// Validate the configuration
	if err := ValidateRuleConfig(config); err != nil {
		return nil, fmt.Errorf("invalid rule configuration in %s: %w", configPath, err)
	}

	return config, nil
}

// SaveRuleConfig saves rule configuration to file (for custom configs)
func SaveRuleConfig(config *RuleConfig, configPath string) error {
	// Convert internal config to external format
	externalConfig := RuleBasedConfig{
		Enabled:                 config.Enabled,
		Files:                   config.Files,
		RequireFullCoverage:     config.RequireFullCoverage,
		ManualReviewOnUncovered: config.ManualReviewOnUncovered,
	}

	// Marshal to YAML
	data, err := yaml.Marshal(&externalConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal section config: %w", err)
	}

	// Ensure directory exists
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Write to file
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write section config file: %w", err)
	}

	return nil
}

// ValidateRuleConfig validates the rule configuration
func ValidateRuleConfig(config *RuleConfig) error {
	if config == nil {
		return fmt.Errorf("section config is nil")
	}

	// Validate file configurations
	if len(config.Files) == 0 {
		return fmt.Errorf("no file patterns defined")
	}

	// Validate each file configuration
	for i, fileConfig := range config.Files {
		if fileConfig.Name == "" {
			return fmt.Errorf("file configuration at index %d missing name", i)
		}
		if fileConfig.Path == "" {
			return fmt.Errorf("file configuration %s missing path", fileConfig.Name)
		}
		if fileConfig.Filename == "" {
			return fmt.Errorf("file configuration %s missing filename", fileConfig.Name)
		}
		if fileConfig.ParserType == "" {
			return fmt.Errorf("file configuration %s missing parser type", fileConfig.Name)
		}

		// Validate default_action if specified
		if fileConfig.DefaultAction != "" &&
			fileConfig.DefaultAction != "manual_review" &&
			fileConfig.DefaultAction != "auto_approve" {
			return fmt.Errorf("invalid default_action '%s' for file config '%s'. Must be 'manual_review' or 'auto_approve'",
				fileConfig.DefaultAction, fileConfig.Name)
		}

		if len(fileConfig.Sections) == 0 {
			return fmt.Errorf("file configuration %s has no section definitions", fileConfig.Name)
		}

		for _, section := range fileConfig.Sections {
			if section.Name == "" {
				return fmt.Errorf("section definition missing name in file configuration %s", fileConfig.Name)
			}
			if section.YAMLPath == "" {
				return fmt.Errorf("section %s missing YAML path in file configuration %s", section.Name, fileConfig.Name)
			}

			// Auto-approve sections can have no rules, but warn if auto_approve is set with no rules
			if len(section.RuleNames) == 0 && !section.AutoApprove {
				return fmt.Errorf("section %s has no rules defined and auto_approve is false in file configuration %s", section.Name, fileConfig.Name)
			}
		}
	}

	return nil
}

// GetRuleConfigFromEnv loads rule config with environment variable overrides
func GetRuleConfigFromEnv() (*RuleConfig, error) {
	// Load base config
	configPath := os.Getenv("RULE_CONFIG_PATH")
	if configPath == "" {
		configPath = os.Getenv("SECTION_CONFIG_PATH") // Backward compatibility
	}

	config, err := LoadRuleConfig(configPath)
	if err != nil {
		return nil, err
	}

	// Apply environment variable overrides
	if enabled := os.Getenv("RULE_VALIDATION_ENABLED"); enabled != "" {
		config.Enabled = enabled == "true"
	} else if enabled := os.Getenv("SECTION_VALIDATION_ENABLED"); enabled != "" {
		config.Enabled = enabled == "true" // Backward compatibility
	}

	if requireCoverage := os.Getenv("RULE_REQUIRE_FULL_COVERAGE"); requireCoverage != "" {
		config.RequireFullCoverage = requireCoverage == "true"
	} else if requireCoverage := os.Getenv("SECTION_REQUIRE_FULL_COVERAGE"); requireCoverage != "" {
		config.RequireFullCoverage = requireCoverage == "true" // Backward compatibility
	}

	if manualReview := os.Getenv("RULE_MANUAL_REVIEW_ON_UNCOVERED"); manualReview != "" {
		config.ManualReviewOnUncovered = manualReview == "true"
	} else if manualReview := os.Getenv("SECTION_MANUAL_REVIEW_ON_UNCOVERED"); manualReview != "" {
		config.ManualReviewOnUncovered = manualReview == "true" // Backward compatibility
	}

	return config, nil
}
