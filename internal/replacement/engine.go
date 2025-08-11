// Package replacement provides a middleware-based string replacement engine.
// It implements a composable processing pipeline that enables extensible
// replacement logic with proper context propagation and error handling.
package replacement

import (
	"bufio"
	"io"
	"strings"

	"remap/internal/config"
	"remap/internal/parser"
)

// Replacement represents a single string replacement operation with context.
// This structure captures detailed information about each replacement,
// enabling precise reporting and potential reversal operations.
type Replacement struct {
	From       string
	To         string
	Line       int
	Column     int
	LineText   string
	NewText    string
	ByteOffset int64
}

// FileResult contains the complete result of processing a single file.
// This structure provides comprehensive information about all replacements
// performed, enabling detailed reporting and change tracking.
type FileResult struct {
	Path         string
	Replacements []Replacement
	Modified     bool
	OriginalSize int64
	NewSize      int64
}

// Middleware defines a processing step in the replacement pipeline.
// This function type enables composable processing logic where each
// middleware can transform the context and pass control to the next step.
type Middleware func(ProcessContext) ProcessContext

// ProcessContext carries state through the replacement pipeline.
// This structure provides all necessary context for replacement operations
// while enabling middleware to add metadata and handle errors.
type ProcessContext struct {
	Config   *config.Config
	FilePath string
	Content  []byte
	Mappings *parser.MappingTable
	Result   *FileResult
	Error    error
	Metadata map[string]interface{}
}

// Engine orchestrates string replacement operations using a middleware pipeline.
// This design enables extensible replacement logic while maintaining
// separation of concerns between different processing stages.
type Engine struct {
	config     *config.Config
	middleware []Middleware
}

// NewEngine creates a replacement engine with the standard middleware pipeline.
// This constructor sets up the default processing pipeline that handles
// validation, detection, replacement, and output validation in the correct order.
func NewEngine(config *config.Config) *Engine {
	engine := &Engine{
		config:     config,
		middleware: []Middleware{},
	}

	engine.Use(validateInputMiddleware)
	engine.Use(detectReplacementsMiddleware)
	engine.Use(applyReplacementsMiddleware)
	engine.Use(validateOutputMiddleware)

	return engine
}

// Use adds a middleware to the processing pipeline.
// This method enables extending the engine with custom processing logic,
// allowing for specialized behaviors while maintaining the pipeline architecture.
func (e *Engine) Use(middleware Middleware) {
	e.middleware = append(e.middleware, middleware)
}

// ProcessFile processes a single file through the middleware pipeline.
// This method orchestrates the complete replacement workflow, passing
// context through each middleware stage and returning the final result.
func (e *Engine) ProcessFile(filePath string, content []byte, mappings *parser.MappingTable) *FileResult {
	ctx := ProcessContext{
		Config:   e.config,
		FilePath: filePath,
		Content:  content,
		Mappings: mappings,
		Result: &FileResult{
			Path:         filePath,
			OriginalSize: int64(len(content)),
		},
		Metadata: make(map[string]interface{}),
	}

	for _, mw := range e.middleware {
		ctx = mw(ctx)
		if ctx.Error != nil {
			ctx.Result.Modified = false
			return ctx.Result
		}
	}

	return ctx.Result
}

func validateInputMiddleware(ctx ProcessContext) ProcessContext {
	if len(ctx.Content) == 0 {
		ctx.Result.Modified = false
		return ctx
	}

	if ctx.Mappings == nil || ctx.Mappings.Size() == 0 {
		ctx.Result.Modified = false
		return ctx
	}

	return ctx
}

func detectReplacementsMiddleware(ctx ProcessContext) ProcessContext {
	content := string(ctx.Content)
	var replacements []Replacement

	scanner := bufio.NewScanner(strings.NewReader(content))
	lineNum := 0
	byteOffset := int64(0)

	for scanner.Scan() {
		lineNum++
		lineText := scanner.Text()
		lineBytes := scanner.Bytes()

		for _, mapping := range ctx.Mappings.GetSortedMappings() {
			searchText := mapping.From
			if !ctx.Config.CaseSensitive {
				searchText = strings.ToLower(searchText)
				lineText = strings.ToLower(lineText)
			}

			startIndex := 0
			for {
				index := strings.Index(lineText[startIndex:], searchText)
				if index == -1 {
					break
				}

				actualIndex := startIndex + index
				replacement := Replacement{
					From:       mapping.From,
					To:         mapping.To,
					Line:       lineNum,
					Column:     actualIndex + 1,
					LineText:   string(lineBytes),
					ByteOffset: byteOffset + int64(actualIndex),
				}

				replacements = append(replacements, replacement)
				startIndex = actualIndex + len(mapping.From)
			}
		}

		byteOffset += int64(len(lineBytes)) + 1 // +1 for newline
	}

	ctx.Result.Replacements = replacements
	ctx.Result.Modified = len(replacements) > 0

	return ctx
}

func applyReplacementsMiddleware(ctx ProcessContext) ProcessContext {
	if !ctx.Result.Modified || ctx.Config.DryRun {
		return ctx
	}

	content := string(ctx.Content)

	for _, mapping := range ctx.Mappings.GetSortedMappings() {
		if ctx.Config.CaseSensitive {
			content = strings.ReplaceAll(content, mapping.From, mapping.To)
		} else {
			content = caseInsensitiveReplace(content, mapping.From, mapping.To)
		}
	}

	newContent := []byte(content)
	ctx.Content = newContent
	ctx.Result.NewSize = int64(len(newContent))

	for i := range ctx.Result.Replacements {
		ctx.Result.Replacements[i].NewText = content
	}

	return ctx
}

func validateOutputMiddleware(ctx ProcessContext) ProcessContext {
	if ctx.Result.Modified && !ctx.Config.DryRun {
		if len(ctx.Content) == 0 && ctx.Result.OriginalSize > 0 {
			ctx.Result.Modified = false
		}
	}

	return ctx
}

func caseInsensitiveReplace(content, from, to string) string {
	if from == "" {
		return content
	}

	lowerContent := strings.ToLower(content)
	lowerFrom := strings.ToLower(from)

	var result strings.Builder
	start := 0

	for {
		index := strings.Index(lowerContent[start:], lowerFrom)
		if index == -1 {
			result.WriteString(content[start:])
			break
		}

		actualIndex := start + index
		result.WriteString(content[start:actualIndex])
		result.WriteString(to)

		start = actualIndex + len(from)
	}

	return result.String()
}

// ProcessReader applies string replacements to content from an io.Reader.
// This function provides a streaming interface for processing content without
// requiring file system access, enabling flexible content transformation workflows.
func ProcessReader(reader io.Reader, mappings *parser.MappingTable, caseSensitive bool) ([]byte, []Replacement, error) {
	content, err := io.ReadAll(reader)
	if err != nil {
		return nil, nil, err
	}

	config := &config.Config{
		CaseSensitive: caseSensitive,
		DryRun:        false,
	}

	engine := NewEngine(config)
	result := engine.ProcessFile("", content, mappings)

	return content, result.Replacements, nil
}
