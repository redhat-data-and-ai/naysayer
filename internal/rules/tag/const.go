package tag

import "regexp"

// Kind constant
const TagKind = "Tag"

// Allowed values constraints
const (
	MinAllowedValues = 1
	MaxAllowedValues = 20
)

// Regex patterns for validation
var (
	// Tag name: 3-30 alphanumeric chars, underscore, then at least one alphanumeric
	// Examples: analytics_pii, hellosource_restricted, sfsupportsale003_pii
	TagNameRegex = regexp.MustCompile(`^[a-z0-9]{3,30}_[a-z0-9]+$`)

	// Data product name: 3-30 lowercase alphanumeric characters
	// Examples: analytics, hellosource, sfsupportsale003
	DataProductRegex = regexp.MustCompile(`^[a-z0-9]{3,30}$`)

	// Masking policy name: <dp>_<classification>_(string|float|number)_policy
	// Examples: analytics_pii_string_policy, hellosource_restricted_float_policy
	MaskingPolicyNameRegex = regexp.MustCompile(`^[a-z0-9]+(_[a-z0-9]+)*_(string|float|number)_policy$`)
)
