// Package config provides configuration management and validation for remap.
// It centralizes all command-line options and runtime settings, providing
// validation logic to catch configuration errors early before processing begins.
package config

import (
	"path/filepath"
	"strings"

	"remap/internal/errors"
)

// LogFormat represents the supported output formats for operation logs.
// This enumeration ensures type safety and enables format-specific
// output generation logic throughout the logging system.
type LogFormat string

// Supported log format constants define the available output formats for operation logs.
// These constants ensure type safety and enable format-specific rendering logic
// throughout the logging system for consistent output generation.
const (
	LogFormatJSON LogFormat = "json"
	LogFormatCSV  LogFormat = "csv"
)

// Config holds all runtime configuration options for remap operations.
// It provides a single source of truth for all settings, enabling consistent
// behavior across all components and simplifying dependency injection throughout
// the application architecture.
type Config struct {
	Directory     string
	MappingFile   string
	MappingType   string
	Include       []string
	Exclude       []string
	ExcludeDir    []string
	Extensions    []string
	DryRun        bool
	Revert        bool
	Apply         bool
	Backup        bool
	NoBackup      bool
	CaseSensitive bool
	Verbose       bool
	Debug         bool
	Quiet         bool
	LogFile       string
	LogFormat     LogFormat
}

// Validate performs comprehensive validation of configuration settings.
// This method catches configuration errors early in the application lifecycle,
// preventing resource waste and providing clear feedback to users about
// invalid settings before processing begins.
func (c *Config) Validate() error {
	if err := c.validateDirectory(); err != nil {
		return err
	}

	if err := c.validateMappingFile(); err != nil {
		return err
	}

	if err := c.validateMappingType(); err != nil {
		return err
	}

	if err := c.validateLogFormat(); err != nil {
		return err
	}

	c.normalizeConfig()
	return nil
}

func (c *Config) validateDirectory() error {
	// Directory is not required in revert or apply mode
	if (c.Revert || c.Apply) && c.Directory == "" {
		return nil
	}

	if c.Directory == "" {
		return errors.NewConfigError("directory is required", nil)
	}

	absDir, err := filepath.Abs(c.Directory)
	if err != nil {
		return errors.NewConfigErrorWithPath(c.Directory, "invalid directory path", err)
	}
	c.Directory = absDir
	return nil
}

func (c *Config) validateMappingFile() error {
	if c.MappingFile == "" && !c.Revert && !c.Apply {
		return errors.NewConfigError("mapping file is required (use --csv or --json)", nil)
	}

	if c.MappingFile != "" {
		absMappingFile, err := filepath.Abs(c.MappingFile)
		if err != nil {
			return errors.NewConfigErrorWithPath(c.MappingFile, "invalid mapping file path", err)
		}
		c.MappingFile = absMappingFile
	}
	return nil
}

func (c *Config) validateMappingType() error {
	if c.MappingType != "" && c.MappingType != "csv" && c.MappingType != "json" {
		return errors.NewConfigError("mapping type must be 'csv' or 'json'", nil)
	}
	return nil
}

func (c *Config) validateLogFormat() error {
	if c.LogFormat != "" && c.LogFormat != LogFormatJSON && c.LogFormat != LogFormatCSV {
		return errors.NewConfigError("log format must be 'json' or 'csv'", nil)
	}
	return nil
}

func (c *Config) normalizeConfig() {
	if c.LogFormat == "" {
		c.LogFormat = LogFormatJSON
	}
	c.Extensions = c.normalizeExtensions()
}

// normalizeExtensions standardizes file extension format for consistent matching.
// This method ensures all extensions have proper dot prefixes and consistent casing,
// preventing match failures due to format variations in user input.
func (c *Config) normalizeExtensions() []string {
	var normalized []string
	for _, ext := range c.Extensions {
		ext = strings.TrimSpace(ext)
		if ext == "" {
			continue
		}
		if !strings.HasPrefix(ext, ".") {
			ext = "." + ext
		}
		normalized = append(normalized, strings.ToLower(ext))
	}
	return normalized
}

// ShouldProcessExtension determines if files with the given extension should be processed.
// This method implements the core filtering logic for file extension-based inclusion,
// enabling users to limit processing to specific file types for performance and safety.
func (c *Config) ShouldProcessExtension(ext string) bool {
	if len(c.Extensions) == 0 {
		return true
	}

	ext = strings.ToLower(ext)
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}

	for _, allowed := range c.Extensions {
		if allowed == ext {
			return true
		}
	}
	return false
}

// IsVerbose determines if verbose logging is enabled.
// This method implements the precedence logic where Quiet mode overrides
// Verbose mode, ensuring consistent logging behavior across the application.
func (c *Config) IsVerbose() bool {
	return c.Verbose && !c.Quiet
}

// IsDebug determines if debug logging is enabled.
// This method implements the precedence logic where Quiet mode overrides
// Debug mode, preventing unwanted debug output during silent operations.
func (c *Config) IsDebug() bool {
	return c.Debug && !c.Quiet
}

// ShouldLog determines if any logging should occur.
// This method provides a central check for logging enablement, allowing
// components to skip expensive log preparation when output will be suppressed.
func (c *Config) ShouldLog() bool {
	return !c.Quiet
}

// ShouldCreateBackup determines if backup files should be created.
// This method implements the precedence logic where NoBackup takes precedence
// over Backup. By default, backups are enabled unless explicitly disabled.
func (c *Config) ShouldCreateBackup() bool {
	if c.NoBackup {
		return false
	}
	// Default to true (backups enabled) unless explicitly disabled
	return true
}
