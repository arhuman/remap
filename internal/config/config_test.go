package config

import (
	"testing"
)

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name        string
		config      Config
		expectError bool
	}{
		{
			name: "valid config",
			config: Config{
				Directory:   ".",
				MappingFile: "test.csv",
				MappingType: "csv",
			},
			expectError: false,
		},
		{
			name: "missing directory",
			config: Config{
				MappingFile: "test.csv",
				MappingType: "csv",
			},
			expectError: true,
		},
		{
			name: "missing mapping file for non-revert",
			config: Config{
				Directory: ".",
				Revert:    false,
			},
			expectError: true,
		},
		{
			name: "valid revert config",
			config: Config{
				Directory: ".",
				Revert:    true,
			},
			expectError: false,
		},
		{
			name: "invalid mapping type",
			config: Config{
				Directory:   ".",
				MappingFile: "test.csv",
				MappingType: "xml",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.expectError && err == nil {
				t.Errorf("expected error, got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestShouldProcessExtension(t *testing.T) {
	config := Config{
		Extensions: []string{".go", ".txt"},
	}

	tests := []struct {
		ext      string
		expected bool
	}{
		{".go", true},
		{".txt", true},
		{".py", false},
		{"go", true},  // Should normalize
		{"txt", true}, // Should normalize
		{".GO", true}, // Case insensitive
	}

	for _, tt := range tests {
		result := config.ShouldProcessExtension(tt.ext)
		if result != tt.expected {
			t.Errorf("ShouldProcessExtension(%q) = %v, expected %v", tt.ext, result, tt.expected)
		}
	}

	emptyConfig := Config{Extensions: []string{}}
	if !emptyConfig.ShouldProcessExtension(".any") {
		t.Errorf("empty extensions should allow any extension")
	}
}
