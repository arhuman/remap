// Package filter provides file discovery and filtering capabilities.
// It implements a composable filter system that enables efficient file
// traversal with configurable inclusion/exclusion rules and extension filtering.
package filter

import (
	"os"
	"path/filepath"
	"strings"

	"remap/internal/config"
	"remap/internal/errors"
)

// FileInfo contains essential metadata about discovered files.
// This lightweight structure provides the minimum information needed
// for processing decisions while avoiding expensive stat operations.
type FileInfo struct {
	Path    string
	Size    int64
	IsDir   bool
	ModTime int64
}

// FileFilter defines a predicate function for file filtering.
// This function type enables composable filtering logic where multiple
// filters can be chained together for complex file selection criteria.
type FileFilter func(path string, info os.FileInfo) (bool, error)

// NewFileDiscovery creates a FileDiscovery with configured filters.
// This constructor builds an optimized filter chain based on configuration,
// enabling efficient file traversal with early rejection of unwanted files.
func NewFileDiscovery(cfg *config.Config) *FileDiscovery {
	return &FileDiscovery{
		config:  cfg,
		filters: buildFilters(cfg),
	}
}

// FileDiscovery handles recursive directory traversal with filtering.
// It provides efficient file discovery using a composable filter system,
// enabling complex file selection rules while maintaining performance.
type FileDiscovery struct {
	config  *config.Config
	filters []FileFilter
}

// Discover recursively traverses the configured directory and returns filtered files.
// This method implements efficient directory walking with early filtering,
// reducing memory usage and processing time by excluding unwanted files immediately.
func (fd *FileDiscovery) Discover() ([]FileInfo, error) {
	var files []FileInfo

	err := filepath.Walk(fd.config.Directory, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			if os.IsPermission(err) {
				return nil
			}
			return errors.WrapFileError(path, err)
		}

		if info.IsDir() {
			// Check if this directory should be excluded
			if fd.shouldExcludeDirectory(path) {
				return filepath.SkipDir
			}
			return nil
		}

		shouldProcess, err := fd.shouldProcessFile(path, info)
		if err != nil {
			return err
		}

		if shouldProcess {
			files = append(files, FileInfo{
				Path:    path,
				Size:    info.Size(),
				IsDir:   info.IsDir(),
				ModTime: info.ModTime().Unix(),
			})
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return files, nil
}

func (fd *FileDiscovery) shouldProcessFile(path string, info os.FileInfo) (bool, error) {
	for _, filter := range fd.filters {
		should, err := filter(path, info)
		if err != nil {
			return false, err
		}
		if !should {
			return false, nil
		}
	}
	return true, nil
}

func buildFilters(cfg *config.Config) []FileFilter {
	var filters []FileFilter

	filters = append(filters, extensionFilter(cfg))

	if len(cfg.Include) > 0 {
		filters = append(filters, includeFilter(cfg.Include))
	}

	if len(cfg.Exclude) > 0 {
		filters = append(filters, excludeFilter(cfg.Exclude))
	}

	filters = append(filters, regularFileFilter())

	return filters
}

// shouldExcludeDirectory determines if a directory should be excluded from traversal.
// This method checks both the basename and full path against the ExcludeDir patterns,
// enabling flexible directory exclusion rules while maintaining performance.
func (fd *FileDiscovery) shouldExcludeDirectory(dirPath string) bool {
	if len(fd.config.ExcludeDir) == 0 {
		return false
	}

	baseName := filepath.Base(dirPath)

	for _, excludePattern := range fd.config.ExcludeDir {
		// Check basename match
		if baseName == excludePattern {
			return true
		}

		// Check full path match
		if dirPath == excludePattern {
			return true
		}

		// Check if pattern matches using filepath.Match for glob patterns
		if matched, err := filepath.Match(excludePattern, baseName); err == nil && matched {
			return true
		}

		if matched, err := filepath.Match(excludePattern, dirPath); err == nil && matched {
			return true
		}
	}

	return false
}

func extensionFilter(cfg *config.Config) FileFilter {
	return func(path string, _ os.FileInfo) (bool, error) {
		if len(cfg.Extensions) == 0 {
			return true, nil
		}

		ext := filepath.Ext(path)
		return cfg.ShouldProcessExtension(ext), nil
	}
}

func includeFilter(patterns []string) FileFilter {
	return func(path string, _ os.FileInfo) (bool, error) {
		for _, pattern := range patterns {
			// Try base name first
			matched, err := filepath.Match(pattern, filepath.Base(path))
			if err != nil {
				return false, errors.NewConfigError("invalid include pattern: "+pattern, err)
			}
			if matched {
				return true, nil
			}

			// Try full path
			matched, err = filepath.Match(pattern, path)
			if err != nil {
				return false, errors.NewConfigError("invalid include pattern: "+pattern, err)
			}
			if matched {
				return true, nil
			}

			// For absolute paths, also try matching without the leading slash
			if filepath.IsAbs(path) {
				relPath := strings.TrimPrefix(path, "/")
				matched, err = filepath.Match(pattern, relPath)
				if err != nil {
					return false, errors.NewConfigError("invalid include pattern: "+pattern, err)
				}
				if matched {
					return true, nil
				}
			}

			// Handle patterns like "*/file.ext" that should match any subdirectory
			if matched, err := matchFlexiblePattern(pattern, path); err != nil {
				return false, err
			} else if matched {
				return true, nil
			}
		}
		return false, nil
	}
}

// matchFlexiblePattern handles patterns like "*/main.go" to match any subdirectory depth
func matchFlexiblePattern(pattern, path string) (bool, error) {
	// Convert absolute path to relative for matching
	if filepath.IsAbs(path) {
		path = strings.TrimPrefix(path, "/")
	}

	// If pattern starts with "*/" and we have a file basename that matches the pattern suffix
	if strings.HasPrefix(pattern, "*/") && strings.Count(pattern, "/") == 1 {
		expectedFile := strings.TrimPrefix(pattern, "*/")
		return filepath.Base(path) == expectedFile, nil
	}

	return false, nil
}

func excludeFilter(patterns []string) FileFilter {
	return func(path string, _ os.FileInfo) (bool, error) {
		for _, pattern := range patterns {
			matched, err := filepath.Match(pattern, filepath.Base(path))
			if err != nil {
				return false, errors.NewConfigError("invalid exclude pattern: "+pattern, err)
			}
			if matched {
				return false, nil
			}

			matched, err = filepath.Match(pattern, path)
			if err != nil {
				return false, errors.NewConfigError("invalid exclude pattern: "+pattern, err)
			}
			if matched {
				return false, nil
			}
		}
		return true, nil
	}
}

func regularFileFilter() FileFilter {
	return func(path string, info os.FileInfo) (bool, error) {
		if info.IsDir() {
			return false, nil
		}

		if !info.Mode().IsRegular() {
			return false, nil
		}

		if strings.HasPrefix(filepath.Base(path), ".") {
			return false, nil
		}

		if strings.HasSuffix(path, ".bak") {
			return false, nil
		}

		return true, nil
	}
}
