package shared

import (
	"testing"

	"github.com/redhat-data-and-ai/naysayer/internal/gitlab"
	"github.com/stretchr/testify/assert"
)

func TestIsDraftMR(t *testing.T) {
	tests := []struct {
		name     string
		mrCtx    *MRContext
		expected bool
	}{
		{
			name:     "nil MR info",
			mrCtx:    &MRContext{MRInfo: nil},
			expected: false,
		},
		{
			name: "draft in title",
			mrCtx: &MRContext{
				MRInfo: &gitlab.MRInfo{Title: "Draft: Add new feature"},
			},
			expected: true,
		},
		{
			name: "WIP in title",
			mrCtx: &MRContext{
				MRInfo: &gitlab.MRInfo{Title: "WIP: Work in progress"},
			},
			expected: true,
		},
		{
			name: "draft prefix",
			mrCtx: &MRContext{
				MRInfo: &gitlab.MRInfo{Title: "draft: lowercase prefix"},
			},
			expected: true,
		},
		{
			name: "wip prefix",
			mrCtx: &MRContext{
				MRInfo: &gitlab.MRInfo{Title: "wip: lowercase prefix"},
			},
			expected: true,
		},
		{
			name: "draft anywhere in title",
			mrCtx: &MRContext{
				MRInfo: &gitlab.MRInfo{Title: "This is a draft implementation"},
			},
			expected: true,
		},
		{
			name: "wip anywhere in title",
			mrCtx: &MRContext{
				MRInfo: &gitlab.MRInfo{Title: "This is wip code"},
			},
			expected: true,
		},
		{
			name: "normal title",
			mrCtx: &MRContext{
				MRInfo: &gitlab.MRInfo{Title: "Add new feature"},
			},
			expected: false,
		},
		{
			name: "case insensitive - DRAFT",
			mrCtx: &MRContext{
				MRInfo: &gitlab.MRInfo{Title: "DRAFT: New feature"},
			},
			expected: true,
		},
		{
			name: "case insensitive - WIP",
			mrCtx: &MRContext{
				MRInfo: &gitlab.MRInfo{Title: "WIP Implementation"},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsDraftMR(tt.mrCtx)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsAutomatedUser(t *testing.T) {
	tests := []struct {
		name     string
		mrCtx    *MRContext
		expected bool
	}{
		{
			name:     "nil MR info",
			mrCtx:    &MRContext{MRInfo: nil},
			expected: false,
		},
		{
			name: "dependabot user",
			mrCtx: &MRContext{
				MRInfo: &gitlab.MRInfo{Author: "dependabot[bot]"},
			},
			expected: true,
		},
		{
			name: "renovate user",
			mrCtx: &MRContext{
				MRInfo: &gitlab.MRInfo{Author: "renovate-bot"},
			},
			expected: true,
		},
		{
			name: "greenkeeper user",
			mrCtx: &MRContext{
				MRInfo: &gitlab.MRInfo{Author: "greenkeeper"},
			},
			expected: true,
		},
		{
			name: "snyk-bot user",
			mrCtx: &MRContext{
				MRInfo: &gitlab.MRInfo{Author: "snyk-bot"},
			},
			expected: true,
		},
		{
			name: "human user",
			mrCtx: &MRContext{
				MRInfo: &gitlab.MRInfo{Author: "john.doe"},
			},
			expected: false,
		},
		{
			name: "case insensitive - DEPENDABOT",
			mrCtx: &MRContext{
				MRInfo: &gitlab.MRInfo{Author: "DEPENDABOT"},
			},
			expected: true,
		},
		{
			name: "case insensitive - RENOVATE",
			mrCtx: &MRContext{
				MRInfo: &gitlab.MRInfo{Author: "RENOVATE-BOT"},
			},
			expected: true,
		},
		{
			name: "partial match - user with bot in name",
			mrCtx: &MRContext{
				MRInfo: &gitlab.MRInfo{Author: "user-dependabot-service"},
			},
			expected: true,
		},
		{
			name: "partial match - renovate in username",
			mrCtx: &MRContext{
				MRInfo: &gitlab.MRInfo{Author: "my-renovate-account"},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsAutomatedUser(tt.mrCtx)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestContainsLine(t *testing.T) {
	lineRanges := []LineRange{
		{StartLine: 1, EndLine: 10, FilePath: "test.yaml"},
		{StartLine: 20, EndLine: 30, FilePath: "test.yaml"},
		{StartLine: 50, EndLine: 50, FilePath: "test.yaml"}, // Single line
	}

	tests := []struct {
		name       string
		lineNumber int
		expected   bool
	}{
		{"line in first range - start", 1, true},
		{"line in first range - middle", 5, true},
		{"line in first range - end", 10, true},
		{"line in second range", 25, true},
		{"line in single line range", 50, true},
		{"line before first range", 0, false},
		{"line between ranges", 15, false},
		{"line after last range", 60, false},
		{"negative line number", -1, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ContainsLine(lineRanges, tt.lineNumber)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestContainsLine_EmptyRanges(t *testing.T) {
	result := ContainsLine([]LineRange{}, 5)
	assert.False(t, result, "Empty ranges should not contain any line")
}

func TestMergeLineRanges(t *testing.T) {
	tests := []struct {
		name     string
		ranges   []LineRange
		expected []LineRange
	}{
		{
			name:     "empty ranges",
			ranges:   []LineRange{},
			expected: []LineRange{},
		},
		{
			name: "single range",
			ranges: []LineRange{
				{StartLine: 1, EndLine: 10, FilePath: "test.yaml"},
			},
			expected: []LineRange{
				{StartLine: 1, EndLine: 10, FilePath: "test.yaml"},
			},
		},
		{
			name: "overlapping ranges",
			ranges: []LineRange{
				{StartLine: 1, EndLine: 10, FilePath: "test.yaml"},
				{StartLine: 5, EndLine: 15, FilePath: "test.yaml"},
			},
			expected: []LineRange{
				{StartLine: 1, EndLine: 15, FilePath: "test.yaml"},
			},
		},
		{
			name: "adjacent ranges",
			ranges: []LineRange{
				{StartLine: 1, EndLine: 10, FilePath: "test.yaml"},
				{StartLine: 11, EndLine: 20, FilePath: "test.yaml"},
			},
			expected: []LineRange{
				{StartLine: 1, EndLine: 20, FilePath: "test.yaml"},
			},
		},
		{
			name: "non-overlapping ranges",
			ranges: []LineRange{
				{StartLine: 1, EndLine: 10, FilePath: "test.yaml"},
				{StartLine: 20, EndLine: 30, FilePath: "test.yaml"},
			},
			expected: []LineRange{
				{StartLine: 1, EndLine: 10, FilePath: "test.yaml"},
				{StartLine: 20, EndLine: 30, FilePath: "test.yaml"},
			},
		},
		{
			name: "complex merge scenario",
			ranges: []LineRange{
				{StartLine: 1, EndLine: 5, FilePath: "test.yaml"},
				{StartLine: 3, EndLine: 8, FilePath: "test.yaml"},
				{StartLine: 10, EndLine: 15, FilePath: "test.yaml"},
				{StartLine: 16, EndLine: 20, FilePath: "test.yaml"},
			},
			expected: []LineRange{
				{StartLine: 1, EndLine: 8, FilePath: "test.yaml"},
				{StartLine: 10, EndLine: 20, FilePath: "test.yaml"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MergeLineRanges(tt.ranges)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetUncoveredLines(t *testing.T) {
	tests := []struct {
		name          string
		totalLines    int
		coveredRanges []LineRange
		expected      []LineRange
	}{
		{
			name:          "no lines",
			totalLines:    0,
			coveredRanges: []LineRange{},
			expected:      nil,
		},
		{
			name:       "fully covered",
			totalLines: 10,
			coveredRanges: []LineRange{
				{StartLine: 1, EndLine: 10},
			},
			expected: nil, // Function returns nil for fully covered
		},
		{
			name:       "gap at beginning",
			totalLines: 10,
			coveredRanges: []LineRange{
				{StartLine: 5, EndLine: 10},
			},
			expected: []LineRange{
				{StartLine: 1, EndLine: 4},
			},
		},
		{
			name:       "gap at end",
			totalLines: 10,
			coveredRanges: []LineRange{
				{StartLine: 1, EndLine: 5},
			},
			expected: []LineRange{
				{StartLine: 6, EndLine: 10},
			},
		},
		{
			name:       "gap in middle",
			totalLines: 20,
			coveredRanges: []LineRange{
				{StartLine: 1, EndLine: 5},
				{StartLine: 15, EndLine: 20},
			},
			expected: []LineRange{
				{StartLine: 6, EndLine: 14},
			},
		},
		{
			name:       "multiple gaps",
			totalLines: 20,
			coveredRanges: []LineRange{
				{StartLine: 3, EndLine: 5},
				{StartLine: 10, EndLine: 12},
			},
			expected: []LineRange{
				{StartLine: 1, EndLine: 2},
				{StartLine: 6, EndLine: 9},
				{StartLine: 13, EndLine: 20},
			},
		},
		{
			name:          "no coverage",
			totalLines:    10,
			coveredRanges: []LineRange{},
			expected:      []LineRange{{StartLine: 1, EndLine: 10}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetUncoveredLines(tt.totalLines, tt.coveredRanges)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCountLines(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected int
	}{
		{"empty content", "", 0},
		{"single line no newline", "hello", 1},
		{"single line with newline", "hello\n", 2},
		{"multiple lines", "line1\nline2\nline3", 3},
		{"multiple lines with trailing newline", "line1\nline2\nline3\n", 4},
		{"only newlines", "\n\n\n", 4},
		{"mixed content", "hello\n\nworld\n", 4},
		{"windows line endings", "line1\r\nline2\r\n", 3}, // \r\n counts as 2 characters, but only \n is counted
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CountLines(tt.content)
			assert.Equal(t, tt.expected, result)
		})
	}
}
