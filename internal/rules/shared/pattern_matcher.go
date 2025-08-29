package shared

import (
	"path/filepath"
	"strings"
)

// PatternMatcher provides consolidated file pattern matching logic
type PatternMatcher struct{}

// NewPatternMatcher creates a new pattern matcher
func NewPatternMatcher() *PatternMatcher {
	return &PatternMatcher{}
}

// MatchesPattern checks if a file path matches a glob pattern
// This consolidates the pattern matching logic used across the codebase
func (pm *PatternMatcher) MatchesPattern(filePath, pattern string) bool {
	if filePath == "" || pattern == "" {
		return false
	}

	// Try standard filepath.Match first (handles *, ?, [])
	matched, err := filepath.Match(pattern, filePath)
	if err != nil {
		// Fallback to substring matching for complex patterns
		return strings.Contains(filePath, strings.ReplaceAll(pattern, "*", ""))
	}
	return matched
}

// MatchesAnyPattern checks if a file path matches any of the given patterns
func (pm *PatternMatcher) MatchesAnyPattern(filePath string, patterns []string) bool {
	if len(patterns) == 0 {
		return true // No patterns means match all
	}

	for _, pattern := range patterns {
		if pm.MatchesPattern(filePath, pattern) {
			return true
		}
	}
	return false
}

// Global pattern matcher instance for convenience
var GlobalPatternMatcher = NewPatternMatcher()

// Convenience functions for backward compatibility
func MatchesPattern(filePath, pattern string) bool {
	return GlobalPatternMatcher.MatchesPattern(filePath, pattern)
}

func MatchesAnyPattern(filePath string, patterns []string) bool {
	return GlobalPatternMatcher.MatchesAnyPattern(filePath, patterns)
}
