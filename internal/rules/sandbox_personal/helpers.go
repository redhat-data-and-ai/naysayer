package sandbox_personal

import (
	"fmt"
	"strings"

	"github.com/redhat-data-and-ai/naysayer/internal/gitlab"
	"github.com/redhat-data-and-ai/naysayer/internal/logging"
	"github.com/redhat-data-and-ai/naysayer/internal/rules/shared"
)

// GetSandboxProductYAMLPath derives sandbox/unstructured-data-product.yaml from any file within an unstructured data product.
// Example: dataproducts/unstructured/aif-test/developers.yaml -> dataproducts/unstructured/aif-test/sandbox/unstructured-data-product.yaml
func GetSandboxProductYAMLPath(filePath string) (string, bool) {
	parts := strings.Split(filePath, "/")
	for i, part := range parts {
		// Look for: dataproducts/unstructured/{productname}/...
		if part == "dataproducts" && i+2 < len(parts) && parts[i+1] == "unstructured" && i+3 < len(parts) {
			productRoot := strings.Join(parts[:i+3], "/")
			return productRoot + "/sandbox/unstructured-data-product.yaml", true
		}
	}
	return "", false
}

func isSandboxUnstructuredProductContent(content string) bool {
	hasUnstructuredKind := strings.Contains(content, "kind: UnstructuredDataProduct") ||
		strings.Contains(content, "kind:UnstructuredDataProduct")
	return hasUnstructuredKind
}

// ExtractProductNameFromYAML extracts the name field from unstructured-data-product.yaml content
func ExtractProductNameFromYAML(content string) string {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "name:") || strings.HasPrefix(trimmed, "name :") {
			parts := strings.SplitN(trimmed, ":", 2)
			if len(parts) == 2 {
				name := strings.TrimSpace(parts[1])
				// Remove comments
				if idx := strings.Index(name, "#"); idx != -1 {
					name = strings.TrimSpace(name[:idx])
				}
				// Remove quotes if present
				name = strings.Trim(name, "\"'")
				return name
			}
		}
	}
	return ""
}

// IsAIFProduct checks if the product name starts with "aif-"
func IsAIFProduct(content string) bool {
	productName := ExtractProductNameFromYAML(content)
	return strings.HasPrefix(productName, "aif-")
}

// IsSandboxPersonalProductForFile checks whether the data product associated with filePath
// has a sandbox/unstructured-data-product.yaml with kind=UnstructuredDataProduct and name starting with "aif-".
// Returns (isAIFProduct bool, err error). On error, callers should fail-closed (require manual review).
func IsSandboxPersonalProductForFile(mrContext *shared.MRContext, client gitlab.GitLabClient, filePath string) (bool, error) {
	if mrContext == nil || client == nil {
		return false, nil // Not applicable, not an error
	}

	sandboxProductPath, ok := GetSandboxProductYAMLPath(filePath)
	if !ok {
		return false, nil // Not an unstructured product path, not an error
	}

	sourceBranch := mrContext.MRInfo.SourceBranch
	if sourceBranch == "" {
		return false, nil // No source branch, not an error
	}

	fileContent, err := client.FetchFileContent(mrContext.ProjectID, sandboxProductPath, sourceBranch)
	if err != nil {
		// Network error, API error, or rate limit - fail-closed (return error)
		logging.Warn("Failed to fetch %s: %v - requiring manual review", sandboxProductPath, err)
		return false, err
	}

	if fileContent == nil {
		// File doesn't exist (404) - this is expected for non-aif products
		return false, nil
	}

	// Check if it's an UnstructuredDataProduct with name starting with "aif-"
	isAIFProduct := isSandboxUnstructuredProductContent(fileContent.Content) && IsAIFProduct(fileContent.Content)
	return isAIFProduct, nil
}

// IsSandboxPersonalMR checks if any file changed in this MR belongs to a sandbox Personal UnstructuredDataProduct.
func IsSandboxPersonalMR(mrContext *shared.MRContext, client gitlab.GitLabClient) bool {
	if mrContext == nil {
		return false
	}

	checked := make(map[string]bool)
	for _, change := range mrContext.Changes {
		filePath := change.NewPath
		if filePath == "" {
			filePath = change.OldPath
		}

		sandboxProductPath, ok := GetSandboxProductYAMLPath(filePath)
		if !ok || checked[sandboxProductPath] {
			continue
		}
		checked[sandboxProductPath] = true

		isAIFProduct, err := IsSandboxPersonalProductForFile(mrContext, client, filePath)
		if err != nil {
			// On error, log and continue checking other files
			logging.Warn("Error checking %s: %v", sandboxProductPath, err)
			continue
		}

		if isAIFProduct {
			logging.Info("MR affects sandbox Personal UnstructuredDataProduct at %s - sandbox rules will apply", sandboxProductPath)
			return true
		}
	}

	return false
}

// IsNewSandboxProductFile checks if sandbox/unstructured-data-product.yaml is newly added in this MR.
// Returns (isNew bool, err error). On error, callers should fail-closed (require manual review).
func IsNewSandboxProductFile(mrContext *shared.MRContext, client gitlab.GitLabClient, filePath string) (bool, error) {
	if !strings.HasSuffix(filePath, "/sandbox/unstructured-data-product.yaml") && !strings.HasSuffix(filePath, "/sandbox/unstructured-data-product.yml") {
		return false, nil
	}
	return isNewFileInMR(mrContext, client, filePath)
}

// isNewFileInMR checks if a file is newly added in the MR.
// Returns (isNew bool, err error). Distinguishes between "file not found" (404 = new file, no error)
// and other errors (network/API issues that should fail-closed).
func isNewFileInMR(mrContext *shared.MRContext, client gitlab.GitLabClient, filePath string) (bool, error) {
	if mrContext == nil {
		return false, nil
	}

	for _, change := range mrContext.Changes {
		if change.NewPath != filePath {
			continue
		}

		// If GitLab API explicitly marks it as new, trust that
		if change.NewFile || change.OldPath == "" {
			return true, nil
		}

		// Double-check by fetching from target branch
		if client != nil {
			targetBranch := mrContext.MRInfo.TargetBranch
			if targetBranch != "" {
				beforeContent, err := client.FetchFileContent(mrContext.ProjectID, filePath, targetBranch)
				if err != nil {
					// Distinguish between "file not found" (404 = new file) and other errors (fail-closed)
					if strings.Contains(err.Error(), "file not found") {
						return true, nil
					}
					// Network error, API error, rate limit, etc. - return error to fail-closed
					return false, fmt.Errorf("failed to check if file exists in target branch: %w", err)
				}
				if beforeContent == nil {
					// Shouldn't happen if no error, but treat as new file
					return true, nil
				}
			}
		}

		return false, nil
	}

	return false, nil
}
