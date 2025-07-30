package decision

import "fmt"

// Maker handles decision making logic for warehouse changes
type Maker struct{}

// NewMaker creates a new decision maker
func NewMaker() *Maker {
	return &Maker{}
}

// Decide makes an approval decision based on warehouse changes
func (m *Maker) Decide(changes []WarehouseChange) Decision {
	if len(changes) == 0 {
		return Decision{
			AutoApprove: false,
			Reason:      "no warehouse changes detected in YAML files",
			Summary:     "🚫 No warehouse changes in YAML - requires approval",
		}
	}

	// Check if all changes are decreases
	for _, change := range changes {
		if !change.IsDecrease {
			return Decision{
				AutoApprove: false,
				Reason:      fmt.Sprintf("warehouse increase detected: %s → %s", change.FromSize, change.ToSize),
				Summary:     "🚫 Warehouse increase - platform approval required",
				Details:     fmt.Sprintf("File: %s", change.FilePath),
			}
		}
	}

	// All changes are decreases - auto-approve
	details := fmt.Sprintf("Found %d warehouse decrease(s)", len(changes))
	return Decision{
		AutoApprove: true,
		Reason:      "all warehouse changes are decreases",
		Summary:     "✅ Warehouse decrease(s) - auto-approved",
		Details:     details,
	}
}

// NoTokenDecision returns a decision for when GitLab token is not configured
func (m *Maker) NoTokenDecision() Decision {
	return Decision{
		AutoApprove: false,
		Reason:      "GitLab token not configured",
		Summary:     "🚫 Cannot analyze YAML files - missing GitLab token",
		Details:     "Set GITLAB_TOKEN environment variable to enable YAML analysis",
	}
}

// APIErrorDecision returns a decision for when GitLab API fails
func (m *Maker) APIErrorDecision(err error) Decision {
	return Decision{
		AutoApprove: false,
		Reason:      "Failed to fetch file changes",
		Summary:     "🚫 API error - requires manual approval",
		Details:     fmt.Sprintf("Error: %v", err),
	}
}

// AnalysisErrorDecision returns a decision for when YAML analysis fails
func (m *Maker) AnalysisErrorDecision(err error) Decision {
	return Decision{
		AutoApprove: false,
		Reason:      "YAML analysis failed",
		Summary:     "🚫 Analysis error - requires manual approval",
		Details:     fmt.Sprintf("Could not analyze warehouse changes: %v", err),
	}
}