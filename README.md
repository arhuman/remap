# Remap

A fast, concurrent CLI tool for bulk string replacement across file hierarchies using mapping tables. Perfect for migrating IP addresses, CNAMEs, or any systematic string replacements in large codebases.

## Features

- **Concurrent Processing**: Leverages Go goroutines for fast file processing
- **Flexible Mapping**: Support for both CSV and JSON mapping formats
- **Advanced Filtering**: Include/exclude patterns with glob support
- **Safe Operations**: Dry-run mode and automatic backups (enabled by default)
- **Comprehensive Logging**: Detailed reports in JSON or CSV format
- **Revert Capability**: Undo transformations using log files or backup files
- **Case-Sensitive Options**: Control search behavior
- **Extensive File Support**: Filter by extensions and patterns

## Installation

### From Source

```bash
git clone <repository-url>
cd remap
go build -o remap .
```

### Using Go Install

```bash
go install remap@latest
```

## Quick Start

### Basic Usage

```bash
# Replace strings using CSV mapping
remap --csv mappings.csv /path/to/directory

# Dry run to see what would be changed
remap --csv mappings.csv --dry-run /path/to/directory

# Process only specific file types
remap --csv mappings.csv --extensions .go,.js,.py /path/to/directory
```

### Example Mapping Files

**CSV Format** (`mappings.csv`):
```csv
old,new
192.168.1.1,10.0.0.1
old-server.com,new-server.com
foo,bar
```

**JSON Format** (`mappings.json`):
```json
[
  {"old": "192.168.1.1", "new": "10.0.0.1"},
  {"old": "old-server.com", "new": "new-server.com"},
  {"old": "foo", "new": "bar"}
]
```

## Command Reference

### Basic Syntax
```bash
remap [options] <directory>
```

### Required Arguments
- `<directory>`: Target directory to process

### Mapping Options (exactly one required)
- `--csv <file>`: CSV mapping file (columns: source,destination)
- `--json <file>`: JSON mapping file

### File Filtering
- `--include <pattern>`: Include files matching glob pattern (repeatable)
- `--exclude <pattern>`: Exclude files matching glob pattern (repeatable)
- `--exclude-dir <dir>`: Exclude directories from traversal (repeatable)
- `--extensions <exts>`: Process only specified extensions (e.g., `.txt,.go,.js`)

### Processing Options
- `--dry-run`: Simulate changes without modifying files
- `--backup`: Create `.bak` files before modifications (deprecated, backups enabled by default)
- `--nobackup`: Disable automatic backup file creation
- `--case-sensitive`: Enable case-sensitive string matching (default: insensitive)
- `--revert, -r`: Revert transformations using log file

### Logging & Output
- `--verbose, -v`: Enable verbose output
- `--debug`: Enable debug mode with detailed logging
- `--quiet, -q`: Suppress non-essential output
- `--log <file>`: Write log to file (default: stdout)
- `--log-format <format>`: Log format (`json` or `csv`)

## Usage Examples

### 1. Server Migration
Replace old server references across a codebase:

```bash
# Create mapping file
cat > server-migration.csv << EOF
old-api.company.com,new-api.company.com
192.168.1.100,10.0.0.100
staging.old.com,staging.new.com
EOF

# Dry run first
remap --csv server-migration.csv --dry-run --verbose ./src

# Apply changes with backup
remap --csv server-migration.csv --backup --log migration.log ./src
```

### 2. IP Address Migration
Migrate old IP addresses to new ones across configuration files:

```bash
# Create IP migration mapping
cat > ip-migration.csv << EOF
old,new
10.1.1.10,10.2.1.10
10.1.1.11,10.2.1.11
10.1.1.12,10.2.1.12
10.1.2.100,10.2.2.100
192.168.1.50,172.16.1.50
192.168.1.51,172.16.1.51
EOF

# Apply to configuration files with backup
remap --csv ip-migration.csv \
  --extensions .conf,.ini,.yaml,.yml \
  --backup \
  --log ip-migration.log \
  --verbose ./configs

# Check what would be changed in specific files
remap --csv ip-migration.csv \
  --include "*.properties" \
  --include "application-*.yml" \
  --dry-run ./application
```

### 3. Configuration Updates
Update configuration files with filtering:

```bash
# Include only config files
remap --csv config-updates.csv \
  --include "*.conf" \
  --include "*.ini" \
  --include "config.*" \
  --backup \
  --log config-changes.json \
  --log-format json \
  ./configs
```

### 4. Revert Changes
Undo transformations using log files:

```bash
# Revert using previous log
remap --revert --log migration.log
```

### 5. Disable Backups for Performance
When processing large file sets where backups aren't needed:

```bash
# Process without creating backup files
remap --csv mappings.csv --nobackup --extensions .log,.tmp ./temp-files

# Combine with dry-run to see what would change
remap --csv mappings.csv --nobackup --dry-run ./large-dataset
```

### 6. Exclude Common Development Directories
Skip version control and build directories during processing:

```bash
# Exclude common development directories
remap --csv mappings.csv \
  --exclude-dir .git \
  --exclude-dir .venv \
  --exclude-dir node_modules \
  --exclude-dir target \
  --exclude-dir build \
  --exclude-dir dist \
  --extensions .js,.py,.go \
  ./project

# Exclude cache and temporary directories
remap --csv config-updates.csv \
  --exclude-dir .cache \
  --exclude-dir __pycache__ \
  --exclude-dir .pytest_cache \
  --exclude-dir temp \
  --backup \
  ./codebase
```

## Backup and Safety Features

### Automatic Backup Creation

Remap automatically creates backup files before modifying originals to ensure data safety:

- **Default Behavior**: Backups are **enabled by default** for all file modifications
- **Backup Format**: `filename.YYYYMMDD_HHMMSS.bak` (timestamped for uniqueness)
- **Atomic Operations**: Backups are created atomically to prevent corruption
- **Permission Preservation**: Original file permissions are maintained in backup copies

### Backup Control Options

```bash
# Default: backups automatically created
remap --csv mappings.csv /path/to/files

# Explicitly disable backups for performance
remap --csv mappings.csv --nobackup /path/to/files

# Legacy backup flag (deprecated but supported)
remap --csv mappings.csv --backup /path/to/files
```

### Revert Capabilities

Remap provides dual reversion strategies:

1. **Backup-based reversion** (preferred):
   - Restores original files from timestamped backups
   - Complete file state restoration
   - Works even if mapping log is corrupted

2. **Log-based reversion** (fallback):
   - Applies inverse string replacements from operation logs
   - Requires intact log files
   - Useful when backup files are unavailable

```bash
# Revert using operation log (applies inverse replacements)
remap --revert --log operation.log

# Manual restoration from backup files
cp file.txt.20240107_143052.bak file.txt
```

### Safety Best Practices

- **Always test with `--dry-run`** before production changes
- **Keep operation logs** for audit trails and potential reverts
- **Use `--verbose`** to monitor processing progress
- **Backup important data** independently before bulk operations
- **Use `--nobackup` only** when you have alternative backup strategies

## Architecture

```
remap/
├── cmd/                    # CLI command definitions
│   ├── root.go            # Root command and flags
│   └── execute.go         # Main execution logic
├── internal/
│   ├── config/            # Configuration management
│   ├── parser/            # Mapping file parsers
│   ├── filter/            # File discovery and filtering
│   ├── replacement/       # String replacement engine
│   ├── concurrent/        # Concurrent file processing
│   ├── backup/            # Backup and revert functionality
│   ├── log/               # Logging and reporting
│   └── errors/            # Error type hierarchy
```

### Key Components

- **Concurrent Processor**: Handles parallel file processing using worker pools
- **Filter Engine**: Implements glob pattern matching for file inclusion/exclusion
- **Replacement Engine**: Performs string replacements with configurable case sensitivity
- **Backup Manager**: Creates backups and handles revert operations
- **Logger**: Generates detailed processing reports in multiple formats

## Testing

Run the test suite:

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -v -cover ./...

# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
```

## Development

### Prerequisites
- Go 1.24.4 or later
- Make (optional, for build automation)

### Building
```bash
# Build binary
go build -o remap .

# Build with version info
go build -ldflags "-X main.version=v1.0.0" -o remap .

# Cross-compile for different platforms
GOOS=linux GOARCH=amd64 go build -o remap-linux .
GOOS=windows GOARCH=amd64 go build -o remap.exe .
```

### Code Guidelines
- Write idiomatic Go code
- Ensure all functions are testable
- Use goroutines safely without race conditions
- Favor composition and middleware chains over monolithic functions
- Implement comprehensive error handling with typed errors

## Error Handling

Remap uses a hierarchical error system:

```
RemapError
├── ConfigError          # Configuration issues
├── FileError           # File operation errors
│   ├── FileNotFound    # Missing files
│   └── FileNotWritable # Permission errors
├── MappingError        # Mapping file issues
└── ProcessingError     # Runtime processing errors
```

## Exit Codes

- `0`: Success
- `1`: Error (details in processing report)

## License

[Add your license here]

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## Support

For issues and questions:
- Create an issue on GitHub
- Check the documentation
- Review existing issues for solutions

## Changelog

### v1.0.0
- Initial release
- CSV and JSON mapping support
- Concurrent processing
- Comprehensive logging
- Backup and revert functionality

---

**Remap** - Transform your codebase efficiently and safely!