package parser

import (
	"strings"
	"testing"
)

func TestParseCSVMappings(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectError bool
		expectCount int
	}{
		{
			name:        "valid CSV with header",
			input:       "old,new\nfoo,bar\nhello,world",
			expectError: false,
			expectCount: 2,
		},
		{
			name:        "valid CSV without header",
			input:       "foo,bar\nhello,world",
			expectError: false,
			expectCount: 2,
		},
		{
			name:        "empty CSV",
			input:       "",
			expectError: true,
			expectCount: 0,
		},
		{
			name:        "CSV with empty lines",
			input:       "foo,bar\n,\nhello,world",
			expectError: false,
			expectCount: 2,
		},
		{
			name:        "CSV with comments",
			input:       "# This is a comment\nold,new\n# Another comment\nfoo,bar\ntest,result",
			expectError: false,
			expectCount: 2,
		},
		{
			name:        "CSV with only comments",
			input:       "# This is a comment\n# Another comment\n# Yet another comment",
			expectError: true,
			expectCount: 0,
		},
		{
			name:        "CSV with header and comments",
			input:       "# Configuration mappings\nold,new\n# Legacy values\nfoo,bar\nhello,world",
			expectError: false,
			expectCount: 2,
		},
		{
			name:        "CSV with mixed empty lines and comments",
			input:       "# Comment 1\n\nold,new\n# Comment 2\n\nfoo,bar\n\n# Final comment\ntest,result",
			expectError: false,
			expectCount: 2,
		},
		{
			name:        "CSV with comments but no header",
			input:       "# This is a comment\n# Another comment\nfoo,bar\ntest,result\nhello,world",
			expectError: false,
			expectCount: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.input)
			table, err := parseCSVMappings(reader, "test.csv")

			if tt.expectError && err == nil {
				t.Errorf("expected error, got nil")
			}

			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if !tt.expectError && table.Size() != tt.expectCount {
				t.Errorf("expected %d mappings, got %d", tt.expectCount, table.Size())
			}
		})
	}
}

func TestParseJSONMappings(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectError bool
		expectCount int
	}{
		{
			name:        "valid JSON",
			input:       `[{"old": "foo", "new": "bar"}, {"old": "hello", "new": "world"}]`,
			expectError: false,
			expectCount: 2,
		},
		{
			name:        "empty JSON array",
			input:       `[]`,
			expectError: true,
			expectCount: 0,
		},
		{
			name:        "invalid JSON",
			input:       `{invalid json}`,
			expectError: true,
			expectCount: 0,
		},
		{
			name:        "JSON with empty old field",
			input:       `[{"old": "", "new": "bar"}, {"old": "hello", "new": "world"}]`,
			expectError: false,
			expectCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.input)
			table, err := parseJSONMappings(reader, "test.json")

			if tt.expectError && err == nil {
				t.Errorf("expected error, got nil")
			}

			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if !tt.expectError && table.Size() != tt.expectCount {
				t.Errorf("expected %d mappings, got %d", tt.expectCount, table.Size())
			}
		})
	}
}

func TestMappingTableSorting(t *testing.T) {
	mappings := []Mapping{
		{From: "a", To: "1"},
		{From: "abc", To: "123"},
		{From: "ab", To: "12"},
	}

	table := NewMappingTable(mappings)
	sorted := table.GetSortedMappings()

	expected := []string{"abc", "ab", "a"}
	for i, mapping := range sorted {
		if mapping.From != expected[i] {
			t.Errorf("expected %s at position %d, got %s", expected[i], i, mapping.From)
		}
	}
}
