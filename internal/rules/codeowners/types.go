package codeowners

// DevelopersYAML represents the structure of developers.yaml
type DevelopersYAML struct {
	Group struct {
		Owners []string `yaml:"owners"`
	} `yaml:"group"`
}

// GroupYAML represents the structure of groups/*.yaml files
type GroupYAML struct {
	GroupName string   `yaml:"group_name"`
	Approvers []string `yaml:"approvers"`
}

// DataProductInfo contains information about a data product extracted from file paths
type DataProductInfo struct {
	Type string // "aggregate", "source", or "platform"
	Name string // e.g., "bookingsmaster"
	Path string // Full path e.g., "dataproducts/aggregate/bookingsmaster"
}

// YAMLChangeInfo contains information about a changed YAML file
type YAMLChangeInfo struct {
	FilePath        string
	FileType        string // "developers" or "group"
	DataProduct     DataProductInfo
	OwnersApprovers []string
	IsNewFile       bool
	GroupName       string // Only for groups/*.yaml
}

// CODEOWNERSEntry represents a parsed CODEOWNERS entry
type CODEOWNERSEntry struct {
	Path   string
	Owners []string
}
