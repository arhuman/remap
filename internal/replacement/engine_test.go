package replacement

import (
	"strings"
	"testing"

	"remap/internal/config"
	"remap/internal/parser"
)

func TestCaseInsensitiveReplace(t *testing.T) {
	tests := []struct {
		content  string
		from     string
		to       string
		expected string
	}{
		{
			content:  "Hello World",
			from:     "hello",
			to:       "hi",
			expected: "hi World",
		},
		{
			content:  "FOO bar FOO",
			from:     "foo",
			to:       "baz",
			expected: "baz bar baz",
		},
		{
			content:  "nothing to replace",
			from:     "xyz",
			to:       "abc",
			expected: "nothing to replace",
		},
	}

	for _, tt := range tests {
		result := caseInsensitiveReplace(tt.content, tt.from, tt.to)
		if result != tt.expected {
			t.Errorf("caseInsensitiveReplace(%q, %q, %q) = %q, expected %q",
				tt.content, tt.from, tt.to, result, tt.expected)
		}
	}
}

func TestEngineProcessFile(t *testing.T) {
	mappings := []parser.Mapping{
		{From: "foo", To: "bar"},
		{From: "hello", To: "hi"},
	}
	table := parser.NewMappingTable(mappings)

	config := &config.Config{
		CaseSensitive: false,
		DryRun:        true,
	}

	engine := NewEngine(config)
	content := []byte("Hello foo world, foo is here!")
	result := engine.ProcessFile("test.txt", content, table)

	if !result.Modified {
		t.Errorf("expected file to be marked as modified")
	}

	if len(result.Replacements) != 3 {
		t.Errorf("expected 3 replacements, got %d", len(result.Replacements))
	}

	expectedReplacements := []string{"hello", "foo", "foo"}
	for i, replacement := range result.Replacements {
		if i < len(expectedReplacements) && replacement.From != expectedReplacements[i] {
			t.Errorf("replacement %d: expected %q, got %q", i, expectedReplacements[i], replacement.From)
		}
	}
}

func TestProcessReader(t *testing.T) {
	mappings := []parser.Mapping{
		{From: "test", To: "demo"},
	}
	table := parser.NewMappingTable(mappings)

	reader := strings.NewReader("This is a test file for test purposes")
	content, replacements, err := ProcessReader(reader, table, false)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(replacements) == 0 {
		t.Errorf("expected replacements, got none")
	}

	if len(content) == 0 {
		t.Errorf("expected content, got none")
	}
}
