package serviceaccount

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestScopingValidator_ValidateScoping(t *testing.T) {
	validator := NewScopingValidator()

	tests := []struct {
		name      string
		sa        ServiceAccount
		saFile    ServiceAccountFile
		expectIssues bool
	}{
		{
			name: "perfect scoping - name and comment include data product",
			sa: ServiceAccount{
				Name: "marketo_tableau_dev_appuser",
				Comment: "service account for marketo data",
			},
			saFile: ServiceAccountFile{DataProduct: "marketo"},
			expectIssues: false,
		},
		{
			name: "missing data product in name",
			sa: ServiceAccount{
				Name: "generic_tableau_dev_appuser",
				Comment: "service account for marketo data",
			},
			saFile: ServiceAccountFile{DataProduct: "marketo"},
			expectIssues: true,
		},
		{
			name: "missing data product in comment",
			sa: ServiceAccount{
				Name: "marketo_tableau_dev_appuser",
				Comment: "generic service account",
			},
			saFile: ServiceAccountFile{DataProduct: "marketo"},
			expectIssues: true,
		},
		{
			name: "unknown data product - no validation",
			sa: ServiceAccount{
				Name: "generic_service_dev_appuser",
				Comment: "generic service account",
			},
			saFile: ServiceAccountFile{DataProduct: "unknown"},
			expectIssues: false,
		},
		{
			name: "case insensitive matching",
			sa: ServiceAccount{
				Name: "MARKETO_tableau_dev_appuser",
				Comment: "service account for MARKETO data",
			},
			saFile: ServiceAccountFile{DataProduct: "marketo"},
			expectIssues: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issues := validator.ValidateScoping(tt.sa, tt.saFile)
			if tt.expectIssues {
				assert.NotEmpty(t, issues, "Expected scoping issues")
				assert.Equal(t, "scoping", issues[0].Type)
				assert.Equal(t, "warning", issues[0].Severity)
			} else {
				assert.Empty(t, issues, "Expected no scoping issues")
			}
		})
	}
}