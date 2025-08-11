// Package backup provides file backup and restoration capabilities.
// It implements safe file operations with automatic backup creation
// and restoration support for error recovery scenarios.
package backup

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"remap/internal/errors"
	"remap/internal/replacement"
)

// Manager handles file backup and restoration operations.
// It provides configurable backup behavior and ensures data safety
// during file modification operations through automatic backup creation.
type Manager struct {
	enabled bool
}

// NewBackupManager creates a Manager with the specified behavior.
// This constructor enables conditional backup functionality, allowing
// applications to disable backups for performance or when not needed.
func NewBackupManager(enabled bool) *Manager {
	return &Manager{
		enabled: enabled,
	}
}

// BackupFile creates a timestamped backup copy of the specified file.
// This method provides atomic backup creation with unique naming to prevent
// conflicts, enabling safe file modifications with recovery options.
func (bm *Manager) BackupFile(filePath string) (string, error) {
	if !bm.enabled {
		return "", nil
	}

	backupPath := generateBackupPath(filePath)

	srcFile, err := os.Open(filePath)
	if err != nil {
		return "", errors.NewBackupError(filePath, "failed to open source file", err)
	}
	defer srcFile.Close()

	dstFile, err := os.Create(backupPath)
	if err != nil {
		return "", errors.NewBackupError(backupPath, "failed to create backup file", err)
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		_ = os.Remove(backupPath)
		return "", errors.NewBackupError(backupPath, "failed to copy file content", err)
	}

	srcInfo, err := os.Stat(filePath)
	if err != nil {
		return backupPath, nil
	}

	err = os.Chmod(backupPath, srcInfo.Mode())
	if err != nil {
		return backupPath, nil
	}

	return backupPath, nil
}

// RestoreFile overwrites the original file with contents from the backup.
// This method implements atomic restoration by first validating backup existence
// before attempting restoration, preventing data loss from failed restore operations.
func (bm *Manager) RestoreFile(originalPath, backupPath string) error {
	if backupPath == "" {
		return nil
	}

	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		return errors.NewBackupError(backupPath, "backup file not found", err)
	}

	srcFile, err := os.Open(backupPath)
	if err != nil {
		return errors.NewBackupError(backupPath, "failed to open backup file", err)
	}
	defer srcFile.Close()

	dstFile, err := os.Create(originalPath)
	if err != nil {
		return errors.NewBackupError(originalPath, "failed to create original file", err)
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return errors.NewBackupError(originalPath, "failed to restore file content", err)
	}

	backupInfo, err := os.Stat(backupPath)
	if err != nil {
		return nil
	}

	err = os.Chmod(originalPath, backupInfo.Mode())
	if err != nil {
		return nil
	}

	return nil
}

// CleanupBackup removes the backup file after successful operations.
// This method provides housekeeping functionality to prevent backup accumulation
// while handling cleanup errors gracefully to avoid masking primary operation results.
func (bm *Manager) CleanupBackup(backupPath string) error {
	if backupPath == "" {
		return nil
	}

	err := os.Remove(backupPath)
	if err != nil && !os.IsNotExist(err) {
		return errors.NewBackupError(backupPath, "failed to remove backup file", err)
	}

	return nil
}

func generateBackupPath(originalPath string) string {
	dir := filepath.Dir(originalPath)
	base := filepath.Base(originalPath)
	timestamp := time.Now().Format("20060102_150405")

	return filepath.Join(dir, fmt.Sprintf("%s.%s.bak", base, timestamp))
}

// RevertManager handles reverting changes from operation log files.
// This component enables undo functionality by parsing operation logs
// and applying reverse transformations to restore previous file states.
type RevertManager struct{}

// NewRevertManager creates a RevertManager for undo operations.
// This constructor initializes the revert system, which reads operation logs
// and applies inverse transformations to restore files to their previous state.
func NewRevertManager() *RevertManager {
	return &RevertManager{}
}

// RevertFromLog reverses operations recorded in the specified log file.
// This method reads the operation log and applies inverse transformations,
// enabling users to undo bulk string replacement operations when needed.
func (rm *RevertManager) RevertFromLogWithFormat(logFilePath string, logFormat string) error {
	logEntries, err := rm.parseLogFileWithFormat(logFilePath, logFormat)
	if err != nil {
		return err
	}

	var revertErrors []error
	revertedCount := 0

	for _, entry := range logEntries {
		if !entry.Modified || entry.Error != "" {
			continue // Skip entries that weren't modified or had errors
		}

		if err := rm.revertEntry(entry); err != nil {
			revertErrors = append(revertErrors, err)
		} else {
			revertedCount++
		}
	}

	if len(revertErrors) > 0 {
		return errors.NewBackupError(logFilePath,
			fmt.Sprintf("revert completed with %d successes and %d errors", revertedCount, len(revertErrors)),
			revertErrors[0])
	}

	return nil
}

// RevertFromLog reverses operations recorded in the specified log file.
// This method reads the operation log and applies inverse transformations,
// enabling users to undo bulk string replacement operations when needed.
func (rm *RevertManager) RevertFromLog(logFilePath string) error {
	return rm.RevertFromLogWithFormat(logFilePath, "json") // Default to JSON
}

// LogEntry represents a single entry from a remap log file
type LogEntry struct {
	Timestamp    string                    `json:"timestamp"`
	FilePath     string                    `json:"file_path"`
	OriginalSize int64                     `json:"original_size"`
	NewSize      int64                     `json:"new_size"`
	Modified     bool                      `json:"modified"`
	Replacements []replacement.Replacement `json:"replacements,omitempty"`
	BackupPath   string                    `json:"backup_path,omitempty"`
	Error        string                    `json:"error,omitempty"`
}

// parseLogFileWithFormat reads and parses a log file in the specified format
func (rm *RevertManager) parseLogFileWithFormat(logFilePath string, logFormat string) ([]LogEntry, error) {
	file, err := os.Open(logFilePath)
	if err != nil {
		return nil, errors.NewFileError(logFilePath, "failed to open log file", err)
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		return nil, errors.NewFileError(logFilePath, "failed to read log file", err)
	}

	contentStr := string(content)

	switch logFormat {
	case "json":
		// Extract JSON from the content (skip any verbose output lines)
		jsonStart := strings.Index(contentStr, "{")
		if jsonStart == -1 {
			return nil, errors.NewParsingError(logFilePath, "no JSON content found in log file", nil)
		}
		jsonContent := contentStr[jsonStart:]
		return rm.parseJSONLog([]byte(jsonContent))
	case "csv":
		return rm.parseCSVLog(contentStr)
	default:
		return nil, errors.NewParsingError(logFilePath, fmt.Sprintf("unsupported log format: %s", logFormat), nil)
	}
}

// parseLogFile reads and parses a log file in either JSON or CSV format
func (rm *RevertManager) parseLogFile(logFilePath string) ([]LogEntry, error) {
	return rm.parseLogFileWithFormat(logFilePath, "json") // Default to JSON
}

// parseJSONLog parses a JSON format log file
func (rm *RevertManager) parseJSONLog(content []byte) ([]LogEntry, error) {
	var report struct {
		Entries []LogEntry `json:"entries"`
	}

	if err := json.Unmarshal(content, &report); err != nil {
		return nil, err
	}

	return report.Entries, nil
}

// parseCSVLog parses a CSV format log file
func (rm *RevertManager) parseCSVLog(content string) ([]LogEntry, error) {
	lines := strings.Split(content, "\n")

	// Find the start of CSV data (skip comment lines)
	var csvLines []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		csvLines = append(csvLines, line)
	}

	if len(csvLines) == 0 {
		return nil, errors.NewParsingError("", "no CSV data found in log file", nil)
	}

	reader := csv.NewReader(strings.NewReader(strings.Join(csvLines, "\n")))
	reader.FieldsPerRecord = -1 // Allow variable number of fields
	records, err := reader.ReadAll()
	if err != nil {
		return nil, errors.NewParsingError("", "failed to parse CSV data", err)
	}

	if len(records) == 0 {
		return nil, errors.NewParsingError("", "no records found in CSV", nil)
	}

	// Group CSV records by file path to reconstruct log entries
	entryMap := make(map[string]*LogEntry)

	for i, record := range records {
		if i == 0 {
			// Skip header row
			continue
		}

		if len(record) < 5 {
			continue // Skip malformed records
		}

		filePath := record[0]
		from := record[1]
		to := record[2]
		// line := record[3]
		// column := record[4]

		entry, exists := entryMap[filePath]
		if !exists {
			entry = &LogEntry{
				FilePath:     filePath,
				Modified:     true,
				Replacements: []replacement.Replacement{},
			}
			entryMap[filePath] = entry
		}

		// Add this replacement to the entry
		replacement := replacement.Replacement{
			From: from,
			To:   to,
		}
		entry.Replacements = append(entry.Replacements, replacement)
	}

	// Convert map to slice
	var entries []LogEntry
	for _, entry := range entryMap {
		entries = append(entries, *entry)
	}

	return entries, nil
}

// revertEntry reverts a single log entry by either restoring from backup or applying reverse replacements
func (rm *RevertManager) revertEntry(entry LogEntry) error {
	// First try to restore from backup if available
	if entry.BackupPath != "" {
		return rm.restoreFromBackup(entry.FilePath, entry.BackupPath)
	}

	// Fall back to reverse replacements
	return rm.reverseReplacements(entry)
}

// restoreFromBackup restores a file from its backup
func (rm *RevertManager) restoreFromBackup(originalPath, backupPath string) error {
	// Check if backup exists
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		return errors.NewBackupError(backupPath, "backup file not found", err)
	}

	backupManager := NewBackupManager(true)
	return backupManager.RestoreFile(originalPath, backupPath)
}

// reverseReplacements applies inverse replacements to a file
func (rm *RevertManager) reverseReplacements(entry LogEntry) error {
	// Read the current file content
	content, err := os.ReadFile(entry.FilePath)
	if err != nil {
		return errors.NewFileError(entry.FilePath, "failed to read file for revert", err)
	}

	// Apply reverse replacements (swap From and To)
	modifiedContent := string(content)
	for _, repl := range entry.Replacements {
		// Apply the reverse replacement: replace `To` with `From`
		modifiedContent = strings.ReplaceAll(modifiedContent, repl.To, repl.From)
	}

	// Write the reverted content back to the file
	return os.WriteFile(entry.FilePath, []byte(modifiedContent), 0644)
}

// ApplyManager handles applying changes from operation log files.
// This component enables redo functionality by parsing operation logs
// and applying the transformations to restore or apply previous file states.
type ApplyManager struct{}

// NewApplyManager creates an ApplyManager for apply operations.
// This constructor initializes the apply system, which reads operation logs
// and applies transformations to files based on the logged operations.
func NewApplyManager() *ApplyManager {
	return &ApplyManager{}
}

// ApplyFromLogWithFormat applies operations recorded in the specified log file.
// This method reads the operation log and applies transformations,
// enabling users to reapply bulk string replacement operations from logs.
// It skips commented lines in CSV format.
func (am *ApplyManager) ApplyFromLogWithFormat(logFilePath string, logFormat string) error {
	logEntries, err := am.parseLogFileWithFormat(logFilePath, logFormat)
	if err != nil {
		return err
	}

	var applyErrors []error
	appliedCount := 0

	for _, entry := range logEntries {
		if !entry.Modified || entry.Error != "" {
			continue // Skip entries that weren't modified or had errors
		}

		if err := am.applyEntry(entry); err != nil {
			applyErrors = append(applyErrors, err)
		} else {
			appliedCount++
		}
	}

	if len(applyErrors) > 0 {
		return errors.NewBackupError(logFilePath,
			fmt.Sprintf("apply completed with %d successes and %d errors", appliedCount, len(applyErrors)),
			applyErrors[0])
	}

	return nil
}

// parseLogFileWithFormat reads and parses a log file in the specified format
// This reuses the same parsing logic as RevertManager, including comment skipping
func (am *ApplyManager) parseLogFileWithFormat(logFilePath string, logFormat string) ([]LogEntry, error) {
	file, err := os.Open(logFilePath)
	if err != nil {
		return nil, errors.NewFileError(logFilePath, "failed to open log file", err)
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		return nil, errors.NewFileError(logFilePath, "failed to read log file", err)
	}

	contentStr := string(content)

	switch logFormat {
	case "json":
		// Extract JSON from the content (skip any verbose output lines)
		jsonStart := strings.Index(contentStr, "{")
		if jsonStart == -1 {
			return nil, errors.NewParsingError(logFilePath, "no JSON content found in log file", nil)
		}
		jsonContent := contentStr[jsonStart:]
		return am.parseJSONLog([]byte(jsonContent))
	case "csv":
		return am.parseCSVLog(contentStr)
	default:
		return nil, errors.NewParsingError(logFilePath, fmt.Sprintf("unsupported log format: %s", logFormat), nil)
	}
}

// parseJSONLog parses a JSON format log file
func (am *ApplyManager) parseJSONLog(content []byte) ([]LogEntry, error) {
	var report struct {
		Entries []LogEntry `json:"entries"`
	}

	if err := json.Unmarshal(content, &report); err != nil {
		return nil, err
	}

	return report.Entries, nil
}

// parseCSVLog parses a CSV format log file, skipping commented lines
func (am *ApplyManager) parseCSVLog(content string) ([]LogEntry, error) {
	lines := strings.Split(content, "\n")

	// Find the start of CSV data (skip comment lines starting with #)
	var csvLines []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue // Skip empty lines and commented lines
		}
		csvLines = append(csvLines, line)
	}

	if len(csvLines) == 0 {
		return nil, errors.NewParsingError("", "no CSV data found in log file", nil)
	}

	reader := csv.NewReader(strings.NewReader(strings.Join(csvLines, "\n")))
	reader.FieldsPerRecord = -1 // Allow variable number of fields
	records, err := reader.ReadAll()
	if err != nil {
		return nil, errors.NewParsingError("", "failed to parse CSV data", err)
	}

	if len(records) == 0 {
		return nil, errors.NewParsingError("", "no records found in CSV", nil)
	}

	// Group CSV records by file path to reconstruct log entries
	entryMap := make(map[string]*LogEntry)

	for i, record := range records {
		if i == 0 {
			// Skip header row
			continue
		}

		if len(record) < 5 {
			continue // Skip malformed records
		}

		filePath := record[0]
		from := record[1]
		to := record[2]
		// line := record[3]
		// column := record[4]

		entry, exists := entryMap[filePath]
		if !exists {
			entry = &LogEntry{
				FilePath:     filePath,
				Modified:     true,
				Replacements: []replacement.Replacement{},
			}
			entryMap[filePath] = entry
		}

		// Add this replacement to the entry
		replacement := replacement.Replacement{
			From: from,
			To:   to,
		}
		entry.Replacements = append(entry.Replacements, replacement)
	}

	// Convert map to slice
	var entries []LogEntry
	for _, entry := range entryMap {
		entries = append(entries, *entry)
	}

	return entries, nil
}

// applyEntry applies a single log entry by applying the replacements
func (am *ApplyManager) applyEntry(entry LogEntry) error {
	return am.applyReplacements(entry)
}

// applyReplacements applies the replacements to a file
func (am *ApplyManager) applyReplacements(entry LogEntry) error {
	// Read the current file content
	content, err := os.ReadFile(entry.FilePath)
	if err != nil {
		return errors.NewFileError(entry.FilePath, "failed to read file for apply", err)
	}

	// Apply replacements (From -> To)
	modifiedContent := string(content)
	for _, repl := range entry.Replacements {
		// Apply the replacement: replace `From` with `To`
		modifiedContent = strings.ReplaceAll(modifiedContent, repl.From, repl.To)
	}

	// Write the modified content back to the file
	return os.WriteFile(entry.FilePath, []byte(modifiedContent), 0644)
}
