// Package concurrent provides parallel file processing capabilities.
// It implements a worker pool pattern with proper synchronization
// to safely process multiple files concurrently while managing resources.
package concurrent

import (
	"context"
	"os"
	"runtime"
	"strings"
	"sync"

	"remap/internal/backup"
	"remap/internal/config"
	"remap/internal/errors"
	"remap/internal/filter"
	"remap/internal/parser"
	"remap/internal/replacement"
)

// ProcessJob represents a single file processing task.
// This structure encapsulates all information needed for a worker
// to process a file independently without shared state dependencies.
type ProcessJob struct {
	FilePath string
	FileInfo filter.FileInfo
}

// ProcessResult contains the complete result of processing a single file.
// This structure provides comprehensive information about the processing
// outcome, enabling detailed reporting and error handling.
type ProcessResult struct {
	Job        ProcessJob
	Result     *replacement.FileResult
	BackupPath string
	Error      error
}

// Processor orchestrates concurrent file processing operations.
// It implements a worker pool pattern that scales with available CPU cores
// while managing shared resources safely across goroutines.
type Processor struct {
	config        *config.Config
	mappings      *parser.MappingTable
	engine        *replacement.Engine
	backupManager *backup.Manager
	workerCount   int
}

// NewProcessor creates a Processor with optimal worker pool sizing.
// This constructor automatically determines the ideal number of workers
// based on CPU cores while capping it to prevent resource contention.
func NewProcessor(cfg *config.Config, mappings *parser.MappingTable) *Processor {
	workerCount := runtime.NumCPU()
	if workerCount > 8 {
		workerCount = 8
	}

	return &Processor{
		config:        cfg,
		mappings:      mappings,
		engine:        replacement.NewEngine(cfg),
		backupManager: backup.NewBackupManager(cfg.ShouldCreateBackup()),
		workerCount:   workerCount,
	}
}

// ProcessFiles processes multiple files concurrently using a worker pool.
// This method coordinates parallel file processing with proper cancellation
// support and resource cleanup, returning results through a channel.
func (p *Processor) ProcessFiles(ctx context.Context, files []filter.FileInfo) (<-chan ProcessResult, error) {
	jobs := make(chan ProcessJob, len(files))
	results := make(chan ProcessResult, len(files))

	var wg sync.WaitGroup

	for i := 0; i < p.workerCount; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			p.worker(ctx, workerID, jobs, results)
		}(i)
	}

	go func() {
		defer close(jobs)
		for _, fileInfo := range files {
			select {
			case jobs <- ProcessJob{FilePath: fileInfo.Path, FileInfo: fileInfo}:
			case <-ctx.Done():
				return
			}
		}
	}()

	go func() {
		wg.Wait()
		close(results)
	}()

	return results, nil
}

func (p *Processor) worker(ctx context.Context, workerID int, jobs <-chan ProcessJob, results chan<- ProcessResult) {
	for {
		select {
		case job, ok := <-jobs:
			if !ok {
				return
			}
			result := p.processFile(job)
			select {
			case results <- result:
			case <-ctx.Done():
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

func (p *Processor) processFile(job ProcessJob) ProcessResult {
	result := ProcessResult{
		Job: job,
	}

	content, err := os.ReadFile(job.FilePath)
	if err != nil {
		result.Error = errors.WrapFileError(job.FilePath, err)
		return result
	}

	replacementResult := p.engine.ProcessFile(job.FilePath, content, p.mappings)
	result.Result = replacementResult

	if !replacementResult.Modified {
		return result
	}

	if p.config.ShouldCreateBackup() {
		backupPath, backupErr := p.backupManager.BackupFile(job.FilePath)
		if backupErr != nil {
			result.Error = backupErr
			return result
		}
		result.BackupPath = backupPath
	}

	if !p.config.DryRun {
		err = p.writeFile(job.FilePath, result.Result.Replacements, content)
		if err != nil {
			if result.BackupPath != "" {
				_ = p.backupManager.RestoreFile(job.FilePath, result.BackupPath)
			}
			result.Error = err
			return result
		}
	}

	return result
}

func (p *Processor) writeFile(filePath string, _ []replacement.Replacement, originalContent []byte) error {
	info, err := os.Stat(filePath)
	if err != nil {
		return errors.WrapFileError(filePath, err)
	}

	tempFile := filePath + ".tmp"

	file, err := os.Create(tempFile)
	if err != nil {
		return errors.NewFileNotWritableError(filePath, err)
	}
	defer file.Close()
	defer os.Remove(tempFile)

	content := string(originalContent)

	for _, mapping := range p.mappings.GetSortedMappings() {
		if p.config.CaseSensitive {
			content = replaceAll(content, mapping.From, mapping.To)
		} else {
			content = caseInsensitiveReplaceAll(content, mapping.From, mapping.To)
		}
	}

	_, err = file.WriteString(content)
	if err != nil {
		return errors.NewFileNotWritableError(filePath, err)
	}

	err = file.Sync()
	if err != nil {
		return errors.NewFileNotWritableError(filePath, err)
	}

	_ = file.Close()

	err = os.Chmod(tempFile, info.Mode())
	if err != nil {
		return errors.NewFileNotWritableError(filePath, err)
	}

	err = os.Rename(tempFile, filePath)
	if err != nil {
		return errors.NewFileNotWritableError(filePath, err)
	}

	return nil
}

func replaceAll(content, from, to string) string {
	if from == "" {
		return content
	}

	var result strings.Builder
	start := 0

	for start < len(content) {
		index := findStringFrom(content, from, start)
		if index == -1 {
			// No more matches, append the rest
			result.WriteString(content[start:])
			break
		}

		// Append content before the match
		result.WriteString(content[start:index])
		// Append the replacement
		result.WriteString(to)
		// Move past this match to prevent re-processing
		start = index + len(from)
	}

	return result.String()
}

func replaceFirst(content, from, to string) string {
	index := findString(content, from)
	if index == -1 {
		return content
	}
	return content[:index] + to + content[index+len(from):]
}

func findStringFrom(content, search string, start int) int {
	for i := start; i <= len(content)-len(search); i++ {
		if content[i:i+len(search)] == search {
			return i
		}
	}
	return -1
}

func findString(content, search string) int {
	for i := 0; i <= len(content)-len(search); i++ {
		if content[i:i+len(search)] == search {
			return i
		}
	}
	return -1
}

func caseInsensitiveReplaceAll(content, from, to string) string {
	if from == "" {
		return content
	}

	var result strings.Builder
	start := 0

	for start < len(content) {
		index := caseInsensitiveFindStringFrom(content, from, start)
		if index == -1 {
			// No more matches, append the rest
			result.WriteString(content[start:])
			break
		}

		// Append content before the match
		result.WriteString(content[start:index])
		// Append the replacement
		result.WriteString(to)
		// Move past this match to prevent re-processing
		start = index + len(from)
	}

	return result.String()
}

func caseInsensitiveReplaceFirst(content, from, to string) string {
	index := caseInsensitiveFindString(content, from)
	if index == -1 {
		return content
	}
	return content[:index] + to + content[index+len(from):]
}

func caseInsensitiveFindString(content, search string) int {
	return caseInsensitiveFindStringFrom(content, search, 0)
}

func caseInsensitiveFindStringFrom(content, search string, start int) int {
	contentLower := toLower(content)
	searchLower := toLower(search)

	for i := start; i <= len(contentLower)-len(searchLower); i++ {
		if contentLower[i:i+len(searchLower)] == searchLower {
			return i
		}
	}
	return -1
}

func toLower(s string) string {
	result := make([]byte, len(s))
	for i, b := range []byte(s) {
		if b >= 'A' && b <= 'Z' {
			result[i] = b + 32
		} else {
			result[i] = b
		}
	}
	return string(result)
}
