// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package file

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dagucloud/dagu/v2/internal/cmn/config"
	"github.com/dagucloud/dagu/v2/internal/cmn/fileutil"
	"github.com/dagucloud/dagu/v2/internal/cmn/logger"
	"github.com/dagucloud/dagu/v2/internal/cmn/logger/tag"
	"github.com/dagucloud/dagu/v2/internal/ir"
	"github.com/dagucloud/dagu/v2/internal/persis"
	filedag "github.com/dagucloud/dagu/v2/internal/persis/file/dag"
	"github.com/dagucloud/dagu/v2/internal/workspace"
)

// DAGRepositoryOption configures the file-backed DAG repository.
type DAGRepositoryOption func(*DAGRepositoryOptions)

// DAGRepositoryOptions contains file-backed DAG repository settings.
type DAGRepositoryOptions struct {
	Cache                 *fileutil.Cache[*ir.DAG]
	SearchPaths           []string
	SkipExamples          *bool
	Symlinks              bool
	SkipDirectoryCreation bool
}

// WithDAGFileCache sets the cache used for loading DAG definitions.
func WithDAGFileCache(cache *fileutil.Cache[*ir.DAG]) DAGRepositoryOption {
	return func(o *DAGRepositoryOptions) {
		o.Cache = cache
	}
}

// WithDAGSearchPaths sets additional directories used to resolve DAG definitions.
func WithDAGSearchPaths(paths []string) DAGRepositoryOption {
	return func(o *DAGRepositoryOptions) {
		o.SearchPaths = append([]string{}, paths...)
	}
}

// WithDAGSkipExamples controls whether example DAG files are created.
func WithDAGSkipExamples(skip bool) DAGRepositoryOption {
	return func(o *DAGRepositoryOptions) {
		o.SkipExamples = &skip
	}
}

// WithDAGSymlinks includes file symlinks in recursive discovery and permits external targets.
func WithDAGSymlinks(enabled bool) DAGRepositoryOption {
	return func(o *DAGRepositoryOptions) {
		o.Symlinks = enabled
	}
}

// WithDAGSkipDirectoryCreation controls whether the DAG directory is created on startup.
func WithDAGSkipDirectoryCreation(skip bool) DAGRepositoryOption {
	return func(o *DAGRepositoryOptions) {
		o.SkipDirectoryCreation = skip
	}
}

// NewDAGRepository connects the file-backed definition store to the shared repository.
func NewDAGRepository(cfg *config.Config, opts ...DAGRepositoryOption) (*persis.DAGRepository, error) {
	options := DAGRepositoryOptions{Symlinks: cfg.DAGDiscovery.Symlinks}
	for _, opt := range opts {
		if opt != nil {
			opt(&options)
		}
	}

	skipExamples := cfg.Core.SkipExamples
	if options.SkipExamples != nil {
		skipExamples = *options.SkipExamples
	}
	if err := migrateSuspendFlags(cfg.Paths.SuspendFlagsDirLegacy, cfg.Paths.SuspendFlagsDir); err != nil {
		return nil, fmt.Errorf("migrate suspend flags: %w", err)
	}
	workspaceBaseConfigDir := workspace.BaseConfigDir(cfg.Paths.DAGsDir)
	dagStore := filedag.NewStore(
		cfg.Paths.DAGsDir,
		filedag.WithFlagsBaseDir(cfg.Paths.SuspendFlagsDir),
		filedag.WithSearchPaths(options.SearchPaths),
		filedag.WithBaseConfig(cfg.Paths.BaseConfig),
		filedag.WithWorkspaceBaseConfigDir(workspaceBaseConfigDir),
		filedag.WithFileCache(options.Cache),
		filedag.WithSkipExamples(skipExamples),
		filedag.WithRecursiveDiscovery(cfg.DAGDiscovery.Recursive),
		filedag.WithSymlinks(options.Symlinks),
		filedag.WithSkipDirectoryCreation(options.SkipDirectoryCreation),
	)
	if err := dagStore.Initialize(); err != nil {
		return nil, fmt.Errorf("initialize DAG definition store: %w", err)
	}
	return persis.NewDAGRepository(dagStore, persis.DAGRepositoryOptions{
		BaseConfigPath:         cfg.Paths.BaseConfig,
		WorkspaceBaseConfigDir: workspaceBaseConfigDir,
	}), nil
}

// suspendFlagFilePermission matches the permission used for suspend flag files.
const suspendFlagFilePermission os.FileMode = 0750

// migrateSuspendFlags moves suspend flag files written under the legacy default
// location into the current flags directory. It is a no-op when no legacy
// directory is recorded or when it does not exist.
func migrateSuspendFlags(legacyDir, flagsDir string) error {
	if legacyDir == "" || flagsDir == "" || legacyDir == flagsDir {
		return nil
	}
	entries, err := os.ReadDir(legacyDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("read legacy suspend flags directory %s: %w", legacyDir, err)
	}
	if err := os.MkdirAll(flagsDir, suspendFlagFilePermission); err != nil {
		return err
	}
	migrated := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		target := filepath.Join(flagsDir, entry.Name())
		if !fileutil.FileExists(target) {
			if err := fileutil.WriteFileAtomic(target, []byte{}, suspendFlagFilePermission); err != nil {
				return err
			}
		}
		if err := fileutil.Remove(filepath.Join(legacyDir, entry.Name())); err != nil {
			return err
		}
		migrated++
	}
	// Best effort: the legacy directory may contain entries the migration does
	// not own (for example subdirectories), in which case it is left in place.
	if err := fileutil.Remove(legacyDir); err != nil {
		logger.Warn(context.Background(), "Failed to remove legacy suspend flags directory",
			tag.Dir(legacyDir), tag.Error(err))
	}
	if migrated > 0 {
		logger.Info(context.Background(), "Migrated suspend flags to the data directory",
			tag.Dir(legacyDir), tag.Dir(flagsDir))
	}
	return nil
}
