package masking

import "regexp"

// Kind constant
const (
	MaskingPolicyKind = "MaskingPolicy"
)

// Valid datatype values
const (
	DataTypeString = "string"
	DataTypeFloat  = "float"
	DataTypeNumber = "number"
)

// Valid strategy values
const (
	StrategyUnmasked = "UNMASKED"
	StrategyHashSha1 = "HASH_SHA1"
)

// Valid consumer kinds
const (
	ConsumerKindGroup          = "consumer_group"
	ConsumerKindServiceAccount = "service_account"
)

// Valid classification values
const (
	ClassificationPii           = "pii"
	ClassificationRestricted    = "restricted"
	ClassificationRestrictedPii = "restrictedpii"
)

// Valid values slices for validation
var (
	ValidDataTypes       = []string{DataTypeString, DataTypeFloat, DataTypeNumber}
	ValidStrategies      = []string{StrategyUnmasked, StrategyHashSha1}
	ValidConsumerKinds   = []string{ConsumerKindGroup, ConsumerKindServiceAccount}
	ValidClassifications = []string{ClassificationPii, ClassificationRestricted, ClassificationRestrictedPii}
)

// Regex patterns for validation
var (
	// Data product name: 3-30 lowercase alphanumeric characters
	DataProductRegex = regexp.MustCompile(`^[a-z0-9]{3,30}$`)

	// Masking policy name format: <dataproduct>_<classification>_<datatype>_policy
	// Example: hellosource_pii_string_policy, ciam_restricted_float_policy
	MaskingPolicyNameRegex = regexp.MustCompile(`^[a-z0-9]{3,30}_(pii|restricted|restrictedpii)_(string|float|number)_policy$`)

	// Float mask format: valid decimal number (e.g., "-9.0", "0.0", "123.45")
	FloatMaskRegex = regexp.MustCompile(`^-?\d+(\.\d+)?$`)

	// Number/Integer mask format: valid integer (e.g., "8888", "-9", "0")
	NumberMaskRegex = regexp.MustCompile(`^-?\d+$`)

	// Consumer group naming: dataverse-(source|aggregate|consumer)-<dataproduct>(-<suffix>)?
	ConsumerGroupRegex = regexp.MustCompile(`^dataverse-(source|aggregate|consumer)-[a-z0-9]{3,30}(-[a-z0-9]{3,50})?$`)

	// Service account naming: <dataproduct>_<tool>_<env>_appuser
	ServiceAccountRegex = regexp.MustCompile(`^[a-z0-9]{3,30}_[a-z0-9]+_(sandbox|dev|preprod|prod|platformtest)_appuser$`)
)

// contains checks if a string is in a slice
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
