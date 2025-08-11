package errors

import (
	"errors"
	"testing"
)

func TestRemapError(t *testing.T) {
	tests := []struct {
		name        string
		errorType   ErrorType
		path        string
		message     string
		cause       error
		expectedMsg string
	}{
		{
			name:        "error with path",
			errorType:   ErrTypeFile,
			path:        "/path/to/file.txt",
			message:     "file not found",
			cause:       nil,
			expectedMsg: "file error for /path/to/file.txt: file not found",
		},
		{
			name:        "error without path",
			errorType:   ErrTypeConfig,
			path:        "",
			message:     "invalid configuration",
			cause:       nil,
			expectedMsg: "config error: invalid configuration",
		},
		{
			name:        "error with cause",
			errorType:   ErrTypeFile,
			path:        "/test.txt",
			message:     "access denied",
			cause:       errors.New("permission denied"),
			expectedMsg: "file error for /test.txt: access denied",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &RemapError{
				Type:    tt.errorType,
				Path:    tt.path,
				Message: tt.message,
				Cause:   tt.cause,
			}

			if err.Error() != tt.expectedMsg {
				t.Errorf("expected %q, got %q", tt.expectedMsg, err.Error())
			}

			if err.Unwrap() != tt.cause {
				t.Errorf("expected cause %v, got %v", tt.cause, err.Unwrap())
			}
		})
	}
}

func TestRemapErrorIs(t *testing.T) {
	tests := []struct {
		name   string
		err1   *RemapError
		err2   error
		expect bool
	}{
		{
			name:   "same error type",
			err1:   &RemapError{Type: ErrTypeFile},
			err2:   &RemapError{Type: ErrTypeFile},
			expect: true,
		},
		{
			name:   "different error type",
			err1:   &RemapError{Type: ErrTypeFile},
			err2:   &RemapError{Type: ErrTypeConfig},
			expect: false,
		},
		{
			name:   "not a RemapError",
			err1:   &RemapError{Type: ErrTypeFile},
			err2:   errors.New("standard error"),
			expect: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.err1.Is(tt.err2)
			if result != tt.expect {
				t.Errorf("expected %v, got %v", tt.expect, result)
			}
		})
	}
}

func TestNewFileError(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		message string
		cause   error
	}{
		{
			name:    "basic file error",
			path:    "/test/file.txt",
			message: "read failed",
			cause:   nil,
		},
		{
			name:    "file error with cause",
			path:    "/test/file.txt",
			message: "write failed",
			cause:   errors.New("disk full"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewFileError(tt.path, tt.message, tt.cause)

			if err == nil {
				t.Fatal("expected non-nil error")
			}
			if err.Type != ErrTypeFile {
				t.Errorf("expected type %s, got %s", ErrTypeFile, err.Type)
			}
			if err.Path != tt.path {
				t.Errorf("expected path %s, got %s", tt.path, err.Path)
			}
			if err.Message != tt.message {
				t.Errorf("expected message %s, got %s", tt.message, err.Message)
			}
			if err.Cause != tt.cause {
				t.Errorf("expected cause %v, got %v", tt.cause, err.Cause)
			}
		})
	}
}

func TestNewFileNotFoundError(t *testing.T) {
	tests := []struct {
		name  string
		path  string
		cause error
	}{
		{
			name:  "file not found without cause",
			path:  "/missing/file.txt",
			cause: nil,
		},
		{
			name:  "file not found with cause",
			path:  "/missing/file.txt",
			cause: errors.New("no such file or directory"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewFileNotFoundError(tt.path, tt.cause)

			if err == nil {
				t.Fatal("expected non-nil error")
			}
			if err.Type != ErrTypeFile {
				t.Errorf("expected type %s, got %s", ErrTypeFile, err.Type)
			}
			if err.Message != "file not found" {
				t.Errorf("expected message 'file not found', got %s", err.Message)
			}
		})
	}
}

func TestNewFileNotWritableError(t *testing.T) {
	tests := []struct {
		name  string
		path  string
		cause error
	}{
		{
			name:  "file not writable",
			path:  "/readonly/file.txt",
			cause: errors.New("permission denied"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewFileNotWritableError(tt.path, tt.cause)

			if err == nil {
				t.Fatal("expected non-nil error")
			}
			if err.Type != ErrTypeFile {
				t.Errorf("expected type %s, got %s", ErrTypeFile, err.Type)
			}
			if err.Message != "file not writable" {
				t.Errorf("expected message 'file not writable', got %s", err.Message)
			}
		})
	}
}

func TestNewFileNotReadableError(t *testing.T) {
	tests := []struct {
		name  string
		path  string
		cause error
	}{
		{
			name:  "file not readable",
			path:  "/protected/file.txt",
			cause: errors.New("permission denied"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewFileNotReadableError(tt.path, tt.cause)

			if err == nil {
				t.Fatal("expected non-nil error")
			}
			if err.Type != ErrTypeFile {
				t.Errorf("expected type %s, got %s", ErrTypeFile, err.Type)
			}
			if err.Message != "file not readable" {
				t.Errorf("expected message 'file not readable', got %s", err.Message)
			}
		})
	}
}

func TestNewConfigError(t *testing.T) {
	tests := []struct {
		name    string
		message string
		cause   error
	}{
		{
			name:    "config error without cause",
			message: "invalid setting",
			cause:   nil,
		},
		{
			name:    "config error with cause",
			message: "failed to parse",
			cause:   errors.New("syntax error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewConfigError(tt.message, tt.cause)

			if err == nil {
				t.Fatal("expected non-nil error")
			}
			if err.Type != ErrTypeConfig {
				t.Errorf("expected type %s, got %s", ErrTypeConfig, err.Type)
			}
			if err.Message != tt.message {
				t.Errorf("expected message %s, got %s", tt.message, err.Message)
			}
		})
	}
}

func TestNewConfigErrorWithPath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		message string
		cause   error
	}{
		{
			name:    "config file error",
			path:    "/config/settings.json",
			message: "invalid JSON format",
			cause:   errors.New("unexpected token"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewConfigErrorWithPath(tt.path, tt.message, tt.cause)

			if err == nil {
				t.Fatal("expected non-nil error")
			}
			if err.Type != ErrTypeConfig {
				t.Errorf("expected type %s, got %s", ErrTypeConfig, err.Type)
			}
			if err.Path != tt.path {
				t.Errorf("expected path %s, got %s", tt.path, err.Path)
			}
		})
	}
}

func TestNewParsingError(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		message string
		cause   error
	}{
		{
			name:    "CSV parsing error",
			path:    "/data/mappings.csv",
			message: "invalid CSV format",
			cause:   errors.New("wrong number of fields"),
		},
		{
			name:    "JSON parsing error",
			path:    "/data/mappings.json",
			message: "malformed JSON",
			cause:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewParsingError(tt.path, tt.message, tt.cause)

			if err == nil {
				t.Fatal("expected non-nil error")
			}
			if err.Type != ErrTypeParsing {
				t.Errorf("expected type %s, got %s", ErrTypeParsing, err.Type)
			}
			if err.Path != tt.path {
				t.Errorf("expected path %s, got %s", tt.path, err.Path)
			}
		})
	}
}

func TestNewReplacementError(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		message string
		cause   error
	}{
		{
			name:    "replacement operation failed",
			path:    "/target/file.txt",
			message: "pattern matching failed",
			cause:   errors.New("regex error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewReplacementError(tt.path, tt.message, tt.cause)

			if err == nil {
				t.Fatal("expected non-nil error")
			}
			if err.Type != ErrTypeReplacement {
				t.Errorf("expected type %s, got %s", ErrTypeReplacement, err.Type)
			}
		})
	}
}

func TestNewBackupError(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		message string
		cause   error
	}{
		{
			name:    "backup creation failed",
			path:    "/backup/file.txt.bak",
			message: "insufficient disk space",
			cause:   errors.New("no space left on device"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewBackupError(tt.path, tt.message, tt.cause)

			if err == nil {
				t.Fatal("expected non-nil error")
			}
			if err.Type != ErrTypeBackup {
				t.Errorf("expected type %s, got %s", ErrTypeBackup, err.Type)
			}
		})
	}
}

func TestWrapFileError(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		inputError  error
		expectType  string
		expectError bool
	}{
		{
			name:        "nil error",
			path:        "/test/file.txt",
			inputError:  nil,
			expectError: false,
		},
		{
			name:        "generic error",
			path:        "/test/file.txt",
			inputError:  errors.New("generic error"),
			expectType:  "file",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := WrapFileError(tt.path, tt.inputError)

			if !tt.expectError && result != nil {
				t.Errorf("expected nil error, got %v", result)
			}
			if tt.expectError && result == nil {
				t.Error("expected error, got nil")
			}

			if tt.expectError && result != nil {
				if te, ok := result.(*FileError); ok {
					if string(te.Type) != tt.expectType {
						t.Errorf("expected type %s, got %s", tt.expectType, te.Type)
					}
				} else if te, ok := result.(*FileNotFoundError); ok {
					if string(te.Type) != tt.expectType {
						t.Errorf("expected type %s, got %s", tt.expectType, te.Type)
					}
				} else if te, ok := result.(*FileNotWritableError); ok {
					if string(te.Type) != tt.expectType {
						t.Errorf("expected type %s, got %s", tt.expectType, te.Type)
					}
				}
			}
		})
	}
}

func TestErrorTypeConstants(t *testing.T) {
	tests := []struct {
		name      string
		errorType ErrorType
		expected  string
	}{
		{
			name:      "file error type",
			errorType: ErrTypeFile,
			expected:  "file",
		},
		{
			name:      "config error type",
			errorType: ErrTypeConfig,
			expected:  "config",
		},
		{
			name:      "parsing error type",
			errorType: ErrTypeParsing,
			expected:  "parsing",
		},
		{
			name:      "replacement error type",
			errorType: ErrTypeReplacement,
			expected:  "replacement",
		},
		{
			name:      "backup error type",
			errorType: ErrTypeBackup,
			expected:  "backup",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.errorType) != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, string(tt.errorType))
			}
		})
	}
}

func TestIsNotFoundError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "absolute path error",
			err:      errors.New("/absolute/path/error"),
			expected: false,
		},
		{
			name:     "relative path error",
			err:      errors.New("relative/path/error"),
			expected: true,
		},
		{
			name:     "simple error message",
			err:      errors.New("simple error"),
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isNotFoundError(tt.err)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestIsPermissionError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "any error returns false",
			err:      errors.New("permission denied"),
			expected: false,
		},
		{
			name:     "nil error returns false",
			err:      nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isPermissionError(tt.err)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}
