package gitlab

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMRChanges_JSONMarshaling(t *testing.T) {
	// Test MRChanges JSON marshaling and unmarshaling
	original := MRChanges{
		Changes: []struct {
			OldPath     string `json:"old_path"`
			NewPath     string `json:"new_path"`
			AMode       string `json:"a_mode"`
			BMode       string `json:"b_mode"`
			NewFile     bool   `json:"new_file"`
			RenamedFile bool   `json:"renamed_file"`
			DeletedFile bool   `json:"deleted_file"`
			Diff        string `json:"diff"`
		}{
			{
				OldPath:     "old/file.yaml",
				NewPath:     "new/file.yaml",
				AMode:       "100644",
				BMode:       "100644",
				NewFile:     false,
				RenamedFile: true,
				DeletedFile: false,
				Diff:        "@@ -1,3 +1,3 @@\n-old content\n+new content",
			},
			{
				OldPath:     "",
				NewPath:     "brand/new/file.yaml",
				AMode:       "000000",
				BMode:       "100644",
				NewFile:     true,
				RenamedFile: false,
				DeletedFile: false,
				Diff:        "@@ -0,0 +1,5 @@\n+new file content",
			},
		},
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(original)
	assert.NoError(t, err)
	assert.Contains(t, string(jsonData), `"old_path":"old/file.yaml"`)
	assert.Contains(t, string(jsonData), `"new_file":true`)

	// Unmarshal back to struct
	var unmarshaled MRChanges
	err = json.Unmarshal(jsonData, &unmarshaled)
	assert.NoError(t, err)
	assert.Equal(t, original, unmarshaled)
}

func TestMRChanges_EmptyChanges(t *testing.T) {
	// Test empty changes array
	mrChanges := MRChanges{Changes: []struct {
		OldPath     string `json:"old_path"`
		NewPath     string `json:"new_path"`
		AMode       string `json:"a_mode"`
		BMode       string `json:"b_mode"`
		NewFile     bool   `json:"new_file"`
		RenamedFile bool   `json:"renamed_file"`
		DeletedFile bool   `json:"deleted_file"`
		Diff        string `json:"diff"`
	}{}}

	jsonData, err := json.Marshal(mrChanges)
	assert.NoError(t, err)
	assert.Contains(t, string(jsonData), `"changes":[]`)

	var unmarshaled MRChanges
	err = json.Unmarshal(jsonData, &unmarshaled)
	assert.NoError(t, err)
	assert.Empty(t, unmarshaled.Changes)
}

func TestFileChange_JSONMarshaling(t *testing.T) {
	// Test FileChange JSON marshaling and unmarshaling
	original := FileChange{
		OldPath:     "src/main.go",
		NewPath:     "src/main.go",
		AMode:       "100644",
		BMode:       "100644",
		NewFile:     false,
		RenamedFile: false,
		DeletedFile: false,
		Diff:        "@@ -10,7 +10,7 @@ func main() {\n-\toldCode()\n+\tnewCode()",
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(original)
	assert.NoError(t, err)
	assert.Contains(t, string(jsonData), `"old_path":"src/main.go"`)
	assert.Contains(t, string(jsonData), `"new_file":false`)

	// Unmarshal back to struct
	var unmarshaled FileChange
	err = json.Unmarshal(jsonData, &unmarshaled)
	assert.NoError(t, err)
	assert.Equal(t, original, unmarshaled)
}

func TestFileChange_NewFile(t *testing.T) {
	// Test new file scenario
	fileChange := FileChange{
		OldPath:     "",
		NewPath:     "docs/README.md",
		AMode:       "000000",
		BMode:       "100644",
		NewFile:     true,
		RenamedFile: false,
		DeletedFile: false,
		Diff:        "@@ -0,0 +1,10 @@\n+# README\n+Documentation",
	}

	jsonData, err := json.Marshal(fileChange)
	assert.NoError(t, err)
	assert.Contains(t, string(jsonData), `"old_path":""`)
	assert.Contains(t, string(jsonData), `"new_file":true`)

	var unmarshaled FileChange
	err = json.Unmarshal(jsonData, &unmarshaled)
	assert.NoError(t, err)
	assert.Equal(t, fileChange, unmarshaled)
}

func TestFileChange_DeletedFile(t *testing.T) {
	// Test deleted file scenario
	fileChange := FileChange{
		OldPath:     "deprecated/old.yaml",
		NewPath:     "",
		AMode:       "100644",
		BMode:       "000000",
		NewFile:     false,
		RenamedFile: false,
		DeletedFile: true,
		Diff:        "@@ -1,5 +0,0 @@\n-old content\n-to be deleted",
	}

	jsonData, err := json.Marshal(fileChange)
	assert.NoError(t, err)
	assert.Contains(t, string(jsonData), `"new_path":""`)
	assert.Contains(t, string(jsonData), `"deleted_file":true`)

	var unmarshaled FileChange
	err = json.Unmarshal(jsonData, &unmarshaled)
	assert.NoError(t, err)
	assert.Equal(t, fileChange, unmarshaled)
}

func TestFileChange_RenamedFile(t *testing.T) {
	// Test renamed file scenario
	fileChange := FileChange{
		OldPath:     "config/old-name.yaml",
		NewPath:     "config/new-name.yaml",
		AMode:       "100644",
		BMode:       "100644",
		NewFile:     false,
		RenamedFile: true,
		DeletedFile: false,
		Diff:        "",
	}

	jsonData, err := json.Marshal(fileChange)
	assert.NoError(t, err)
	assert.Contains(t, string(jsonData), `"renamed_file":true`)

	var unmarshaled FileChange
	err = json.Unmarshal(jsonData, &unmarshaled)
	assert.NoError(t, err)
	assert.Equal(t, fileChange, unmarshaled)
}

func TestMRInfo_StructFields(t *testing.T) {
	// Test MRInfo struct field assignment and access
	mrInfo := MRInfo{
		ProjectID:    12345,
		MRIID:        678,
		Title:        "Add new feature for data processing",
		Author:       "developer@company.com",
		SourceBranch: "feature/data-processing",
		TargetBranch: "main",
	}

	assert.Equal(t, 12345, mrInfo.ProjectID)
	assert.Equal(t, 678, mrInfo.MRIID)
	assert.Equal(t, "Add new feature for data processing", mrInfo.Title)
	assert.Equal(t, "developer@company.com", mrInfo.Author)
	assert.Equal(t, "feature/data-processing", mrInfo.SourceBranch)
	assert.Equal(t, "main", mrInfo.TargetBranch)
}

func TestMRInfo_ZeroValues(t *testing.T) {
	// Test MRInfo with zero values
	var mrInfo MRInfo

	assert.Equal(t, 0, mrInfo.ProjectID)
	assert.Equal(t, 0, mrInfo.MRIID)
	assert.Equal(t, "", mrInfo.Title)
	assert.Equal(t, "", mrInfo.Author)
	assert.Equal(t, "", mrInfo.SourceBranch)
	assert.Equal(t, "", mrInfo.TargetBranch)
}

func TestMRInfo_PartialData(t *testing.T) {
	// Test MRInfo with only some fields populated
	mrInfo := MRInfo{
		ProjectID: 999,
		MRIID:     111,
		Title:     "Partial MR",
		// Author, SourceBranch, TargetBranch left empty
	}

	assert.Equal(t, 999, mrInfo.ProjectID)
	assert.Equal(t, 111, mrInfo.MRIID)
	assert.Equal(t, "Partial MR", mrInfo.Title)
	assert.Empty(t, mrInfo.Author)
	assert.Empty(t, mrInfo.SourceBranch)
	assert.Empty(t, mrInfo.TargetBranch)
}

func TestMRChanges_FromGitLabAPI(t *testing.T) {
	// Test unmarshaling from GitLab API response format
	gitlabJSON := `{
		"changes": [
			{
				"old_path": "dataproducts/test/product.yaml",
				"new_path": "dataproducts/test/product.yaml",
				"a_mode": "100644",
				"b_mode": "100644",
				"new_file": false,
				"renamed_file": false,
				"deleted_file": false,
				"diff": "@@ -5,7 +5,7 @@ warehouses:\n   - type: snowflake\n-    size: MEDIUM\n+    size: LARGE"
			},
			{
				"old_path": "",
				"new_path": "docs/new-feature.md",
				"a_mode": "000000",
				"b_mode": "100644",
				"new_file": true,
				"renamed_file": false,
				"deleted_file": false,
				"diff": "@@ -0,0 +1,3 @@\n+# New Feature\n+\n+Documentation for new feature."
			}
		]
	}`

	var mrChanges MRChanges
	err := json.Unmarshal([]byte(gitlabJSON), &mrChanges)
	assert.NoError(t, err)
	assert.Len(t, mrChanges.Changes, 2)

	// Verify first change
	firstChange := mrChanges.Changes[0]
	assert.Equal(t, "dataproducts/test/product.yaml", firstChange.OldPath)
	assert.Equal(t, "dataproducts/test/product.yaml", firstChange.NewPath)
	assert.Equal(t, "100644", firstChange.AMode)
	assert.Equal(t, "100644", firstChange.BMode)
	assert.False(t, firstChange.NewFile)
	assert.False(t, firstChange.RenamedFile)
	assert.False(t, firstChange.DeletedFile)
	assert.Contains(t, firstChange.Diff, "size: LARGE")

	// Verify second change (new file)
	secondChange := mrChanges.Changes[1]
	assert.Equal(t, "", secondChange.OldPath)
	assert.Equal(t, "docs/new-feature.md", secondChange.NewPath)
	assert.Equal(t, "000000", secondChange.AMode)
	assert.Equal(t, "100644", secondChange.BMode)
	assert.True(t, secondChange.NewFile)
	assert.False(t, secondChange.RenamedFile)
	assert.False(t, secondChange.DeletedFile)
	assert.Contains(t, secondChange.Diff, "# New Feature")
}

func TestFileChange_SpecialCharacterPaths(t *testing.T) {
	// Test file paths with special characters
	fileChange := FileChange{
		OldPath: "data products/test file@domain.yaml",
		NewPath: "data products/test file@domain.yaml",
		AMode:   "100644",
		BMode:   "100644",
		Diff:    "@@ -1,1 +1,1 @@\n-old: value\n+new: value",
	}

	jsonData, err := json.Marshal(fileChange)
	assert.NoError(t, err)
	assert.Contains(t, string(jsonData), `"old_path":"data products/test file@domain.yaml"`)

	var unmarshaled FileChange
	err = json.Unmarshal(jsonData, &unmarshaled)
	assert.NoError(t, err)
	assert.Equal(t, fileChange, unmarshaled)
}

func TestMRChanges_InvalidJSON(t *testing.T) {
	// Test handling of invalid JSON
	invalidJSON := `{"changes": [{"old_path": "test", "invalid": json}]}`

	var mrChanges MRChanges
	err := json.Unmarshal([]byte(invalidJSON), &mrChanges)
	assert.Error(t, err)
}

func TestFileChange_MissingFields(t *testing.T) {
	// Test JSON with missing fields (should use zero values)
	partialJSON := `{
		"old_path": "test.yaml",
		"new_path": "test.yaml"
	}`

	var fileChange FileChange
	err := json.Unmarshal([]byte(partialJSON), &fileChange)
	assert.NoError(t, err)
	assert.Equal(t, "test.yaml", fileChange.OldPath)
	assert.Equal(t, "test.yaml", fileChange.NewPath)
	assert.Equal(t, "", fileChange.AMode) // Missing field defaults to zero value
	assert.Equal(t, "", fileChange.BMode)
	assert.False(t, fileChange.NewFile)
	assert.False(t, fileChange.RenamedFile)
	assert.False(t, fileChange.DeletedFile)
	assert.Equal(t, "", fileChange.Diff)
}

func TestMRChanges_LargeChangeset(t *testing.T) {
	// Test handling of large number of changes
	var changes []struct {
		OldPath     string `json:"old_path"`
		NewPath     string `json:"new_path"`
		AMode       string `json:"a_mode"`
		BMode       string `json:"b_mode"`
		NewFile     bool   `json:"new_file"`
		RenamedFile bool   `json:"renamed_file"`
		DeletedFile bool   `json:"deleted_file"`
		Diff        string `json:"diff"`
	}

	// Create 100 file changes
	for i := 0; i < 100; i++ {
		changes = append(changes, struct {
			OldPath     string `json:"old_path"`
			NewPath     string `json:"new_path"`
			AMode       string `json:"a_mode"`
			BMode       string `json:"b_mode"`
			NewFile     bool   `json:"new_file"`
			RenamedFile bool   `json:"renamed_file"`
			DeletedFile bool   `json:"deleted_file"`
			Diff        string `json:"diff"`
		}{
			OldPath: "file" + string(rune(i)) + ".yaml",
			NewPath: "file" + string(rune(i)) + ".yaml",
			AMode:   "100644",
			BMode:   "100644",
			Diff:    "@@ -1,1 +1,1 @@\n-old\n+new",
		})
	}

	mrChanges := MRChanges{Changes: changes}

	jsonData, err := json.Marshal(mrChanges)
	assert.NoError(t, err)
	assert.NotEmpty(t, jsonData)

	var unmarshaled MRChanges
	err = json.Unmarshal(jsonData, &unmarshaled)
	assert.NoError(t, err)
	assert.Len(t, unmarshaled.Changes, 100)
}
