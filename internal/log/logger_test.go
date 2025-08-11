package log

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"remap/internal/concurrent"
	"remap/internal/config"
	"remap/internal/replacement"
)

func TestNewLogger(t *testing.T) {
	tests := []struct {
		name        string
		config      *config.Config
		expectError bool
	}{
		{
			name: "logger with stdout",
			config: &config.Config{
				LogFile: "",
				DryRun:  false,
			},
			expectError: false,
		},
		{
			name: "logger with file",
			config: &config.Config{
				LogFile: filepath.Join(t.TempDir(), "test.log"),
				DryRun:  true,
			},
			expectError: false,
		},
		{
			name: "logger with invalid file path",
			config: &config.Config{
				LogFile: "/invalid/path/that/does/not/exist/test.log",
				DryRun:  false,
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, err := NewLogger(tt.config)

			if tt.expectError && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if !tt.expectError {
				if logger == nil {
					t.Fatal("expected non-nil logger")
				}
				if logger.config != tt.config {
					t.Error("config not properly set")
				}
				if logger.summary.DryRun != tt.config.DryRun {
					t.Errorf("expected DryRun=%v, got %v", tt.config.DryRun, logger.summary.DryRun)
				}
				if logger.entries == nil {
					t.Error("entries should be initialized")
				}
				logger.Close() // Clean up
			}
		})
	}
}

func TestLogResult(t *testing.T) {
	tests := []struct {
		name                 string
		result               concurrent.ProcessResult
		expectedFiles        int
		expectedModified     int
		expectedErrors       int
		expectedReplacements int
	}{
		{
			name: "successful result with modifications",
			result: concurrent.ProcessResult{
				Job: concurrent.ProcessJob{
					FilePath: "/test/file.txt",
				},
				Result: &replacement.FileResult{
					OriginalSize: 100,
					NewSize:      110,
					Modified:     true,
					Replacements: []replacement.Replacement{
						{From: "old", To: "new", Line: 1, Column: 1},
						{From: "foo", To: "bar", Line: 2, Column: 1},
					},
				},
				BackupPath: "/test/file.txt.bak",
				Error:      nil,
			},
			expectedFiles:        1,
			expectedModified:     1,
			expectedErrors:       0,
			expectedReplacements: 2,
		},
		{
			name: "result with no modifications",
			result: concurrent.ProcessResult{
				Job: concurrent.ProcessJob{
					FilePath: "/test/unchanged.txt",
				},
				Result: &replacement.FileResult{
					OriginalSize: 50,
					NewSize:      50,
					Modified:     false,
					Replacements: []replacement.Replacement{},
				},
				BackupPath: "",
				Error:      nil,
			},
			expectedFiles:        1,
			expectedModified:     0,
			expectedErrors:       0,
			expectedReplacements: 0,
		},
		{
			name: "result with error",
			result: concurrent.ProcessResult{
				Job: concurrent.ProcessJob{
					FilePath: "/test/error.txt",
				},
				Result:     nil,
				BackupPath: "",
				Error:      errors.New("file not found"),
			},
			expectedFiles:        1,
			expectedModified:     0,
			expectedErrors:       1,
			expectedReplacements: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &config.Config{Quiet: true} // Suppress output
			logger, err := NewLogger(config)
			if err != nil {
				t.Fatal(err)
			}
			defer logger.Close()

			logger.LogResult(tt.result)

			if logger.summary.TotalFiles != tt.expectedFiles {
				t.Errorf("expected %d total files, got %d",
					tt.expectedFiles, logger.summary.TotalFiles)
			}
			if logger.summary.ModifiedFiles != tt.expectedModified {
				t.Errorf("expected %d modified files, got %d",
					tt.expectedModified, logger.summary.ModifiedFiles)
			}
			if logger.summary.ErrorCount != tt.expectedErrors {
				t.Errorf("expected %d errors, got %d",
					tt.expectedErrors, logger.summary.ErrorCount)
			}
			if logger.summary.TotalReplacements != tt.expectedReplacements {
				t.Errorf("expected %d replacements, got %d",
					tt.expectedReplacements, logger.summary.TotalReplacements)
			}
			if len(logger.entries) != 1 {
				t.Errorf("expected 1 entry, got %d", len(logger.entries))
			}
		})
	}
}

func TestSetProcessingTime(t *testing.T) {
	config := &config.Config{}
	logger, err := NewLogger(config)
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Close()

	duration := 5 * time.Second
	logger.SetProcessingTime(duration)

	if logger.summary.ProcessingTime != duration {
		t.Errorf("expected processing time %v, got %v",
			duration, logger.summary.ProcessingTime)
	}
}

func TestWriteJSONReport(t *testing.T) {
	var buf bytes.Buffer
	config := &config.Config{
		LogFormat: config.LogFormatJSON,
	}

	logger := &Logger{
		config: config,
		writer: &buf,
		entries: []Entry{
			{
				Timestamp: "2023-01-01T12:00:00Z",
				FilePath:  "/test/file.txt",
				Modified:  true,
				Replacements: []replacement.Replacement{
					{From: "old", To: "new", Line: 1, Column: 1},
				},
			},
		},
		summary: Summary{
			TotalFiles:        1,
			ModifiedFiles:     1,
			TotalReplacements: 1,
			ProcessingTime:    time.Second,
		},
	}

	err := logger.writeJSONReport()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Verify JSON structure
	var report struct {
		Summary Summary `json:"summary"`
		Entries []Entry `json:"entries"`
	}
	err = json.Unmarshal(buf.Bytes(), &report)
	if err != nil {
		t.Errorf("invalid JSON output: %v", err)
	}

	if report.Summary.TotalFiles != 1 {
		t.Errorf("expected 1 total file in JSON, got %d", report.Summary.TotalFiles)
	}
	if len(report.Entries) != 1 {
		t.Errorf("expected 1 entry in JSON, got %d", len(report.Entries))
	}
}

func TestWriteCSVReport(t *testing.T) {
	tests := []struct {
		name                 string
		entries              []Entry
		expectedReplacements int
		expectedCSVRows      int
		expectReport         bool
		reportPosition       string // "after_all" or "after_3"
	}{
		{
			name: "single replacement - report after all",
			entries: []Entry{
				{
					Timestamp:    "2023-01-01T12:00:00Z",
					FilePath:     "/test/file1.txt",
					Modified:     true,
					OriginalSize: 100,
					NewSize:      110,
					Replacements: []replacement.Replacement{
						{From: "old", To: "new", Line: 5, Column: 10},
					},
					BackupPath: "/test/file1.txt.bak",
				},
			},
			expectedReplacements: 1,
			expectedCSVRows:      2, // header + 1 replacement row
			expectReport:         true,
			reportPosition:       "after_all",
		},
		{
			name: "three replacements - report after all",
			entries: []Entry{
				{
					FilePath: "/test/file1.txt",
					Replacements: []replacement.Replacement{
						{From: "old1", To: "new1", Line: 1, Column: 1},
						{From: "old2", To: "new2", Line: 2, Column: 1},
					},
				},
				{
					FilePath: "/test/file2.txt",
					Replacements: []replacement.Replacement{
						{From: "old3", To: "new3", Line: 3, Column: 1},
					},
				},
			},
			expectedReplacements: 3,
			expectedCSVRows:      4, // header + 3 replacement rows
			expectReport:         true,
			reportPosition:       "after_3",
		},
		{
			name: "five replacements - report after all",
			entries: []Entry{
				{
					FilePath: "/test/file1.txt",
					Replacements: []replacement.Replacement{
						{From: "old1", To: "new1", Line: 1, Column: 1},
						{From: "old2", To: "new2", Line: 2, Column: 1},
						{From: "old3", To: "new3", Line: 3, Column: 1},
					},
				},
				{
					FilePath: "/test/file2.txt",
					Replacements: []replacement.Replacement{
						{From: "old4", To: "new4", Line: 4, Column: 1},
						{From: "old5", To: "new5", Line: 5, Column: 1},
					},
				},
			},
			expectedReplacements: 5,
			expectedCSVRows:      6, // header + 5 replacement rows
			expectReport:         true,
			reportPosition:       "after_all",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			config := &config.Config{
				LogFormat: config.LogFormatCSV,
			}

			logger := &Logger{
				config:  config,
				writer:  &buf,
				entries: tt.entries,
				summary: Summary{
					TotalFiles:        len(tt.entries),
					ModifiedFiles:     len(tt.entries),
					TotalReplacements: tt.expectedReplacements,
					ProcessingTime:    time.Second,
					DryRun:            false,
				},
			}

			err := logger.writeCSVReport()
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			output := buf.String()
			lines := strings.Split(output, "\n")

			var csvLines []string
			var commentLines []string
			var reportStartIndex int = -1

			for i, line := range lines {
				if strings.HasPrefix(line, "# Remap CSV Report") {
					reportStartIndex = i
				}
				if strings.HasPrefix(line, "#") {
					commentLines = append(commentLines, line)
				} else if strings.TrimSpace(line) != "" {
					csvLines = append(csvLines, line)
				}
			}

			// Verify we have both CSV data and comments
			if len(csvLines) == 0 {
				t.Error("expected CSV data, got none")
			}
			if tt.expectReport && len(commentLines) == 0 {
				t.Error("expected comment lines, got none")
			}

			// Verify CSV structure
			csvData := strings.Join(csvLines, "\n")
			reader := csv.NewReader(strings.NewReader(csvData))
			records, err := reader.ReadAll()
			if err != nil {
				t.Errorf("invalid CSV output: %v", err)
			}

			if len(records) != tt.expectedCSVRows {
				t.Errorf("expected %d CSV rows, got %d", tt.expectedCSVRows, len(records))
			}

			// Verify header
			header := records[0]
			expectedHeaders := []string{
				"file_path", "old_string", "new_string", "line", "column",
			}
			for i, expected := range expectedHeaders {
				if i >= len(header) || header[i] != expected {
					t.Errorf("expected header[%d] = %s, got %s", i, expected, header[i])
				}
			}

			// Verify replacement data in CSV rows
			if len(records) > 1 {
				// Check first replacement row format
				firstRow := records[1]
				if len(firstRow) != 5 {
					t.Errorf("expected 5 columns in replacement row, got %d", len(firstRow))
				}
			}

			// Report should always appear after all CSV data
			if tt.expectReport && reportStartIndex >= 0 {
				// Count CSV lines before the report
				csvLinesBeforeReport := 0
				for i := 0; i < reportStartIndex && i < len(lines); i++ {
					line := lines[i]
					if !strings.HasPrefix(line, "#") && strings.TrimSpace(line) != "" {
						csvLinesBeforeReport++
					}
				}
				// Should be header + all replacement rows
				if csvLinesBeforeReport != tt.expectedCSVRows {
					t.Errorf("expected report after %d CSV lines, found after %d lines", tt.expectedCSVRows, csvLinesBeforeReport)
				}
			}
		})
	}
}

func TestWriteSummaryReport(t *testing.T) {
	tests := []struct {
		name     string
		summary  Summary
		entries  []Entry
		expected []string
	}{
		{
			name: "basic summary",
			summary: Summary{
				TotalFiles:        5,
				ModifiedFiles:     3,
				TotalReplacements: 10,
				ErrorCount:        0,
				ProcessingTime:    2 * time.Second,
				DryRun:            false,
			},
			expected: []string{
				"production",
				"Total files processed: 5",
				"Files modified: 3",
				"Total replacements: 10",
				"Errors: 0",
			},
		},
		{
			name: "dry run summary",
			summary: Summary{
				TotalFiles:        2,
				ModifiedFiles:     1,
				TotalReplacements: 3,
				ErrorCount:        0,
				ProcessingTime:    time.Second,
				DryRun:            true,
			},
			expected: []string{
				"dry-run",
				"Total files processed: 2",
				"Files modified: 1",
			},
		},
		{
			name: "summary with errors",
			summary: Summary{
				TotalFiles:        3,
				ModifiedFiles:     1,
				TotalReplacements: 2,
				ErrorCount:        2,
				ProcessingTime:    time.Second,
				DryRun:            false,
			},
			entries: []Entry{
				{FilePath: "/test/error1.txt", Error: "permission denied"},
				{FilePath: "/test/error2.txt", Error: "file not found"},
			},
			expected: []string{
				"Errors: 2",
				"Errors encountered:",
				"/test/error1.txt: permission denied",
				"/test/error2.txt: file not found",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := &Logger{
				config:  &config.Config{},
				writer:  &buf,
				summary: tt.summary,
				entries: tt.entries,
			}

			err := logger.writeSummaryReport()
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			output := buf.String()
			for _, expected := range tt.expected {
				if !strings.Contains(output, expected) {
					t.Errorf("expected output to contain %q, got:\n%s", expected, output)
				}
			}
		})
	}
}

func TestWriteReport(t *testing.T) {
	tests := []struct {
		name        string
		config      *config.Config
		expectError bool
	}{
		{
			name: "quiet mode",
			config: &config.Config{
				Quiet: true,
			},
			expectError: false,
		},
		{
			name: "JSON format",
			config: &config.Config{
				LogFormat: config.LogFormatJSON,
			},
			expectError: false,
		},
		{
			name: "CSV format",
			config: &config.Config{
				LogFormat: config.LogFormatCSV,
			},
			expectError: false,
		},
		{
			name: "default format",
			config: &config.Config{
				LogFormat: "",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := &Logger{
				config:  tt.config,
				writer:  &buf,
				entries: []Entry{},
				summary: Summary{},
			}

			err := logger.WriteReport()

			if tt.expectError && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestLogVerbose(t *testing.T) {
	tests := []struct {
		name     string
		entry    Entry
		debug    bool
		expected []string
	}{
		{
			name: "error entry",
			entry: Entry{
				FilePath: "/test/error.txt",
				Error:    "file not found",
			},
			expected: []string{"ERROR:", "/test/error.txt", "file not found"},
		},
		{
			name: "modified entry",
			entry: Entry{
				FilePath: "/test/modified.txt",
				Modified: true,
				Replacements: []replacement.Replacement{
					{From: "old", To: "new", Line: 1, Column: 1},
				},
			},
			expected: []string{"MODIFIED:", "/test/modified.txt", "(1 replacements)"},
		},
		{
			name: "modified entry with debug",
			entry: Entry{
				FilePath: "/test/debug.txt",
				Modified: true,
				Replacements: []replacement.Replacement{
					{From: "old", To: "new", Line: 1, Column: 5},
				},
			},
			debug:    true,
			expected: []string{"MODIFIED:", "Line 1:5:", "'old' -> 'new'"},
		},
		{
			name: "skipped entry",
			entry: Entry{
				FilePath: "/test/skipped.txt",
				Modified: false,
			},
			expected: []string{"SKIPPED:", "/test/skipped.txt", "(no changes)"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			config := &config.Config{
				Debug: tt.debug,
			}
			logger := &Logger{
				config: config,
				writer: &buf,
			}

			logger.logVerbose(tt.entry)

			output := buf.String()
			for _, expected := range tt.expected {
				if !strings.Contains(output, expected) {
					t.Errorf("expected output to contain %q, got:\n%s", expected, output)
				}
			}
		})
	}
}

func TestLogBasic(t *testing.T) {
	tests := []struct {
		name  string
		entry Entry
	}{
		{
			name: "modified file",
			entry: Entry{
				FilePath: "/test/file.txt",
				Modified: true,
				Replacements: []replacement.Replacement{
					{From: "old", To: "new"},
					{From: "foo", To: "bar"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := &Logger{
				config: &config.Config{},
				writer: &buf,
			}

			// logBasic is now commented out and produces no output
			logger.logBasic(tt.entry)

			output := strings.TrimSpace(buf.String())
			// Expect no output since logBasic is commented out
			if output != "" {
				t.Errorf("expected no output from logBasic, got: %q", output)
			}
		})
	}
}

func TestLoggerClose(t *testing.T) {
	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "test.log")

	config := &config.Config{
		LogFile: logFile,
	}

	logger, err := NewLogger(config)
	if err != nil {
		t.Fatal(err)
	}

	err = logger.Close()
	if err != nil {
		t.Errorf("unexpected error closing logger: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		t.Error("log file should have been created")
	}
}

func TestLoggerIntegration(t *testing.T) {
	var buf bytes.Buffer
	config := &config.Config{
		Verbose:   true,
		LogFormat: "",
	}

	logger := &Logger{
		config:  config,
		writer:  &buf,
		entries: []Entry{},
		summary: Summary{DryRun: false},
	}

	// Simulate processing results
	results := []concurrent.ProcessResult{
		{
			Job: concurrent.ProcessJob{FilePath: "/test/file1.txt"},
			Result: &replacement.FileResult{
				Modified:     true,
				Replacements: []replacement.Replacement{{From: "old", To: "new"}},
			},
		},
		{
			Job:    concurrent.ProcessJob{FilePath: "/test/file2.txt"},
			Result: &replacement.FileResult{Modified: false},
		},
		{
			Job:   concurrent.ProcessJob{FilePath: "/test/error.txt"},
			Error: errors.New("access denied"),
		},
	}

	for _, result := range results {
		logger.LogResult(result)
	}

	logger.SetProcessingTime(2 * time.Second)

	err := logger.WriteReport()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Verify summary statistics
	if logger.summary.TotalFiles != 3 {
		t.Errorf("expected 3 total files, got %d", logger.summary.TotalFiles)
	}
	if logger.summary.ModifiedFiles != 1 {
		t.Errorf("expected 1 modified file, got %d", logger.summary.ModifiedFiles)
	}
	if logger.summary.ErrorCount != 1 {
		t.Errorf("expected 1 error, got %d", logger.summary.ErrorCount)
	}
}
