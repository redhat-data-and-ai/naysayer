package sandbox_personal

import (
	"strings"

	"github.com/redhat-data-and-ai/naysayer/internal/gitlab"
	"github.com/redhat-data-and-ai/naysayer/internal/logging"
	"github.com/redhat-data-and-ai/naysayer/internal/rules/shared"
)

// GetSandboxProductYAMLPath derives sandbox/product.yaml from any file within a data product.
// Example: dataproducts/source/myproduct/developers.yaml -> dataproducts/source/myproduct/sandbox/product.yaml
func GetSandboxProductYAMLPath(filePath string) (string, bool) {
	parts := strings.Split(filePath, "/")
	for i, part := range parts {
		if part == "dataproducts" && i+2 < len(parts) {
			productRoot := strings.Join(parts[:i+3], "/")
			return productRoot + "/sandbox/product.yaml", true
		}
	}
	return "", false
}

func isSandboxPersonalProductContent(content string) bool {
	hasUnstructuredKind := strings.Contains(content, "kind: UnstructuredDataProduct") ||
		strings.Contains(content, "kind:UnstructuredDataProduct")
	hasPersonalType := strings.Contains(content, "type: Personal") ||
		strings.Contains(content, "type:Personal")
	return hasUnstructuredKind && hasPersonalType
}

// IsSandboxPersonalProductForFile checks whether the data product associated with filePath
// has a sandbox/product.yaml with kind=UnstructuredDataProduct and type=Personal.
func IsSandboxPersonalProductForFile(mrContext *shared.MRContext, client gitlab.GitLabClient, filePath string) bool {
	if mrContext == nil || client == nil {
		return false
	}

	sandboxProductPath, ok := GetSandboxProductYAMLPath(filePath)
	if !ok {
		return false
	}

	sourceBranch := mrContext.MRInfo.SourceBranch
	if sourceBranch == "" {
		return false
	}

	fileContent, err := client.FetchFileContent(mrContext.ProjectID, sandboxProductPath, sourceBranch)
	if err != nil || fileContent == nil {
		return false
	}

	return isSandboxPersonalProductContent(fileContent.Content)
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

		if IsSandboxPersonalProductForFile(mrContext, client, filePath) {
			logging.Info("MR affects sandbox Personal UnstructuredDataProduct at %s - sandbox rules will apply", sandboxProductPath)
			return true
		}
	}

	return false
}

// IsNewSandboxProductFile checks if sandbox/product.yaml is newly added in this MR.
func IsNewSandboxProductFile(mrContext *shared.MRContext, client gitlab.GitLabClient, filePath string) bool {
	if !strings.Contains(filePath, "/sandbox/product.yaml") && !strings.Contains(filePath, "/sandbox/product.yml") {
		return false
	}
	return isNewFileInMR(mrContext, client, filePath)
}

func isNewFileInMR(mrContext *shared.MRContext, client gitlab.GitLabClient, filePath string) bool {
	if mrContext == nil {
		return false
	}

	for _, change := range mrContext.Changes {
		if change.NewPath != filePath {
			continue
		}

		if change.NewFile || change.OldPath == "" {
			return true
		}

		if client != nil {
			targetBranch := mrContext.MRInfo.TargetBranch
			if targetBranch != "" {
				beforeContent, err := client.FetchFileContent(mrContext.ProjectID, filePath, targetBranch)
				if err != nil || beforeContent == nil {
					return true
				}
			}
		}

		return false
	}

	return false
}
