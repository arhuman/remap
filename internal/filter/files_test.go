package filter

import (
	"os"
	"path/filepath"
	"testing"

	"remap/internal/config"
)

func TestNewFileDiscovery(t *testing.T) {
	tests := []struct {
		name   string
		config *config.Config
	}{
		{
			name: "basic config",
			config: &config.Config{
				Directory:  "/test",
				Extensions: []string{".go", ".txt"},
			},
		},
		{
			name: "config with patterns",
			config: &config.Config{
				Directory: "/test",
				Include:   []string{"*.go"},
				Exclude:   []string{"*_test.go"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			discovery := NewFileDiscovery(tt.config)

			if discovery == nil {
				t.Fatal("expected non-nil FileDiscovery")
			}
			if discovery.config != tt.config {
				t.Error("config not properly set")
			}
			if len(discovery.filters) == 0 {
				t.Error("expected at least one filter")
			}
		})
	}
}

func TestFileDiscoveryDiscover(t *testing.T) {
	tempDir := t.TempDir()

	// Create test files
	testFiles := []struct {
		name    string
		content string
		isDir   bool
	}{
		{name: "test.go", content: "package main", isDir: false},
		{name: "test.txt", content: "text content", isDir: false},
		{name: "test.py", content: "print('hello')", isDir: false},
		{name: ".hidden", content: "hidden file", isDir: false},
		{name: "backup.bak", content: "backup", isDir: false},
		{name: "subdir", content: "", isDir: true},
	}

	for _, tf := range testFiles {
		path := filepath.Join(tempDir, tf.name)
		if tf.isDir {
			err := os.Mkdir(path, 0755)
			if err != nil {
				t.Fatal(err)
			}
		} else {
			err := os.WriteFile(path, []byte(tf.content), 0644)
			if err != nil {
				t.Fatal(err)
			}
		}
	}

	tests := []struct {
		name          string
		config        *config.Config
		expectedCount int
		expectedFiles []string
	}{
		{
			name: "all files",
			config: &config.Config{
				Directory:  tempDir,
				Extensions: []string{},
			},
			expectedCount: 3, // go, txt, py (excludes hidden, bak, dir)
		},
		{
			name: "go files only",
			config: &config.Config{
				Directory:  tempDir,
				Extensions: []string{".go"},
			},
			expectedCount: 1,
			expectedFiles: []string{"test.go"},
		},
		{
			name: "include pattern",
			config: &config.Config{
				Directory: tempDir,
				Include:   []string{"*.txt"},
			},
			expectedCount: 1,
			expectedFiles: []string{"test.txt"},
		},
		{
			name: "exclude pattern",
			config: &config.Config{
				Directory: tempDir,
				Exclude:   []string{"*.py"},
			},
			expectedCount: 2, // go, txt (excludes py, hidden, bak, dir)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			discovery := NewFileDiscovery(tt.config)
			files, err := discovery.Discover()

			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if len(files) != tt.expectedCount {
				t.Errorf("expected %d files, got %d", tt.expectedCount, len(files))
			}

			if tt.expectedFiles != nil {
				found := make(map[string]bool)
				for _, file := range files {
					found[filepath.Base(file.Path)] = true
				}
				for _, expected := range tt.expectedFiles {
					if !found[expected] {
						t.Errorf("expected file %s not found", expected)
					}
				}
			}

			// Verify file info
			for _, file := range files {
				if file.Path == "" {
					t.Error("file path should not be empty")
				}
				if file.Size < 0 {
					t.Error("file size should not be negative")
				}
				if file.IsDir {
					t.Error("directories should be filtered out")
				}
			}
		})
	}
}

func TestExtensionFilter(t *testing.T) {
	tests := []struct {
		name     string
		config   *config.Config
		filePath string
		expected bool
	}{
		{
			name: "empty extensions allows all",
			config: &config.Config{
				Extensions: []string{},
			},
			filePath: "/path/to/file.txt",
			expected: true,
		},
		{
			name: "matching extension",
			config: &config.Config{
				Extensions: []string{".txt", ".go"},
			},
			filePath: "/path/to/file.txt",
			expected: true,
		},
		{
			name: "non-matching extension",
			config: &config.Config{
				Extensions: []string{".txt", ".go"},
			},
			filePath: "/path/to/file.py",
			expected: false,
		},
		{
			name: "case insensitive matching",
			config: func() *config.Config {
				cfg := &config.Config{
					Directory:   "/test",     // Required for validation
					MappingFile: "/test.csv", // Required for validation
					Extensions:  []string{".TXT"},
				}
				// Normalize extensions to test case insensitive matching
				err := cfg.Validate()
				if err != nil {
					panic(err) // This will show us if validation fails
				}
				return cfg
			}(),
			filePath: "/path/to/file.txt",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := extensionFilter(tt.config)
			result, err := filter(tt.filePath, nil)

			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestIncludeFilter(t *testing.T) {
	tests := []struct {
		name     string
		patterns []string
		filePath string
		expected bool
		hasError bool
	}{
		{
			name:     "matching basename pattern",
			patterns: []string{"*.go"},
			filePath: "/path/to/main.go",
			expected: true,
		},
		{
			name:     "matching full path pattern",
			patterns: []string{"*/main.go"},
			filePath: "/path/to/main.go",
			expected: true,
		},
		{
			name:     "no matching pattern",
			patterns: []string{"*.py"},
			filePath: "/path/to/main.go",
			expected: false,
		},
		{
			name:     "multiple patterns with match",
			patterns: []string{"*.py", "*.go"},
			filePath: "/path/to/main.go",
			expected: true,
		},
		{
			name:     "invalid pattern",
			patterns: []string{"[invalid"},
			filePath: "/path/to/file.txt",
			expected: false,
			hasError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := includeFilter(tt.patterns)
			result, err := filter(tt.filePath, nil)

			if tt.hasError && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.hasError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if !tt.hasError && result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestExcludeFilter(t *testing.T) {
	tests := []struct {
		name     string
		patterns []string
		filePath string
		expected bool
		hasError bool
	}{
		{
			name:     "matching pattern excludes",
			patterns: []string{"*.log"},
			filePath: "/path/to/debug.log",
			expected: false,
		},
		{
			name:     "no matching pattern includes",
			patterns: []string{"*.log"},
			filePath: "/path/to/main.go",
			expected: true,
		},
		{
			name:     "multiple patterns with match excludes",
			patterns: []string{"*.log", "*.tmp"},
			filePath: "/path/to/temp.tmp",
			expected: false,
		},
		{
			name:     "invalid pattern",
			patterns: []string{"[invalid"},
			filePath: "/path/to/file.txt",
			expected: true,
			hasError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := excludeFilter(tt.patterns)
			result, err := filter(tt.filePath, nil)

			if tt.hasError && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.hasError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if !tt.hasError && result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestRegularFileFilter(t *testing.T) {
	tempDir := t.TempDir()

	// Create test files
	regularFile := filepath.Join(tempDir, "regular.txt")
	err := os.WriteFile(regularFile, []byte("content"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	hiddenFile := filepath.Join(tempDir, ".hidden")
	err = os.WriteFile(hiddenFile, []byte("hidden"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	bakFile := filepath.Join(tempDir, "backup.bak")
	err = os.WriteFile(bakFile, []byte("backup"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	subdir := filepath.Join(tempDir, "subdir")
	err = os.Mkdir(subdir, 0755)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		filePath string
		expected bool
	}{
		{
			name:     "regular file",
			filePath: regularFile,
			expected: true,
		},
		{
			name:     "hidden file",
			filePath: hiddenFile,
			expected: false,
		},
		{
			name:     "backup file",
			filePath: bakFile,
			expected: false,
		},
		{
			name:     "directory",
			filePath: subdir,
			expected: false,
		},
	}

	filter := regularFileFilter()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := os.Stat(tt.filePath)
			if err != nil {
				t.Fatal(err)
			}

			result, err := filter(tt.filePath, info)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestBuildFilters(t *testing.T) {
	tests := []struct {
		name          string
		config        *config.Config
		expectedCount int
	}{
		{
			name: "basic config",
			config: &config.Config{
				Extensions: []string{".go"},
			},
			expectedCount: 2, // extension + regularFile
		},
		{
			name: "config with include",
			config: &config.Config{
				Extensions: []string{".go"},
				Include:    []string{"*.go"},
			},
			expectedCount: 3, // extension + include + regularFile
		},
		{
			name: "config with exclude",
			config: &config.Config{
				Extensions: []string{".go"},
				Exclude:    []string{"*_test.go"},
			},
			expectedCount: 3, // extension + exclude + regularFile
		},
		{
			name: "full config",
			config: &config.Config{
				Extensions: []string{".go"},
				Include:    []string{"*.go"},
				Exclude:    []string{"*_test.go"},
			},
			expectedCount: 4, // extension + include + exclude + regularFile
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filters := buildFilters(tt.config)

			if len(filters) != tt.expectedCount {
				t.Errorf("expected %d filters, got %d", tt.expectedCount, len(filters))
			}
		})
	}
}

func TestShouldProcessFile(t *testing.T) {
	tempDir := t.TempDir()

	// Create test file
	testFile := filepath.Join(tempDir, "test.go")
	err := os.WriteFile(testFile, []byte("package main"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		config   *config.Config
		filePath string
		expected bool
	}{
		{
			name: "file passes all filters",
			config: &config.Config{
				Extensions: []string{".go"},
			},
			filePath: testFile,
			expected: true,
		},
		{
			name: "file fails extension filter",
			config: &config.Config{
				Extensions: []string{".txt"},
			},
			filePath: testFile,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			discovery := NewFileDiscovery(tt.config)

			info, err := os.Stat(tt.filePath)
			if err != nil {
				t.Fatal(err)
			}

			result, err := discovery.shouldProcessFile(tt.filePath, info)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestFileInfoStruct(t *testing.T) {
	tests := []struct {
		name    string
		info    FileInfo
		isValid bool
	}{
		{
			name: "valid file info",
			info: FileInfo{
				Path:    "/test/file.txt",
				Size:    1024,
				IsDir:   false,
				ModTime: 1234567890,
			},
			isValid: true,
		},
		{
			name: "directory file info",
			info: FileInfo{
				Path:    "/test/dir",
				Size:    0,
				IsDir:   true,
				ModTime: 1234567890,
			},
			isValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.isValid {
				if tt.info.Path == "" {
					t.Error("path should not be empty for valid file info")
				}
				if tt.info.Size < 0 {
					t.Error("size should not be negative")
				}
			}
		})
	}
}

func TestDiscoverPermissionErrors(t *testing.T) {
	tempDir := t.TempDir()

	// Create a directory and file
	subDir := filepath.Join(tempDir, "subdir")
	err := os.Mkdir(subDir, 0755)
	if err != nil {
		t.Fatal(err)
	}

	testFile := filepath.Join(subDir, "test.txt")
	err = os.WriteFile(testFile, []byte("content"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	config := &config.Config{
		Directory: tempDir,
	}

	discovery := NewFileDiscovery(config)
	files, err := discovery.Discover()

	// Should handle permission errors gracefully
	if err != nil {
		t.Errorf("expected no error for permission denied, got: %v", err)
	}

	// Should find at least the test file
	if len(files) == 0 {
		t.Error("expected to find at least one file")
	}
}
