// Package parser provides functionality for loading and parsing mapping tables.
// It supports both CSV and JSON formats, converting them into internal data
// structures optimized for fast string replacement operations with proper ordering.
package parser

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"remap/internal/errors"
)

// Mapping represents a single string replacement rule.
// The JSON tags enable loading from JSON files while maintaining
// clear field names that match the domain terminology.
type Mapping struct {
	From string `json:"old"`
	To   string `json:"new"`
}

// MappingTable holds string replacement mappings with optimized access patterns.
// It maintains both original and length-sorted versions of mappings to enable
// correct replacement order (longest first) while preserving the original data.
type MappingTable struct {
	mappings []Mapping
	sorted   []Mapping
}

// NewMappingTable creates a MappingTable with optimized sorting for replacements.
// The constructor sorts mappings by decreasing string length to ensure that
// longer patterns are matched first, preventing incorrect partial replacements.
func NewMappingTable(mappings []Mapping) *MappingTable {
	mt := &MappingTable{
		mappings: mappings,
		sorted:   make([]Mapping, len(mappings)),
	}
	copy(mt.sorted, mappings)

	sort.Slice(mt.sorted, func(i, j int) bool {
		return len(mt.sorted[i].From) > len(mt.sorted[j].From)
	})

	return mt
}

// GetMappings returns the original unsorted mappings.
// This method provides access to mappings in their original order,
// useful for reporting and debugging purposes where order matters.
func (mt *MappingTable) GetMappings() []Mapping {
	return mt.mappings
}

// GetSortedMappings returns mappings sorted by string length (descending).
// This method provides the optimal order for string replacement operations,
// ensuring longer patterns are processed first to prevent partial matches.
func (mt *MappingTable) GetSortedMappings() []Mapping {
	return mt.sorted
}

// Size returns the number of mappings in the table.
// This method provides a consistent way to check table size across
// different components without exposing internal slice details.
func (mt *MappingTable) Size() int {
	return len(mt.mappings)
}

// LoadMappingTable loads and parses a mapping table from a file.
// This function provides the main entry point for loading mapping tables,
// automatically dispatching to the appropriate parser based on format.
func LoadMappingTable(filePath, format string) (*MappingTable, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, errors.WrapFileError(filePath, err)
	}
	defer file.Close()

	switch format {
	case "csv":
		return parseCSVMappings(file, filePath)
	case "json":
		return parseJSONMappings(file, filePath)
	default:
		return nil, errors.NewParsingError(filePath, fmt.Sprintf("unsupported format: %s", format), nil)
	}
}

func parseCSVMappings(reader io.Reader, filePath string) (*MappingTable, error) {
	records, err := readCSVRecords(reader, filePath)
	if err != nil {
		return nil, err
	}

	startIndex := determineCSVStartIndex(records)
	mappings, err := extractCSVMappings(records, startIndex, filePath)
	if err != nil {
		return nil, err
	}

	if len(mappings) == 0 {
		return nil, errors.NewParsingError(filePath, "no valid mappings found in CSV", nil)
	}

	return NewMappingTable(mappings), nil
}

func readCSVRecords(reader io.Reader, filePath string) ([][]string, error) {
	// Read all content first to filter comment lines
	content, err := io.ReadAll(reader)
	if err != nil {
		return nil, errors.NewParsingError(filePath, "failed to read CSV content", err)
	}

	// Filter out comment lines (lines starting with #)
	lines := strings.Split(string(content), "\n")
	var filteredLines []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Skip empty lines and comment lines (starting with #)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		filteredLines = append(filteredLines, line)
	}

	if len(filteredLines) == 0 {
		return nil, errors.NewParsingError(filePath, "CSV file contains no data after filtering comments", nil)
	}

	// Parse the filtered content
	filteredContent := strings.Join(filteredLines, "\n")
	csvReader := csv.NewReader(strings.NewReader(filteredContent))
	csvReader.TrimLeadingSpace = true

	records, err := csvReader.ReadAll()
	if err != nil {
		return nil, errors.NewParsingError(filePath, "failed to parse CSV", err)
	}

	if len(records) == 0 {
		return nil, errors.NewParsingError(filePath, "CSV file is empty", nil)
	}

	return records, nil
}

func determineCSVStartIndex(records [][]string) int {
	if len(records[0]) < 2 {
		return 0
	}

	firstRow := records[0]
	if isHeaderRow(firstRow) {
		return 1
	}
	return 0
}

func isHeaderRow(row []string) bool {
	return strings.EqualFold(row[0], "old") || strings.EqualFold(row[0], "source") ||
		strings.EqualFold(row[1], "new") || strings.EqualFold(row[1], "destination")
}

func extractCSVMappings(records [][]string, startIndex int, filePath string) ([]Mapping, error) {
	var mappings []Mapping

	for i := startIndex; i < len(records); i++ {
		record := records[i]
		if len(record) < 2 {
			return nil, errors.NewParsingError(filePath, fmt.Sprintf("invalid CSV row at line %d: expected 2 columns", i+1), nil)
		}

		from := strings.TrimSpace(record[0])
		to := strings.TrimSpace(record[1])

		if from == "" {
			continue
		}

		mappings = append(mappings, Mapping{
			From: from,
			To:   to,
		})
	}

	return mappings, nil
}

func parseJSONMappings(reader io.Reader, filePath string) (*MappingTable, error) {
	var mappings []Mapping

	decoder := json.NewDecoder(reader)
	if err := decoder.Decode(&mappings); err != nil {
		return nil, errors.NewParsingError(filePath, "failed to parse JSON", err)
	}

	if len(mappings) == 0 {
		return nil, errors.NewParsingError(filePath, "no mappings found in JSON", nil)
	}

	var validMappings []Mapping
	for _, mapping := range mappings {
		if mapping.From == "" {
			continue
		}

		if mapping.To == "" {
			mapping.To = ""
		}

		validMappings = append(validMappings, Mapping{
			From: strings.TrimSpace(mapping.From),
			To:   strings.TrimSpace(mapping.To),
		})
	}

	if len(validMappings) == 0 {
		return nil, errors.NewParsingError(filePath, "no valid mappings found in JSON", nil)
	}

	return NewMappingTable(validMappings), nil
}
