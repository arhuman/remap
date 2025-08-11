package backup

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"remap/internal/replacement"
)

func TestNewBackupManager(t *testing.T) {
	tests := []struct {
		name    string
		enabled bool
		want    bool
	}{
		{
			name:    "enabled manager",
			enabled: true,
			want:    true,
		},
		{
			name:    "disabled manager",
			enabled: false,
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewBackupManager(tt.enabled)
			if manager == nil {
				t.Fatal("expected non-nil manager")
			}
			if manager.enabled != tt.want {
				t.Errorf("expected enabled=%v, got enabled=%v", tt.want, manager.enabled)
			}
		})
	}
}

func TestBackupFile(t *testing.T) {
	tempDir := t.TempDir()

	tests := []struct {
		name        string
		enabled     bool
		setupFile   bool
		fileContent string
		expectError bool
		expectPath  bool
	}{
		{
			name:        "backup enabled with valid file",
			enabled:     true,
			setupFile:   true,
			fileContent: "test content",
			expectError: false,
			expectPath:  true,
		},
		{
			name:        "backup disabled",
			enabled:     false,
			setupFile:   true,
			fileContent: "test content",
			expectError: false,
			expectPath:  false,
		},
		{
			name:        "backup enabled with non-existent file",
			enabled:     true,
			setupFile:   false,
			fileContent: "",
			expectError: true,
			expectPath:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewBackupManager(tt.enabled)

			var filePath string
			if tt.setupFile {
				file, err := os.CreateTemp(tempDir, "test*.txt")
				if err != nil {
					t.Fatal(err)
				}
				filePath = file.Name()

				_, err = file.WriteString(tt.fileContent)
				if err != nil {
					t.Fatal(err)
				}
				file.Close()
			} else {
				filePath = filepath.Join(tempDir, "nonexistent.txt")
			}

			backupPath, err := manager.BackupFile(filePath)

			if tt.expectError && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if tt.expectPath && backupPath == "" {
				t.Error("expected backup path, got empty string")
			}
			if !tt.expectPath && backupPath != "" {
				t.Errorf("expected empty backup path, got %s", backupPath)
			}

			if tt.expectPath && backupPath != "" {
				if _, err := os.Stat(backupPath); os.IsNotExist(err) {
					t.Error("backup file was not created")
				}

				backupContent, err := os.ReadFile(backupPath)
				if err != nil {
					t.Errorf("failed to read backup file: %v", err)
				}
				if string(backupContent) != tt.fileContent {
					t.Errorf("backup content mismatch: expected %q, got %q", tt.fileContent, string(backupContent))
				}
			}
		})
	}
}

func TestRestoreFile(t *testing.T) {
	tempDir := t.TempDir()

	tests := []struct {
		name            string
		setupFiles      bool
		originalPath    string
		backupPath      string
		backupContent   string
		emptyBackupPath bool
		expectError     bool
	}{
		{
			name:            "restore from valid backup",
			setupFiles:      true,
			backupContent:   "backup content",
			emptyBackupPath: false,
			expectError:     false,
		},
		{
			name:            "restore with empty backup path",
			setupFiles:      false,
			emptyBackupPath: true,
			expectError:     false,
		},
		{
			name:            "restore with non-existent backup",
			setupFiles:      false,
			emptyBackupPath: false,
			expectError:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewBackupManager(true)

			originalPath := filepath.Join(tempDir, "original.txt")
			var backupPath string

			if tt.emptyBackupPath {
				backupPath = ""
			} else if tt.setupFiles {
				backupFile, err := os.CreateTemp(tempDir, "backup*.txt")
				if err != nil {
					t.Fatal(err)
				}
				backupPath = backupFile.Name()

				_, err = backupFile.WriteString(tt.backupContent)
				if err != nil {
					t.Fatal(err)
				}
				backupFile.Close()
			} else {
				backupPath = filepath.Join(tempDir, "nonexistent_backup.txt")
			}

			err := manager.RestoreFile(originalPath, backupPath)

			if tt.expectError && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if tt.setupFiles && !tt.expectError {
				restoredContent, err := os.ReadFile(originalPath)
				if err != nil {
					t.Errorf("failed to read restored file: %v", err)
				}
				if string(restoredContent) != tt.backupContent {
					t.Errorf("restored content mismatch: expected %q, got %q", tt.backupContent, string(restoredContent))
				}
			}
		})
	}
}

func TestCleanupBackup(t *testing.T) {
	tempDir := t.TempDir()

	tests := []struct {
		name        string
		setupFile   bool
		emptyPath   bool
		expectError bool
	}{
		{
			name:        "cleanup existing backup",
			setupFile:   true,
			emptyPath:   false,
			expectError: false,
		},
		{
			name:        "cleanup non-existent backup",
			setupFile:   false,
			emptyPath:   false,
			expectError: false,
		},
		{
			name:        "cleanup with empty path",
			setupFile:   false,
			emptyPath:   true,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewBackupManager(true)

			var backupPath string
			if tt.emptyPath {
				backupPath = ""
			} else if tt.setupFile {
				file, err := os.CreateTemp(tempDir, "backup*.txt")
				if err != nil {
					t.Fatal(err)
				}
				backupPath = file.Name()
				file.Close()
			} else {
				backupPath = filepath.Join(tempDir, "nonexistent.txt")
			}

			err := manager.CleanupBackup(backupPath)

			if tt.expectError && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if tt.setupFile && backupPath != "" {
				if _, err := os.Stat(backupPath); !os.IsNotExist(err) {
					t.Error("backup file should have been deleted")
				}
			}
		})
	}
}

func TestGenerateBackupPath(t *testing.T) {
	tests := []struct {
		name         string
		originalPath string
		wantPattern  string
	}{
		{
			name:         "simple filename",
			originalPath: "/path/to/file.txt",
			wantPattern:  ".bak",
		},
		{
			name:         "filename with multiple extensions",
			originalPath: "/path/to/file.tar.gz",
			wantPattern:  ".bak",
		},
		{
			name:         "filename without extension",
			originalPath: "/path/to/filename",
			wantPattern:  ".bak",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generateBackupPath(tt.originalPath)

			if result == "" {
				t.Error("expected non-empty backup path")
			}
			if !strings.HasSuffix(result, tt.wantPattern) {
				t.Errorf("expected backup path to end with %s, got %s", tt.wantPattern, result)
			}
			if !strings.Contains(result, filepath.Base(tt.originalPath)) {
				t.Errorf("expected backup path to contain original filename, got %s", result)
			}

			// Check timestamp format
			parts := strings.Split(filepath.Base(result), ".")
			if len(parts) < 3 {
				t.Errorf("expected timestamp in backup filename, got %s", result)
			}

			timestampPart := parts[len(parts)-2]
			if len(timestampPart) != 15 { // YYYYMMDD_HHMMSS format
				t.Errorf("expected timestamp format YYYYMMDD_HHMMSS, got %s", timestampPart)
			}
		})
	}
}

func TestRevertManager(t *testing.T) {
	manager := NewRevertManager()
	if manager == nil {
		t.Fatal("expected non-nil revert manager")
	}

	// Test with non-existent log file
	err := manager.RevertFromLog("nonexistent.log")
	if err == nil {
		t.Error("expected error for non-existent log file")
	}
}

func TestParseJSONLog(t *testing.T) {
	manager := NewRevertManager()

	tests := []struct {
		name        string
		jsonContent string
		expectError bool
		entryCount  int
	}{
		{
			name: "valid JSON log",
			jsonContent: `{
				"summary": {"total_files": 2},
				"entries": [
					{
						"timestamp": "2024-01-01T00:00:00Z",
						"file_path": "/path/to/file1.txt",
						"modified": true,
						"replacements": [
							{"from": "old", "to": "new", "line": 1, "column": 1}
						],
						"backup_path": "/path/to/file1.txt.backup"
					},
					{
						"timestamp": "2024-01-01T00:00:00Z",
						"file_path": "/path/to/file2.txt",
						"modified": false,
						"replacements": []
					}
				]
			}`,
			expectError: false,
			entryCount:  2,
		},
		{
			name:        "invalid JSON",
			jsonContent: `{invalid json}`,
			expectError: true,
			entryCount:  0,
		},
		{
			name: "empty entries",
			jsonContent: `{
				"summary": {"total_files": 0},
				"entries": []
			}`,
			expectError: false,
			entryCount:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entries, err := manager.parseJSONLog([]byte(tt.jsonContent))

			if tt.expectError && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if len(entries) != tt.entryCount {
				t.Errorf("expected %d entries, got %d", tt.entryCount, len(entries))
			}
		})
	}
}

func TestParseCSVLog(t *testing.T) {
	manager := NewRevertManager()

	tests := []struct {
		name        string
		csvContent  string
		expectError bool
		entryCount  int
	}{
		{
			name: "valid CSV log",
			csvContent: `file_path,old_string,new_string,line,column
/path/to/file1.txt,old1,new1,1,5
/path/to/file1.txt,old2,new2,2,10
/path/to/file2.txt,foo,bar,1,1`,
			expectError: false,
			entryCount:  2, // Two unique files
		},
		{
			name: "CSV with comments",
			csvContent: `# This is a comment
# Another comment
file_path,old_string,new_string,line,column
/path/to/file.txt,test,result,1,1
# More comments
`,
			expectError: false,
			entryCount:  1,
		},
		{
			name:        "empty CSV",
			csvContent:  "",
			expectError: true,
			entryCount:  0,
		},
		{
			name: "malformed CSV",
			csvContent: `file_path,old_string,new_string,line,column
/path/to/file.txt,incomplete`,
			expectError: false,
			entryCount:  0, // Malformed records should be skipped
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entries, err := manager.parseCSVLog(tt.csvContent)

			if tt.expectError && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if len(entries) != tt.entryCount {
				t.Errorf("expected %d entries, got %d", tt.entryCount, len(entries))
			}
		})
	}
}

func TestRevertFromBackup(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewRevertManager()

	// Create original file
	originalPath := filepath.Join(tempDir, "original.txt")
	err := os.WriteFile(originalPath, []byte("modified content"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// Create backup file
	backupPath := filepath.Join(tempDir, "original.txt.backup")
	err = os.WriteFile(backupPath, []byte("original content"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// Test restore from backup
	err = manager.restoreFromBackup(originalPath, backupPath)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Verify content was restored
	content, err := os.ReadFile(originalPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "original content" {
		t.Errorf("expected 'original content', got %q", string(content))
	}

	// Test with non-existent backup
	err = manager.restoreFromBackup(originalPath, filepath.Join(tempDir, "nonexistent.backup"))
	if err == nil {
		t.Error("expected error for non-existent backup")
	}
}

func TestReverseReplacements(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewRevertManager()

	// Create test file
	filePath := filepath.Join(tempDir, "test.txt")
	content := "This is new content with new words"
	err := os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// Create log entry with replacements
	entry := LogEntry{
		FilePath: filePath,
		Modified: true,
		Replacements: []replacement.Replacement{
			{From: "old", To: "new"},
		},
	}

	// Apply reverse replacements
	err = manager.reverseReplacements(entry)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Verify content was reverted
	revertedContent, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatal(err)
	}
	expected := "This is old content with old words"
	if string(revertedContent) != expected {
		t.Errorf("expected %q, got %q", expected, string(revertedContent))
	}
}

func TestRevertFromLogIntegration(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewRevertManager()

	// Create test files
	file1Path := filepath.Join(tempDir, "file1.txt")
	file2Path := filepath.Join(tempDir, "file2.txt")

	err := os.WriteFile(file1Path, []byte("modified content"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile(file2Path, []byte("another modified file"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// Create backup for file1
	backupPath := filepath.Join(tempDir, "file1.txt.backup")
	err = os.WriteFile(backupPath, []byte("original content"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// Create JSON log
	logPath := filepath.Join(tempDir, "test.log")
	logData := struct {
		Summary interface{} `json:"summary"`
		Entries []LogEntry  `json:"entries"`
	}{
		Summary: map[string]interface{}{"total_files": 2},
		Entries: []LogEntry{
			{
				FilePath:   file1Path,
				Modified:   true,
				BackupPath: backupPath,
				Replacements: []replacement.Replacement{
					{From: "original", To: "modified"},
				},
			},
			{
				FilePath: file2Path,
				Modified: true,
				Replacements: []replacement.Replacement{
					{From: "original", To: "modified"},
				},
			},
		},
	}

	logBytes, err := json.Marshal(logData)
	if err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile(logPath, logBytes, 0644)
	if err != nil {
		t.Fatal(err)
	}

	// Test revert operation
	err = manager.RevertFromLog(logPath)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Verify file1 was restored from backup
	content1, err := os.ReadFile(file1Path)
	if err != nil {
		t.Fatal(err)
	}
	if string(content1) != "original content" {
		t.Errorf("file1 not restored correctly: expected 'original content', got %q", string(content1))
	}

	// Verify file2 had reverse replacements applied
	content2, err := os.ReadFile(file2Path)
	if err != nil {
		t.Fatal(err)
	}
	expected2 := "another original file"
	if string(content2) != expected2 {
		t.Errorf("file2 not reverted correctly: expected %q, got %q", expected2, string(content2))
	}
}

func TestRevertErrorHandling(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewRevertManager()

	// Test with non-readable file
	unreadablePath := filepath.Join(tempDir, "unreadable.txt")
	err := os.WriteFile(unreadablePath, []byte("content"), 0000) // No read permissions
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(unreadablePath, 0644) // Cleanup

	entry := LogEntry{
		FilePath: unreadablePath,
		Modified: true,
		Replacements: []replacement.Replacement{
			{From: "old", To: "new"},
		},
	}

	err = manager.reverseReplacements(entry)
	if err == nil {
		t.Error("expected error for unreadable file")
	}
}

func TestBackupFilePermissions(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewBackupManager(true)

	originalFile, err := os.CreateTemp(tempDir, "test*.txt")
	if err != nil {
		t.Fatal(err)
	}
	originalPath := originalFile.Name()
	originalFile.Close()

	// Set specific permissions
	testMode := os.FileMode(0644)
	err = os.Chmod(originalPath, testMode)
	if err != nil {
		t.Fatal(err)
	}

	backupPath, err := manager.BackupFile(originalPath)
	if err != nil {
		t.Fatal(err)
	}

	if backupPath != "" {
		backupInfo, err := os.Stat(backupPath)
		if err != nil {
			t.Fatal(err)
		}

		if backupInfo.Mode().Perm() != testMode.Perm() {
			t.Errorf("backup file permissions mismatch: expected %v, got %v",
				testMode.Perm(), backupInfo.Mode().Perm())
		}
	}
}

func TestBackupFileEdgeCases(t *testing.T) {
	tempDir := t.TempDir()

	tests := []struct {
		name        string
		enabled     bool
		fileSize    int64
		expectError bool
	}{
		{
			name:        "backup empty file",
			enabled:     true,
			fileSize:    0,
			expectError: false,
		},
		{
			name:        "backup large file content",
			enabled:     true,
			fileSize:    1024,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewBackupManager(tt.enabled)

			file, err := os.CreateTemp(tempDir, "test*.txt")
			if err != nil {
				t.Fatal(err)
			}
			filePath := file.Name()

			// Write data of specified size
			if tt.fileSize > 0 {
				data := make([]byte, tt.fileSize)
				for i := range data {
					data[i] = byte('A' + (i % 26))
				}
				_, err = file.Write(data)
				if err != nil {
					t.Fatal(err)
				}
			}
			file.Close()

			backupPath, err := manager.BackupFile(filePath)

			if tt.expectError && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if !tt.expectError && backupPath != "" {
				backupInfo, err := os.Stat(backupPath)
				if err != nil {
					t.Fatal(err)
				}
				if backupInfo.Size() != tt.fileSize {
					t.Errorf("backup file size mismatch: expected %d, got %d",
						tt.fileSize, backupInfo.Size())
				}
			}
		})
	}
}
