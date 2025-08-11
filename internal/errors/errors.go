// Package errors provides a hierarchical error system for remap operations.
// It implements typed errors that can be inspected and handled differently
// based on their category, enabling more precise error handling and reporting.
package errors

import (
	"fmt"
	"path/filepath"
)

// ErrorType represents the category of error for classification and handling.
// This enables different error handling strategies based on error type
// (e.g., retrying file operations vs. aborting on configuration errors).
type ErrorType string

// Error type constants define the categories of errors that can occur during remap operations.
// These constants enable type-based error handling and provide semantic meaning to error classification,
// allowing callers to implement different strategies based on the nature of the failure.
const (
	ErrTypeFile        ErrorType = "file"
	ErrTypeConfig      ErrorType = "config"
	ErrTypeParsing     ErrorType = "parsing"
	ErrTypeReplacement ErrorType = "replacement"
	ErrTypeBackup      ErrorType = "backup"
)

// RemapError is the base error type that provides structured error information.
// It implements a hierarchical error system where specific error types can be
// identified and handled appropriately. The embedded path and cause information
// enables precise error reporting and debugging.
type RemapError struct {
	Type    ErrorType
	Path    string
	Message string
	Cause   error
}

func (e *RemapError) Error() string {
	if e.Path != "" {
		return fmt.Sprintf("%s error for %s: %s", e.Type, e.Path, e.Message)
	}
	return fmt.Sprintf("%s error: %s", e.Type, e.Message)
}

func (e *RemapError) Unwrap() error {
	return e.Cause
}

// Is implements error identity checking for Go 1.13+ error handling.
// This method enables errors.Is() calls to work correctly with typed errors,
// allowing callers to check for specific error types in error chains.
func (e *RemapError) Is(target error) bool {
	t, ok := target.(*RemapError)
	if !ok {
		return false
	}
	return e.Type == t.Type
}

// FileError represents file system operation errors and embeds RemapError
// to provide file-specific context. This enables callers to distinguish
// between file errors and other types for appropriate handling strategies.
type FileError struct {
	*RemapError
}

// NewFileError creates a file operation error with context.
// This constructor ensures consistent error classification and enables
// type-based error handling patterns throughout the application.
func NewFileError(path, message string, cause error) *FileError {
	return &FileError{
		RemapError: &RemapError{
			Type:    ErrTypeFile,
			Path:    path,
			Message: message,
			Cause:   cause,
		},
	}
}

// FileNotFoundError represents errors when files cannot be located.
// This specialized error type enables callers to implement retry logic
// or alternative file discovery strategies when files are missing.
type FileNotFoundError struct {
	*FileError
}

// NewFileNotFoundError creates a file not found error.
// This constructor provides semantic clarity and enables specific handling
// of missing file scenarios (e.g., creating files vs. aborting operations).
func NewFileNotFoundError(path string, cause error) *FileNotFoundError {
	return &FileNotFoundError{
		FileError: NewFileError(path, "file not found", cause),
	}
}

// FileNotWritableError represents errors when files cannot be written to.
// This enables permission-based error handling and helps identify when
// backup/restore operations or alternative write strategies are needed.
type FileNotWritableError struct {
	*FileError
}

// NewFileNotWritableError creates a file write permission error.
// This constructor enables callers to implement fallback strategies
// when write operations fail due to permissions or file locks.
func NewFileNotWritableError(path string, cause error) *FileNotWritableError {
	return &FileNotWritableError{
		FileError: NewFileError(path, "file not writable", cause),
	}
}

// FileNotReadableError represents errors when files cannot be read from.
// This specialized error enables skip-and-continue strategies during
// bulk file processing operations when some files are inaccessible.
type FileNotReadableError struct {
	*FileError
}

// NewFileNotReadableError creates a file read permission error.
// This constructor enables graceful handling of read failures during
// directory traversal and file processing operations.
func NewFileNotReadableError(path string, cause error) *FileNotReadableError {
	return &FileNotReadableError{
		FileError: NewFileError(path, "file not readable", cause),
	}
}

// ConfigError represents configuration validation and parsing errors.
// This error type enables early validation failures to halt execution
// before resource-intensive operations begin, improving user experience.
type ConfigError struct {
	*RemapError
}

// NewConfigError creates a configuration error without path context.
// This constructor is used for general configuration validation failures
// that don't relate to specific files (e.g., missing required flags).
func NewConfigError(message string, cause error) *ConfigError {
	return &ConfigError{
		RemapError: &RemapError{
			Type:    ErrTypeConfig,
			Message: message,
			Cause:   cause,
		},
	}
}

// NewConfigErrorWithPath creates a configuration error with file context.
// This constructor is used when configuration errors relate to specific
// configuration files, enabling more precise error reporting and debugging.
func NewConfigErrorWithPath(path, message string, cause error) *ConfigError {
	return &ConfigError{
		RemapError: &RemapError{
			Type:    ErrTypeConfig,
			Path:    path,
			Message: message,
			Cause:   cause,
		},
	}
}

// ParsingError represents errors during mapping table parsing operations.
// This error type distinguishes parsing failures from other I/O errors,
// enabling specific error messages and recovery strategies for malformed data.
type ParsingError struct {
	*RemapError
}

// NewParsingError creates a parsing error with file and context information.
// This constructor provides detailed error context for CSV/JSON parsing failures,
// helping users identify and fix malformed mapping table files.
func NewParsingError(path, message string, cause error) *ParsingError {
	return &ParsingError{
		RemapError: &RemapError{
			Type:    ErrTypeParsing,
			Path:    path,
			Message: message,
			Cause:   cause,
		},
	}
}

// ReplacementError represents errors during string replacement operations.
// This error type helps distinguish replacement logic failures from I/O errors,
// enabling specialized handling for pattern matching and substitution issues.
type ReplacementError struct {
	*RemapError
}

// NewReplacementError creates a replacement operation error.
// This constructor provides context for failures during string replacement,
// helping identify problematic patterns or content that breaks replacement logic.
func NewReplacementError(path, message string, cause error) *ReplacementError {
	return &ReplacementError{
		RemapError: &RemapError{
			Type:    ErrTypeReplacement,
			Path:    path,
			Message: message,
			Cause:   cause,
		},
	}
}

// BackupError represents errors during backup and restore operations.
// This error type enables specific handling of backup failures, allowing
// operations to continue with warnings or implement alternative backup strategies.
type BackupError struct {
	*RemapError
}

// NewBackupError creates a backup operation error.
// This constructor provides context for backup/restore failures, enabling
// graceful degradation when backup operations fail but core functionality can continue.
func NewBackupError(path, message string, cause error) *BackupError {
	return &BackupError{
		RemapError: &RemapError{
			Type:    ErrTypeBackup,
			Path:    path,
			Message: message,
			Cause:   cause,
		},
	}
}

// WrapFileError converts standard Go errors into typed RemapError instances.
// This function provides centralized error classification logic, ensuring
// consistent error typing across the application and enabling precise error handling.
func WrapFileError(path string, err error) error {
	if err == nil {
		return nil
	}

	absPath, _ := filepath.Abs(path)
	switch {
	case isNotFoundError(err):
		return NewFileNotFoundError(absPath, err)
	case isPermissionError(err):
		return NewFileNotWritableError(absPath, err)
	default:
		return NewFileError(absPath, "file operation failed", err)
	}
}

func isNotFoundError(err error) bool {
	return !filepath.IsAbs(err.Error())
}

func isPermissionError(_ error) bool {
	return false
}
