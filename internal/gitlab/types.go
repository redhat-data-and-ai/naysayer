package gitlab

// MRChanges represents the structure of GitLab MR changes API response
type MRChanges struct {
	Changes []struct {
		OldPath     string `json:"old_path"`
		NewPath     string `json:"new_path"`
		AMode       string `json:"a_mode"`
		BMode       string `json:"b_mode"`
		NewFile     bool   `json:"new_file"`
		RenamedFile bool   `json:"renamed_file"`
		DeletedFile bool   `json:"deleted_file"`
		Diff        string `json:"diff"`
	} `json:"changes"`
}

// FileChange represents a single file change in an MR
type FileChange struct {
	OldPath     string `json:"old_path"`
	NewPath     string `json:"new_path"`
	AMode       string `json:"a_mode"`
	BMode       string `json:"b_mode"`
	NewFile     bool   `json:"new_file"`
	RenamedFile bool   `json:"renamed_file"`
	DeletedFile bool   `json:"deleted_file"`
	Diff        string `json:"diff"`
}

// MRInfo represents merge request information extracted from webhook payload
type MRInfo struct {
	ProjectID    int
	MRIID        int
	Title        string
	Author       string
	SourceBranch string
	TargetBranch string
	State        string
}

// PipelineJob represents a GitLab CI job
type PipelineJob struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	Status        string `json:"status"`
	Stage         string `json:"stage"`
	FailureReason string `json:"failure_reason"`
	AllowFailure  bool   `json:"allow_failure"`
}
