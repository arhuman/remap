// Package log provides structured logging and reporting for remap operations.
// It supports multiple output formats (JSON, CSV, summary) and tracks detailed
// operation statistics, enabling comprehensive audit trails and progress reporting.
package log

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"remap/internal/concurrent"
	"remap/internal/config"
	"remap/internal/replacement"
)

// Entry represents a single file processing operation with complete context.
// This structure captures all relevant information about file processing,
// including replacements made, backup paths, and errors, enabling comprehensive
// audit trails and operation analysis.
type Entry struct {
	Timestamp    string                    `json:"timestamp"`
	FilePath     string                    `json:"file_path"`
	OriginalSize int64                     `json:"original_size"`
	NewSize      int64                     `json:"new_size"`
	Modified     bool                      `json:"modified"`
	Replacements []replacement.Replacement `json:"replacements,omitempty"`
	BackupPath   string                    `json:"backup_path,omitempty"`
	Error        string                    `json:"error,omitempty"`
}

// Summary provides aggregate statistics for the entire remap operation.
// This structure enables quick assessment of operation success and provides
// metrics for performance analysis and reporting purposes.
type Summary struct {
	TotalFiles        int           `json:"total_files"`
	ModifiedFiles     int           `json:"modified_files"`
	TotalReplacements int           `json:"total_replacements"`
	ErrorCount        int           `json:"error_count"`
	ProcessingTime    time.Duration `json:"processing_time"`
	DryRun            bool          `json:"dry_run"`
}

// Logger manages operation logging and reporting with configurable output formats.
// It maintains both individual entry records and aggregate statistics, supporting
// real-time progress updates and final comprehensive reports.
type Logger struct {
	config  *config.Config
	writer  io.Writer
	entries []Entry
	summary Summary
}

// NewLogger creates a Logger with the specified configuration and output destination.
// This constructor handles output file creation when needed and initializes
// the logging system with proper format support and error handling.
func NewLogger(cfg *config.Config) (*Logger, error) {
	var writer io.Writer = os.Stdout

	if cfg.LogFile != "" {
		file, err := os.Create(cfg.LogFile)
		if err != nil {
			return nil, fmt.Errorf("failed to create log file %s: %w", cfg.LogFile, err)
		}
		writer = file
	}

	return &Logger{
		config:  cfg,
		writer:  writer,
		entries: []Entry{},
		summary: Summary{
			DryRun: cfg.DryRun,
		},
	}, nil
}

// LogResult records the outcome of a file processing operation.
// This method handles both successful and failed operations, maintaining
// comprehensive statistics and supporting real-time progress reporting.
func (l *Logger) LogResult(result concurrent.ProcessResult) {
	entry := Entry{
		Timestamp:  time.Now().Format(time.RFC3339),
		FilePath:   result.Job.FilePath,
		BackupPath: result.BackupPath,
	}

	if result.Error != nil {
		entry.Error = result.Error.Error()
		l.summary.ErrorCount++
	} else if result.Result != nil {
		entry.OriginalSize = result.Result.OriginalSize
		entry.NewSize = result.Result.NewSize
		entry.Modified = result.Result.Modified
		entry.Replacements = result.Result.Replacements

		if result.Result.Modified {
			l.summary.ModifiedFiles++
			l.summary.TotalReplacements += len(result.Result.Replacements)
		}
	}

	l.entries = append(l.entries, entry)
	l.summary.TotalFiles++

	if l.config.IsVerbose() {
		l.logVerbose(entry)
	} else if l.config.ShouldLog() && entry.Modified {
		l.logBasic(entry)
	}
}

// SetProcessingTime records the total operation duration for reporting.
// This method enables performance analysis and helps users understand
// the time cost of large-scale string replacement operations.
func (l *Logger) SetProcessingTime(duration time.Duration) {
	l.summary.ProcessingTime = duration
}

// WriteReport generates the final operation report in the configured format.
// This method supports multiple output formats and provides comprehensive
// operation summaries with detailed statistics and error information.
func (l *Logger) WriteReport() error {
	if l.config.Quiet {
		return nil
	}

	switch l.config.LogFormat {
	case config.LogFormatJSON:
		return l.writeJSONReport()
	case config.LogFormatCSV:
		return l.writeCSVReport()
	default:
		return l.writeSummaryReport()
	}
}

func (l *Logger) logVerbose(entry Entry) {
	if entry.Error != "" {
		fmt.Fprintf(l.writer, "ERROR: %s - %s\n", entry.FilePath, entry.Error)
		return
	}

	if entry.Modified {
		fmt.Fprintf(l.writer, "MODIFIED: %s (%d replacements)\n", entry.FilePath, len(entry.Replacements))
		if l.config.IsDebug() {
			for _, replacement := range entry.Replacements {
				fmt.Fprintf(l.writer, "  Line %d:%d: '%s' -> '%s'\n",
					replacement.Line, replacement.Column, replacement.From, replacement.To)
			}
		}
	} else {
		fmt.Fprintf(l.writer, "SKIPPED: %s (no changes)\n", entry.FilePath)
	}
}

func (l *Logger) logBasic(entry Entry) {
	//if entry.Modified {
	//	fmt.Fprintf(l.writer, "%s (%d replacements)\n", entry.FilePath, len(entry.Replacements))
	//}
}

func (l *Logger) writeJSONReport() error {
	report := struct {
		Summary Summary `json:"summary"`
		Entries []Entry `json:"entries"`
	}{
		Summary: l.summary,
		Entries: l.entries,
	}

	encoder := json.NewEncoder(l.writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}

func (l *Logger) writeCSVReport() error {
	mode := "production"
	if l.summary.DryRun {
		mode = "dry-run"
	}

	writer := csv.NewWriter(l.writer)
	defer writer.Flush()

	header := []string{
		"file_path", "old_string", "new_string", "line", "column",
	}
	if err := writer.Write(header); err != nil {
		return err
	}

	// Write all CSV records first
	for _, entry := range l.entries {
		for _, repl := range entry.Replacements {
			record := []string{
				entry.FilePath,
				repl.From,
				repl.To,
				fmt.Sprintf("%d", repl.Line),
				fmt.Sprintf("%d", repl.Column),
			}
			if err := writer.Write(record); err != nil {
				return err
			}
		}
	}

	// Flush CSV writer and then write the statistics report at the end
	writer.Flush()
	fmt.Fprintf(l.writer, "# Remap CSV Report (%s)\n", mode)
	fmt.Fprintf(l.writer, "# Total files processed: %d\n", l.summary.TotalFiles)
	fmt.Fprintf(l.writer, "# Files modified: %d\n", l.summary.ModifiedFiles)
	fmt.Fprintf(l.writer, "# Total replacements: %d\n", l.summary.TotalReplacements)
	fmt.Fprintf(l.writer, "# Errors: %d\n", l.summary.ErrorCount)
	fmt.Fprintf(l.writer, "# Processing time: %v\n", l.summary.ProcessingTime)
	fmt.Fprintf(l.writer, "#\n")

	return nil
}

func (l *Logger) writeSummaryReport() error {
	mode := "production"
	if l.summary.DryRun {
		mode = "dry-run"
	}

	fmt.Fprintf(l.writer, "\n=== Remap Summary (%s) ===\n", mode)
	fmt.Fprintf(l.writer, "Total files processed: %d\n", l.summary.TotalFiles)
	fmt.Fprintf(l.writer, "Files modified: %d\n", l.summary.ModifiedFiles)
	fmt.Fprintf(l.writer, "Total replacements: %d\n", l.summary.TotalReplacements)
	fmt.Fprintf(l.writer, "Errors: %d\n", l.summary.ErrorCount)
	fmt.Fprintf(l.writer, "Processing time: %v\n", l.summary.ProcessingTime)

	if l.summary.ErrorCount > 0 {
		fmt.Fprintf(l.writer, "\nErrors encountered:\n")
		for _, entry := range l.entries {
			if entry.Error != "" {
				fmt.Fprintf(l.writer, "  %s: %s\n", entry.FilePath, entry.Error)
			}
		}
	}

	return nil
}

// Close releases any resources held by the logger, including output files.
// This method ensures proper cleanup of file handles and should be called
// when logging operations are complete to prevent resource leaks.
// Note: os.Stdout is never closed to prevent interfering with coverage tools.
func (l *Logger) Close() error {
	if closer, ok := l.writer.(io.Closer); ok && l.writer != os.Stdout {
		return closer.Close()
	}
	return nil
}
