// Package cmd implements the command-line interface and orchestration logic for remap.
// It coordinates between different components to execute file processing operations,
// providing the main business logic that connects configuration, discovery, processing, and reporting.
package cmd

import (
	"context"
	"time"

	"remap/internal/backup"
	"remap/internal/concurrent"
	"remap/internal/config"
	"remap/internal/errors"
	"remap/internal/filter"
	"remap/internal/log"
	"remap/internal/parser"
)

func executeRemap(cfg *config.Config) error {
	startTime := time.Now()

	if cfg.Revert {
		return executeRevert(cfg)
	}

	if cfg.Apply {
		return executeApply(cfg)
	}

	mappings, err := parser.LoadMappingTable(cfg.MappingFile, cfg.MappingType)
	if err != nil {
		return err
	}

	discovery := filter.NewFileDiscovery(cfg)
	files, err := discovery.Discover()
	if err != nil {
		return err
	}

	logger, err := log.NewLogger(cfg)
	if err != nil {
		return err
	}
	defer logger.Close()

	processor := concurrent.NewProcessor(cfg, mappings)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	results, err := processor.ProcessFiles(ctx, files)
	if err != nil {
		return err
	}

	for result := range results {
		logger.LogResult(result)
	}

	logger.SetProcessingTime(time.Since(startTime))
	return logger.WriteReport()
}

func executeRevert(cfg *config.Config) error {
	if cfg.LogFile == "" {
		return errors.NewConfigError("log file is required for revert operation", nil)
	}

	revertManager := backup.NewRevertManager()
	return revertManager.RevertFromLogWithFormat(cfg.LogFile, string(cfg.LogFormat))
}

func executeApply(cfg *config.Config) error {
	if cfg.LogFile == "" {
		return errors.NewConfigError("log file is required for apply operation", nil)
	}

	applyManager := backup.NewApplyManager()
	return applyManager.ApplyFromLogWithFormat(cfg.LogFile, string(cfg.LogFormat))
}
