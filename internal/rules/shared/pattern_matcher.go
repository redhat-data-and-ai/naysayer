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
// Supports advanced patterns like ** (globstar) and {yaml,yml} (brace expansion)
func (pm *PatternMatcher) MatchesPattern(filePath, pattern string) bool {
	if filePath == "" || pattern == "" {
		return false
	}

	// Handle brace expansion first: {yaml,yml} -> yaml,yml
	expandedPatterns := pm.expandBracePattern(pattern)

	for _, expandedPattern := range expandedPatterns {
		if pm.matchGlobstar(filePath, expandedPattern) {
			return true
		}
	}

	return false
}

// expandBracePattern expands brace patterns like {yaml,yml} into separate patterns
func (pm *PatternMatcher) expandBracePattern(pattern string) []string {
	// Find brace pattern
	startBrace := strings.Index(pattern, "{")
	endBrace := strings.Index(pattern, "}")

	if startBrace == -1 || endBrace == -1 || endBrace <= startBrace {
		// No brace pattern, return as-is
		return []string{pattern}
	}

	prefix := pattern[:startBrace]
	suffix := pattern[endBrace+1:]
	options := strings.Split(pattern[startBrace+1:endBrace], ",")

	var expanded []string
	for _, option := range options {
		expanded = append(expanded, prefix+option+suffix)
	}

	return expanded
}

// matchGlobstar handles ** globstar patterns
func (pm *PatternMatcher) matchGlobstar(filePath, pattern string) bool {
	// Handle ** globstar pattern
	if strings.Contains(pattern, "**") {
		return pm.matchGlobstarPattern(filePath, pattern)
	}

	// Use standard filepath.Match for simple patterns
	matched, err := filepath.Match(pattern, filePath)
	if err != nil {
		// Fallback to substring matching for complex patterns
		return strings.Contains(filePath, strings.ReplaceAll(pattern, "*", ""))
	}
	return matched
}

// matchGlobstarPattern handles patterns with ** (matches zero or more directories)
func (pm *PatternMatcher) matchGlobstarPattern(filePath, pattern string) bool {
	// Split pattern by **
	parts := strings.Split(pattern, "**")
	if len(parts) != 2 {
		// Multiple ** not supported, fallback to contains
		return strings.Contains(filePath, strings.ReplaceAll(pattern, "*", ""))
	}

	prefix := parts[0]
	suffix := parts[1]

	// Remove trailing slash from prefix and leading slash from suffix
	prefix = strings.TrimSuffix(prefix, "/")
	suffix = strings.TrimPrefix(suffix, "/")

	// Check if file path starts with prefix and ends with suffix
	if !strings.HasPrefix(filePath, prefix) {
		return false
	}

	if suffix == "" {
		return true // Pattern ends with **, matches everything after prefix
	}

	// For suffix, we need to match it at any directory level after prefix
	remaining := filePath[len(prefix):]
	if strings.HasPrefix(remaining, "/") {
		remaining = remaining[1:]
	}

	// Split remaining path into segments and check if any segment matches suffix pattern
	segments := strings.Split(remaining, "/")
	for i := 0; i < len(segments); i++ {
		// Reconstruct path from current segment to end
		testPath := strings.Join(segments[i:], "/")
		if matched, _ := filepath.Match(suffix, testPath); matched {
			return true
		}
	}

	return false
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
