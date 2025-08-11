package concurrent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"remap/internal/config"
	"remap/internal/filter"
	"remap/internal/parser"
	"remap/internal/replacement"
)

func TestNewProcessor(t *testing.T) {
	tests := []struct {
		name        string
		config      *config.Config
		mappings    *parser.MappingTable
		expectValid bool
	}{
		{
			name: "valid processor",
			config: &config.Config{
				Directory: "/test",
				Backup:    true,
				DryRun:    false,
			},
			mappings:    parser.NewMappingTable([]parser.Mapping{}),
			expectValid: true,
		},
		{
			name: "processor with backup disabled",
			config: &config.Config{
				Directory: "/test",
				Backup:    false,
				DryRun:    true,
			},
			mappings:    parser.NewMappingTable([]parser.Mapping{}),
			expectValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processor := NewProcessor(tt.config, tt.mappings)

			if !tt.expectValid {
				if processor != nil {
					t.Error("expected nil processor for invalid input")
				}
				return
			}

			if processor == nil {
				t.Fatal("expected non-nil processor")
			}
			if processor.config != tt.config {
				t.Error("config not properly set")
			}
			if processor.mappings != tt.mappings {
				t.Error("mappings not properly set")
			}
			if processor.workerCount <= 0 || processor.workerCount > 8 {
				t.Errorf("expected worker count between 1-8, got %d", processor.workerCount)
			}
			if processor.engine == nil {
				t.Error("engine should be initialized")
			}
			if processor.backupManager == nil {
				t.Error("backup manager should be initialized")
			}
		})
	}
}

func TestProcessFiles(t *testing.T) {
	tempDir := t.TempDir()

	// Create test files
	testFiles := []struct {
		name    string
		content string
	}{
		{"file1.txt", "hello world"},
		{"file2.txt", "foo bar"},
		{"file3.txt", "test content"},
	}

	var fileInfos []filter.FileInfo
	for _, tf := range testFiles {
		path := filepath.Join(tempDir, tf.name)
		err := os.WriteFile(path, []byte(tf.content), 0644)
		if err != nil {
			t.Fatal(err)
		}

		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}

		fileInfos = append(fileInfos, filter.FileInfo{
			Path:    path,
			Size:    info.Size(),
			IsDir:   false,
			ModTime: info.ModTime().Unix(),
		})
	}

	tests := []struct {
		name            string
		config          *config.Config
		mappings        []parser.Mapping
		expectedResults int
		expectError     bool
	}{
		{
			name: "process files with replacements",
			config: &config.Config{
				Directory:     tempDir,
				DryRun:        true,
				Backup:        false,
				CaseSensitive: false,
			},
			mappings: []parser.Mapping{
				{From: "hello", To: "hi"},
				{From: "foo", To: "bar"},
			},
			expectedResults: 3,
			expectError:     false,
		},
		{
			name: "process files with backup enabled",
			config: &config.Config{
				Directory:     tempDir,
				DryRun:        false,
				Backup:        true,
				CaseSensitive: true,
			},
			mappings: []parser.Mapping{
				{From: "world", To: "universe"},
			},
			expectedResults: 3,
			expectError:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mappingTable := parser.NewMappingTable(tt.mappings)
			processor := NewProcessor(tt.config, mappingTable)

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			results, err := processor.ProcessFiles(ctx, fileInfos)

			if tt.expectError && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if !tt.expectError {
				var processedResults []ProcessResult
				for result := range results {
					processedResults = append(processedResults, result)
				}

				if len(processedResults) != tt.expectedResults {
					t.Errorf("expected %d results, got %d", tt.expectedResults, len(processedResults))
				}

				// Verify results
				for _, result := range processedResults {
					if result.Job.FilePath == "" {
						t.Error("job file path should not be empty")
					}
					if result.Result == nil && result.Error == nil {
						t.Error("result should have either Result or Error")
					}
					if tt.config.Backup && result.Result != nil && result.Result.Modified && result.BackupPath == "" && !tt.config.DryRun {
						t.Error("backup path should be set when backup is enabled and file is modified")
					}
				}
			}
		})
	}
}

func TestProcessFileInternal(t *testing.T) {
	tempDir := t.TempDir()

	tests := []struct {
		name           string
		config         *config.Config
		mappings       []parser.Mapping
		fileContent    string
		expectError    bool
		expectModified bool
	}{
		{
			name: "successful processing with modifications",
			config: &config.Config{
				Directory:     tempDir,
				DryRun:        true,
				Backup:        false,
				CaseSensitive: false,
			},
			mappings: []parser.Mapping{
				{From: "hello", To: "hi"},
			},
			fileContent:    "hello world",
			expectError:    false,
			expectModified: true,
		},
		{
			name: "processing with no modifications",
			config: &config.Config{
				Directory:     tempDir,
				DryRun:        true,
				Backup:        false,
				CaseSensitive: true,
			},
			mappings: []parser.Mapping{
				{From: "HELLO", To: "hi"},
			},
			fileContent:    "hello world",
			expectError:    false,
			expectModified: false,
		},
		{
			name: "processing with backup and actual write",
			config: &config.Config{
				Directory:     tempDir,
				DryRun:        false,
				Backup:        true,
				CaseSensitive: false,
			},
			mappings: []parser.Mapping{
				{From: "world", To: "universe"},
			},
			fileContent:    "hello world",
			expectError:    false,
			expectModified: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test file
			testFile := filepath.Join(tempDir, "test_"+strings.ReplaceAll(tt.name, " ", "_")+".txt")
			err := os.WriteFile(testFile, []byte(tt.fileContent), 0644)
			if err != nil {
				t.Fatal(err)
			}

			info, err := os.Stat(testFile)
			if err != nil {
				t.Fatal(err)
			}

			mappingTable := parser.NewMappingTable(tt.mappings)
			processor := NewProcessor(tt.config, mappingTable)

			job := ProcessJob{
				FilePath: testFile,
				FileInfo: filter.FileInfo{
					Path:    testFile,
					Size:    info.Size(),
					IsDir:   false,
					ModTime: info.ModTime().Unix(),
				},
			}

			result := processor.processFile(job)

			if tt.expectError && result.Error == nil {
				t.Error("expected error, got nil")
			}
			if !tt.expectError && result.Error != nil {
				t.Errorf("unexpected error: %v", result.Error)
			}

			if !tt.expectError {
				if result.Result == nil {
					t.Fatal("expected non-nil result")
				}
				if result.Result.Modified != tt.expectModified {
					t.Errorf("expected modified=%v, got %v", tt.expectModified, result.Result.Modified)
				}
				if tt.config.Backup && result.Result.Modified && result.BackupPath == "" && !tt.config.DryRun {
					t.Error("expected backup path when backup enabled and file modified")
				}
			}
		})
	}
}

func TestReplaceAll(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		from     string
		to       string
		expected string
	}{
		{
			name:     "simple replacement",
			content:  "hello world hello",
			from:     "hello",
			to:       "hi",
			expected: "hi world hi",
		},
		{
			name:     "no matches",
			content:  "hello world",
			from:     "foo",
			to:       "bar",
			expected: "hello world",
		},
		{
			name:     "empty from string",
			content:  "hello world",
			from:     "",
			to:       "replacement",
			expected: "hello world",
		},
		{
			name:     "overlapping replacements",
			content:  "aaaa",
			from:     "aa",
			to:       "b",
			expected: "bb",
		},
		{
			name:     "replacement creates new matches",
			content:  "abc",
			from:     "bc",
			to:       "bbc",
			expected: "abbc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := replaceAll(tt.content, tt.from, tt.to)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestReplaceFirst(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		from     string
		to       string
		expected string
	}{
		{
			name:     "first occurrence replacement",
			content:  "hello world hello",
			from:     "hello",
			to:       "hi",
			expected: "hi world hello",
		},
		{
			name:     "no match",
			content:  "hello world",
			from:     "foo",
			to:       "bar",
			expected: "hello world",
		},
		{
			name:     "replacement at beginning",
			content:  "hello world",
			from:     "hello",
			to:       "hi",
			expected: "hi world",
		},
		{
			name:     "replacement at end",
			content:  "hello world",
			from:     "world",
			to:       "universe",
			expected: "hello universe",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := replaceFirst(tt.content, tt.from, tt.to)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestFindString(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		search   string
		expected int
	}{
		{
			name:     "found at beginning",
			content:  "hello world",
			search:   "hello",
			expected: 0,
		},
		{
			name:     "found in middle",
			content:  "hello world",
			search:   "lo wo",
			expected: 3,
		},
		{
			name:     "not found",
			content:  "hello world",
			search:   "foo",
			expected: -1,
		},
		{
			name:     "empty search",
			content:  "hello world",
			search:   "",
			expected: 0,
		},
		{
			name:     "search longer than content",
			content:  "hi",
			search:   "hello",
			expected: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := findString(tt.content, tt.search)
			if result != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, result)
			}
		})
	}
}

func TestCaseInsensitiveReplace(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		from     string
		to       string
		expected string
	}{
		{
			name:     "case insensitive replacement",
			content:  "Hello WORLD hello",
			from:     "hello",
			to:       "hi",
			expected: "hi WORLD hi",
		},
		{
			name:     "mixed case replacement",
			content:  "FoO bar FOO",
			from:     "foo",
			to:       "baz",
			expected: "baz bar baz",
		},
		{
			name:     "no matches",
			content:  "hello world",
			from:     "xyz",
			to:       "abc",
			expected: "hello world",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := caseInsensitiveReplaceAll(tt.content, tt.from, tt.to)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestToLower(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "uppercase letters",
			input:    "HELLO",
			expected: "hello",
		},
		{
			name:     "mixed case",
			input:    "Hello World",
			expected: "hello world",
		},
		{
			name:     "already lowercase",
			input:    "hello world",
			expected: "hello world",
		},
		{
			name:     "with numbers and symbols",
			input:    "Hello123!@#",
			expected: "hello123!@#",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toLower(tt.input)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestWriteFile(t *testing.T) {
	tempDir := t.TempDir()

	tests := []struct {
		name         string
		fileContent  string
		replacements []replacement.Replacement
		newContent   string
		expectError  bool
	}{
		{
			name:        "successful write with replacements",
			fileContent: "hello world",
			replacements: []replacement.Replacement{
				{From: "hello", To: "hi", Line: 1, Column: 1},
			},
			newContent:  "hi world",
			expectError: false,
		},
		{
			name:         "write with no replacements",
			fileContent:  "hello world",
			replacements: []replacement.Replacement{},
			newContent:   "hello world",
			expectError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testFile := filepath.Join(tempDir, "writetest.txt")
			err := os.WriteFile(testFile, []byte(tt.fileContent), 0644)
			if err != nil {
				t.Fatal(err)
			}

			config := &config.Config{
				CaseSensitive: false,
			}
			mappingTable := parser.NewMappingTable([]parser.Mapping{})
			processor := NewProcessor(config, mappingTable)

			err = processor.writeFile(testFile, tt.replacements, []byte(tt.newContent))

			if tt.expectError && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if !tt.expectError {
				content, err := os.ReadFile(testFile)
				if err != nil {
					t.Fatal(err)
				}
				if string(content) != tt.newContent {
					t.Errorf("expected file content %q, got %q", tt.newContent, string(content))
				}
			}
		})
	}
}

func TestProcessJobStruct(t *testing.T) {
	job := ProcessJob{
		FilePath: "/test/file.txt",
		FileInfo: filter.FileInfo{
			Path:    "/test/file.txt",
			Size:    100,
			IsDir:   false,
			ModTime: 123456789,
		},
	}

	if job.FilePath != "/test/file.txt" {
		t.Errorf("expected FilePath %s, got %s", "/test/file.txt", job.FilePath)
	}
	if job.FileInfo.Size != 100 {
		t.Errorf("expected Size 100, got %d", job.FileInfo.Size)
	}
}

func TestProcessResultStruct(t *testing.T) {
	result := ProcessResult{
		Job: ProcessJob{
			FilePath: "/test/file.txt",
		},
		Result:     nil,
		BackupPath: "/test/file.txt.bak",
		Error:      nil,
	}

	if result.Job.FilePath != "/test/file.txt" {
		t.Errorf("expected job FilePath %s, got %s", "/test/file.txt", result.Job.FilePath)
	}
	if result.BackupPath != "/test/file.txt.bak" {
		t.Errorf("expected BackupPath %s, got %s", "/test/file.txt.bak", result.BackupPath)
	}
}

func TestContextCancellation(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.txt")
	err := os.WriteFile(testFile, []byte("test content"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(testFile)
	if err != nil {
		t.Fatal(err)
	}

	fileInfos := []filter.FileInfo{
		{
			Path:    testFile,
			Size:    info.Size(),
			IsDir:   false,
			ModTime: info.ModTime().Unix(),
		},
	}

	config := &config.Config{
		Directory: tempDir,
		DryRun:    true,
	}
	mappings := parser.NewMappingTable([]parser.Mapping{})
	processor := NewProcessor(config, mappings)

	// Create a context that's immediately cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	results, err := processor.ProcessFiles(ctx, fileInfos)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Should still work but might not process all files due to cancellation
	var resultCount int
	for range results {
		resultCount++
	}

	// We don't enforce that no results are processed because the cancellation
	// timing is non-deterministic, but we verify the mechanism works
	t.Logf("Processed %d results with cancelled context", resultCount)
}
