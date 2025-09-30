package shared

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPatternMatcher_MatchesPattern(t *testing.T) {
	pm := NewPatternMatcher()

	tests := []struct {
		name     string
		filePath string
		pattern  string
		expected bool
	}{
		// Basic glob patterns
		{"exact match", "product.yaml", "product.yaml", true},
		{"wildcard match", "product.yaml", "*.yaml", true},
		{"wildcard no match", "product.txt", "*.yaml", false},
		{"question mark match", "product.yaml", "product.???l", true},
		{"question mark no match", "product.yaml", "product.??", false},

		// Character classes
		{"character class match", "product.yaml", "product.[yt]*", true},
		{"character class no match", "product.json", "product.[yt]*", false},

		// Path patterns (filepath.Match doesn't support ** patterns)
		{"path wildcard", "dataproducts/analytics/product.yaml", "dataproducts/*/product.yaml", true},
		{"nested path wildcard", "dataproducts/source/platform/dev/product.yaml", "dataproducts/*/*/product.yaml", false},

		// Edge cases
		{"empty file path", "", "*.yaml", false},
		{"empty pattern", "product.yaml", "", false},
		{"both empty", "", "", false},

		// Complex patterns - ** actually matches as literal **
		{"complex pattern matches literally", "dataproducts/analytics/product.yaml", "dataproducts/*/product.yaml", true},
		{"exact path match", "some/path/file.txt", "some/path/file.txt", true},

		// Case sensitivity
		{"case sensitive match", "Product.YAML", "Product.YAML", true},
		{"case sensitive no match", "product.yaml", "Product.YAML", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := pm.MatchesPattern(tt.filePath, tt.pattern)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPatternMatcher_MatchesAnyPattern(t *testing.T) {
	pm := NewPatternMatcher()

	tests := []struct {
		name     string
		filePath string
		patterns []string
		expected bool
	}{
		{
			name:     "matches first pattern",
			filePath: "product.yaml",
			patterns: []string{"*.yaml", "*.yml", "*.json"},
			expected: true,
		},
		{
			name:     "matches middle pattern",
			filePath: "config.yml",
			patterns: []string{"*.json", "*.yml", "*.txt"},
			expected: true,
		},
		{
			name:     "matches last pattern",
			filePath: "readme.txt",
			patterns: []string{"*.yaml", "*.yml", "*.txt"},
			expected: true,
		},
		{
			name:     "no patterns match",
			filePath: "script.sh",
			patterns: []string{"*.yaml", "*.yml", "*.json"},
			expected: false,
		},
		{
			name:     "empty patterns - should match all",
			filePath: "anything.txt",
			patterns: []string{},
			expected: true,
		},
		{
			name:     "nil patterns - should match all",
			filePath: "anything.txt",
			patterns: nil,
			expected: true,
		},
		{
			name:     "single pattern match",
			filePath: "dataproducts/analytics/product.yaml",
			patterns: []string{"dataproducts/*/product.yaml"},
			expected: true,
		},
		{
			name:     "single pattern no match",
			filePath: "README.md",
			patterns: []string{"*.yaml"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := pm.MatchesAnyPattern(tt.filePath, tt.patterns)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGlobalPatternMatcher(t *testing.T) {
	// Test that the global instance works
	assert.NotNil(t, GlobalPatternMatcher)

	// Test convenience functions
	assert.True(t, MatchesPattern("product.yaml", "*.yaml"))
	assert.False(t, MatchesPattern("product.txt", "*.yaml"))

	assert.True(t, MatchesAnyPattern("product.yaml", []string{"*.txt", "*.yaml"}))
	assert.False(t, MatchesAnyPattern("product.json", []string{"*.txt", "*.yaml"}))
}

func TestNewPatternMatcher(t *testing.T) {
	pm := NewPatternMatcher()
	assert.NotNil(t, pm)

	// Test that it works
	assert.True(t, pm.MatchesPattern("test.yaml", "*.yaml"))
}

func TestPatternMatcher_EdgeCases(t *testing.T) {
	pm := NewPatternMatcher()

	tests := []struct {
		name     string
		filePath string
		pattern  string
		expected bool
	}{
		// Test patterns that cause filepath.Match errors and trigger fallback
		{"invalid pattern bracket", "test.txt", "[", false}, // Error but no match in fallback
		{"invalid pattern escape", "test.txt", "\\", false},
		{"complex nested path", "a/very/deep/nested/path/file.yaml", "**/file.yaml", true}, // Now supports ** globstar patterns
		{"pattern with spaces", "file with spaces.txt", "*spaces*", true},
		{"unicode filename", "测试.yaml", "*.yaml", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := pm.MatchesPattern(tt.filePath, tt.pattern)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestPatternMatcher_GlobstarPatterns tests the new ** globstar functionality
func TestPatternMatcher_GlobstarPatterns(t *testing.T) {
	pm := NewPatternMatcher()

	tests := []struct {
		name     string
		filePath string
		pattern  string
		expected bool
	}{
		{"dataproduct nested path", "dataproducts/source/revstream/dev/product.yaml", "dataproducts/**/product.yaml", true},
		{"dataproduct with environment", "dataproducts/source/revstream/preprod/product.yaml", "dataproducts/**/product.yaml", true},
		{"brace expansion yaml", "dataproducts/test/product.yaml", "dataproducts/**/product.{yaml,yml}", true},
		{"brace expansion yml", "dataproducts/test/product.yml", "dataproducts/**/product.{yaml,yml}", true},
		{"brace expansion no match", "dataproducts/test/product.json", "dataproducts/**/product.{yaml,yml}", false},
		{"globstar at end", "dataproducts/any/deep/path", "dataproducts/**", true},
		{"globstar with suffix", "deep/nested/file.txt", "**/file.txt", true},
		{"no match prefix", "other/nested/file.txt", "dataproducts/**/file.txt", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := pm.MatchesPattern(tt.filePath, tt.pattern)
			assert.Equal(t, tt.expected, result)
		})
	}
}
