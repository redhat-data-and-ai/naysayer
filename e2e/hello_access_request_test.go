package e2e

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// helloAccessRequestScenarios are E2E cases for the hello_access_request rule.
var helloAccessRequestScenarios = []string{
	"42_access_request_hellosource",
	"43_access_request_helloaggregate",
	"44_access_request_multi_hellosource",
	"45_access_request_multi_cross_dp",
	"46_access_request_name_mismatch",
	"47_access_request_data_product_mismatch",
	"48_access_request_with_uncovered_file",
	"49_access_request_deletion",
	"50_access_request_wrong_path",
}

// TestE2E_HelloAccessRequest runs all hello_access_request E2E scenarios in isolation.
func TestE2E_HelloAccessRequest(t *testing.T) {
	for _, dir := range helloAccessRequestScenarios {
		t.Run(dir, func(t *testing.T) {
			scenarioDir := filepath.Join("testdata", "scenarios", dir)
			scenario, err := LoadScenario(scenarioDir)
			require.NoError(t, err, "load scenario %s", dir)
			runScenario(t, *scenario)
		})
	}
}
