package cmd

import (
	"fmt"
	"os"
	"strings"

	"remap/internal/config"
	"remap/internal/errors"

	"github.com/spf13/cobra"
)

var cfg = &config.Config{}
var extensionsStr string

var rootCmd = &cobra.Command{
	Use:   "remap [options] <directory>",
	Short: "Replace strings in files recursively based on mapping tables",
	Long: `Remap is a CLI tool that recursively traverses a directory and replaces
string occurrences according to a mapping table. It supports CSV and JSON mapping
formats and provides extensive filtering and logging capabilities.`,
	Args: func(cmd *cobra.Command, args []string) error {
		// Directory is required except in revert or apply mode
		if cfg.Revert || cfg.Apply {
			return cobra.RangeArgs(0, 1)(cmd, args)
		}
		return cobra.ExactArgs(1)(cmd, args)
	},
	RunE: runRemap,
}

// Execute runs the root command and handles top-level error reporting.
// This function serves as the main entry point for the CLI, providing
// consistent error formatting and exit code management for all command failures.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		if te, ok := err.(*errors.RemapError); ok {
			fmt.Fprintf(os.Stderr, "Error: %s\n", te.Error())
		} else {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err.Error())
		}
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().StringVar(&cfg.MappingFile, "csv", "", "CSV mapping file (columns: source,destination)")
	rootCmd.Flags().StringVar(&cfg.MappingFile, "json", "", "JSON mapping file")
	rootCmd.Flags().StringSliceVar(&cfg.Include, "include", []string{}, "Include file patterns (glob, repeatable)")
	rootCmd.Flags().StringSliceVar(&cfg.Exclude, "exclude", []string{}, "Exclude file patterns (glob, repeatable)")
	rootCmd.Flags().StringSliceVar(&cfg.ExcludeDir, "exclude-dir", []string{}, "Exclude directories (repeatable)")
	rootCmd.Flags().StringVar(&extensionsStr, "extensions", "", "File extensions to process (.txt,.go, etc.)")
	rootCmd.Flags().BoolVar(&cfg.DryRun, "dry-run", false, "Simulate without making changes")
	rootCmd.Flags().BoolVar(&cfg.DryRun, "fake", false, "Simulate without making changes (alias for --dry-run)")
	rootCmd.Flags().BoolVarP(&cfg.Revert, "revert", "r", false, "Revert transformations from log file")
	rootCmd.Flags().BoolVar(&cfg.Apply, "apply", false, "Apply transformations from log file (ignores commented lines)")
	rootCmd.Flags().BoolVar(&cfg.Backup, "backup", false, "Create .bak files (deprecated, backups enabled by default)")
	rootCmd.Flags().BoolVar(&cfg.NoBackup, "nobackup", false, "Disable backup file creation")
	rootCmd.Flags().BoolVar(&cfg.CaseSensitive, "case-sensitive", false, "Case-sensitive search (default: insensitive)")
	rootCmd.Flags().BoolVarP(&cfg.Verbose, "verbose", "v", false, "Verbose mode")
	rootCmd.Flags().BoolVar(&cfg.Debug, "debug", false, "Debug mode")
	rootCmd.Flags().BoolVarP(&cfg.Quiet, "quiet", "q", false, "Quiet mode")
	rootCmd.Flags().StringVar(&cfg.LogFile, "log", "", "Log file (default: stdout)")
	rootCmd.Flags().Var((*logFormatFlag)(&cfg.LogFormat), "log-format", "Log format (json, csv)")

	rootCmd.MarkFlagsMutuallyExclusive("csv", "json")
	rootCmd.MarkFlagsMutuallyExclusive("verbose", "quiet")
	rootCmd.MarkFlagsMutuallyExclusive("debug", "quiet")
	rootCmd.MarkFlagsMutuallyExclusive("backup", "nobackup")
	rootCmd.MarkFlagsMutuallyExclusive("revert", "apply")
}

func runRemap(cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		cfg.Directory = args[0]
	}

	if cmd.Flag("csv").Changed {
		cfg.MappingType = "csv"
	} else if cmd.Flag("json").Changed {
		cfg.MappingType = "json"
	}

	if extensionsStr != "" {
		cfg.Extensions = strings.Split(extensionsStr, ",")
	}

	if err := cfg.Validate(); err != nil {
		return err
	}

	return executeRemap(cfg)
}

type logFormatFlag config.LogFormat

func (f *logFormatFlag) String() string {
	return string(*f)
}

func (f *logFormatFlag) Set(v string) error {
	switch v {
	case "json", "csv":
		*f = logFormatFlag(v)
		return nil
	default:
		return fmt.Errorf("must be 'json' or 'csv'")
	}
}

func (f *logFormatFlag) Type() string {
	return "string"
}
