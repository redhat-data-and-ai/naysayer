package dataproduct_consumer

// DataProductConsumerConfig holds configuration for consumer access validation
type DataProductConsumerConfig struct {
	// Environments where consumer access is allowed (preprod, prod)
	AllowedEnvironments []string
	// Whether environment matching is case-sensitive
	CaseSensitive bool
}

// DefaultDataProductConsumerConfig returns default configuration
func DefaultDataProductConsumerConfig() *DataProductConsumerConfig {
	return &DataProductConsumerConfig{
		AllowedEnvironments: []string{"preprod", "prod"},
		CaseSensitive:       false,
	}
}

// ConsumerContext holds analysis context for consumer changes
type ConsumerContext struct {
	FilePath        string
	Environment     string
	HasConsumers    bool
	IsConsumerOnly  bool // Only consumer fields are being modified
	RequiresReview  bool
	ApprovalReason  string
}
