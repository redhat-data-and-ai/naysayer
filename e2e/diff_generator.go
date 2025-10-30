package e2e

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pmezard/go-difflib/difflib"
	"github.com/redhat-data-and-ai/naysayer/internal/gitlab"
)

// FileOperation represents the type of file operation
type FileOperation int

const (
	FileAdded FileOperation = iota
	FileModified
	FileDeleted
)

// FileComparison represents a comparison between before and after files
type FileComparison struct {
	Path       string
	Operation  FileOperation
	OldContent string
	NewContent string
}

// CompareFolders compares before/ and after/ directories and returns file changes
func CompareFolders(beforeDir, afterDir string) ([]gitlab.FileChange, error) {
	// Build file maps for both directories
	beforeFiles, err := buildFileMap(beforeDir)
	if err != nil {
		return nil, fmt.Errorf("failed to scan before directory: %w", err)
	}

	afterFiles, err := buildFileMap(afterDir)
	if err != nil {
		return nil, fmt.Errorf("failed to scan after directory: %w", err)
	}

	// Compare files and generate changes
	comparisons := compareFileMaps(beforeFiles, afterFiles)

	// Convert to gitlab.FileChange format
	var changes []gitlab.FileChange
	for _, comp := range comparisons {
		change, err := createFileChange(comp)
		if err != nil {
			return nil, fmt.Errorf("failed to create file change for %s: %w", comp.Path, err)
		}
		changes = append(changes, change)
	}

	return changes, nil
}

// buildFileMap walks a directory and builds a map of relative paths to content
func buildFileMap(rootDir string) (map[string]string, error) {
	fileMap := make(map[string]string)

	// Check if directory exists
	if _, err := os.Stat(rootDir); os.IsNotExist(err) {
		// Empty directory is valid (e.g., for new file scenarios)
		return fileMap, nil
	}

	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Get relative path from root
		relPath, err := filepath.Rel(rootDir, path)
		if err != nil {
			return err
		}

		// Read file content
		content, err := os.ReadFile(path) // #nosec G304 - walking test directories
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", path, err)
		}

		fileMap[relPath] = string(content)
		return nil
	})

	return fileMap, err
}

// compareFileMaps compares two file maps and returns comparisons
func compareFileMaps(beforeFiles, afterFiles map[string]string) []FileComparison {
	var comparisons []FileComparison

	// Find added and modified files
	for path, afterContent := range afterFiles {
		beforeContent, existedBefore := beforeFiles[path]

		if !existedBefore {
			// New file
			comparisons = append(comparisons, FileComparison{
				Path:       path,
				Operation:  FileAdded,
				OldContent: "",
				NewContent: afterContent,
			})
		} else if beforeContent != afterContent {
			// Modified file
			comparisons = append(comparisons, FileComparison{
				Path:       path,
				Operation:  FileModified,
				OldContent: beforeContent,
				NewContent: afterContent,
			})
		}
		// If content is the same, skip (no change)
	}

	// Find deleted files
	for path, beforeContent := range beforeFiles {
		if _, existsAfter := afterFiles[path]; !existsAfter {
			comparisons = append(comparisons, FileComparison{
				Path:       path,
				Operation:  FileDeleted,
				OldContent: beforeContent,
				NewContent: "",
			})
		}
	}

	return comparisons
}

// createFileChange creates a gitlab.FileChange from a FileComparison
func createFileChange(comp FileComparison) (gitlab.FileChange, error) {
	change := gitlab.FileChange{}

	switch comp.Operation {
	case FileAdded:
		change.NewPath = comp.Path
		change.NewFile = true
		change.Diff = generateGitDiff("", comp.NewContent, comp.Path)

	case FileModified:
		change.NewPath = comp.Path
		change.OldPath = comp.Path
		change.Diff = generateGitDiff(comp.OldContent, comp.NewContent, comp.Path)

	case FileDeleted:
		change.OldPath = comp.Path
		change.DeletedFile = true
		change.Diff = generateGitDiff(comp.OldContent, "", comp.Path)
	}

	return change, nil
}

// generateGitDiff generates a unified diff in git format
func generateGitDiff(oldContent, newContent, filePath string) string {
	// Use difflib to generate unified diff
	diff := difflib.UnifiedDiff{
		A:        difflib.SplitLines(oldContent),
		B:        difflib.SplitLines(newContent),
		FromFile: "a/" + filePath,
		ToFile:   "b/" + filePath,
		Context:  3, // 3 lines of context like git
	}

	result, err := difflib.GetUnifiedDiffString(diff)
	if err != nil {
		// If diff generation fails, create a simple diff
		return createSimpleDiff(oldContent, newContent)
	}

	// Remove the file header lines (--- and +++) as GitLab provides these separately
	lines := strings.Split(result, "\n")
	var diffLines []string
	for i, line := range lines {
		// Skip the first two lines (file paths)
		if i >= 2 {
			diffLines = append(diffLines, line)
		}
	}

	return strings.Join(diffLines, "\n")
}

// createSimpleDiff creates a simple diff when difflib fails
func createSimpleDiff(oldContent, newContent string) string {
	oldLines := strings.Split(oldContent, "\n")
	newLines := strings.Split(newContent, "\n")

	var diff strings.Builder

	// Add hunk header
	diff.WriteString(fmt.Sprintf("@@ -1,%d +1,%d @@\n", len(oldLines), len(newLines)))

	// Remove old lines
	for _, line := range oldLines {
		diff.WriteString(fmt.Sprintf("-%s\n", line))
	}

	// Add new lines
	for _, line := range newLines {
		diff.WriteString(fmt.Sprintf("+%s\n", line))
	}

	return diff.String()
}
